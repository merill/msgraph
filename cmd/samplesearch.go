package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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

		indexPath := findSamplesIndexPath()
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

func init() {
	sampleSearchCmd.Flags().String("query", "", "Free-text search query (searches intent and query fields)")
	sampleSearchCmd.Flags().String("product", "", "Filter by product (entra, intune, exchange, teams, sharepoint, security, general)")
	sampleSearchCmd.Flags().Int("limit", 10, "Maximum number of results to return")

	rootCmd.AddCommand(sampleSearchCmd)
}

// findSamplesIndexPath locates the samples index JSON file.
func findSamplesIndexPath() string {
	candidates := []string{
		// When running from the repo root
		"skills/msgraph/references/samples-index.json",
	}

	// Also check relative to the executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			// Binary at msgraph/scripts/bin/ → index at msgraph/references/
			filepath.Join(exeDir, "..", "..", "references", "samples-index.json"),
			// Binary next to msgraph/
			filepath.Join(exeDir, "..", "msgraph", "references", "samples-index.json"),
			filepath.Join(exeDir, "msgraph", "references", "samples-index.json"),
		)
	}

	// Check environment variable override
	if envPath := os.Getenv("MSGRAPH_SAMPLES_PATH"); envPath != "" {
		candidates = append([]string{envPath}, candidates...)
	}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
