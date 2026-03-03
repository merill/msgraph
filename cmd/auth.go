package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/merill/msgraph/internal/auth"
	"github.com/merill/msgraph/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  "Sign in, sign out, check status, or switch tenants for Microsoft Graph API access.",
}

var signinCmd = &cobra.Command{
	Use:   "signin",
	Short: "Sign in to Microsoft 365",
	Long:  "Authenticate to a Microsoft 365 tenant. For delegated auth, uses interactive browser or device code flow. For app-only auth (client secret, certificate, managed identity, workload identity), verifies that credentials are valid.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		deviceCode, _ := cmd.Flags().GetBool("device-code")
		scopes, _ := cmd.Flags().GetStringSlice("scopes")
		if len(scopes) == 0 {
			scopes = []string{config.DefaultScope}
		}

		provider, err := auth.NewTokenProvider(cfg)
		if err != nil {
			return err
		}

		var token string

		// For delegated auth, support --device-code flag
		if dc, ok := provider.(*auth.DelegatedClient); ok && deviceCode {
			token, err = dc.AcquireTokenDeviceCode(ctx, scopes)
		} else {
			token, err = provider.AcquireToken(ctx, scopes)
		}
		if err != nil {
			return err
		}

		// Output success status as JSON
		status, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		status["message"] = "Successfully signed in"
		_ = token // token is used internally

		return outputJSON(status)
	},
}

var signoutCmd = &cobra.Command{
	Use:   "signout",
	Short: "Sign out of Microsoft 365",
	Long:  "Clear the current authentication session and token cache.",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider, err := auth.NewTokenProvider(cfg)
		if err != nil {
			return err
		}

		if err := provider.SignOut(); err != nil {
			return err
		}

		return outputJSON(map[string]interface{}{
			"signedIn": false,
			"message":  "Successfully signed out",
		})
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  "Display the current sign-in state, including the signed-in user/app and tenant.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		provider, err := auth.NewTokenProvider(cfg)
		if err != nil {
			return err
		}

		status, err := provider.Status(ctx)
		if err != nil {
			return err
		}

		return outputJSON(status)
	},
}

var switchTenantCmd = &cobra.Command{
	Use:   "switch-tenant [tenant-id]",
	Short: "Switch to a different tenant",
	Long:  "Sign out of the current tenant and sign in to a new one.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		tenantID := args[0]

		// Sign out of current session
		provider, err := auth.NewTokenProvider(cfg)
		if err != nil {
			return err
		}
		_ = provider.SignOut()

		// Update config with new tenant and sign in
		cfg.TenantID = tenantID
		provider, err = auth.NewTokenProvider(cfg)
		if err != nil {
			return err
		}

		deviceCode, _ := cmd.Flags().GetBool("device-code")
		scopes, _ := cmd.Flags().GetStringSlice("scopes")
		if len(scopes) == 0 {
			scopes = []string{config.DefaultScope}
		}

		var token string
		if dc, ok := provider.(*auth.DelegatedClient); ok && deviceCode {
			token, err = dc.AcquireTokenDeviceCode(ctx, scopes)
		} else {
			token, err = provider.AcquireToken(ctx, scopes)
		}
		if err != nil {
			return err
		}
		_ = token

		status, err := provider.Status(ctx)
		if err != nil {
			return err
		}
		status["message"] = fmt.Sprintf("Switched to tenant %s", tenantID)

		return outputJSON(status)
	},
}

func init() {
	// Add flags
	signinCmd.Flags().Bool("device-code", false, "Use device code flow instead of browser (delegated auth only)")
	signinCmd.Flags().StringSlice("scopes", nil, "Permission scopes to request (default: User.Read, delegated auth only)")

	switchTenantCmd.Flags().Bool("device-code", false, "Use device code flow instead of browser (delegated auth only)")
	switchTenantCmd.Flags().StringSlice("scopes", nil, "Permission scopes to request (default: User.Read, delegated auth only)")

	// Build command tree
	authCmd.AddCommand(signinCmd, signoutCmd, statusCmd, switchTenantCmd)
	rootCmd.AddCommand(authCmd)
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
