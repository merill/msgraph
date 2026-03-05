package samples

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/merill/msgraph/internal/ftsutil"
)

// FTSIndex provides full-text search over a SQLite FTS5 database
// containing sample data.
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

// FTSSearchResult contains a sample search result with all available fields.
type FTSSearchResult struct {
	Intent      string  `json:"intent"`
	Query       string  `json:"query"`
	Product     string  `json:"product,omitempty"`
	File        string  `json:"file,omitempty"`
	MatchReason string  `json:"matchReason"`
	Score       float64 `json:"score,omitempty"`
}

// Search finds samples matching the given criteria using FTS5 full-text
// search with BM25 ranking.
func (idx *FTSIndex) Search(query, product string, limit int) []FTSSearchResult {
	if limit <= 0 {
		limit = 10
	}

	ftsQuery := ftsutil.BuildFTSQuery(query)
	product = strings.ToLower(strings.TrimSpace(product))

	if ftsQuery != "" {
		return idx.searchFTS(ftsQuery, product, limit)
	}

	// No free-text query — filter by product only.
	return idx.searchFilters(product, limit)
}

func (idx *FTSIndex) searchFTS(ftsQuery, product string, limit int) []FTSSearchResult {
	// BM25 column weights: intent(5), query(3), product(2)
	q := `
		SELECT s.intent, s.query, s.product, s.file,
			   bm25(samples_fts, 5, 3, 2) AS rank
		FROM samples_fts f
		JOIN samples s ON f.rowid = s.id
		WHERE samples_fts MATCH ?`

	args := []interface{}{ftsQuery}

	if product != "" {
		q += ` AND LOWER(s.product) = ?`
		args = append(args, product)
	}

	q += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	return idx.execSearch(q, args, "FTS match")
}

func (idx *FTSIndex) searchFilters(product string, limit int) []FTSSearchResult {
	q := `
		SELECT s.intent, s.query, s.product, s.file,
			   0 AS rank
		FROM samples s
		WHERE 1=1`

	var args []interface{}

	if product != "" {
		q += ` AND LOWER(s.product) = ?`
		args = append(args, product)
	}

	q += ` ORDER BY s.intent LIMIT ?`
	args = append(args, limit)

	return idx.execSearch(q, args, "matched product filter")
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
		var rank float64
		var queryText, product, file sql.NullString

		err := rows.Scan(&r.Intent, &queryText, &product, &file, &rank)
		if err != nil {
			continue
		}

		r.Query = queryText.String
		r.Product = product.String
		r.File = file.String
		r.Score = rank
		r.MatchReason = defaultReason

		results = append(results, r)
	}

	return results
}

// FormatFTSResults converts a slice of FTSSearchResult into the standard
// JSON output format.
func FormatFTSResults(results []FTSSearchResult) interface{} {
	if len(results) == 0 {
		return map[string]interface{}{
			"results": []interface{}{},
			"message": fmt.Sprintf("No matching samples found. Try broadening your search or use openapi-search for endpoint discovery."),
		}
	}

	items := make([]interface{}, len(results))
	for i, r := range results {
		m := map[string]interface{}{
			"intent":      r.Intent,
			"query":       r.Query,
			"matchReason": r.MatchReason,
		}
		if r.Product != "" {
			m["product"] = r.Product
		}
		if r.File != "" {
			m["file"] = r.File
		}
		items[i] = m
	}

	return map[string]interface{}{
		"count":   len(results),
		"results": items,
	}
}
