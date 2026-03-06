package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/merill/msgraph/internal/openapi"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "openapi-search",
	Short: "Search the Microsoft Graph OpenAPI index",
	Long: `Search a pre-processed index of the Microsoft Graph OpenAPI specification 
to find available endpoints, required scopes, and API details.

Examples:
  msgraph openapi-search --query "list users"
  msgraph openapi-search --resource users --method GET
  msgraph openapi-search --query "send mail" --method POST`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")
		resource, _ := cmd.Flags().GetString("resource")
		method, _ := cmd.Flags().GetString("method")
		limit, _ := cmd.Flags().GetInt("limit")

		if query == "" && resource == "" && method == "" {
			return fmt.Errorf("at least one of --query, --resource, or --method is required")
		}

		// Try FTS database first, fall back to JSON index.
		if dbPath := findFTSIndexPath(); dbPath != "" {
			return searchFTS(dbPath, query, resource, method, limit)
		}

		// Fall back to legacy JSON search.
		indexPath := findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("OpenAPI index not found. Set MSGRAPH_INDEX_DB_PATH or ensure references/graph-api-index.db exists relative to the binary")
		}

		idx, err := openapi.LoadIndex(indexPath)
		if err != nil {
			return err
		}

		results := idx.Search(query, resource, method, limit)

		if len(results) == 0 {
			return outputJSON(map[string]interface{}{
				"results": []interface{}{},
				"message": "No matching endpoints found. Try broadening your search.",
			})
		}

		return outputJSON(map[string]interface{}{
			"count":   len(results),
			"results": results,
		})
	},
}

func searchFTS(dbPath, query, resource, method string, limit int) error {
	idx, err := openapi.LoadFTSIndex(dbPath)
	if err != nil {
		return err
	}
	defer idx.Close()

	results := idx.Search(query, resource, method, limit)
	return outputJSON(openapi.FormatFTSResults(results))
}

func init() {
	searchCmd.Flags().String("query", "", "Free-text search query (searches path, summary, description)")
	searchCmd.Flags().String("resource", "", "Filter by resource name (e.g. users, groups, messages)")
	searchCmd.Flags().String("method", "", "Filter by HTTP method (GET, POST, PUT, PATCH)")
	searchCmd.Flags().Int("limit", 20, "Maximum number of results to return")

	rootCmd.AddCommand(searchCmd)
}

// findFTSIndexPath locates the SQLite FTS database file.
func findFTSIndexPath() string {
	return findFile("graph-api-index.db", "MSGRAPH_INDEX_DB_PATH")
}

// findIndexPath locates the OpenAPI JSON index file.
func findIndexPath() string {
	return findFile("graph-api-index.json", "MSGRAPH_INDEX_PATH")
}

// findFile searches for a file in standard locations relative to the
// binary and working directory.
func findFile(filename, envVar string) string {
	var candidates []string

	// Check environment variable override first.
	if envPath := os.Getenv(envVar); envPath != "" {
		candidates = append(candidates, envPath)
	}

	// Working directory paths.
	candidates = append(candidates,
		filepath.Join("skills", "msgraph", "references", filename),
		filepath.Join("references", filename),
	)

	// Relative to the executable.
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			// Binary at scripts/bin/ -> references/ is at ../../references/
			filepath.Join(exeDir, "..", "..", "references", filename),
			filepath.Join(exeDir, "..", "msgraph", "references", filename),
			filepath.Join(exeDir, "msgraph", "references", filename),
		)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
