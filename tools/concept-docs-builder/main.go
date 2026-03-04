// Command concept-docs-builder downloads Microsoft Graph concept documentation,
// strips SDK code tabs and boilerplate, and outputs clean HTTP-only markdown files
// suitable for LLM consumption.
//
// Usage:
//
//	go run ./tools/concept-docs-builder/... -output skills/msgraph/references/docs
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const docsRepo = "https://github.com/microsoftgraph/microsoft-graph-docs-contrib.git"

// conceptDoc maps a source file in the docs repo to an output filename.
type conceptDoc struct {
	source string // path relative to repo root
	output string // output filename
	title  string // human-readable title for the header
}

var docs = []conceptDoc{
	{"concepts/query-parameters.md", "query-parameters.md", "Query Parameters"},
	{"concepts/aad-advanced-queries.md", "advanced-queries.md", "Advanced Queries on Directory Objects"},
	{"concepts/errors.md", "errors.md", "Error Responses"},
	{"concepts/paging.md", "paging.md", "Paging"},
	{"concepts/throttling.md", "throttling.md", "Throttling"},
	{"concepts/json-batching.md", "batching.md", "JSON Batching"},
	{"concepts/best-practices-concept.md", "best-practices.md", "Best Practices"},
}

func main() {
	outputDir := flag.String("output", "skills/msgraph/references/docs", "Output directory for cleaned markdown files")
	input := flag.String("input", "", "Local docs repo directory (skips clone if set)")
	flag.Parse()

	var repoDir string
	if *input != "" {
		fmt.Fprintf(os.Stderr, "Using local docs directory: %s\n", *input)
		repoDir = *input
	} else {
		var err error
		repoDir, err = cloneDocs()
		if err != nil {
			fatal("Failed to clone docs repo: %v", err)
		}
		defer os.RemoveAll(repoDir)
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fatal("Failed to create output directory: %v", err)
	}

	for _, doc := range docs {
		srcPath := filepath.Join(repoDir, doc.source)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", doc.source, err)
			continue
		}

		cleaned := cleanDoc(string(data), doc.title)
		outPath := filepath.Join(*outputDir, doc.output)
		if err := os.WriteFile(outPath, []byte(cleaned), 0644); err != nil {
			fatal("Failed to write %s: %v", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s (%d bytes)\n", outPath, len(cleaned))
	}

	fmt.Fprintf(os.Stderr, "Done: %d concept docs written to %s\n", len(docs), *outputDir)
}

// cloneDocs performs a shallow sparse checkout of the docs repo, fetching only
// the concepts/ directory.
func cloneDocs() (string, error) {
	tmpDir, err := os.MkdirTemp("", "msgraph-concept-docs-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Cloning docs repo (sparse checkout for concepts/)...")

	cloneCmd := exec.Command("git", "clone",
		"--depth", "1",
		"--filter=blob:none",
		"--sparse",
		docsRepo,
		tmpDir,
	)
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	sparseCmd := exec.Command("git", "sparse-checkout", "set", "concepts")
	sparseCmd.Dir = tmpDir
	sparseCmd.Stderr = os.Stderr
	if err := sparseCmd.Run(); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git sparse-checkout failed: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Docs repo cloned successfully")
	return tmpDir, nil
}

// cleanDoc processes a raw markdown doc and returns cleaned output.
func cleanDoc(raw, title string) string {
	// 1. Remove YAML frontmatter
	raw = removeFrontmatter(raw)

	// 2. Process lines: handle tabs, includes, comments, etc.
	lines := strings.Split(raw, "\n")
	var out []string
	inSDKTab := false  // inside a non-HTTP SDK tab
	inHTTPTab := false // inside the HTTP tab
	inHTMLComment := false
	tabDepth := 0 // header depth of current tab set (1 = #, 2 = ##)

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Handle multi-line HTML comments
		if inHTMLComment {
			if strings.Contains(line, "-->") {
				inHTMLComment = false
			}
			continue
		}
		if strings.Contains(line, "<!--") {
			if !strings.Contains(line, "-->") {
				inHTMLComment = true
				continue
			}
			// Single-line comment — skip the whole line
			continue
		}

		// Detect tab headers: # [Name](#tab/...) or ## [Name](#tab/...)
		if isTabHeader(line) {
			depth := tabHeaderDepth(line)
			if isHTTPTab(line) {
				inHTTPTab = true
				inSDKTab = false
				tabDepth = depth
				continue // skip the tab header line itself
			} else {
				inHTTPTab = false
				inSDKTab = true
				tabDepth = depth
				continue
			}
		}

		// Tab set closer: a line that is exactly "---" after tabs
		if (inSDKTab || inHTTPTab) && isTabSetCloser(line) {
			inSDKTab = false
			inHTTPTab = false
			tabDepth = 0
			continue
		}

		// If inside an SDK tab, skip
		if inSDKTab {
			continue
		}

		// Skip [!INCLUDE ...] lines
		if isIncludeLine(line) {
			continue
		}

		// Skip [!VIDEO ...] lines
		if isVideoLine(line) {
			continue
		}

		// Skip [!div ...] lines
		if isDivLine(line) {
			continue
		}

		// Skip markdownlint directives
		if isMarkdownLintDirective(line) {
			continue
		}

		// Stop at "Related content" or "Next step" sections
		if isTerminalSection(line) {
			break
		}

		// Convert code fence language
		line = convertCodeFence(line)

		// Skip link reference definitions at end of visible content
		// (we do this after terminal section check)

		out = append(out, line)
	}

	result := strings.Join(out, "\n")

	// Remove link reference definitions (lines like [identifier]: URL at the end)
	result = removeLinkRefDefs(result)

	// Clean up excessive blank lines (max 2 consecutive)
	result = collapseBlankLines(result)

	// Trim trailing whitespace
	result = strings.TrimRight(result, " \t\n") + "\n"

	_ = tabDepth
	return result
}

// removeFrontmatter strips YAML frontmatter (between --- markers at the start).
func removeFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return s
	}
	end := strings.Index(s[3:], "---")
	if end == -1 {
		return s
	}
	return s[end+6:] // skip past closing ---
}

var tabHeaderRe = regexp.MustCompile(`^#{1,3}\s+\[.+\]\(#tab/`)

func isTabHeader(line string) bool {
	return tabHeaderRe.MatchString(line)
}

func tabHeaderDepth(line string) int {
	for i, c := range line {
		if c != '#' {
			return i
		}
	}
	return 0
}

var httpTabRe = regexp.MustCompile(`^#{1,3}\s+\[HTTP\]\(#tab/http\)`)

func isHTTPTab(line string) bool {
	return httpTabRe.MatchString(line)
}

func isTabSetCloser(line string) bool {
	return strings.TrimSpace(line) == "---"
}

var includeRe = regexp.MustCompile(`\[!INCLUDE\s+`)

func isIncludeLine(line string) bool {
	return includeRe.MatchString(line)
}

var videoRe = regexp.MustCompile(`\[!VIDEO\s+`)

func isVideoLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return videoRe.MatchString(trimmed)
}

var divRe = regexp.MustCompile(`\[!div\s+`)

func isDivLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return divRe.MatchString(trimmed)
}

func isMarkdownLintDirective(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "<!-- markdownlint-")
}

var terminalSectionRe = regexp.MustCompile(`^#{1,3}\s+(Related content|Next step|Related topics)`)

func isTerminalSection(line string) bool {
	return terminalSectionRe.MatchString(line)
}

func convertCodeFence(line string) string {
	if strings.HasPrefix(line, "```msgraph-interactive") {
		return "```http"
	}
	return line
}

var linkRefDefRe = regexp.MustCompile(`(?m)^\[[\w.-]+\]:\s+\S+.*$`)

func removeLinkRefDefs(s string) string {
	return linkRefDefRe.ReplaceAllString(s, "")
}

func collapseBlankLines(s string) string {
	re := regexp.MustCompile(`\n{4,}`)
	s = re.ReplaceAllString(s, "\n\n\n")
	// Also collapse at start
	s = strings.TrimLeft(s, "\n")
	return s
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
