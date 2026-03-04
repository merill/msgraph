package main

import (
	"regexp"
	"strings"
)

// extractFrontmatter returns the YAML frontmatter content (without delimiters)
// and the remaining body content.
func extractFrontmatter(content string) (frontmatter, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	end := strings.Index(content[3:], "\n---")
	if end < 0 {
		return "", content
	}
	fm := content[3 : end+3]
	rest := content[end+3+4:] // skip past closing ---\n
	return strings.TrimSpace(fm), rest
}

// getFrontmatterField extracts a simple key: value field from frontmatter YAML.
func getFrontmatterField(fm, key string) string {
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+":") {
			val := strings.TrimPrefix(line, key+":")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			return val
		}
	}
	return ""
}

// extractSection returns the content between a heading and the next heading of
// the same or higher level. For example, extractSection(body, "## Permissions")
// returns everything from that heading to the next ## heading.
func extractSection(body, heading string) string {
	level := 0
	for _, c := range heading {
		if c == '#' {
			level++
		} else {
			break
		}
	}

	idx := strings.Index(body, heading)
	if idx < 0 {
		// Try case-insensitive match
		bodyLower := strings.ToLower(body)
		headingLower := strings.ToLower(heading)
		idx = strings.Index(bodyLower, headingLower)
		if idx < 0 {
			return ""
		}
	}

	// Skip past the heading line
	start := idx + len(heading)
	nlIdx := strings.Index(body[start:], "\n")
	if nlIdx >= 0 {
		start += nlIdx + 1
	}

	// Find the next heading at same or higher level
	rest := body[start:]
	prefix := strings.Repeat("#", level) + " "
	lines := strings.Split(rest, "\n")
	var end int
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if this line is a heading at same or higher level
		if strings.HasPrefix(trimmed, prefix) || (level > 1 && strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ")) {
			found = true
			break
		}
		// Also match any heading with fewer or equal # characters
		if len(trimmed) > 0 && trimmed[0] == '#' {
			hLevel := 0
			for _, c := range trimmed {
				if c == '#' {
					hLevel++
				} else {
					break
				}
			}
			if hLevel > 0 && hLevel <= level {
				found = true
				break
			}
		}
		end += len(line) + 1
	}
	if !found {
		return strings.TrimSpace(rest)
	}
	if end > len(rest) {
		end = len(rest)
	}
	return strings.TrimSpace(rest[:end])
}

// parseMarkdownTable parses a markdown table into rows of column values.
// Returns headers and rows separately.
func parseMarkdownTable(section string) (headers []string, rows [][]string) {
	lines := strings.Split(section, "\n")
	var tableLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			tableLines = append(tableLines, trimmed)
		} else if len(tableLines) > 0 && trimmed == "" {
			// End of table on blank line
			break
		}
	}

	if len(tableLines) < 2 {
		return nil, nil
	}

	// First line is headers
	headers = parseTableRow(tableLines[0])

	// Skip separator line (second line with :--- patterns)
	startRow := 1
	if startRow < len(tableLines) {
		sep := tableLines[startRow]
		if strings.Contains(sep, "---") || strings.Contains(sep, ":--") {
			startRow = 2
		}
	}

	for i := startRow; i < len(tableLines); i++ {
		row := parseTableRow(tableLines[i])
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	return headers, rows
}

// parseTableRow splits a markdown table row into cell values.
func parseTableRow(line string) []string {
	// Remove leading/trailing pipes and split
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	var cells []string
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// extractCodeBlocks finds all fenced code blocks with the given language tag.
// If lang is empty, matches any code block.
func extractCodeBlocks(section, lang string) []string {
	var blocks []string
	lines := strings.Split(section, "\n")
	inBlock := false
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if strings.HasPrefix(trimmed, "```") {
				blockLang := strings.TrimPrefix(trimmed, "```")
				blockLang = strings.TrimSpace(blockLang)
				if lang == "" || strings.EqualFold(blockLang, lang) || strings.Contains(strings.ToLower(blockLang), strings.ToLower(lang)) {
					inBlock = true
					current = nil
				}
			}
		} else {
			if strings.TrimSpace(trimmed) == "```" {
				blocks = append(blocks, strings.Join(current, "\n"))
				inBlock = false
				current = nil
			} else {
				current = append(current, line)
			}
		}
	}

	return blocks
}

// extractCallouts extracts note and important callout content from the body.
// Looks for patterns like >**Note:** ..., > [!IMPORTANT], > [!Note]
func extractCallouts(body string) []string {
	var notes []string
	lines := strings.Split(body, "\n")

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Pattern: > [!IMPORTANT] or > [!NOTE] — content is on following > lines
		if reCalloutTag.MatchString(trimmed) {
			var content []string
			for j := i + 1; j < len(lines); j++ {
				nextTrimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(nextTrimmed, ">") {
					text := strings.TrimPrefix(nextTrimmed, ">")
					text = strings.TrimSpace(text)
					if text != "" && !reCalloutTag.MatchString("> "+text) {
						content = append(content, text)
					}
				} else {
					break
				}
			}
			if len(content) > 0 {
				note := strings.Join(content, " ")
				note = cleanMarkdown(note)
				if note != "" {
					notes = append(notes, truncate(note, 300))
				}
			}
		}

		// Pattern: >**Note:** inline text
		if reInlineNote.MatchString(trimmed) {
			match := reInlineNote.FindStringSubmatch(trimmed)
			if len(match) > 1 {
				note := cleanMarkdown(match[1])
				if note != "" {
					notes = append(notes, truncate(note, 300))
				}
			}
		}
	}

	return notes
}

var (
	reCalloutTag = regexp.MustCompile(`(?i)>\s*\[!(IMPORTANT|NOTE)\]`)
	reInlineNote = regexp.MustCompile(`(?i)>\s*\*\*Note:\*\*\s*(.+)`)
)

// resolveIncludePath resolves a relative include path from an API doc to an absolute path.
// apiDocPath is the full path to the API doc file, relativePath is the include reference.
func resolveIncludePath(apiDocPath, relativePath string) string {
	// Get the directory of the API doc
	dir := apiDocPath
	lastSlash := strings.LastIndex(dir, "/")
	if lastSlash >= 0 {
		dir = dir[:lastSlash]
	}

	// Handle ../ prefix
	for strings.HasPrefix(relativePath, "../") {
		relativePath = relativePath[3:]
		lastSlash = strings.LastIndex(dir, "/")
		if lastSlash >= 0 {
			dir = dir[:lastSlash]
		}
	}

	return dir + "/" + relativePath
}

// extractIncludePath extracts the file path from an [!INCLUDE ...] directive.
var reInclude = regexp.MustCompile(`\[!INCLUDE\s+\[[^\]]*\]\(([^)]+)\)`)

func extractIncludePath(section string) string {
	match := reInclude.FindStringSubmatch(section)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// cleanMarkdown strips common markdown formatting from text.
func cleanMarkdown(s string) string {
	// Remove bold
	s = strings.ReplaceAll(s, "**", "")
	// Remove italic underscores
	s = strings.ReplaceAll(s, "__", "")
	// Remove inline code backticks
	s = strings.ReplaceAll(s, "`", "")
	// Remove link syntax [text](url) → text
	s = reLinkSyntax.ReplaceAllString(s, "$1")
	// Collapse whitespace
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(s)
}

var reLinkSyntax = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// truncate limits a string to maxLen characters with ellipsis.
func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
