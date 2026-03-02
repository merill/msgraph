// Package auth provides MSAL-based authentication for Microsoft Graph.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/merill/msgraph-skill/internal/config"
)

// SessionData stores the current auth session state.
type SessionData struct {
	Account    public.Account `json:"account"`
	TenantID   string         `json:"tenantId"`
	ClientID   string         `json:"clientId"`
	Scopes     []string       `json:"scopes"`
}

// Client wraps the MSAL public client application with session management.
type Client struct {
	app     public.Client
	cfg     *config.Config
	session *SessionData
	cachePath string
}

// NewClient creates a new MSAL auth client.
func NewClient(cfg *config.Config) (*Client, error) {
	cachePath := sessionCachePath(cfg.ClientID, cfg.TenantID)

	app, err := public.New(cfg.ClientID,
		public.WithAuthority(cfg.AuthorityURL()),
		public.WithCache(&tokenCache{path: cachePath}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create MSAL client: %w", err)
	}

	c := &Client{
		app:       app,
		cfg:       cfg,
		cachePath: cachePath,
	}

	return c, nil
}

// AcquireTokenSilent attempts to get a token from the cache without user interaction.
func (c *Client) AcquireTokenSilent(ctx context.Context, scopes []string) (string, error) {
	accounts, err := c.app.Accounts(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get accounts: %w", err)
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no accounts in cache, sign-in required")
	}

	result, err := c.app.AcquireTokenSilent(ctx, scopes, public.WithSilentAccount(accounts[0]))
	if err != nil {
		return "", fmt.Errorf("silent token acquisition failed: %w", err)
	}

	return result.AccessToken, nil
}

// AcquireTokenInteractive opens the system browser for authentication.
func (c *Client) AcquireTokenInteractive(ctx context.Context, scopes []string) (string, error) {
	result, err := c.app.AcquireTokenInteractive(ctx, scopes,
		public.WithRedirectURI(config.RedirectURL),
	)
	if err != nil {
		return "", fmt.Errorf("interactive auth failed: %w", err)
	}

	c.session = &SessionData{
		Account:  result.Account,
		TenantID: c.cfg.TenantID,
		ClientID: c.cfg.ClientID,
		Scopes:   scopes,
	}

	return result.AccessToken, nil
}

// AcquireTokenDeviceCode uses the device code flow for authentication.
func (c *Client) AcquireTokenDeviceCode(ctx context.Context, scopes []string) (string, error) {
	dc, err := c.app.AcquireTokenByDeviceCode(ctx, scopes)
	if err != nil {
		return "", fmt.Errorf("device code auth failed: %w", err)
	}

	// Print the device code message for the user
	fmt.Fprintln(os.Stderr, dc.Result.Message)

	// Wait for the user to authenticate
	result, err := dc.AuthenticationResult(ctx)
	if err != nil {
		return "", fmt.Errorf("device code authentication failed: %w", err)
	}

	c.session = &SessionData{
		Account:  result.Account,
		TenantID: c.cfg.TenantID,
		ClientID: c.cfg.ClientID,
		Scopes:   scopes,
	}

	return result.AccessToken, nil
}

// AcquireToken attempts silent auth first, then falls back to interactive/device code.
func (c *Client) AcquireToken(ctx context.Context, scopes []string, forceDeviceCode bool) (string, error) {
	// Try silent first
	token, err := c.AcquireTokenSilent(ctx, scopes)
	if err == nil {
		return token, nil
	}

	// Fall back to interactive or device code
	if forceDeviceCode || !isBrowserAvailable() {
		return c.AcquireTokenDeviceCode(ctx, scopes)
	}
	return c.AcquireTokenInteractive(ctx, scopes)
}

// AcquireTokenWithExtraScopes re-authenticates with additional scopes for incremental consent.
func (c *Client) AcquireTokenWithExtraScopes(ctx context.Context, existingScopes, extraScopes []string, forceDeviceCode bool) (string, error) {
	// Merge scopes
	scopeSet := make(map[string]bool)
	for _, s := range existingScopes {
		scopeSet[s] = true
	}
	for _, s := range extraScopes {
		scopeSet[s] = true
	}
	merged := make([]string, 0, len(scopeSet))
	for s := range scopeSet {
		merged = append(merged, s)
	}

	// Must do interactive auth for consent
	if forceDeviceCode || !isBrowserAvailable() {
		return c.AcquireTokenDeviceCode(ctx, merged)
	}
	return c.AcquireTokenInteractive(ctx, merged)
}

// SignOut clears the session cache.
func (c *Client) SignOut() error {
	if err := os.Remove(c.cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session cache: %w", err)
	}
	c.session = nil
	return nil
}

// Status returns information about the current auth state.
func (c *Client) Status(ctx context.Context) (map[string]interface{}, error) {
	accounts, err := c.app.Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return map[string]interface{}{
			"signedIn": false,
			"message":  "Not signed in. Run 'auth signin' to authenticate.",
		}, nil
	}

	acct := accounts[0]
	info := map[string]interface{}{
		"signedIn":    true,
		"username":    acct.PreferredUsername,
		"tenantId":    c.cfg.TenantID,
		"clientId":    c.cfg.ClientID,
		"environment": acct.Environment,
	}

	return info, nil
}

// GetAccounts returns the accounts currently in the cache.
func (c *Client) GetAccounts(ctx context.Context) ([]public.Account, error) {
	return c.app.Accounts(ctx)
}

// sessionCachePath returns the path to the session-scoped token cache file.
func sessionCachePath(clientID, tenantID string) string {
	h := sha256.Sum256([]byte(clientID + ":" + tenantID))
	filename := fmt.Sprintf("msgraph-skill-session-%x.json", h[:8])
	return filepath.Join(os.TempDir(), filename)
}

// isBrowserAvailable does a basic check to see if we can open a browser.
func isBrowserAvailable() bool {
	// Check for common indicators that a browser is NOT available:
	// - SSH_CONNECTION or SSH_CLIENT set (remote session)
	// - DISPLAY not set on Linux (no X server)
	// - TERM_PROGRAM not set and no DISPLAY
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return false
	}
	// On Linux, check for display server
	if goos := os.Getenv("GOOS"); goos == "linux" || goos == "" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			// Could be Linux without a display, but we might be on macOS/Windows
			// where these aren't relevant. Check runtime OS.
			if _, err := os.Stat("/proc/version"); err == nil {
				// We're on Linux
				return false
			}
		}
	}
	return true
}

// tokenCache implements the MSAL cache.ExportReplace interface for file-based caching.
type tokenCache struct {
	path string
}

func (t *tokenCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No cache file yet, that's fine
		}
		return err
	}
	return cache.Unmarshal(data)
}

func (t *tokenCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	data, err := cache.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(t.path, data, 0600)
}

// StatusJSON returns the status as JSON bytes.
func StatusJSON(ctx context.Context, client *Client) ([]byte, error) {
	status, err := client.Status(ctx)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(status, "", "  ")
}
