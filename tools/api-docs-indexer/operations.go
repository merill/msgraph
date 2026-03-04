package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// EndpointDoc represents a parsed API operation document.
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

// parseOperationDoc parses an API operation markdown file into an EndpointDoc.
// baseDir is the root of the cloned docs repo (for resolving includes).
func parseOperationDoc(filePath, baseDir string) (*EndpointDoc, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	content := string(data)

	fm, body := extractFrontmatter(content)
	docType := getFrontmatterField(fm, "doc_type")
	if docType != "" && docType != "apiPageType" {
		return nil, fmt.Errorf("not an API operation doc (doc_type=%s)", docType)
	}

	// Extract HTTP request (method + path)
	method, path := extractHTTPRequest(body)
	if method == "" || path == "" {
		return nil, fmt.Errorf("could not extract HTTP request")
	}

	doc := &EndpointDoc{
		Path:   path,
		Method: method,
	}

	// Extract permissions
	doc.Permissions = extractPermissions(body, filePath, baseDir)

	// Extract query parameters
	doc.QueryParams = extractQueryParams(body)

	// Extract required headers
	doc.RequiredHeaders = extractRequiredHeaders(body)

	// Extract default properties
	doc.DefaultProperties = extractDefaultProperties(body)

	// Extract notes/gotchas from top of document (before first ## section)
	doc.Notes = extractTopNotes(body)

	return doc, nil
}

// extractHTTPRequest finds the first HTTP method + path from the HTTP request section.
func extractHTTPRequest(body string) (method, path string) {
	section := extractSection(body, "## HTTP request")
	if section == "" {
		return "", ""
	}

	// Look in code blocks first
	blocks := extractCodeBlocks(section, "http")
	if len(blocks) == 0 {
		// Try without language specifier
		blocks = extractCodeBlocks(section, "")
	}

	for _, block := range blocks {
		m, p := parseHTTPLine(block)
		if m != "" && p != "" {
			return m, p
		}
	}

	// Fallback: search entire section for HTTP method patterns
	for _, line := range strings.Split(section, "\n") {
		m, p := parseHTTPLine(line)
		if m != "" && p != "" {
			return m, p
		}
	}

	return "", ""
}

var reHTTPMethod = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE)\s+(/\S+)`)

// parseHTTPLine extracts method and path from a line like "GET /users"
func parseHTTPLine(text string) (method, path string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		match := reHTTPMethod.FindStringSubmatch(line)
		if len(match) > 2 {
			return match[1], match[2]
		}
	}
	return "", ""
}

// extractPermissions extracts permission scopes from the Permissions section.
func extractPermissions(body, filePath, baseDir string) Permissions {
	section := extractSection(body, "## Permissions")
	if section == "" {
		return Permissions{}
	}

	// Check for [!INCLUDE [permissions-table](...)]
	includePath := extractIncludePath(section)
	if includePath != "" {
		resolved := resolveIncludePath(filePath, includePath)
		data, err := os.ReadFile(resolved)
		if err == nil {
			return parsePermissionsTable(string(data))
		}
		// If include resolution fails, fall through to try inline table
	}

	// Try parsing inline permissions table
	return parsePermissionsTable(section)
}

// parsePermissionsTable extracts permission scopes from a markdown table.
// Expected format: |Permission type|Permissions (from least to most privileged)|
func parsePermissionsTable(content string) Permissions {
	_, rows := parseMarkdownTable(content)
	var perms Permissions

	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		permType := strings.ToLower(row[0])
		scopesStr := row[1]

		// Skip "not supported" or "not available"
		scopesLower := strings.ToLower(scopesStr)
		if strings.Contains(scopesLower, "not supported") || strings.Contains(scopesLower, "not available") || scopesStr == "" {
			continue
		}

		scopes := parsePermissionScopes(scopesStr)

		if strings.Contains(permType, "delegated") && strings.Contains(permType, "work") {
			perms.DelegatedWork = scopes
		} else if strings.Contains(permType, "delegated") && strings.Contains(permType, "personal") {
			perms.DelegatedPersonal = scopes
		} else if strings.Contains(permType, "application") {
			perms.Application = scopes
		}
	}

	return perms
}

// parsePermissionScopes splits a comma-separated permissions string into individual scopes.
func parsePermissionScopes(s string) []string {
	// Clean markdown formatting
	s = cleanMarkdown(s)

	// Split by comma
	parts := strings.Split(s, ",")
	var scopes []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Extract just the scope name (skip prose like "or")
		// Scopes look like: User.Read.All, Directory.ReadWrite.All
		if p != "" && strings.Contains(p, ".") && !strings.Contains(p, " ") {
			scopes = append(scopes, p)
		} else if p != "" {
			// Try to extract scope from mixed text like "User.Read.All or User.ReadWrite.All"
			words := strings.Fields(p)
			for _, w := range words {
				w = strings.Trim(w, ".,;")
				if strings.Contains(w, ".") && len(w) > 3 {
					scopes = append(scopes, w)
				}
			}
		}
	}
	return scopes
}

var reODataParam = regexp.MustCompile(`\$[a-zA-Z]+`)

// extractQueryParams finds OData query parameter names from the query parameters section.
func extractQueryParams(body string) []string {
	section := extractSection(body, "## Optional query parameters")
	if section == "" {
		section = extractSection(body, "## Query parameters")
	}
	if section == "" {
		return nil
	}

	seen := make(map[string]bool)
	var params []string

	matches := reODataParam.FindAllString(section, -1)
	for _, m := range matches {
		m = strings.ToLower(m)
		if !seen[m] && isKnownODataParam(m) {
			seen[m] = true
			params = append(params, m)
		}
	}

	return params
}

func isKnownODataParam(p string) bool {
	switch p {
	case "$count", "$expand", "$filter", "$format", "$orderby",
		"$search", "$select", "$skip", "$top":
		return true
	}
	return false
}

// extractRequiredHeaders looks for special required headers, especially ConsistencyLevel.
func extractRequiredHeaders(body string) []string {
	var headers []string

	// Check request headers section
	section := extractSection(body, "## Request headers")
	if section == "" {
		return nil
	}

	sectionLower := strings.ToLower(section)
	if strings.Contains(sectionLower, "consistencylevel") {
		headers = append(headers, "ConsistencyLevel: eventual")
	}

	return headers
}

// reDefaultPropsList matches a parenthesized, comma-separated list of at least 3
// camelCase identifiers that typically appears after text like "returned by default"
// or "only returns the following properties by default".
// Example: "...returned by default (**businessPhones**, **displayName**, **givenName**, ...)"
var reDefaultPropsList = regexp.MustCompile(`(?i)(?:returned|default|returns)[\s\S]{0,200}?\(([a-zA-Z]\w+(?:\s*,\s*(?:and\s+)?[a-zA-Z]\w+){2,})\)`)

// extractDefaultProperties finds the list of default properties mentioned in the query params section.
func extractDefaultProperties(body string) []string {
	section := extractSection(body, "## Optional query parameters")
	if section == "" {
		section = extractSection(body, "## Query parameters")
	}
	if section == "" {
		return nil
	}

	// Strip markdown formatting so bold/backtick-wrapped names become plain identifiers
	cleaned := cleanMarkdown(section)

	match := reDefaultPropsList.FindStringSubmatch(cleaned)
	if match == nil {
		return nil
	}

	listStr := match[1]
	parts := strings.Split(listStr, ",")
	var props []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "and ")
		p = strings.TrimSpace(p)

		// Must look like a camelCase property name: starts with letter, no
		// spaces, no slashes, no dots (to exclude permission scopes), length > 1
		if p == "" || len(p) < 2 {
			continue
		}
		if !isLetter(p[0]) || strings.ContainsAny(p, " ./\\<>#=()") {
			continue
		}
		// Reject known filter operators
		pLower := strings.ToLower(p)
		if isKnownFilterOp(pLower) {
			continue
		}
		props = append(props, p)
	}

	// Require at least 3 results to reduce false positives
	if len(props) < 3 {
		return nil
	}

	return props
}

// isLetter reports whether b is an ASCII letter.
func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// extractTopNotes extracts notes from the beginning of the document (before sections)
// and from the Permissions and Query Parameters sections.
func extractTopNotes(body string) []string {
	// Get content before first ## heading
	firstSection := strings.Index(body, "\n## ")
	topContent := body
	if firstSection >= 0 {
		topContent = body[:firstSection]
	}

	notes := extractCallouts(topContent)

	// Also check the query params section for important notes
	qpSection := extractSection(body, "## Optional query parameters")
	if qpSection == "" {
		qpSection = extractSection(body, "## Query parameters")
	}
	if qpSection != "" {
		qpNotes := extractCallouts(qpSection)
		notes = append(notes, qpNotes...)
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, n := range notes {
		if !seen[n] {
			seen[n] = true
			unique = append(unique, n)
		}
	}

	return unique
}
