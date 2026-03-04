package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ResourceDoc represents a parsed resource type document.
type ResourceDoc struct {
	Name       string        `json:"name"`
	Properties []PropertyDoc `json:"properties,omitempty"`
}

// PropertyDoc describes a single property of a resource.
type PropertyDoc struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Filter  []string `json:"filter,omitempty"`
	Default bool     `json:"default,omitempty"`
	Notes   string   `json:"notes,omitempty"`
}

// parseResourceDoc parses a resource type markdown file into a ResourceDoc.
func parseResourceDoc(filePath string) (*ResourceDoc, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	content := string(data)

	fm, body := extractFrontmatter(content)
	docType := getFrontmatterField(fm, "doc_type")
	if docType != "" && docType != "resourcePageType" {
		return nil, fmt.Errorf("not a resource type doc (doc_type=%s)", docType)
	}

	name := extractResourceName(body)
	if name == "" {
		return nil, fmt.Errorf("could not extract resource name")
	}

	doc := &ResourceDoc{
		Name: name,
	}

	// Parse properties section
	section := extractSection(body, "## Properties")
	if section != "" {
		doc.Properties = parsePropertiesTable(section)
	}

	return doc, nil
}

// extractResourceName extracts the resource name from the H1 heading.
// e.g. "# user resource type" → "user"
// e.g. "# educationUser resource type" → "educationUser"
func extractResourceName(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			title := strings.TrimPrefix(line, "# ")
			// Remove "resource type" suffix
			title = strings.TrimSuffix(title, " resource type")
			title = strings.TrimSuffix(title, " Resource Type")
			// Take the first word as the resource name
			parts := strings.Fields(title)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

// parsePropertiesTable parses the Properties section's markdown table.
func parsePropertiesTable(section string) []PropertyDoc {
	headers, rows := parseMarkdownTable(section)
	if len(headers) < 2 {
		return nil
	}

	// Find column indices
	nameCol := -1
	typeCol := -1
	descCol := -1
	for i, h := range headers {
		hLower := strings.ToLower(h)
		if hLower == "property" || hLower == "name" {
			nameCol = i
		} else if hLower == "type" {
			typeCol = i
		} else if hLower == "description" || strings.Contains(hLower, "description") {
			descCol = i
		}
	}

	if nameCol < 0 {
		nameCol = 0
	}
	if typeCol < 0 && len(headers) > 1 {
		typeCol = 1
	}
	if descCol < 0 && len(headers) > 2 {
		descCol = 2
	}

	var props []PropertyDoc
	for _, row := range rows {
		prop := PropertyDoc{}

		if nameCol >= 0 && nameCol < len(row) {
			prop.Name = cleanPropertyName(row[nameCol])
		}
		if prop.Name == "" {
			continue
		}

		if typeCol >= 0 && typeCol < len(row) {
			prop.Type = cleanPropertyType(row[typeCol])
		}

		if descCol >= 0 && descCol < len(row) {
			desc := row[descCol]
			prop.Filter = parseFilterOperators(desc)
			prop.Default = isDefaultProperty(desc)
			prop.Notes = extractPropertyNotes(desc)
		}

		props = append(props, prop)
	}

	return props
}

// cleanPropertyName strips markdown formatting from a property name.
func cleanPropertyName(s string) string {
	s = strings.TrimSpace(s)
	// Remove bold markers
	s = strings.ReplaceAll(s, "**", "")
	// Remove link syntax
	s = reLinkSyntax.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// cleanPropertyType strips markdown link syntax from type names.
// e.g. "[assignedLicense](assignedlicense.md) collection" → "assignedLicense collection"
func cleanPropertyType(s string) string {
	s = strings.TrimSpace(s)
	s = reLinkSyntax.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

var reFilterOps = regexp.MustCompile(`(?i)[Ss]upports\s+\$filter\s*\(([^)]+)\)`)

// parseFilterOperators extracts supported $filter operators from a property description.
// e.g. "Supports $filter (eq, ne, not, ge, le, in, startsWith, and eq on null values)"
// → ["eq", "ne", "not", "ge", "le", "in", "startsWith"]
func parseFilterOperators(desc string) []string {
	// Clean markdown formatting so backtick-wrapped names become plain text
	cleaned := cleanMarkdown(desc)

	match := reFilterOps.FindStringSubmatch(cleaned)
	if len(match) < 2 {
		return nil
	}

	opsStr := match[1]
	// Split by comma and clean
	parts := strings.Split(opsStr, ",")
	seen := make(map[string]bool)
	var ops []string

	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Remove "and" prefix
		p = strings.TrimPrefix(p, "and ")
		p = strings.TrimSpace(p)

		// Extract just the operator keyword (first word)
		words := strings.Fields(p)
		if len(words) > 0 {
			op := strings.ToLower(words[0])
			// Only include known filter operators
			if isKnownFilterOp(op) && !seen[op] {
				seen[op] = true
				ops = append(ops, op)
			}
		}
	}

	return ops
}

func isKnownFilterOp(op string) bool {
	switch op {
	case "eq", "ne", "not", "gt", "ge", "lt", "le", "in",
		"startswith", "endswith", "contains", "has", "any", "all":
		return true
	}
	return false
}

// isDefaultProperty checks if a property description indicates it's returned by default.
func isDefaultProperty(desc string) bool {
	descLower := strings.ToLower(cleanMarkdown(desc))
	if strings.Contains(descLower, "returned by default") {
		return true
	}
	// Explicitly NOT default if "returned only on $select"
	return false
}

// extractPropertyNotes extracts notable information from a property description
// beyond filter/default status. Looks for license requirements, read-only status, etc.
func extractPropertyNotes(desc string) string {
	var notes []string

	cleaned := cleanMarkdown(desc)
	cleanedLower := strings.ToLower(cleaned)

	// Check for read-only
	if strings.Contains(cleanedLower, "read-only") {
		notes = append(notes, "Read-only")
	}

	// Check for not nullable
	if strings.Contains(cleanedLower, "not nullable") {
		notes = append(notes, "Not nullable")
	}

	// Check for required on create
	if strings.Contains(cleanedLower, "required when") && strings.Contains(cleanedLower, "created") {
		notes = append(notes, "Required on create")
	}

	// Check for license requirements (e.g. P1/P2, specific permissions)
	if strings.Contains(cleanedLower, "p1") || strings.Contains(cleanedLower, "p2") ||
		strings.Contains(cleanedLower, "premium") {
		// Extract the relevant sentence
		for _, sentence := range strings.Split(cleaned, ".") {
			sentLower := strings.ToLower(sentence)
			if strings.Contains(sentLower, "p1") || strings.Contains(sentLower, "p2") || strings.Contains(sentLower, "premium") {
				note := strings.TrimSpace(sentence)
				if note != "" {
					notes = append(notes, note)
					break
				}
			}
		}
	}

	if len(notes) == 0 {
		return ""
	}
	return strings.Join(notes, "; ")
}
