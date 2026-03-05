// Command openapi-indexer downloads and processes the Microsoft Graph
// OpenAPI specification into a compact searchable JSON index and a
// SQLite FTS5 full-text search database.
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

	"github.com/merill/msgraph/internal/openapi"
	"gopkg.in/yaml.v3"
)

const (
	metadataBaseURL = "https://raw.githubusercontent.com/microsoftgraph/msgraph-metadata/master/openapi"
)

// jsonEndpoint is the compact representation stored in the JSON index.
// Kept for backward compatibility with the existing JSON format.
type jsonEndpoint struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	Resource    string   `json:"resource,omitempty"`
}

// jsonIndex is the top-level JSON output structure.
type jsonIndex struct {
	Version   string         `json:"version"`
	Generated string         `json:"generated"`
	Count     int            `json:"count"`
	Endpoints []jsonEndpoint `json:"endpoints"`
}

// httpMethods is the set of valid HTTP method keys we extract from paths.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true,
	"patch": true, "delete": true, "head": true, "options": true,
}

func main() {
	version := flag.String("version", "beta", "API version to index: 'v1.0' or 'beta'")
	output := flag.String("output", "skills/msgraph/references/graph-api-index.json", "Output file path")
	dbOutput := flag.String("db-output", "", "SQLite FTS database output path (defaults to same dir as JSON with .db extension)")
	input := flag.String("input", "", "Local OpenAPI YAML file (skips download if set)")
	flag.Parse()

	// Determine DB output path.
	if *dbOutput == "" {
		*dbOutput = strings.TrimSuffix(*output, filepath.Ext(*output)) + ".db"
	}

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

	// Extract full endpoints (for FTS DB) and compact endpoints (for JSON).
	fullEndpoints, compactEndpoints := extractEndpoints(pathsMap)
	fmt.Fprintf(os.Stderr, "Extracted %d endpoints\n", len(fullEndpoints))

	// --- Write JSON index (backward compatible, compact) ---
	idx := jsonIndex{
		Version:   *version,
		Generated: time.Now().UTC().Format(time.RFC3339),
		Count:     len(compactEndpoints),
		Endpoints: compactEndpoints,
	}

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
	fmt.Fprintf(os.Stderr, "Wrote JSON index to %s (%d bytes, %d endpoints)\n", *output, len(outData), len(compactEndpoints))

	// --- Write SQLite FTS database (full data) ---
	dbDir := filepath.Dir(*dbOutput)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		fatal("Failed to create DB output directory: %v", err)
	}
	// Remove existing DB to start fresh.
	os.Remove(*dbOutput)

	if err := openapi.BuildFTSDatabase(fullEndpoints, *dbOutput); err != nil {
		fatal("Failed to build FTS database: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Wrote FTS database to %s (%d endpoints)\n", *dbOutput, len(fullEndpoints))
}

// extractEndpoints returns both full endpoints (for FTS) and compact endpoints (for JSON).
func extractEndpoints(pathsMap map[string]interface{}) ([]openapi.FullEndpoint, []jsonEndpoint) {
	var fullEps []openapi.FullEndpoint
	var compactEps []jsonEndpoint

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

		// Extract path-level description if present.
		pathDesc := getString(pathItem, "description")

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

			summary := cleanWhitespace(getString(opRaw, "summary"))
			description := cleanWhitespace(getString(opRaw, "description"))
			resource := extractResource(path)

			// --- Compact endpoint (JSON, backward compatible) ---
			ce := jsonEndpoint{
				Path:     path,
				Method:   strings.ToUpper(method),
				Summary:  truncate(summary, 200),
				Resource: resource,
			}
			if description != "" && description != ce.Summary {
				ce.Description = truncate(description, 300)
			}
			ce.Scopes = extractScopes(opRaw)
			compactEps = append(compactEps, ce)

			// --- Full endpoint (FTS DB, no truncation) ---
			fe := openapi.FullEndpoint{
				Path:            path,
				Method:          strings.ToUpper(method),
				Summary:         summary,
				Description:     description,
				Resource:        resource,
				PathDescription: pathDesc,
				OperationID:     getString(opRaw, "operationId"),
				OperationType:   getString(opRaw, "x-ms-docs-operation-type"),
				Scopes:          extractScopes(opRaw),
				Tags:            getStringSlice(opRaw, "tags"),
			}

			// Deprecated flag
			if dep, ok := opRaw["deprecated"]; ok {
				if b, ok := dep.(bool); ok {
					fe.Deprecated = b
				}
			}

			// Deprecation info
			if depInfo, ok := opRaw["x-ms-deprecation"].(map[string]interface{}); ok {
				fe.DeprecationDate = getString(depInfo, "date")
				fe.DeprecationRemovalDate = getString(depInfo, "removalDate")
				fe.DeprecationDescription = getString(depInfo, "description")
			}

			// External docs URL
			if extDocs, ok := opRaw["externalDocs"].(map[string]interface{}); ok {
				fe.DocURL = getString(extDocs, "url")
			}

			// Pageable
			if _, ok := opRaw["x-ms-pageable"]; ok {
				fe.Pageable = true
			}

			// Parameters (extract names, types, locations)
			fe.Parameters = extractParameters(opRaw)

			// Request body schema reference
			if reqBody, ok := opRaw["requestBody"].(map[string]interface{}); ok {
				fe.RequestBodyRef = extractSchemaRef(reqBody)
				fe.RequestBodyDesc = getString(reqBody, "description")
			}

			// Response schema reference (from 2XX or 200 or 201)
			fe.ResponseRef = extractResponseRef(opRaw)

			fullEps = append(fullEps, fe)
		}
	}

	return fullEps, compactEps
}

// Parameter represents an extracted API parameter.
type Parameter struct {
	Name string `json:"name"`
	In   string `json:"in"` // query, header, path
}

func extractParameters(opRaw map[string]interface{}) []openapi.Parameter {
	paramsRaw, ok := opRaw["parameters"]
	if !ok {
		return nil
	}
	paramsList, ok := paramsRaw.([]interface{})
	if !ok {
		return nil
	}

	var params []openapi.Parameter
	for _, pRaw := range paramsList {
		p, ok := pRaw.(map[string]interface{})
		if !ok {
			continue
		}
		name := getString(p, "name")
		in := getString(p, "in")
		if name != "" && in != "" {
			params = append(params, openapi.Parameter{Name: name, In: in})
		}
	}
	return params
}

func extractSchemaRef(reqBody map[string]interface{}) string {
	content, ok := reqBody["content"].(map[string]interface{})
	if !ok {
		return ""
	}
	for _, mediaTypeRaw := range content {
		mediaType, ok := mediaTypeRaw.(map[string]interface{})
		if !ok {
			continue
		}
		schema, ok := mediaType["schema"].(map[string]interface{})
		if !ok {
			continue
		}
		if ref := getString(schema, "$ref"); ref != "" {
			// Extract type name from $ref like "#/components/schemas/microsoft.graph.user"
			parts := strings.Split(ref, "/")
			return parts[len(parts)-1]
		}
	}
	return ""
}

func extractResponseRef(opRaw map[string]interface{}) string {
	respRaw, ok := opRaw["responses"].(map[string]interface{})
	if !ok {
		return ""
	}
	// Check common success status codes in order of preference.
	for _, code := range []string{"2XX", "200", "201"} {
		resp, ok := respRaw[code].(map[string]interface{})
		if !ok {
			continue
		}
		content, ok := resp["content"].(map[string]interface{})
		if !ok {
			continue
		}
		for _, mediaTypeRaw := range content {
			mediaType, ok := mediaTypeRaw.(map[string]interface{})
			if !ok {
				continue
			}
			schema, ok := mediaType["schema"].(map[string]interface{})
			if !ok {
				continue
			}
			if ref := getString(schema, "$ref"); ref != "" {
				parts := strings.Split(ref, "/")
				return parts[len(parts)-1]
			}
		}
	}
	return ""
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

func getStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	list, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
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

func cleanWhitespace(s string) string {
	s = strings.TrimSpace(s)
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, maxLen int) string {
	s = cleanWhitespace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
