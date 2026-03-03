// Package config provides configuration management for the msgraph CLI.
package config

import (
	"os"
	"strings"
)

// Version is set at build time via ldflags.
var Version = "dev"

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

	// RedirectURL is the localhost redirect used for interactive browser auth.
	RedirectURL = "http://localhost"
)

// Config holds the runtime configuration.
type Config struct {
	ClientID   string
	TenantID   string
	APIVersion string
	Authority  string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ClientID:   envOrDefault("MSGRAPH_CLIENT_ID", DefaultClientID),
		TenantID:   envOrDefault("MSGRAPH_TENANT_ID", DefaultTenantID),
		APIVersion: envOrDefault("MSGRAPH_API_VERSION", DefaultAPIVersion),
		Authority:  DefaultAuthority,
	}
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
