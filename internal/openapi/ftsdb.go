package openapi

import (
	"fmt"
	"strings"

	"github.com/merill/msgraph/internal/ftsutil"
)

// BuildFTSDatabase creates a SQLite FTS5 database from the full endpoint
// data extracted from the OpenAPI spec. The database contains both a
// content table (for returning results) and an FTS5 virtual table (for
// full-text search with Porter stemming).
func BuildFTSDatabase(endpoints []FullEndpoint, dbPath string) error {
	db, err := ftsutil.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create content table with all extracted fields.
	_, err = db.Exec(`
		CREATE TABLE endpoints (
			id                   INTEGER PRIMARY KEY,
			path                 TEXT NOT NULL,
			method               TEXT NOT NULL,
			operation_id         TEXT,
			summary              TEXT,
			description          TEXT,
			resource             TEXT,
			path_description     TEXT,
			tags                 TEXT,
			scopes               TEXT,
			deprecated           INTEGER DEFAULT 0,
			deprecation_date     TEXT,
			deprecation_removal  TEXT,
			deprecation_desc     TEXT,
			doc_url              TEXT,
			operation_type       TEXT,
			pageable             INTEGER DEFAULT 0,
			parameters           TEXT,
			request_body_ref     TEXT,
			request_body_desc    TEXT,
			response_ref         TEXT
		)`)
	if err != nil {
		return fmt.Errorf("create endpoints table: %w", err)
	}

	// Create FTS5 virtual table for full-text search.
	// Column weights in search will be: path(10), summary(5), description(3),
	// resource(3), tags(3), operation_id(2), parameters(1), scopes(1), path_description(2)
	_, err = db.Exec(`
		CREATE VIRTUAL TABLE endpoints_fts USING fts5(
			path,
			summary,
			description,
			resource,
			tags,
			operation_id,
			parameters,
			scopes,
			path_description,
			content=endpoints,
			content_rowid=id,
			tokenize='porter unicode61'
		)`)
	if err != nil {
		return fmt.Errorf("create FTS table: %w", err)
	}

	// Insert data in a transaction for performance.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	insertStmt, err := tx.Prepare(`
		INSERT INTO endpoints (
			id, path, method, operation_id, summary, description, resource,
			path_description, tags, scopes, deprecated, deprecation_date,
			deprecation_removal, deprecation_desc, doc_url, operation_type,
			pageable, parameters, request_body_ref, request_body_desc, response_ref
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	for i, ep := range endpoints {
		id := i + 1
		tags := strings.Join(ep.Tags, " ")
		scopes := strings.Join(ep.Scopes, " ")
		params := formatParameters(ep.Parameters)

		deprecated := 0
		if ep.Deprecated {
			deprecated = 1
		}
		pageable := 0
		if ep.Pageable {
			pageable = 1
		}

		_, err := insertStmt.Exec(
			id, ep.Path, ep.Method, ep.OperationID, ep.Summary, ep.Description,
			ep.Resource, ep.PathDescription, tags, scopes, deprecated,
			ep.DeprecationDate, ep.DeprecationRemovalDate, ep.DeprecationDescription,
			ep.DocURL, ep.OperationType, pageable, params,
			ep.RequestBodyRef, ep.RequestBodyDesc, ep.ResponseRef,
		)
		if err != nil {
			return fmt.Errorf("insert endpoint %d: %w", id, err)
		}
	}

	// Populate the FTS index from the content table.
	_, err = tx.Exec(`
		INSERT INTO endpoints_fts (rowid, path, summary, description, resource,
			tags, operation_id, parameters, scopes, path_description)
		SELECT id, path, summary, description, resource,
			tags, operation_id, parameters, scopes, path_description
		FROM endpoints`)
	if err != nil {
		return fmt.Errorf("populate FTS index: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Optimize the FTS index for query performance.
	_, err = db.Exec(`INSERT INTO endpoints_fts(endpoints_fts) VALUES('optimize')`)
	if err != nil {
		return fmt.Errorf("optimize FTS: %w", err)
	}

	return nil
}

// formatParameters converts a slice of Parameters into a space-separated
// searchable string like "query:$filter query:$select header:ConsistencyLevel".
func formatParameters(params []Parameter) string {
	if len(params) == 0 {
		return ""
	}
	parts := make([]string, len(params))
	for i, p := range params {
		parts[i] = p.In + ":" + p.Name + " " + p.Name
	}
	return strings.Join(parts, " ")
}
