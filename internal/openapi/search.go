// Package openapi provides search functionality over a pre-processed
// Microsoft Graph OpenAPI index.
package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Endpoint represents a single API endpoint in the index.
type Endpoint struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Summary     string   `json:"summary"`
	Description string   `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	Resource    string   `json:"resource,omitempty"` // e.g. "users", "groups", "messages"
}

// Index holds the full searchable OpenAPI index.
type Index struct {
	Endpoints []Endpoint `json:"endpoints"`
}

// LoadIndex reads and parses the pre-processed OpenAPI index JSON file.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %s: %w", path, err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse index file: %w", err)
	}

	return &idx, nil
}

// SearchResult wraps an endpoint with a relevance indicator.
type SearchResult struct {
	Endpoint
	MatchReason string `json:"matchReason"`
}

// Search finds endpoints matching the given criteria.
// All criteria are ANDed together. Empty criteria are ignored.
func (idx *Index) Search(query, resource, method string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 20
	}

	query = strings.ToLower(query)
	resource = strings.ToLower(resource)
	method = strings.ToUpper(method)

	var results []SearchResult

	for _, ep := range idx.Endpoints {
		if len(results) >= limit {
			break
		}

		// Filter by method
		if method != "" && strings.ToUpper(ep.Method) != method {
			continue
		}

		// Filter by resource
		if resource != "" && !matchesResource(ep, resource) {
			continue
		}

		// Filter by query (search path, summary, description)
		if query != "" {
			reason := matchesQuery(ep, query)
			if reason == "" {
				continue
			}
			results = append(results, SearchResult{
				Endpoint:    ep,
				MatchReason: reason,
			})
		} else {
			// No query filter, just method/resource matched
			results = append(results, SearchResult{
				Endpoint:    ep,
				MatchReason: "matched filters",
			})
		}
	}

	return results
}

// matchesResource checks if an endpoint matches a resource name.
func matchesResource(ep Endpoint, resource string) bool {
	if strings.ToLower(ep.Resource) == resource {
		return true
	}
	// Also check if the resource appears in the path
	pathLower := strings.ToLower(ep.Path)
	return strings.Contains(pathLower, "/"+resource)
}

// matchesQuery checks if an endpoint matches a free-text query.
// Returns the reason for the match, or empty string if no match.
func matchesQuery(ep Endpoint, query string) string {
	// Split query into words for flexible matching
	words := strings.Fields(query)

	pathLower := strings.ToLower(ep.Path)
	summaryLower := strings.ToLower(ep.Summary)
	descLower := strings.ToLower(ep.Description)

	// All words must match somewhere
	for _, word := range words {
		found := false
		if strings.Contains(pathLower, word) {
			found = true
		} else if strings.Contains(summaryLower, word) {
			found = true
		} else if strings.Contains(descLower, word) {
			found = true
		} else {
			// Check scopes
			for _, scope := range ep.Scopes {
				if strings.Contains(strings.ToLower(scope), word) {
					found = true
					break
				}
			}
		}
		if !found {
			return ""
		}
	}

	// Determine primary match location for reporting
	if strings.Contains(pathLower, query) {
		return "path match"
	}
	if strings.Contains(summaryLower, query) {
		return "summary match"
	}
	if strings.Contains(descLower, query) {
		return "description match"
	}
	return "keyword match"
}
