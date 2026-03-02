// Package cmd implements the CLI commands for msgraph-skill.
package cmd

import (
	"github.com/merill/msgraph-skill/internal/config"
	"github.com/spf13/cobra"
)

var cfg *config.Config

// rootCmd is the base command.
var rootCmd = &cobra.Command{
	Use:   "msgraph-skill",
	Short: "Microsoft Graph API agent skill",
	Long: `msgraph-skill is a CLI tool that enables AI agents to authenticate 
to Microsoft 365 tenants and execute Microsoft Graph API calls.

It supports delegated authentication via MSAL with interactive browser 
and device code flows, automatic incremental consent on 403 errors, 
and write safety enforcement.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	cfg = config.Load()
	return rootCmd.Execute()
}
