// Command openapi-indexer downloads and processes the Microsoft Graph
// OpenAPI specification into a compact searchable JSON index.
//
// Usage:
//
//	go run ./tools/openapi-indexer -version beta -output skills/msgraph/references/graph-api-index.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	metadataBaseURL = "https://raw.githubusercontent.com/microsoftgraph/msgraph-metadata/master/openapi"
)

// Endpoint is the compact representation stored in the index.
type Endpoint struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	Resource    string   `json:"resource,omitempty"`
}

// Index is the top-level output structure.
type Index struct {
	Version   string     `json:"version"`
	Generated string     `json:"generated"`
	Count     int        `json:"count"`
	Endpoints []Endpoint `json:"endpoints"`
}

// httpMethods is the set of valid HTTP method keys we extract from paths.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true,
	"patch": true, "delete": true, "head": true, "options": true,
}

func main() {
	version := flag.String("version", "beta", "API version to index: 'v1.0' or 'beta'")
	output := flag.String("output", "skills/msgraph/references/graph-api-index.json", "Output file path")
	input := flag.String("input", "", "Local OpenAPI YAML file (skips download if set)")
	flag.Parse()

	var data []byte
	var err error

	if *input != "" {
		fmt.Fprintf(os.Stderr, "Reading local file: %s\n", *input)
		data, err = os.ReadFile(*input)
		if err != nil {
			fatal("Failed to read input file: %v", err)
		}
	} else {
		url := fmt.Sprintf("%s/%s/openapi.yaml", metadataBaseURL, *version)
		fmt.Fprintf(os.Stderr, "Downloading OpenAPI spec from: %s\n", url)
		data, err = download(url)
		if err != nil {
			fatal("Failed to download: %v", err)
		}
		fmt.Fprintf(os.Stderr, "Downloaded %d bytes\n", len(data))
	}

	fmt.Fprintln(os.Stderr, "Parsing YAML (this may take a moment for large specs)...")

	// Use interface{} to handle the heterogeneous path item structure
	// (paths contain both HTTP methods and non-method keys like "parameters", "description")
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		fatal("Failed to parse YAML: %v", err)
	}

	pathsRaw, ok := raw["paths"]
	if !ok {
		fatal("No 'paths' key found in OpenAPI spec")
	}

	pathsMap, ok := pathsRaw.(map[string]interface{})
	if !ok {
		fatal("'paths' is not a map")
	}
	fmt.Fprintf(os.Stderr, "Found %d paths\n", len(pathsMap))

	endpoints := extractEndpoints(pathsMap)
	fmt.Fprintf(os.Stderr, "Extracted %d endpoints\n", len(endpoints))

	idx := Index{
		Version:   *version,
		Generated: time.Now().UTC().Format(time.RFC3339),
		Count:     len(endpoints),
		Endpoints: endpoints,
	}

	// Ensure output directory exists
	outDir := filepath.Dir(*output)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fatal("Failed to create output directory: %v", err)
	}

	outData, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		fatal("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(*output, outData, 0644); err != nil {
		fatal("Failed to write output: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Wrote index to %s (%d bytes, %d endpoints)\n", *output, len(outData), len(endpoints))
}

func extractEndpoints(pathsMap map[string]interface{}) []Endpoint {
	var endpoints []Endpoint

	// Sort paths for deterministic output
	paths := make([]string, 0, len(pathsMap))
	for p := range pathsMap {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, path := range paths {
		pathItem, ok := pathsMap[path].(map[string]interface{})
		if !ok {
			continue
		}

		// Sort methods for deterministic output
		methodNames := make([]string, 0)
		for key := range pathItem {
			if httpMethods[strings.ToLower(key)] {
				methodNames = append(methodNames, key)
			}
		}
		sort.Strings(methodNames)

		for _, method := range methodNames {
			opRaw, ok := pathItem[method].(map[string]interface{})
			if !ok {
				continue
			}

			ep := Endpoint{
				Path:     path,
				Method:   strings.ToUpper(method),
				Summary:  truncate(getString(opRaw, "summary"), 200),
				Resource: extractResource(path),
			}

			// Only include description if it adds info beyond the summary
			desc := getString(opRaw, "description")
			if desc != "" && desc != ep.Summary {
				ep.Description = truncate(desc, 300)
			}

			// Extract scopes from security requirements
			ep.Scopes = extractScopes(opRaw)

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints
}

// extractResource pulls the primary resource name from a path.
// e.g. "/users/{user-id}/messages/{message-id}" -> "users"
func extractResource(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	for _, p := range parts {
		if !strings.HasPrefix(p, "{") && p != "" {
			return strings.ToLower(p)
		}
	}
	return ""
}

// extractScopes pulls permission scope names from the security field.
func extractScopes(opRaw map[string]interface{}) []string {
	secRaw, ok := opRaw["security"]
	if !ok {
		return nil
	}

	secList, ok := secRaw.([]interface{})
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var scopes []string

	for _, entry := range secList {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		for _, scopeListRaw := range entryMap {
			scopeList, ok := scopeListRaw.([]interface{})
			if !ok {
				continue
			}
			for _, scopeRaw := range scopeList {
				scope, ok := scopeRaw.(string)
				if !ok {
					continue
				}
				if !seen[scope] {
					seen[scope] = true
					scopes = append(scopes, scope)
				}
			}
		}
	}

	return scopes
}

func getString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
