package apidocs

import (
	"fmt"
	"strings"

	"github.com/merill/msgraph/internal/ftsutil"
)

// BuildFTSDatabase creates a SQLite FTS5 database from the API docs index.
// The database contains content tables for both endpoints and resources,
// plus FTS5 virtual tables for full-text search with Porter stemming.
func BuildFTSDatabase(idx *Index, dbPath string) error {
	db, err := ftsutil.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// --- Endpoints ---
	_, err = db.Exec(`
		CREATE TABLE endpoints (
			id                  INTEGER PRIMARY KEY,
			path                TEXT NOT NULL,
			method              TEXT NOT NULL,
			delegated_work      TEXT,
			delegated_personal  TEXT,
			application         TEXT,
			query_params        TEXT,
			required_headers    TEXT,
			default_properties  TEXT,
			notes               TEXT
		)`)
	if err != nil {
		return fmt.Errorf("create endpoints table: %w", err)
	}

	// FTS5 for endpoints.
	// Column weights: path(10), notes(3), query_params(2), required_headers(2),
	// delegated_work(1), application(1), default_properties(1)
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE endpoints_fts USING fts5(
			path,
			notes,
			query_params,
			required_headers,
			delegated_work,
			application,
			default_properties,
			content=endpoints,
			content_rowid=id,
			tokenize='porter unicode61'
		)`)
	if err != nil {
		return fmt.Errorf("create endpoints FTS table: %w", err)
	}

	// --- Resources ---
	_, err = db.Exec(`
		CREATE TABLE resources (
			id          INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			properties  TEXT
		)`)
	if err != nil {
		return fmt.Errorf("create resources table: %w", err)
	}

	// FTS5 for resources.
	// Column weights: name(10), properties(3)
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE resources_fts USING fts5(
			name,
			properties,
			content=resources,
			content_rowid=id,
			tokenize='porter unicode61'
		)`)
	if err != nil {
		return fmt.Errorf("create resources FTS table: %w", err)
	}

	// Insert all data in a single transaction.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert endpoints.
	epStmt, err := tx.Prepare(`
		INSERT INTO endpoints (id, path, method, delegated_work, delegated_personal,
			application, query_params, required_headers, default_properties, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare endpoint insert: %w", err)
	}
	defer epStmt.Close()

	for i, ep := range idx.Endpoints {
		id := i + 1
		delegWork := strings.Join(ep.Permissions.DelegatedWork, " ")
		delegPersonal := strings.Join(ep.Permissions.DelegatedPersonal, " ")
		appPerms := strings.Join(ep.Permissions.Application, " ")
		qParams := strings.Join(ep.QueryParams, " ")
		headers := strings.Join(ep.RequiredHeaders, " ")
		defaults := strings.Join(ep.DefaultProperties, " ")
		notes := strings.Join(ep.Notes, " | ")

		_, err := epStmt.Exec(id, ep.Path, ep.Method, delegWork, delegPersonal,
			appPerms, qParams, headers, defaults, notes)
		if err != nil {
			return fmt.Errorf("insert endpoint %d: %w", id, err)
		}
	}

	// Populate endpoints FTS.
	_, err = tx.Exec(`
		INSERT INTO endpoints_fts (rowid, path, notes, query_params,
			required_headers, delegated_work, application, default_properties)
		SELECT id, path, notes, query_params,
			required_headers, delegated_work, application, default_properties
		FROM endpoints`)
	if err != nil {
		return fmt.Errorf("populate endpoints FTS: %w", err)
	}

	// Insert resources.
	resStmt, err := tx.Prepare(`
		INSERT INTO resources (id, name, properties)
		VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare resource insert: %w", err)
	}
	defer resStmt.Close()

	for i, res := range idx.Resources {
		id := i + 1
		propsText := formatResourceProperties(res.Properties)

		_, err := resStmt.Exec(id, res.Name, propsText)
		if err != nil {
			return fmt.Errorf("insert resource %d: %w", id, err)
		}
	}

	// Populate resources FTS.
	_, err = tx.Exec(`
		INSERT INTO resources_fts (rowid, name, properties)
		SELECT id, name, properties
		FROM resources`)
	if err != nil {
		return fmt.Errorf("populate resources FTS: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Optimize both FTS indexes.
	_, err = db.Exec(`INSERT INTO endpoints_fts(endpoints_fts) VALUES('optimize')`)
	if err != nil {
		return fmt.Errorf("optimize endpoints FTS: %w", err)
	}
	_, err = db.Exec(`INSERT INTO resources_fts(resources_fts) VALUES('optimize')`)
	if err != nil {
		return fmt.Errorf("optimize resources FTS: %w", err)
	}

	return nil
}

// formatResourceProperties converts a slice of PropertyDoc into a searchable
// text string containing all property names, types, and notes.
func formatResourceProperties(props []PropertyDoc) string {
	if len(props) == 0 {
		return ""
	}
	var parts []string
	for _, p := range props {
		entry := p.Name + " " + p.Type
		if len(p.Filter) > 0 {
			entry += " filter:" + strings.Join(p.Filter, ",")
		}
		if p.Notes != "" {
			entry += " " + p.Notes
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, " | ")
}
