// Package samples provides search functionality over community-contributed
// Microsoft Graph API query samples.
package samples

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Sample represents a single community-contributed query sample.
type Sample struct {
	Intent  string      `json:"intent"  yaml:"intent"`
	Query   interface{} `json:"query"   yaml:"query"` // string or []string for multi-step
	Product string      `json:"product" yaml:"-"`     // populated from directory name
	File    string      `json:"file"    yaml:"-"`     // relative path within samples/
}

// QueryStrings returns the query as a string slice, normalizing
// both single-string and multi-step list formats.
func (s *Sample) QueryStrings() []string {
	switch v := s.Query.(type) {
	case string:
		return []string{v}
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

// Index holds the full searchable samples index.
type Index struct {
	Generated string   `json:"generated"`
	Count     int      `json:"count"`
	Samples   []Sample `json:"samples"`
}

// LoadIndex reads and parses the pre-built samples index JSON file.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read samples index %s: %w", path, err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse samples index: %w", err)
	}

	return &idx, nil
}

// SearchResult wraps a sample with a relevance indicator.
type SearchResult struct {
	Sample
	MatchReason string `json:"matchReason"`
}

// Search finds samples whose intent matches the given query string.
// It performs case-insensitive keyword matching — all query words must
// appear somewhere in the intent or query fields.
func (idx *Index) Search(query string, product string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 10
	}

	query = strings.ToLower(strings.TrimSpace(query))
	product = strings.ToLower(strings.TrimSpace(product))

	var results []SearchResult

	for _, s := range idx.Samples {
		if len(results) >= limit {
			break
		}

		// Filter by product
		if product != "" && strings.ToLower(s.Product) != product {
			continue
		}

		// If no query, return all (filtered by product)
		if query == "" {
			results = append(results, SearchResult{
				Sample:      s,
				MatchReason: "matched product filter",
			})
			continue
		}

		reason := matchesSample(s, query)
		if reason != "" {
			results = append(results, SearchResult{
				Sample:      s,
				MatchReason: reason,
			})
		}
	}

	return results
}

// matchesSample checks if a sample matches a free-text query.
// Returns the reason for the match, or empty string if no match.
func matchesSample(s Sample, query string) string {
	words := strings.Fields(query)
	intentLower := strings.ToLower(s.Intent)

	// Build a combined query string for searching
	queryTexts := s.QueryStrings()
	combinedQuery := strings.ToLower(strings.Join(queryTexts, " "))

	// All words must match somewhere in intent or query
	for _, word := range words {
		if !strings.Contains(intentLower, word) && !strings.Contains(combinedQuery, word) {
			return ""
		}
	}

	// Determine primary match location
	if strings.Contains(intentLower, query) {
		return "intent match"
	}
	if strings.Contains(combinedQuery, query) {
		return "query match"
	}
	return "keyword match"
}
