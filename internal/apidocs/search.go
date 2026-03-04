// Package apidocs provides search functionality over a pre-processed
// index of Microsoft Graph API documentation, including per-endpoint
// permissions, query parameters, and per-resource property details.
package apidocs

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Index holds the full searchable API docs index.
type Index struct {
	Version       string        `json:"version"`
	Generated     string        `json:"generated"`
	EndpointCount int           `json:"endpointCount"`
	ResourceCount int           `json:"resourceCount"`
	Endpoints     []EndpointDoc `json:"endpoints"`
	Resources     []ResourceDoc `json:"resources"`
}

// EndpointDoc represents a single API endpoint with documentation details.
type EndpointDoc struct {
	Path              string      `json:"path"`
	Method            string      `json:"method"`
	Permissions       Permissions `json:"permissions"`
	QueryParams       []string    `json:"queryParams,omitempty"`
	RequiredHeaders   []string    `json:"requiredHeaders,omitempty"`
	DefaultProperties []string    `json:"defaultProperties,omitempty"`
	Notes             []string    `json:"notes,omitempty"`
}

// Permissions holds the required permission scopes split by auth type.
type Permissions struct {
	DelegatedWork     []string `json:"delegatedWork,omitempty"`
	DelegatedPersonal []string `json:"delegatedPersonal,omitempty"`
	Application       []string `json:"application,omitempty"`
}

// ResourceDoc represents a resource type with its properties.
type ResourceDoc struct {
	Name       string        `json:"name"`
	Properties []PropertyDoc `json:"properties,omitempty"`
}

// PropertyDoc describes a single property of a resource.
type PropertyDoc struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Filter  []string `json:"filter,omitempty"`
	Default bool     `json:"default,omitempty"`
	Notes   string   `json:"notes,omitempty"`
}

// LoadIndex reads and parses the pre-built API docs index JSON file.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read API docs index %s: %w", path, err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse API docs index: %w", err)
	}

	return &idx, nil
}

// EndpointResult wraps an endpoint doc with a match reason.
type EndpointResult struct {
	EndpointDoc
	MatchReason string `json:"matchReason"`
}

// ResourceResult wraps a resource doc with a match reason.
type ResourceResult struct {
	ResourceDoc
	MatchReason string `json:"matchReason"`
}

// SearchEndpoints finds endpoint docs matching the given criteria.
func (idx *Index) SearchEndpoints(endpoint, method, query string, limit int) []EndpointResult {
	if limit <= 0 {
		limit = 10
	}

	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	method = strings.ToUpper(strings.TrimSpace(method))
	query = strings.ToLower(strings.TrimSpace(query))

	var results []EndpointResult

	for _, ep := range idx.Endpoints {
		if len(results) >= limit {
			break
		}

		// Filter by method
		if method != "" && strings.ToUpper(ep.Method) != method {
			continue
		}

		// Filter by endpoint path — require segment-level match, not substring
		if endpoint != "" {
			pathLower := strings.ToLower(ep.Path)
			if !matchesEndpointPath(pathLower, endpoint) {
				continue
			}
		}

		// Filter by keyword query
		if query != "" {
			reason := matchesEndpointQuery(ep, query)
			if reason == "" {
				continue
			}
			results = append(results, EndpointResult{
				EndpointDoc: ep,
				MatchReason: reason,
			})
		} else {
			reason := "matched filters"
			if endpoint != "" {
				reason = "endpoint match"
			}
			results = append(results, EndpointResult{
				EndpointDoc: ep,
				MatchReason: reason,
			})
		}
	}

	return results
}

// SearchResources finds resource docs matching the given criteria.
// Results are sorted: exact name match first, then prefix matches, then substring matches.
func (idx *Index) SearchResources(resource, query string, limit int) []ResourceResult {
	if limit <= 0 {
		limit = 10
	}

	resource = strings.ToLower(strings.TrimSpace(resource))
	query = strings.ToLower(strings.TrimSpace(query))

	var results []ResourceResult

	for _, res := range idx.Resources {
		// Filter by resource name
		if resource != "" {
			nameLower := strings.ToLower(res.Name)
			if nameLower != resource && !strings.Contains(nameLower, resource) {
				continue
			}
		}

		// Filter by keyword query
		if query != "" {
			reason := matchesResourceQuery(res, query)
			if reason == "" {
				continue
			}
			results = append(results, ResourceResult{
				ResourceDoc: res,
				MatchReason: reason,
			})
		} else {
			reason := "matched filters"
			if resource != "" {
				reason = "resource match"
			}
			results = append(results, ResourceResult{
				ResourceDoc: res,
				MatchReason: reason,
			})
		}
	}

	// Sort by match quality: exact name > prefix > substring
	if resource != "" {
		sortResourceResults(results, resource)
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// resourceMatchRank returns 0 for exact match, 1 for prefix, 2 for substring.
func resourceMatchRank(name, query string) int {
	nameLower := strings.ToLower(name)
	if nameLower == query {
		return 0
	}
	if strings.HasPrefix(nameLower, query) {
		return 1
	}
	return 2
}

// sortResourceResults sorts results by match quality: exact > prefix > substring.
func sortResourceResults(results []ResourceResult, query string) {
	sort.SliceStable(results, func(i, j int) bool {
		return resourceMatchRank(results[i].Name, query) < resourceMatchRank(results[j].Name, query)
	})
}

// matchesEndpointPath checks if a path matches an endpoint query using segment-level matching.
// "/users" matches "/users", "/users/{id}", "/users/{id}/messages" but NOT "/foo/userSources".
func matchesEndpointPath(path, endpoint string) bool {
	// Exact match
	if path == endpoint {
		return true
	}
	// Prefix match: path starts with endpoint followed by / or {
	if strings.HasPrefix(path, endpoint+"/") || strings.HasPrefix(path, endpoint+"{") {
		return true
	}
	// Segment match: endpoint appears as a full segment within the path
	// e.g. searching "/messages" should match "/me/messages" and "/me/messages/{id}"
	segIdx := strings.Index(path, endpoint)
	if segIdx < 0 {
		return false
	}
	// Check that the match is at a segment boundary (preceded by nothing or /)
	if segIdx > 0 && path[segIdx-1] != '/' {
		return false
	}
	// Check that the match ends at a segment boundary (followed by nothing, /, or {)
	endIdx := segIdx + len(endpoint)
	if endIdx < len(path) && path[endIdx] != '/' && path[endIdx] != '{' {
		return false
	}
	return true
}

// matchesEndpointQuery checks if an endpoint matches a free-text query.
func matchesEndpointQuery(ep EndpointDoc, query string) string {
	words := strings.Fields(query)

	pathLower := strings.ToLower(ep.Path)

	// Build searchable text from all fields
	var searchParts []string
	searchParts = append(searchParts, pathLower)
	searchParts = append(searchParts, strings.ToLower(ep.Method))
	for _, p := range ep.Permissions.DelegatedWork {
		searchParts = append(searchParts, strings.ToLower(p))
	}
	for _, p := range ep.Permissions.Application {
		searchParts = append(searchParts, strings.ToLower(p))
	}
	for _, p := range ep.QueryParams {
		searchParts = append(searchParts, strings.ToLower(p))
	}
	for _, h := range ep.RequiredHeaders {
		searchParts = append(searchParts, strings.ToLower(h))
	}
	for _, n := range ep.Notes {
		searchParts = append(searchParts, strings.ToLower(n))
	}
	searchText := strings.Join(searchParts, " ")

	// All words must match somewhere
	for _, word := range words {
		if !strings.Contains(searchText, word) {
			return ""
		}
	}

	// Determine match reason
	if strings.Contains(pathLower, query) {
		return "path match"
	}
	return "keyword match"
}

// matchesResourceQuery checks if a resource matches a free-text query.
func matchesResourceQuery(res ResourceDoc, query string) string {
	words := strings.Fields(query)

	nameLower := strings.ToLower(res.Name)

	// Build searchable text from all fields
	var searchParts []string
	searchParts = append(searchParts, nameLower)
	for _, p := range res.Properties {
		searchParts = append(searchParts, strings.ToLower(p.Name))
		searchParts = append(searchParts, strings.ToLower(p.Type))
		for _, f := range p.Filter {
			searchParts = append(searchParts, strings.ToLower(f))
		}
		if p.Notes != "" {
			searchParts = append(searchParts, strings.ToLower(p.Notes))
		}
	}
	searchText := strings.Join(searchParts, " ")

	// All words must match somewhere
	for _, word := range words {
		if !strings.Contains(searchText, word) {
			return ""
		}
	}

	// Determine match reason
	if strings.Contains(nameLower, query) {
		return "resource name match"
	}
	return "keyword match"
}
