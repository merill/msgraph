package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/managedidentity"
	"github.com/merill/msgraph/internal/config"
)

// ManagedIdentityClient implements TokenProvider using Azure managed identity.
// This works on Azure VMs, App Service, Azure Functions, AKS, and other Azure
// compute resources with a managed identity assigned.
type ManagedIdentityClient struct {
	app managedidentity.Client
	cfg *config.Config
}

// NewManagedIdentityClient creates a managed identity client.
// If MSGRAPH_MANAGED_IDENTITY_CLIENT_ID is set, uses a user-assigned identity.
// Otherwise, uses the system-assigned identity.
func NewManagedIdentityClient(cfg *config.Config) (*ManagedIdentityClient, error) {
	var id managedidentity.ID
	if cfg.ManagedIdentityClientID != "" {
		id = managedidentity.UserAssignedClientID(cfg.ManagedIdentityClientID)
	} else {
		id = managedidentity.SystemAssigned()
	}

	app, err := managedidentity.New(id)
	if err != nil {
		return nil, fmt.Errorf("failed to create managed identity client: %w", err)
	}

	return &ManagedIdentityClient{app: app, cfg: cfg}, nil
}

// AcquireToken acquires a token using managed identity.
func (c *ManagedIdentityClient) AcquireToken(ctx context.Context, _ []string) (string, error) {
	result, err := c.app.AcquireToken(ctx, config.GraphResource)
	if err != nil {
		return "", fmt.Errorf("managed identity auth failed: %w", err)
	}
	return result.AccessToken, nil
}

// AcquireTokenWithExtraScopes is not supported for app-only auth.
func (c *ManagedIdentityClient) AcquireTokenWithExtraScopes(_ context.Context, _, _ []string) (string, error) {
	return "", ErrIncrementalConsentNotSupported
}

// SignOut is a no-op for managed identity (no session to clear).
func (c *ManagedIdentityClient) SignOut() error {
	return nil
}

// Status returns the current auth state.
func (c *ManagedIdentityClient) Status(ctx context.Context) (map[string]interface{}, error) {
	_, err := c.AcquireToken(ctx, nil)
	status := map[string]interface{}{
		"authMethod": string(config.AuthMethodManagedIdentity),
	}
	if c.cfg.ManagedIdentityClientID != "" {
		status["managedIdentityClientId"] = c.cfg.ManagedIdentityClientID
		status["identityType"] = "user-assigned"
	} else {
		status["identityType"] = "system-assigned"
	}

	if err != nil {
		status["signedIn"] = false
		status["message"] = fmt.Sprintf("Managed identity auth failed: %v", err)

		// Provide a hint if not running on Azure
		if os.Getenv("IDENTITY_ENDPOINT") == "" {
			status["hint"] = "Managed identity is only available on Azure compute resources (VMs, App Service, Functions, AKS, etc.)"
		}
	} else {
		status["signedIn"] = true
	}

	return status, nil
}

// IsAppOnly returns true.
func (c *ManagedIdentityClient) IsAppOnly() bool {
	return true
}
