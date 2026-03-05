// Package ftsutil provides shared utilities for building and querying
// SQLite FTS5 full-text search databases.
package ftsutil

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"

	_ "modernc.org/sqlite"
)

// OpenDB opens (or creates) a SQLite database with pragmas tuned for
// read-heavy workloads. Callers must close the returned *sql.DB.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("ftsutil: open %s: %w", path, err)
	}

	// Pragmas for performance.
	pragmas := []string{
		"PRAGMA journal_mode = OFF",
		"PRAGMA synchronous = OFF",
		"PRAGMA cache_size = -64000",   // 64 MB cache
		"PRAGMA mmap_size = 268435456", // 256 MB mmap
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("ftsutil: pragma: %w", err)
		}
	}

	return db, nil
}

// OpenReadOnly opens a SQLite database in read-only mode with pragmas
// tuned for fast querying.
func OpenReadOnly(path string) (*sql.DB, error) {
	dsn := path + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("ftsutil: open readonly %s: %w", path, err)
	}

	pragmas := []string{
		"PRAGMA cache_size = -32000", // 32 MB cache
		"PRAGMA mmap_size = 268435456",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("ftsutil: pragma: %w", err)
		}
	}

	return db, nil
}

// BuildFTSQuery converts a user's free-text search string into an FTS5
// query. Words are joined with OR for broad matching; BM25 ranking
// naturally boosts documents matching more terms.
//
// Examples:
//
//	"subscribedSkus licenses"  -> "subscribedSkus OR licenses"
//	"send mail"                -> "send OR mail"
//	""                         -> ""
func BuildFTSQuery(userQuery string) string {
	userQuery = strings.TrimSpace(userQuery)
	if userQuery == "" {
		return ""
	}

	// Split on whitespace to get individual terms.
	words := strings.Fields(userQuery)
	if len(words) == 0 {
		return ""
	}

	// Escape any FTS5 special characters in each word.
	escaped := make([]string, 0, len(words))
	for _, w := range words {
		w = escapeFTSWord(w)
		if w != "" {
			escaped = append(escaped, w)
		}
	}

	if len(escaped) == 0 {
		return ""
	}

	if len(escaped) == 1 {
		return escaped[0]
	}

	return strings.Join(escaped, " OR ")
}

// escapeFTSWord quotes a word if it contains FTS5 special characters.
func escapeFTSWord(w string) string {
	// FTS5 special chars: * " ( ) : ^
	// If the word contains any, wrap in double-quotes and escape internal quotes.
	needsQuote := false
	for _, r := range w {
		if r == '"' || r == '*' || r == '(' || r == ')' || r == ':' || r == '^' {
			needsQuote = true
			break
		}
	}
	if needsQuote {
		return `"` + strings.ReplaceAll(w, `"`, `""`) + `"`
	}
	return w
}

// JoinStrings joins a slice of strings with a separator, useful for
// converting []string fields to a single searchable text column.
func JoinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

// SplitCamelCase splits camelCase and PascalCase identifiers into
// separate words for better FTS matching.
// e.g. "subscribedSkus" -> "subscribed Skus subscribedSkus"
func SplitCamelCase(s string) string {
	if s == "" {
		return ""
	}

	var parts []string
	current := []rune{}

	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(runes[i-1]) {
			if len(current) > 0 {
				parts = append(parts, string(current))
			}
			current = []rune{r}
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}

	if len(parts) <= 1 {
		return s
	}

	// Return "part1 part2 ... original" so both individual words and
	// the original compound form are searchable.
	return strings.Join(parts, " ") + " " + s
}
