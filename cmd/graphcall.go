package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/merill/msgraph/internal/auth"
	"github.com/merill/msgraph/internal/graph"
	"github.com/spf13/cobra"
)

var graphCallCmd = &cobra.Command{
	Use:   "graph-call <METHOD> <URL>",
	Short: "Execute a Microsoft Graph API call",
	Long: `Execute an HTTP request against the Microsoft Graph API.

The URL can be a full URL or a relative path starting with /.
For example: /me, /users, /me/messages

By default, only GET requests are allowed. Use --allow-writes to enable
POST, PUT, and PATCH requests. DELETE is always blocked for safety.

Examples:
  msgraph graph-call GET /me
  msgraph graph-call GET /me/messages --select "subject,from" --top 10
  msgraph graph-call POST /me/messages --body '{"subject":"Hello"}' --allow-writes`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		method := strings.ToUpper(args[0])
		url := args[1]

		allowWrites, _ := cmd.Flags().GetBool("allow-writes")
		body, _ := cmd.Flags().GetString("body")
		headerStrs, _ := cmd.Flags().GetStringSlice("headers")
		apiVersion, _ := cmd.Flags().GetString("api-version")
		selectParam, _ := cmd.Flags().GetString("select")
		filterParam, _ := cmd.Flags().GetString("filter")
		topParam, _ := cmd.Flags().GetInt("top")
		expandParam, _ := cmd.Flags().GetString("expand")
		orderbyParam, _ := cmd.Flags().GetString("orderby")
		scopeStrs, _ := cmd.Flags().GetStringSlice("scopes")
		outputFmt, _ := cmd.Flags().GetString("output")

		// Parse headers
		headers := make(map[string]string)
		for _, h := range headerStrs {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}

		// Validate API version if specified
		if apiVersion != "" {
			lower := strings.ToLower(apiVersion)
			if lower != "v1.0" && lower != "beta" {
				return fmt.Errorf("invalid api-version %q: must be 'v1.0' or 'beta'", apiVersion)
			}
		}

		authClient, err := auth.NewClient(cfg)
		if err != nil {
			return err
		}

		graphClient := graph.NewClient(authClient, cfg, allowWrites)

		resp, err := graphClient.Call(ctx, graph.CallOptions{
			Method:     method,
			URL:        url,
			Body:       body,
			Headers:    headers,
			APIVersion: apiVersion,
			Select:     selectParam,
			Filter:     filterParam,
			Top:        topParam,
			Expand:     expandParam,
			OrderBy:    orderbyParam,
			Scopes:     scopeStrs,
		})
		if err != nil {
			return err
		}

		return outputResponse(resp, outputFmt)
	},
}

func init() {
	graphCallCmd.Flags().Bool("allow-writes", false, "Allow POST, PUT, PATCH requests (agent must confirm with user)")
	graphCallCmd.Flags().String("body", "", "Request body (JSON)")
	graphCallCmd.Flags().StringSlice("headers", nil, "Custom headers (key:value)")
	graphCallCmd.Flags().String("api-version", "", "API version: 'v1.0' or 'beta' (default: beta)")
	graphCallCmd.Flags().String("select", "", "OData $select query parameter")
	graphCallCmd.Flags().String("filter", "", "OData $filter query parameter")
	graphCallCmd.Flags().Int("top", 0, "OData $top query parameter")
	graphCallCmd.Flags().String("expand", "", "OData $expand query parameter")
	graphCallCmd.Flags().String("orderby", "", "OData $orderby query parameter")
	graphCallCmd.Flags().StringSlice("scopes", nil, "Additional permission scopes to request")
	graphCallCmd.Flags().String("output", "json", "Output format: 'json' or 'raw'")

	rootCmd.AddCommand(graphCallCmd)
}

func outputResponse(resp *graph.Response, format string) error {
	switch format {
	case "raw":
		fmt.Fprintln(os.Stdout, resp.Body)
		return nil
	default:
		// Try to pretty-print the body if it's valid JSON
		var bodyObj interface{}
		if err := json.Unmarshal([]byte(resp.Body), &bodyObj); err == nil {
			output := map[string]interface{}{
				"statusCode": resp.StatusCode,
				"body":       bodyObj,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(output)
		}

		// Not JSON, output as-is with status code
		output := map[string]interface{}{
			"statusCode": resp.StatusCode,
			"body":       resp.Body,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}
}
