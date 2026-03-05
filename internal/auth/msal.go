package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/merill/msgraph/internal/config"
	"github.com/zalando/go-keyring"
)

// DelegatedClient implements TokenProvider using MSAL public client (interactive + device code).
// This is the default auth method for user-delegated authentication.
type DelegatedClient struct {
	app           public.Client
	cfg           *config.Config
	session       *SessionData
	cacheKey      string
	workspaceRoot string
}

// SessionData stores the current auth session state.
type SessionData struct {
	Account  public.Account `json:"account"`
	TenantID string         `json:"tenantId"`
	ClientID string         `json:"clientId"`
	Scopes   []string       `json:"scopes"`
}

// NewDelegatedClient creates a new MSAL public client for delegated auth.
func NewDelegatedClient(cfg *config.Config) (*DelegatedClient, error) {
	workspaceRoot, cacheKey := sessionCacheKey(cfg.ClientID, cfg.TenantID, cfg.WorkspaceRoot)
	cacheOpt := public.WithCache(&tokenCache{service: tokenCacheService, key: cacheKey})
	if cfg.NoTokenCache {
		cacheOpt = nil
	}

	options := []public.Option{public.WithAuthority(cfg.AuthorityURL())}
	if cacheOpt != nil {
		options = append(options, cacheOpt)
	}

	app, err := public.New(cfg.ClientID, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create MSAL client: %w", err)
	}

	return &DelegatedClient{
		app:           app,
		cfg:           cfg,
		cacheKey:      cacheKey,
		workspaceRoot: workspaceRoot,
	}, nil
}

// AcquireToken attempts silent auth first, then falls back to interactive/device code.
func (c *DelegatedClient) AcquireToken(ctx context.Context, scopes []string) (string, error) {
	// Try silent first
	token, err := c.AcquireTokenSilent(ctx, scopes)
	if err == nil {
		return token, nil
	}

	// Fall back to interactive or device code
	if !isBrowserAvailable() {
		return c.AcquireTokenDeviceCode(ctx, scopes)
	}
	return c.AcquireTokenInteractive(ctx, scopes)
}

// AcquireTokenWithExtraScopes re-authenticates with additional scopes for incremental consent.
func (c *DelegatedClient) AcquireTokenWithExtraScopes(ctx context.Context, existingScopes, extraScopes []string) (string, error) {
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
	if !isBrowserAvailable() {
		return c.AcquireTokenDeviceCode(ctx, merged)
	}
	return c.AcquireTokenInteractive(ctx, merged)
}

// AcquireTokenSilent attempts to get a token from the cache without user interaction.
func (c *DelegatedClient) AcquireTokenSilent(ctx context.Context, scopes []string) (string, error) {
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
func (c *DelegatedClient) AcquireTokenInteractive(ctx context.Context, scopes []string) (string, error) {
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
func (c *DelegatedClient) AcquireTokenDeviceCode(ctx context.Context, scopes []string) (string, error) {
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

// SignOut clears the session cache.
func (c *DelegatedClient) SignOut() error {
	if c.cfg.NoTokenCache {
		c.session = nil
		return nil
	}
	if err := keyring.Delete(tokenCacheService, c.cacheKey); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("failed to remove session cache: %w", err)
	}
	if err := os.Remove(cacheFilePath(c.cacheKey)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}
	c.session = nil
	return nil
}

// Status returns information about the current auth state.
func (c *DelegatedClient) Status(ctx context.Context) (map[string]interface{}, error) {
	accounts, err := c.app.Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return map[string]interface{}{
			"signedIn":   false,
			"authMethod": string(config.AuthMethodDelegated),
			"message":    "Not signed in. Run 'auth signin' to authenticate.",
		}, nil
	}

	acct := accounts[0]
	return map[string]interface{}{
		"signedIn":      true,
		"authMethod":    string(config.AuthMethodDelegated),
		"username":      acct.PreferredUsername,
		"tenantId":      c.cfg.TenantID,
		"clientId":      c.cfg.ClientID,
		"environment":   acct.Environment,
		"workspaceRoot": c.workspaceRoot,
		"cacheKey":      c.cacheKey,
	}, nil
}

// IsAppOnly returns false — delegated auth has a user context.
func (c *DelegatedClient) IsAppOnly() bool {
	return false
}

// GetAccounts returns the accounts currently in the cache.
func (c *DelegatedClient) GetAccounts(ctx context.Context) ([]public.Account, error) {
	return c.app.Accounts(ctx)
}

// StatusJSON returns the status as JSON bytes.
func StatusJSON(ctx context.Context, provider TokenProvider) ([]byte, error) {
	status, err := provider.Status(ctx)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(status, "", "  ")
}

// sessionCacheKey returns the per-session cache key used for secure storage.
// It scopes the cache by client, tenant, and current working directory so
// different folders do not share the same session.
func sessionCacheKey(clientID, tenantID, workspaceRoot string) (string, string) {
	cwd := workspaceRoot
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = ""
		} else {
			cwd = filepath.Clean(cwd)
		}
	} else {
		cwd = filepath.Clean(cwd)
	}

	h := sha256.Sum256([]byte(clientID + ":" + tenantID + ":" + cwd))
	key := fmt.Sprintf("msgraph-%x", h[:16])

	if debugEnabled() {
		fmt.Fprintf(os.Stderr, "[msgraph] workspace: %s\n", cwd)
		fmt.Fprintf(os.Stderr, "[msgraph] cache-key: %s\n", key)
	}

	return cwd, key
}

func debugEnabled() bool {
	v := strings.ToLower(os.Getenv("MSGRAPH_DEBUG"))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// isBrowserAvailable does a basic check to see if we can open a browser.
func isBrowserAvailable() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return false
	}
	if goos := os.Getenv("GOOS"); goos == "linux" || goos == "" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			if _, err := os.Stat("/proc/version"); err == nil {
				return false
			}
		}
	}
	return true
}

const tokenCacheService = "msgraph"

// tokenCache implements the MSAL cache.ExportReplace interface with
// encryption-at-rest and a key stored in the OS keyring.
type tokenCache struct {
	service string
	key     string
}

func (t *tokenCache) Replace(ctx context.Context, cache cache.Unmarshaler, hints cache.ReplaceHints) error {
	k, err := loadOrCreateEncryptionKey(t.service, t.key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return err
	}

	encData, err := os.ReadFile(cacheFilePath(t.key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	plain, err := decrypt(k, encData)
	if err != nil {
		return err
	}

	return cache.Unmarshal(plain)
}

func (t *tokenCache) Export(ctx context.Context, cache cache.Marshaler, hints cache.ExportHints) error {
	if t == nil {
		return nil
	}

	plaintext, err := cache.Marshal()
	if err != nil {
		return err
	}

	k, err := loadOrCreateEncryptionKey(t.service, t.key)
	if err != nil {
		// If keyring rejects (e.g., too big), skip persistence silently
		if isKeyringDataTooBig(err) || errors.Is(err, keyring.ErrSetDataTooBig) {
			return nil
		}
		return err
	}

	enc, err := encrypt(k, plaintext)
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFilePath(t.key), enc, 0600)
}

func isKeyringDataTooBig(err error) bool {
	return errors.Is(err, keyring.ErrSetDataTooBig) || strings.Contains(strings.ToLower(err.Error()), "too big")
}

func loadOrCreateEncryptionKey(service, key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("empty cache key")
	}

	if v, err := keyring.Get(service, key); err == nil {
		b, err := base64.StdEncoding.DecodeString(v)
		if err == nil && len(b) == 32 {
			return b, nil
		}
	}

	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(k)
	if err := keyring.Set(service, key, encoded); err != nil {
		return nil, err
	}

	return k, nil
}

func cacheFilePath(key string) string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "msgraph")
	_ = os.MkdirAll(dir, 0700)
	filename := fmt.Sprintf("msgraph-cache-%s.bin", key)
	return filepath.Join(dir, filename)
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, data := ciphertext[:ns], ciphertext[ns:]
	return gcm.Open(nil, nonce, data, nil)
}
