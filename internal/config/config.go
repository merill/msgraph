// Package config provides configuration management for the msgraph CLI.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Version is set at build time via ldflags.
var Version = "dev"

// AuthMethod represents the authentication method to use.
type AuthMethod string

const (
	// AuthMethodDelegated uses interactive browser or device code flow (default).
	AuthMethodDelegated AuthMethod = "delegated"

	// AuthMethodClientSecret uses a client secret for app-only auth.
	AuthMethodClientSecret AuthMethod = "client-secret"

	// AuthMethodCertificate uses a client certificate for app-only auth.
	AuthMethodCertificate AuthMethod = "certificate"

	// AuthMethodManagedIdentity uses Azure managed identity for app-only auth.
	AuthMethodManagedIdentity AuthMethod = "managed-identity"

	// AuthMethodWorkloadIdentity uses federated token assertion for app-only auth.
	AuthMethodWorkloadIdentity AuthMethod = "workload-identity"
)

const (
	// DefaultClientID is the Microsoft Graph Command Line Tools app ID.
	// This is a first-party Microsoft app pre-registered in most M365 tenants.
	DefaultClientID = "14d82eec-204b-4c2f-b7e8-296a70dab67e"

	// DefaultTenantID uses the "common" endpoint for multi-tenant sign-in.
	DefaultTenantID = "common"

	// DefaultAPIVersion is the default Graph API version.
	DefaultAPIVersion = "beta"

	// DefaultAuthority is the base URL for the Microsoft identity platform.
	DefaultAuthority = "https://login.microsoftonline.com/"

	// GraphBaseURL is the base URL for Microsoft Graph API.
	GraphBaseURL = "https://graph.microsoft.com"

	// DefaultScopes are the minimum scopes requested at sign-in.
	DefaultScope = "User.Read"

	// GraphDefaultScope is the scope used for app-only auth (all pre-granted permissions).
	GraphDefaultScope = "https://graph.microsoft.com/.default"

	// GraphResource is the resource identifier for managed identity token acquisition.
	GraphResource = "https://graph.microsoft.com"

	// RedirectURL is the localhost redirect used for interactive browser auth.
	RedirectURL = "http://localhost"
)

// Config holds the runtime configuration.
type Config struct {
	ClientID   string
	TenantID   string
	APIVersion string
	Authority  string

	// App-only auth fields
	AuthMethod                AuthMethod
	ClientSecret              string
	ClientCertificatePath     string
	ClientCertificatePassword string
	ManagedIdentityClientID   string
	FederatedTokenFile        string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	cfg := &Config{
		ClientID:                  envOrDefault("MSGRAPH_CLIENT_ID", DefaultClientID),
		TenantID:                  envOrDefault("MSGRAPH_TENANT_ID", DefaultTenantID),
		APIVersion:                envOrDefault("MSGRAPH_API_VERSION", DefaultAPIVersion),
		Authority:                 DefaultAuthority,
		ClientSecret:              os.Getenv("MSGRAPH_CLIENT_SECRET"),
		ClientCertificatePath:     os.Getenv("MSGRAPH_CLIENT_CERTIFICATE_PATH"),
		ClientCertificatePassword: os.Getenv("MSGRAPH_CLIENT_CERTIFICATE_PASSWORD"),
		ManagedIdentityClientID:   os.Getenv("MSGRAPH_MANAGED_IDENTITY_CLIENT_ID"),
	}

	// Resolve federated token file from multiple env vars
	cfg.FederatedTokenFile = firstNonEmpty(
		os.Getenv("MSGRAPH_FEDERATED_TOKEN_FILE"),
		os.Getenv("AZURE_FEDERATED_TOKEN_FILE"),
		os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"),
	)

	// Auto-detect auth method from env vars
	cfg.AuthMethod = cfg.detectAuthMethod()

	return cfg
}

// detectAuthMethod determines the auth method from environment variables.
// Priority: client-secret > certificate > workload-identity > managed-identity > delegated
func (c *Config) detectAuthMethod() AuthMethod {
	if c.ClientSecret != "" {
		return AuthMethodClientSecret
	}
	if c.ClientCertificatePath != "" {
		return AuthMethodCertificate
	}
	if c.FederatedTokenFile != "" {
		return AuthMethodWorkloadIdentity
	}
	if strings.EqualFold(os.Getenv("MSGRAPH_AUTH_METHOD"), "managed-identity") {
		return AuthMethodManagedIdentity
	}
	return AuthMethodDelegated
}

// IsAppOnly returns true if the configured auth method is application-only (no user context).
func (c *Config) IsAppOnly() bool {
	return c.AuthMethod != AuthMethodDelegated
}

// ValidateForAppOnly checks that the configuration is valid for app-only auth.
// App-only auth requires a specific tenant ID — "common" won't work.
func (c *Config) ValidateForAppOnly() error {
	if !c.IsAppOnly() {
		return nil
	}
	if c.TenantID == "" || c.TenantID == "common" || c.TenantID == "organizations" || c.TenantID == "consumers" {
		return fmt.Errorf(
			"app-only auth (%s) requires a specific tenant ID. Set MSGRAPH_TENANT_ID to your Entra ID tenant ID or domain (e.g., contoso.onmicrosoft.com)",
			c.AuthMethod,
		)
	}
	return nil
}

// AuthorityURL returns the full authority URL for the configured tenant.
func (c *Config) AuthorityURL() string {
	return c.Authority + c.TenantID
}

// GraphURL returns the full Graph API base URL for the given API version.
// If apiVersion is empty, the configured default is used.
func (c *Config) GraphURL(apiVersion string) string {
	v := c.APIVersion
	if apiVersion != "" {
		v = apiVersion
	}
	return GraphBaseURL + "/" + v
}

// ValidAPIVersion checks if the given version string is valid.
func ValidAPIVersion(v string) bool {
	v = strings.ToLower(v)
	return v == "v1.0" || v == "beta"
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
