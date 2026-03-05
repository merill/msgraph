// Command api-docs-indexer downloads and processes the Microsoft Graph
// API documentation into a compact searchable JSON index containing
// per-endpoint permissions, query parameters, and per-resource property details.
//
// Usage:
//
//	go run ./tools/api-docs-indexer -version beta -output skills/msgraph/references/api-docs-index.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/merill/msgraph/internal/apidocs"
)

const docsRepo = "https://github.com/microsoftgraph/microsoft-graph-docs-contrib.git"

// ApiDocsIndex is the top-level output structure.
type ApiDocsIndex struct {
	Version       string        `json:"version"`
	Generated     string        `json:"generated"`
	EndpointCount int           `json:"endpointCount"`
	ResourceCount int           `json:"resourceCount"`
	Endpoints     []EndpointDoc `json:"endpoints"`
	Resources     []ResourceDoc `json:"resources"`
}

func main() {
	version := flag.String("version", "beta", "API version to index: 'v1.0' or 'beta'")
	output := flag.String("output", "skills/msgraph/references/api-docs-index.json", "Output file path")
	dbOutput := flag.String("db-output", "", "Output path for SQLite FTS database (defaults to .db alongside JSON)")
	input := flag.String("input", "", "Local docs repo directory (skips clone if set)")
	flag.Parse()

	var docsDir string
	if *input != "" {
		fmt.Fprintf(os.Stderr, "Using local docs directory: %s\n", *input)
		docsDir = *input
	} else {
		var err error
		docsDir, err = cloneDocs(*version)
		if err != nil {
			fatal("Failed to clone docs repo: %v", err)
		}
		defer os.RemoveAll(docsDir)
	}

	apiDir := filepath.Join(docsDir, "api-reference", *version, "api")
	resourceDir := filepath.Join(docsDir, "api-reference", *version, "resources")

	// Verify directories exist
	if _, err := os.Stat(apiDir); os.IsNotExist(err) {
		fatal("API docs directory not found: %s", apiDir)
	}
	if _, err := os.Stat(resourceDir); os.IsNotExist(err) {
		fatal("Resource docs directory not found: %s", resourceDir)
	}

	// Parse API operation docs
	fmt.Fprintln(os.Stderr, "Parsing API operation docs...")
	endpoints := parseAllOperations(apiDir, docsDir)
	fmt.Fprintf(os.Stderr, "Parsed %d endpoints\n", len(endpoints))

	// Parse resource type docs
	fmt.Fprintln(os.Stderr, "Parsing resource type docs...")
	resources := parseAllResources(resourceDir)
	fmt.Fprintf(os.Stderr, "Parsed %d resources\n", len(resources))

	idx := ApiDocsIndex{
		Version:       *version,
		Generated:     time.Now().UTC().Format(time.RFC3339),
		EndpointCount: len(endpoints),
		ResourceCount: len(resources),
		Endpoints:     endpoints,
		Resources:     resources,
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
	// Append newline
	outData = append(outData, '\n')

	if err := os.WriteFile(*output, outData, 0644); err != nil {
		fatal("Failed to write output: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Wrote index to %s (%d bytes, %d endpoints, %d resources)\n",
		*output, len(outData), len(endpoints), len(resources))

	// Build FTS SQLite database.
	dbPath := *dbOutput
	if dbPath == "" {
		dbPath = strings.TrimSuffix(*output, ".json") + ".db"
	}

	// Convert local types to apidocs package types for the FTS builder.
	ftsIdx := toApiDocsIndex(idx)
	if err := apidocs.BuildFTSDatabase(ftsIdx, dbPath); err != nil {
		fatal("Failed to build FTS database: %v", err)
	}

	fmt.Fprintf(os.Stderr, "Wrote FTS database to %s (%d endpoints, %d resources)\n",
		dbPath, len(endpoints), len(resources))
}

// cloneDocs performs a shallow sparse checkout of the docs repo, fetching only
// the directories needed for the given API version.
func cloneDocs(version string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "msgraph-docs-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Cloning docs repo (sparse checkout)...\n")

	// Shallow clone with blob filter and sparse checkout
	cloneCmd := exec.Command("git", "clone",
		"--depth", "1",
		"--filter=blob:none",
		"--sparse",
		docsRepo,
		tmpDir,
	)
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	// Set sparse-checkout to only the directories we need
	sparseCmd := exec.Command("git", "sparse-checkout", "set",
		fmt.Sprintf("api-reference/%s/api", version),
		fmt.Sprintf("api-reference/%s/resources", version),
		fmt.Sprintf("api-reference/%s/includes/permissions", version),
	)
	sparseCmd.Dir = tmpDir
	sparseCmd.Stderr = os.Stderr
	if err := sparseCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git sparse-checkout failed: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Docs repo cloned successfully")
	return tmpDir, nil
}

// parseAllOperations walks the API docs directory and parses all operation files.
func parseAllOperations(apiDir, baseDir string) []EndpointDoc {
	entries, err := os.ReadDir(apiDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read API dir: %v\n", err)
		return nil
	}

	var endpoints []EndpointDoc
	skipped := 0
	errors := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(apiDir, entry.Name())
		doc, err := parseOperationDoc(filePath, baseDir)
		if err != nil {
			errors++
			continue
		}
		endpoints = append(endpoints, *doc)
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "  Skipped %d files due to parse errors\n", errors)
	}
	_ = skipped

	// Sort by path then method for deterministic output
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path == endpoints[j].Path {
			return endpoints[i].Method < endpoints[j].Method
		}
		return endpoints[i].Path < endpoints[j].Path
	})

	// Merge duplicates: if same path+method appears multiple times (e.g. Intune
	// variants alongside the main doc), merge their permissions, params, and notes.
	merged := make(map[string]*EndpointDoc)
	var order []string
	for i := range endpoints {
		ep := &endpoints[i]
		key := ep.Method + " " + ep.Path
		if existing, ok := merged[key]; ok {
			mergeEndpoint(existing, ep)
		} else {
			merged[key] = ep
			order = append(order, key)
		}
	}

	var deduped []EndpointDoc
	for _, key := range order {
		deduped = append(deduped, *merged[key])
	}

	return deduped
}

// parseAllResources walks the resource docs directory and parses all resource files.
func parseAllResources(resourceDir string) []ResourceDoc {
	entries, err := os.ReadDir(resourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read resource dir: %v\n", err)
		return nil
	}

	var resources []ResourceDoc
	errors := 0

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(resourceDir, entry.Name())
		doc, err := parseResourceDoc(filePath)
		if err != nil {
			errors++
			continue
		}
		// Skip resources with no properties (e.g., enum-only files)
		if len(doc.Properties) == 0 {
			continue
		}
		resources = append(resources, *doc)
	}

	if errors > 0 {
		fmt.Fprintf(os.Stderr, "  Skipped %d files due to parse errors\n", errors)
	}

	// Sort by name for deterministic output
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Name < resources[j].Name
	})

	// Deduplicate by name — keep the entry with the most properties
	// (the main resource file is typically richer than Intune/other variants)
	best := make(map[string]*ResourceDoc)
	var order []string
	for i := range resources {
		r := &resources[i]
		if existing, ok := best[r.Name]; ok {
			if len(r.Properties) > len(existing.Properties) {
				best[r.Name] = r
			}
		} else {
			best[r.Name] = r
			order = append(order, r.Name)
		}
	}

	var deduped []ResourceDoc
	for _, name := range order {
		deduped = append(deduped, *best[name])
	}

	return deduped
}

// mergeEndpoint merges fields from src into dst. This handles the case where
// the same method+path appears in multiple markdown files (e.g. the main
// user-list.md plus Intune-specific intune-shared-user-list.md). We union
// all permission scopes, query params, required headers, and notes — keeping
// the first non-empty DefaultProperties.
func mergeEndpoint(dst, src *EndpointDoc) {
	dst.Permissions.DelegatedWork = unionStrings(dst.Permissions.DelegatedWork, src.Permissions.DelegatedWork)
	dst.Permissions.DelegatedPersonal = unionStrings(dst.Permissions.DelegatedPersonal, src.Permissions.DelegatedPersonal)
	dst.Permissions.Application = unionStrings(dst.Permissions.Application, src.Permissions.Application)
	dst.QueryParams = unionStrings(dst.QueryParams, src.QueryParams)
	dst.RequiredHeaders = unionStrings(dst.RequiredHeaders, src.RequiredHeaders)
	dst.Notes = unionStrings(dst.Notes, src.Notes)

	// Keep first non-empty default properties list
	if len(dst.DefaultProperties) == 0 && len(src.DefaultProperties) > 0 {
		dst.DefaultProperties = src.DefaultProperties
	}
}

// unionStrings returns the deduplicated union of two string slices, preserving
// the order of first appearance.
func unionStrings(a, b []string) []string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	merged := append([]string(nil), a...)
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			merged = append(merged, s)
		}
	}
	return merged
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// toApiDocsIndex converts the local ApiDocsIndex (with local types) into an
// apidocs.Index (with the internal package types) for use by the FTS builder.
func toApiDocsIndex(local ApiDocsIndex) *apidocs.Index {
	idx := &apidocs.Index{
		Version:       local.Version,
		Generated:     local.Generated,
		EndpointCount: local.EndpointCount,
		ResourceCount: local.ResourceCount,
	}

	for _, ep := range local.Endpoints {
		idx.Endpoints = append(idx.Endpoints, apidocs.EndpointDoc{
			Path:   ep.Path,
			Method: ep.Method,
			Permissions: apidocs.Permissions{
				DelegatedWork:     ep.Permissions.DelegatedWork,
				DelegatedPersonal: ep.Permissions.DelegatedPersonal,
				Application:       ep.Permissions.Application,
			},
			QueryParams:       ep.QueryParams,
			RequiredHeaders:   ep.RequiredHeaders,
			DefaultProperties: ep.DefaultProperties,
			Notes:             ep.Notes,
		})
	}

	for _, res := range local.Resources {
		r := apidocs.ResourceDoc{
			Name: res.Name,
		}
		for _, p := range res.Properties {
			r.Properties = append(r.Properties, apidocs.PropertyDoc{
				Name:    p.Name,
				Type:    p.Type,
				Filter:  p.Filter,
				Default: p.Default,
				Notes:   p.Notes,
			})
		}
		idx.Resources = append(idx.Resources, r)
	}

	return idx
}
