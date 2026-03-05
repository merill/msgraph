package cmd

import (
	"fmt"

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

		// Try FTS database first, fall back to JSON index.
		if dbPath := findFile("api-docs-index.db", "MSGRAPH_API_DOCS_DB_PATH"); dbPath != "" {
			return searchApiDocsFTS(dbPath, endpoint, resource, method, query, limit)
		}

		// Fall back to legacy JSON search.
		indexPath := findFile("api-docs-index.json", "MSGRAPH_API_DOCS_PATH")
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

func searchApiDocsFTS(dbPath, endpoint, resource, method, query string, limit int) error {
	idx, err := apidocs.LoadFTSIndex(dbPath)
	if err != nil {
		return err
	}
	defer idx.Close()

	// If --resource is specified, search resources; otherwise search endpoints.
	if resource != "" {
		results := idx.SearchResources(resource, query, limit)
		return outputJSON(apidocs.FormatResourceResults(results))
	}

	results := idx.SearchEndpoints(endpoint, method, query, limit)
	return outputJSON(apidocs.FormatEndpointResults(results))
}

func init() {
	apiDocsSearchCmd.Flags().String("endpoint", "", "Search by endpoint path (e.g. /users, /me/messages)")
	apiDocsSearchCmd.Flags().String("resource", "", "Search by resource type name (e.g. user, group, message)")
	apiDocsSearchCmd.Flags().String("method", "", "Filter by HTTP method (GET, POST, PUT, PATCH)")
	apiDocsSearchCmd.Flags().String("query", "", "Free-text search query across all fields")
	apiDocsSearchCmd.Flags().Int("limit", 10, "Maximum number of results to return")

	rootCmd.AddCommand(apiDocsSearchCmd)
}
