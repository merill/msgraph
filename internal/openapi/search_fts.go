package openapi

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/merill/msgraph/internal/ftsutil"
)

// FTSIndex provides full-text search over a SQLite FTS5 database
// containing OpenAPI endpoint data.
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

// FTSSearchResult contains a search result with all available fields.
type FTSSearchResult struct {
	Path            string `json:"path"`
	Method          string `json:"method"`
	OperationID     string `json:"operationId,omitempty"`
	Summary         string `json:"summary"`
	Description     string `json:"description,omitempty"`
	Resource        string `json:"resource,omitempty"`
	PathDescription string `json:"pathDescription,omitempty"`
	Tags            string `json:"tags,omitempty"`
	Scopes          string `json:"scopes,omitempty"`

	Deprecated      bool   `json:"deprecated,omitempty"`
	DocURL          string `json:"docUrl,omitempty"`
	OperationType   string `json:"operationType,omitempty"`
	Pageable        bool   `json:"pageable,omitempty"`
	Parameters      string `json:"parameters,omitempty"`
	RequestBodyRef  string `json:"requestBodyRef,omitempty"`
	RequestBodyDesc string `json:"requestBodyDesc,omitempty"`
	ResponseRef     string `json:"responseRef,omitempty"`

	MatchReason string  `json:"matchReason"`
	Score       float64 `json:"score,omitempty"`
}

// Search finds endpoints matching the given criteria using FTS5 full-text
// search with BM25 ranking. All criteria are ANDed together. Empty
// criteria are ignored.
func (idx *FTSIndex) Search(query, resource, method string, limit int) []FTSSearchResult {
	if limit <= 0 {
		limit = 20
	}

	ftsQuery := ftsutil.BuildFTSQuery(query)
	resource = strings.ToLower(strings.TrimSpace(resource))
	method = strings.ToUpper(strings.TrimSpace(method))

	// If we have an FTS query, use FTS5 MATCH with BM25 ranking.
	if ftsQuery != "" {
		return idx.searchFTS(ftsQuery, resource, method, limit)
	}

	// No free-text query — filter by method/resource only.
	return idx.searchFilters(resource, method, limit)
}

func (idx *FTSIndex) searchFTS(ftsQuery, resource, method string, limit int) []FTSSearchResult {
	// BM25 column weights correspond to the FTS5 column order:
	// path(10), summary(5), description(3), resource(3), tags(3),
	// operation_id(2), parameters(1), scopes(1), path_description(2)
	q := `
		SELECT e.path, e.method, e.operation_id, e.summary, e.description,
			   e.resource, e.path_description, e.tags, e.scopes,
			   e.deprecated, e.doc_url, e.operation_type, e.pageable,
			   e.parameters, e.request_body_ref, e.request_body_desc,
			   e.response_ref,
			   bm25(endpoints_fts, 10, 5, 3, 3, 3, 2, 1, 1, 2) AS rank
		FROM endpoints_fts f
		JOIN endpoints e ON f.rowid = e.id
		WHERE endpoints_fts MATCH ?`

	args := []interface{}{ftsQuery}

	if method != "" {
		q += ` AND e.method = ?`
		args = append(args, method)
	}
	if resource != "" {
		q += ` AND (e.resource = ? OR e.path LIKE ?)`
		args = append(args, resource, "%/"+resource+"%")
	}

	q += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	return idx.execSearch(q, args, "FTS match")
}

func (idx *FTSIndex) searchFilters(resource, method string, limit int) []FTSSearchResult {
	q := `
		SELECT e.path, e.method, e.operation_id, e.summary, e.description,
			   e.resource, e.path_description, e.tags, e.scopes,
			   e.deprecated, e.doc_url, e.operation_type, e.pageable,
			   e.parameters, e.request_body_ref, e.request_body_desc,
			   e.response_ref,
			   0 AS rank
		FROM endpoints e
		WHERE 1=1`

	var args []interface{}

	if method != "" {
		q += ` AND e.method = ?`
		args = append(args, method)
	}
	if resource != "" {
		q += ` AND (e.resource = ? OR e.path LIKE ?)`
		args = append(args, resource, "%/"+resource+"%")
	}

	q += ` ORDER BY e.path LIMIT ?`
	args = append(args, limit)

	return idx.execSearch(q, args, "matched filters")
}

func (idx *FTSIndex) execSearch(query string, args []interface{}, defaultReason string) []FTSSearchResult {
	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []FTSSearchResult
	for rows.Next() {
		var r FTSSearchResult
		var deprecated, pageable int
		var rank float64
		var opID, desc, resource, pathDesc, tags, scopes sql.NullString
		var docURL, opType, params, reqRef, reqDesc, respRef sql.NullString

		err := rows.Scan(
			&r.Path, &r.Method, &opID, &r.Summary, &desc,
			&resource, &pathDesc, &tags, &scopes,
			&deprecated, &docURL, &opType, &pageable,
			&params, &reqRef, &reqDesc, &respRef,
			&rank,
		)
		if err != nil {
			continue
		}

		r.OperationID = opID.String
		r.Description = desc.String
		r.Resource = resource.String
		r.PathDescription = pathDesc.String
		r.Tags = tags.String
		r.Scopes = scopes.String
		r.Deprecated = deprecated != 0
		r.DocURL = docURL.String
		r.OperationType = opType.String
		r.Pageable = pageable != 0
		r.Parameters = params.String
		r.RequestBodyRef = reqRef.String
		r.RequestBodyDesc = reqDesc.String
		r.ResponseRef = respRef.String
		r.Score = rank
		r.MatchReason = defaultReason

		results = append(results, r)
	}

	return results
}

// SearchResultCompat converts an FTSSearchResult to the legacy SearchResult
// format for backward compatibility with the JSON output format.
func (r *FTSSearchResult) SearchResultCompat() SearchResult {
	ep := Endpoint{
		Path:     r.Path,
		Method:   r.Method,
		Summary:  r.Summary,
		Resource: r.Resource,
	}
	if r.Description != "" {
		ep.Description = r.Description
	}
	if r.Scopes != "" {
		ep.Scopes = strings.Fields(r.Scopes)
	}
	return SearchResult{
		Endpoint:    ep,
		MatchReason: r.MatchReason,
	}
}

// ToOutputMap converts an FTSSearchResult to a map for JSON output,
// including all available fields. Empty fields are omitted.
func (r *FTSSearchResult) ToOutputMap() map[string]interface{} {
	m := map[string]interface{}{
		"path":        r.Path,
		"method":      r.Method,
		"summary":     r.Summary,
		"matchReason": r.MatchReason,
	}

	setIf := func(key, val string) {
		if val != "" {
			m[key] = val
		}
	}

	setIf("operationId", r.OperationID)
	setIf("description", r.Description)
	setIf("resource", r.Resource)
	setIf("pathDescription", r.PathDescription)
	setIf("tags", r.Tags)
	setIf("scopes", r.Scopes)
	setIf("docUrl", r.DocURL)
	setIf("operationType", r.OperationType)
	setIf("parameters", r.Parameters)
	setIf("requestBodyRef", r.RequestBodyRef)
	setIf("requestBodyDesc", r.RequestBodyDesc)
	setIf("responseRef", r.ResponseRef)

	if r.Deprecated {
		m["deprecated"] = true
	}
	if r.Pageable {
		m["pageable"] = true
	}

	return m
}

// FormatFTSResults converts a slice of FTSSearchResult into the standard
// JSON output format.
func FormatFTSResults(results []FTSSearchResult) interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"results": []interface{}{},
			"message": fmt.Sprintf("No matching endpoints found. Try broadening your search."),
		}
	}

	items := make([]interface{}, len(results))
	for i, r := range results {
		items[i] = r.ToOutputMap()
	}

	return map[string]interface{}{
		"count":   len(results),
		"results": items,
	}
}
