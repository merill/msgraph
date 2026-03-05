package samples

import (
	"fmt"
	"os"
	"strings"

	"github.com/merill/msgraph/internal/ftsutil"
)

// BuildFTSDatabase creates a SQLite FTS5 database from the samples index.
// The database contains both a content table (for returning results) and
// an FTS5 virtual table (for full-text search with Porter stemming).
func BuildFTSDatabase(idx *Index, dbPath string) error {
	// Always start from a clean database to avoid table-exists errors when rerun.
	_ = os.Remove(dbPath)

	db, err := ftsutil.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create content table.
	_, err = db.Exec(`
		CREATE TABLE samples (
			id      INTEGER PRIMARY KEY,
			intent  TEXT NOT NULL,
			query   TEXT NOT NULL,
			product TEXT,
			file    TEXT
		)`)
	if err != nil {
		return fmt.Errorf("create samples table: %w", err)
	}

	// Create FTS5 virtual table.
	// Column weights in search: intent(5), query(3), product(2)
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE samples_fts USING fts5(
			intent,
			query,
			product,
			content=samples,
			content_rowid=id,
			tokenize='porter unicode61'
		)`)
	if err != nil {
		return fmt.Errorf("create FTS table: %w", err)
	}

	// Insert data in a transaction.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.Prepare(`
		INSERT INTO samples (id, intent, query, product, file)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	for i, s := range idx.Samples {
		id := i + 1
		queryText := strings.Join(s.QueryStrings(), " ")

		_, err := insertStmt.Exec(id, s.Intent, queryText, s.Product, s.File)
		if err != nil {
			return fmt.Errorf("insert sample %d: %w", id, err)
		}
	}

	// Populate the FTS index from the content table.
	_, err = tx.Exec(`
		INSERT INTO samples_fts (rowid, intent, query, product)
		SELECT id, intent, query, product
		FROM samples`)
	if err != nil {
		return fmt.Errorf("populate FTS index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Optimize the FTS index.
	_, err = db.Exec(`INSERT INTO samples_fts(samples_fts) VALUES('optimize')`)
	if err != nil {
		return fmt.Errorf("optimize FTS: %w", err)
	}

	return nil
}
