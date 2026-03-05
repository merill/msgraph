package apidocs

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/merill/msgraph/internal/ftsutil"
)

// FTSIndex provides full-text search over a SQLite FTS5 database
// containing API documentation data (both endpoints and resources).
type FTSIndex struct {
	db *sql.DB
}

// LoadFTSIndex opens a SQLite FTS database for searching.
// Callers must call Close() when done.
func LoadFTSIndex(path string) (*FTSIndex, error) {
	db, err := ftsutil.OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	return &FTSIndex{db: db}, nil
}

// Close releases the database connection.
func (idx *FTSIndex) Close() error {
	if idx.db != nil {
		return idx.db.Close()
	}
	return nil
}

// FTSEndpointResult contains an endpoint search result.
type FTSEndpointResult struct {
	Path              string   `json:"path"`
	Method            string   `json:"method"`
	DelegatedWork     []string `json:"delegatedWork,omitempty"`
	DelegatedPersonal []string `json:"delegatedPersonal,omitempty"`
	Application       []string `json:"application,omitempty"`
	QueryParams       []string `json:"queryParams,omitempty"`
	RequiredHeaders   []string `json:"requiredHeaders,omitempty"`
	DefaultProperties []string `json:"defaultProperties,omitempty"`
	Notes             []string `json:"notes,omitempty"`
	MatchReason       string   `json:"matchReason"`
	Score             float64  `json:"score,omitempty"`
}

// FTSResourceResult contains a resource search result.
type FTSResourceResult struct {
	Name        string  `json:"name"`
	Properties  string  `json:"properties,omitempty"`
	MatchReason string  `json:"matchReason"`
	Score       float64 `json:"score,omitempty"`
}

// SearchEndpoints finds endpoint docs matching the given criteria using FTS5.
func (idx *FTSIndex) SearchEndpoints(endpoint, method, query string, limit int) []FTSEndpointResult {
	if limit <= 0 {
		limit = 10
	}

	ftsQuery := ftsutil.BuildFTSQuery(query)
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	method = strings.ToUpper(strings.TrimSpace(method))

	if ftsQuery != "" {
		return idx.searchEndpointsFTS(ftsQuery, endpoint, method, limit)
	}

	return idx.searchEndpointsFilters(endpoint, method, limit)
}

func (idx *FTSIndex) searchEndpointsFTS(ftsQuery, endpoint, method string, limit int) []FTSEndpointResult {
	// BM25 column weights: path(10), notes(3), query_params(2),
	// required_headers(2), delegated_work(1), application(1), default_properties(1)
	q := `
		SELECT e.path, e.method, e.delegated_work, e.delegated_personal,
			   e.application, e.query_params, e.required_headers,
			   e.default_properties, e.notes,
			   bm25(endpoints_fts, 10, 3, 2, 2, 1, 1, 1) AS rank
		FROM endpoints_fts f
		JOIN endpoints e ON f.rowid = e.id
		WHERE endpoints_fts MATCH ?`

	args := []interface{}{ftsQuery}

	if method != "" {
		q += ` AND e.method = ?`
		args = append(args, method)
	}
	if endpoint != "" {
		q += ` AND LOWER(e.path) LIKE ?`
		args = append(args, "%"+endpoint+"%")
	}

	q += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	return idx.execEndpointSearch(q, args, "FTS match")
}

func (idx *FTSIndex) searchEndpointsFilters(endpoint, method string, limit int) []FTSEndpointResult {
	q := `
		SELECT e.path, e.method, e.delegated_work, e.delegated_personal,
			   e.application, e.query_params, e.required_headers,
			   e.default_properties, e.notes,
			   0 AS rank
		FROM endpoints e
		WHERE 1=1`

	var args []interface{}

	if method != "" {
		q += ` AND e.method = ?`
		args = append(args, method)
	}
	if endpoint != "" {
		q += ` AND LOWER(e.path) LIKE ?`
		args = append(args, "%"+endpoint+"%")
	}

	q += ` ORDER BY e.path LIMIT ?`
	args = append(args, limit)

	return idx.execEndpointSearch(q, args, "matched filters")
}

func (idx *FTSIndex) execEndpointSearch(query string, args []interface{}, defaultReason string) []FTSEndpointResult {
	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []FTSEndpointResult
	for rows.Next() {
		var r FTSEndpointResult
		var rank float64
		var delegWork, delegPersonal, appPerms sql.NullString
		var qParams, headers, defaults, notes sql.NullString

		err := rows.Scan(
			&r.Path, &r.Method, &delegWork, &delegPersonal,
			&appPerms, &qParams, &headers, &defaults, &notes,
			&rank,
		)
		if err != nil {
			continue
		}

		r.DelegatedWork = splitNonEmpty(delegWork.String)
		r.DelegatedPersonal = splitNonEmpty(delegPersonal.String)
		r.Application = splitNonEmpty(appPerms.String)
		r.QueryParams = splitNonEmpty(qParams.String)
		r.RequiredHeaders = splitNonEmpty(headers.String)
		r.DefaultProperties = splitNonEmpty(defaults.String)
		r.Notes = splitNotes(notes.String)
		r.Score = rank
		r.MatchReason = defaultReason

		results = append(results, r)
	}

	return results
}

// SearchResources finds resource docs matching the given criteria using FTS5.
func (idx *FTSIndex) SearchResources(resource, query string, limit int) []FTSResourceResult {
	if limit <= 0 {
		limit = 10
	}

	ftsQuery := ftsutil.BuildFTSQuery(query)
	resource = strings.ToLower(strings.TrimSpace(resource))

	if ftsQuery != "" {
		return idx.searchResourcesFTS(ftsQuery, resource, limit)
	}

	return idx.searchResourcesFilters(resource, limit)
}

func (idx *FTSIndex) searchResourcesFTS(ftsQuery, resource string, limit int) []FTSResourceResult {
	// BM25 column weights: name(10), properties(3)
	q := `
		SELECT r.name, r.properties,
			   bm25(resources_fts, 10, 3) AS rank
		FROM resources_fts f
		JOIN resources r ON f.rowid = r.id
		WHERE resources_fts MATCH ?`

	args := []interface{}{ftsQuery}

	if resource != "" {
		q += ` AND LOWER(r.name) LIKE ?`
		args = append(args, "%"+resource+"%")
	}

	q += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	return idx.execResourceSearch(q, args, "FTS match")
}

func (idx *FTSIndex) searchResourcesFilters(resource string, limit int) []FTSResourceResult {
	q := `
		SELECT r.name, r.properties,
			   0 AS rank
		FROM resources r
		WHERE 1=1`

	var args []interface{}

	if resource != "" {
		q += ` AND LOWER(r.name) LIKE ?`
		args = append(args, "%"+resource+"%")
	}

	q += ` ORDER BY r.name LIMIT ?`
	args = append(args, limit)

	return idx.execResourceSearch(q, args, "matched filters")
}

func (idx *FTSIndex) execResourceSearch(query string, args []interface{}, defaultReason string) []FTSResourceResult {
	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []FTSResourceResult
	for rows.Next() {
		var r FTSResourceResult
		var rank float64
		var props sql.NullString

		err := rows.Scan(&r.Name, &props, &rank)
		if err != nil {
			continue
		}

		r.Properties = props.String
		r.Score = rank
		r.MatchReason = defaultReason

		results = append(results, r)
	}

	return results
}

// splitNonEmpty splits a space-separated string into a slice, returning nil for empty strings.
func splitNonEmpty(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// splitNotes splits a pipe-separated notes string back into individual notes.
func splitNotes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, " | ")
	var notes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			notes = append(notes, p)
		}
	}
	return notes
}

// FormatEndpointResults converts FTS endpoint results to the standard JSON output.
func FormatEndpointResults(results []FTSEndpointResult) interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"results": []interface{}{},
			"message": "No matching endpoint docs found. Try broadening your search or use openapi-search for endpoint discovery.",
		}
	}

	items := make([]interface{}, len(results))
	for i, r := range results {
		m := map[string]interface{}{
			"path":        r.Path,
			"method":      r.Method,
			"matchReason": r.MatchReason,
		}

		perms := map[string]interface{}{}
		if len(r.DelegatedWork) > 0 {
			perms["delegatedWork"] = r.DelegatedWork
		}
		if len(r.DelegatedPersonal) > 0 {
			perms["delegatedPersonal"] = r.DelegatedPersonal
		}
		if len(r.Application) > 0 {
			perms["application"] = r.Application
		}
		if len(perms) > 0 {
			m["permissions"] = perms
		}

		if len(r.QueryParams) > 0 {
			m["queryParams"] = r.QueryParams
		}
		if len(r.RequiredHeaders) > 0 {
			m["requiredHeaders"] = r.RequiredHeaders
		}
		if len(r.DefaultProperties) > 0 {
			m["defaultProperties"] = r.DefaultProperties
		}
		if len(r.Notes) > 0 {
			m["notes"] = r.Notes
		}

		items[i] = m
	}

	return map[string]interface{}{
		"count":   len(results),
		"results": items,
	}
}

// FormatResourceResults converts FTS resource results to the standard JSON output.
func FormatResourceResults(results []FTSResourceResult) interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"results": []interface{}{},
			"message": fmt.Sprintf("No matching resources found. Try broadening your search."),
		}
	}

	items := make([]interface{}, len(results))
	for i, r := range results {
		m := map[string]interface{}{
			"name":        r.Name,
			"matchReason": r.MatchReason,
		}
		if r.Properties != "" {
			m["properties"] = r.Properties
		}
		items[i] = m
	}

	return map[string]interface{}{
		"count":   len(results),
		"results": items,
	}
}
