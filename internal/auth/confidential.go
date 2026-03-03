package auth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/confidential"
	"github.com/merill/msgraph/internal/config"
)

// ClientSecretClient implements TokenProvider using client secret credentials.
type ClientSecretClient struct {
	app confidential.Client
	cfg *config.Config
}

// NewClientSecretClient creates a confidential client using a client secret.
func NewClientSecretClient(cfg *config.Config) (*ClientSecretClient, error) {
	cred, err := confidential.NewCredFromSecret(cfg.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret credential: %w", err)
	}

	app, err := confidential.New(cfg.AuthorityURL(), cfg.ClientID, cred,
		confidential.WithCache(&tokenCache{path: sessionCachePath(cfg.ClientID, cfg.TenantID)}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create confidential client: %w", err)
	}

	return &ClientSecretClient{app: app, cfg: cfg}, nil
}

// AcquireToken acquires a token using client credentials.
func (c *ClientSecretClient) AcquireToken(ctx context.Context, _ []string) (string, error) {
	result, err := c.app.AcquireTokenByCredential(ctx, []string{config.GraphDefaultScope})
	if err != nil {
		return "", fmt.Errorf("client secret auth failed: %w", err)
	}
	return result.AccessToken, nil
}

// AcquireTokenWithExtraScopes is not supported for app-only auth.
func (c *ClientSecretClient) AcquireTokenWithExtraScopes(_ context.Context, _, _ []string) (string, error) {
	return "", ErrIncrementalConsentNotSupported
}

// SignOut clears the token cache.
func (c *ClientSecretClient) SignOut() error {
	path := sessionCachePath(c.cfg.ClientID, c.cfg.TenantID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session cache: %w", err)
	}
	return nil
}

// Status returns the current auth state.
func (c *ClientSecretClient) Status(ctx context.Context) (map[string]interface{}, error) {
	// Try acquiring a token to verify credentials are valid
	_, err := c.AcquireToken(ctx, nil)
	if err != nil {
		return map[string]interface{}{
			"signedIn":   false,
			"authMethod": string(config.AuthMethodClientSecret),
			"clientId":   c.cfg.ClientID,
			"tenantId":   c.cfg.TenantID,
			"message":    fmt.Sprintf("Client secret auth failed: %v", err),
		}, nil
	}
	return map[string]interface{}{
		"signedIn":   true,
		"authMethod": string(config.AuthMethodClientSecret),
		"clientId":   c.cfg.ClientID,
		"tenantId":   c.cfg.TenantID,
	}, nil
}

// IsAppOnly returns true.
func (c *ClientSecretClient) IsAppOnly() bool {
	return true
}

// ClientCertificateClient implements TokenProvider using a client certificate.
type ClientCertificateClient struct {
	app confidential.Client
	cfg *config.Config
}

// NewClientCertificateClient creates a confidential client using a certificate.
func NewClientCertificateClient(cfg *config.Config) (*ClientCertificateClient, error) {
	pemData, err := os.ReadFile(cfg.ClientCertificatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file %s: %w", cfg.ClientCertificatePath, err)
	}

	certs, key, err := parsePEMCertificate(pemData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	cred, err := confidential.NewCredFromCert(certs, key)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate credential: %w", err)
	}

	app, err := confidential.New(cfg.AuthorityURL(), cfg.ClientID, cred,
		confidential.WithCache(&tokenCache{path: sessionCachePath(cfg.ClientID, cfg.TenantID)}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create confidential client: %w", err)
	}

	return &ClientCertificateClient{app: app, cfg: cfg}, nil
}

// AcquireToken acquires a token using client certificate credentials.
func (c *ClientCertificateClient) AcquireToken(ctx context.Context, _ []string) (string, error) {
	result, err := c.app.AcquireTokenByCredential(ctx, []string{config.GraphDefaultScope})
	if err != nil {
		return "", fmt.Errorf("certificate auth failed: %w", err)
	}
	return result.AccessToken, nil
}

// AcquireTokenWithExtraScopes is not supported for app-only auth.
func (c *ClientCertificateClient) AcquireTokenWithExtraScopes(_ context.Context, _, _ []string) (string, error) {
	return "", ErrIncrementalConsentNotSupported
}

// SignOut clears the token cache.
func (c *ClientCertificateClient) SignOut() error {
	path := sessionCachePath(c.cfg.ClientID, c.cfg.TenantID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session cache: %w", err)
	}
	return nil
}

// Status returns the current auth state.
func (c *ClientCertificateClient) Status(ctx context.Context) (map[string]interface{}, error) {
	_, err := c.AcquireToken(ctx, nil)
	if err != nil {
		return map[string]interface{}{
			"signedIn":        false,
			"authMethod":      string(config.AuthMethodCertificate),
			"clientId":        c.cfg.ClientID,
			"tenantId":        c.cfg.TenantID,
			"certificatePath": c.cfg.ClientCertificatePath,
			"message":         fmt.Sprintf("Certificate auth failed: %v", err),
		}, nil
	}
	return map[string]interface{}{
		"signedIn":        true,
		"authMethod":      string(config.AuthMethodCertificate),
		"clientId":        c.cfg.ClientID,
		"tenantId":        c.cfg.TenantID,
		"certificatePath": c.cfg.ClientCertificatePath,
	}, nil
}

// IsAppOnly returns true.
func (c *ClientCertificateClient) IsAppOnly() bool {
	return true
}

// parsePEMCertificate parses a PEM file containing certificates and a private key.
func parsePEMCertificate(pemData []byte) ([]*x509.Certificate, interface{}, error) {
	var certs []*x509.Certificate
	var privateKey interface{}

	for {
		var block *pem.Block
		block, pemData = pem.Decode(pemData)
		if block == nil {
			break
		}

		switch block.Type {
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
			}
			certs = append(certs, cert)

		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
			}
			privateKey = key

		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse RSA private key: %w", err)
			}
			privateKey = key

		case "EC PRIVATE KEY":
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse EC private key: %w", err)
			}
			privateKey = key
		}
	}

	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no certificates found in PEM file")
	}
	if privateKey == nil {
		return nil, nil, fmt.Errorf("no private key found in PEM file")
	}

	return certs, privateKey, nil
}
