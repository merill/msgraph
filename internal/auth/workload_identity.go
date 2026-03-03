package auth

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/merill/msgraph/internal/config"
)

// WorkloadIdentityClient implements TokenProvider using federated token assertion.
// This supports workload identity federation from Azure Kubernetes Service,
// AWS EKS, GCP GKE, and other environments that provide a JWT token file.
//
// The token file is re-read on each acquisition to pick up rotated tokens.
type WorkloadIdentityClient struct {
	app       confidential.Client
	cfg       *config.Config
	tokenFile string
}

// NewWorkloadIdentityClient creates a confidential client using a federated token assertion.
func NewWorkloadIdentityClient(cfg *config.Config) (*WorkloadIdentityClient, error) {
	tokenFile := cfg.FederatedTokenFile
	if tokenFile == "" {
		return nil, fmt.Errorf("federated token file not configured; set MSGRAPH_FEDERATED_TOKEN_FILE, AZURE_FEDERATED_TOKEN_FILE, or AWS_WEB_IDENTITY_TOKEN_FILE")
	}

	// Verify the token file exists
	if _, err := os.Stat(tokenFile); err != nil {
		return nil, fmt.Errorf("federated token file not found: %w", err)
	}

	// Use ClientID from config; also check AZURE_CLIENT_ID as fallback
	clientID := cfg.ClientID
	if clientID == config.DefaultClientID {
		if azClientID := os.Getenv("AZURE_CLIENT_ID"); azClientID != "" {
			clientID = azClientID
		}
	}

	// Also check AZURE_TENANT_ID as fallback for tenant
	tenantID := cfg.TenantID
	if tenantID == config.DefaultTenantID {
		if azTenantID := os.Getenv("AZURE_TENANT_ID"); azTenantID != "" {
			tenantID = azTenantID
		}
	}

	authority := cfg.Authority + tenantID

	// Create assertion callback that reads the token file on each call
	cred := confidential.NewCredFromAssertionCallback(
		func(ctx context.Context, aro confidential.AssertionRequestOptions) (string, error) {
			data, err := os.ReadFile(tokenFile)
			if err != nil {
				return "", fmt.Errorf("failed to read federated token file %s: %w", tokenFile, err)
			}
			return strings.TrimSpace(string(data)), nil
		},
	)

	app, err := confidential.New(authority, clientID, cred,
		confidential.WithCache(&tokenCache{path: sessionCachePath(clientID, tenantID)}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create workload identity client: %w", err)
	}

	return &WorkloadIdentityClient{
		app:       app,
		cfg:       cfg,
		tokenFile: tokenFile,
	}, nil
}

// AcquireToken acquires a token using the federated assertion.
func (c *WorkloadIdentityClient) AcquireToken(ctx context.Context, _ []string) (string, error) {
	result, err := c.app.AcquireTokenByCredential(ctx, []string{config.GraphDefaultScope})
	if err != nil {
		return "", fmt.Errorf("workload identity auth failed: %w", err)
	}
	return result.AccessToken, nil
}

// AcquireTokenWithExtraScopes is not supported for app-only auth.
func (c *WorkloadIdentityClient) AcquireTokenWithExtraScopes(_ context.Context, _, _ []string) (string, error) {
	return "", ErrIncrementalConsentNotSupported
}

// SignOut clears the token cache.
func (c *WorkloadIdentityClient) SignOut() error {
	path := sessionCachePath(c.cfg.ClientID, c.cfg.TenantID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session cache: %w", err)
	}
	return nil
}

// Status returns the current auth state.
func (c *WorkloadIdentityClient) Status(ctx context.Context) (map[string]interface{}, error) {
	_, err := c.AcquireToken(ctx, nil)
	status := map[string]interface{}{
		"authMethod": string(config.AuthMethodWorkloadIdentity),
		"clientId":   c.cfg.ClientID,
		"tenantId":   c.cfg.TenantID,
		"tokenFile":  c.tokenFile,
	}

	if err != nil {
		status["signedIn"] = false
		status["message"] = fmt.Sprintf("Workload identity auth failed: %v", err)
	} else {
		status["signedIn"] = true
	}

	return status, nil
}

// IsAppOnly returns true.
func (c *WorkloadIdentityClient) IsAppOnly() bool {
	return true
}
