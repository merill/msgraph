// Package auth provides authentication for Microsoft Graph using MSAL.
package auth

import (
	"context"
	"fmt"

	"github.com/merill/msgraph/internal/config"
)

// TokenProvider is the interface for all auth methods. The Graph HTTP client
// uses this interface to acquire tokens without knowing the underlying method.
type TokenProvider interface {
	// AcquireToken gets an access token for the given scopes.
	// For app-only auth, individual scopes are ignored and .default is used.
	AcquireToken(ctx context.Context, scopes []string) (string, error)

	// AcquireTokenWithExtraScopes re-acquires a token with additional scopes
	// for incremental consent. Returns ErrIncrementalConsentNotSupported for
	// app-only auth methods where permissions are pre-granted by admin.
	AcquireTokenWithExtraScopes(ctx context.Context, existingScopes, extraScopes []string) (string, error)

	// SignOut clears any cached credentials/sessions.
	SignOut() error

	// Status returns information about the current auth state.
	Status(ctx context.Context) (map[string]interface{}, error)

	// IsAppOnly returns true for app-only auth methods (no user context).
	IsAppOnly() bool
}

// ErrIncrementalConsentNotSupported is returned when incremental consent is
// attempted with an app-only auth method.
var ErrIncrementalConsentNotSupported = fmt.Errorf("incremental consent is not supported for app-only authentication; grant the required permissions in the Entra ID portal")

// NewTokenProvider creates the appropriate TokenProvider based on the config.
// It auto-detects the auth method from environment variables.
func NewTokenProvider(cfg *config.Config) (TokenProvider, error) {
	// Validate app-only config (tenant ID must be specific)
	if err := cfg.ValidateForAppOnly(); err != nil {
		return nil, err
	}

	switch cfg.AuthMethod {
	case config.AuthMethodClientSecret:
		return NewClientSecretClient(cfg)

	case config.AuthMethodCertificate:
		return NewClientCertificateClient(cfg)

	case config.AuthMethodManagedIdentity:
		return NewManagedIdentityClient(cfg)

	case config.AuthMethodWorkloadIdentity:
		return NewWorkloadIdentityClient(cfg)

	default:
		return NewDelegatedClient(cfg)
	}
}
