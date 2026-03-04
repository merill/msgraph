package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/merill/msgraph/internal/apidocs"
	"github.com/spf13/cobra"
)

var apiDocsSearchCmd = &cobra.Command{
	Use:   "api-docs-search",
	Short: "Search Microsoft Graph API documentation for endpoint details",
	Long: `Search a pre-processed index of Microsoft Graph API documentation to find
per-endpoint permissions, supported query parameters, default properties,
required headers, and endpoint-specific gotchas.

Use --endpoint to look up endpoint details (permissions, query params, headers):
  msgraph api-docs-search --endpoint /users --method GET

Use --resource to look up resource properties (filter operators, default status):
  msgraph api-docs-search --resource user

Examples:
  msgraph api-docs-search --endpoint /users --method GET
  msgraph api-docs-search --endpoint /me/messages
  msgraph api-docs-search --resource user
  msgraph api-docs-search --resource group --query "filter"
  msgraph api-docs-search --query "ConsistencyLevel"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		endpoint, _ := cmd.Flags().GetString("endpoint")
		resource, _ := cmd.Flags().GetString("resource")
		method, _ := cmd.Flags().GetString("method")
		query, _ := cmd.Flags().GetString("query")
		limit, _ := cmd.Flags().GetInt("limit")

		if endpoint == "" && resource == "" && query == "" {
			return fmt.Errorf("at least one of --endpoint, --resource, or --query is required")
		}

		indexPath := findApiDocsIndexPath()
		if indexPath == "" {
			return fmt.Errorf("API docs index file not found. Set MSGRAPH_API_DOCS_PATH or ensure references/api-docs-index.json exists relative to the binary")
		}

		idx, err := apidocs.LoadIndex(indexPath)
		if err != nil {
			return err
		}

		// If --resource is specified, search resources; otherwise search endpoints
		if resource != "" {
			results := idx.SearchResources(resource, query, limit)
			if len(results) == 0 {
				return outputJSON(map[string]interface{}{
					"results": []interface{}{},
					"message": "No matching resources found. Try broadening your search.",
				})
			}
			return outputJSON(map[string]interface{}{
				"count":   len(results),
				"results": results,
			})
		}

		// Search endpoints
		results := idx.SearchEndpoints(endpoint, method, query, limit)
		if len(results) == 0 {
			return outputJSON(map[string]interface{}{
				"results": []interface{}{},
				"message": "No matching endpoint docs found. Try broadening your search or use openapi-search for endpoint discovery.",
			})
		}

		return outputJSON(map[string]interface{}{
			"count":   len(results),
			"results": results,
		})
	},
}

func init() {
	apiDocsSearchCmd.Flags().String("endpoint", "", "Search by endpoint path (e.g. /users, /me/messages)")
	apiDocsSearchCmd.Flags().String("resource", "", "Search by resource type name (e.g. user, group, message)")
	apiDocsSearchCmd.Flags().String("method", "", "Filter by HTTP method (GET, POST, PUT, PATCH)")
	apiDocsSearchCmd.Flags().String("query", "", "Free-text search query across all fields")
	apiDocsSearchCmd.Flags().Int("limit", 10, "Maximum number of results to return")

	rootCmd.AddCommand(apiDocsSearchCmd)
}

// findApiDocsIndexPath locates the API docs index JSON file.
func findApiDocsIndexPath() string {
	candidates := []string{
		// When running from the repo root
		"skills/msgraph/references/api-docs-index.json",
	}

	// Also check relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			// Binary at msgraph/scripts/bin/ -> index at msgraph/references/
			filepath.Join(exeDir, "..", "..", "references", "api-docs-index.json"),
			// Binary next to msgraph/
			filepath.Join(exeDir, "..", "msgraph", "references", "api-docs-index.json"),
			filepath.Join(exeDir, "msgraph", "references", "api-docs-index.json"),
		)
	}

	// Check environment variable override
	if envPath := os.Getenv("MSGRAPH_API_DOCS_PATH"); envPath != "" {
		candidates = append([]string{envPath}, candidates...)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
