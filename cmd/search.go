package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/merill/msgraph-skill/internal/openapi"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "openapi-search",
	Short: "Search the Microsoft Graph OpenAPI index",
	Long: `Search a pre-processed index of the Microsoft Graph OpenAPI specification 
to find available endpoints, required scopes, and API details.

Examples:
  msgraph-skill openapi-search --query "list users"
  msgraph-skill openapi-search --resource users --method GET
  msgraph-skill openapi-search --query "send mail" --method POST`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")
		resource, _ := cmd.Flags().GetString("resource")
		method, _ := cmd.Flags().GetString("method")
		limit, _ := cmd.Flags().GetInt("limit")

		if query == "" && resource == "" && method == "" {
			return fmt.Errorf("at least one of --query, --resource, or --method is required")
		}

		indexPath := findIndexPath()
		if indexPath == "" {
			return fmt.Errorf("OpenAPI index file not found. Set MSGRAPH_INDEX_PATH or ensure references/graph-api-index.json exists relative to the binary")
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

func init() {
	searchCmd.Flags().String("query", "", "Free-text search query (searches path, summary, description)")
	searchCmd.Flags().String("resource", "", "Filter by resource name (e.g. users, groups, messages)")
	searchCmd.Flags().String("method", "", "Filter by HTTP method (GET, POST, PUT, PATCH)")
	searchCmd.Flags().Int("limit", 20, "Maximum number of results to return")

	rootCmd.AddCommand(searchCmd)
}

// findIndexPath locates the OpenAPI index file.
// It checks relative to the binary location and common paths.
func findIndexPath() string {
	candidates := []string{
		// When running from the repo root
		"skills/msgraph/references/graph-api-index.json",
	}

	// Also check relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			// Binary at msgraph/scripts/bin/ → index at msgraph/references/
			filepath.Join(exeDir, "..", "..", "references", "graph-api-index.json"),
			// Binary next to msgraph/
			filepath.Join(exeDir, "..", "msgraph", "references", "graph-api-index.json"),
			filepath.Join(exeDir, "msgraph", "references", "graph-api-index.json"),
		)
	}

	// Check environment variable override
	if envPath := os.Getenv("MSGRAPH_INDEX_PATH"); envPath != "" {
		candidates = append([]string{envPath}, candidates...)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// outputSearchJSON is a helper for search output (reuses outputJSON from auth.go)
func outputSearchJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
