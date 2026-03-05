package cmd

import (
	"fmt"

	"github.com/merill/msgraph/internal/samples"
	"github.com/spf13/cobra"
)

var sampleSearchCmd = &cobra.Command{
	Use:   "sample-search",
	Short: "Search community-contributed Graph API query samples",
	Long: `Search a curated library of community-contributed Microsoft Graph API
query samples to find the right API call for a given task.

These samples map natural-language intents (e.g. "list all Conditional Access
policies") to exact Graph API queries, including multi-step workflows.

Examples:
  msgraph sample-search --query "conditional access policies"
  msgraph sample-search --query "send email"
  msgraph sample-search --product entra
  msgraph sample-search --query "managed devices" --product intune`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetString("query")
		product, _ := cmd.Flags().GetString("product")
		limit, _ := cmd.Flags().GetInt("limit")

		if query == "" && product == "" {
			return fmt.Errorf("at least one of --query or --product is required")
		}

		// Try FTS database first, fall back to JSON index.
		if dbPath := findFile("samples-index.db", "MSGRAPH_SAMPLES_DB_PATH"); dbPath != "" {
			return searchSamplesFTS(dbPath, query, product, limit)
		}

		// Fall back to legacy JSON search.
		indexPath := findFile("samples-index.json", "MSGRAPH_SAMPLES_PATH")
		if indexPath == "" {
			return fmt.Errorf("samples index file not found. Set MSGRAPH_SAMPLES_PATH or ensure references/samples-index.json exists relative to the binary")
		}

		idx, err := samples.LoadIndex(indexPath)
		if err != nil {
			return err
		}

		results := idx.Search(query, product, limit)

		if len(results) == 0 {
			return outputJSON(map[string]interface{}{
				"results": []interface{}{},
				"message": "No matching samples found. Try broadening your search or use openapi-search for endpoint discovery.",
			})
		}

		return outputJSON(map[string]interface{}{
			"count":   len(results),
			"results": results,
		})
	},
}

func searchSamplesFTS(dbPath, query, product string, limit int) error {
	idx, err := samples.LoadFTSIndex(dbPath)
	if err != nil {
		return err
	}
	defer idx.Close()

	results := idx.Search(query, product, limit)
	return outputJSON(samples.FormatFTSResults(results))
}

func init() {
	sampleSearchCmd.Flags().String("query", "", "Free-text search query (searches intent and query fields)")
	sampleSearchCmd.Flags().String("product", "", "Filter by product (entra, intune, exchange, teams, sharepoint, security, general)")
	sampleSearchCmd.Flags().Int("limit", 10, "Maximum number of results to return")

	rootCmd.AddCommand(sampleSearchCmd)
}
