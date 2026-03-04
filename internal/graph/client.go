// Package graph provides an HTTP client for Microsoft Graph API calls
// with built-in safety enforcement and incremental consent support.
package graph

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/merill/msgraph/internal/auth"
	"github.com/merill/msgraph/internal/config"
)

// Client is the Graph API HTTP client.
type Client struct {
	httpClient  *http.Client
	authClient  auth.TokenProvider
	cfg         *config.Config
	allowWrites bool
	scopes      []string
}

// NewClient creates a new Graph API client.
func NewClient(authClient auth.TokenProvider, cfg *config.Config, allowWrites bool) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		authClient:  authClient,
		cfg:         cfg,
		allowWrites: allowWrites,
		scopes:      []string{config.DefaultScope},
	}
}

// CallOptions configures a single Graph API request.
type CallOptions struct {
	Method     string
	URL        string // Can be a full URL or a relative path (e.g. "/me")
	Body       string
	Headers    map[string]string
	APIVersion string
	Select     string
	Filter     string
	Top        int
	Expand     string
	OrderBy    string
	Scopes     []string // Extra scopes to request
}

// Response holds the Graph API response.
type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body"`
}

// Call executes a Graph API request.
func (c *Client) Call(ctx context.Context, opts CallOptions) (*Response, error) {
	// Enforce safety
	if err := CheckSafety(opts.Method, c.allowWrites); err != nil {
		return nil, err
	}

	// Build the full URL
	reqURL, err := c.buildURL(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build request URL: %w", err)
	}

	// Determine scopes
	scopes := c.scopes
	if len(opts.Scopes) > 0 {
		scopes = opts.Scopes
	}

	// Get access token
	token, err := c.authClient.AcquireToken(ctx, scopes)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Execute with retry on 403 for incremental consent
	resp, err := c.doRequest(ctx, opts, reqURL, token)
	if err != nil {
		return nil, err
	}

	// If 403 and not app-only, try incremental consent
	if resp.StatusCode == http.StatusForbidden && !c.authClient.IsAppOnly() {
		extraScopes := ParseRequiredScopes(resp.Body)
		if len(extraScopes) > 0 {
			newToken, authErr := c.authClient.AcquireTokenWithExtraScopes(ctx, scopes, extraScopes)
			if authErr != nil {
				// Return the original 403 response — auth retry failed
				return resp, nil
			}
			// Retry with the new token
			retryResp, retryErr := c.doRequest(ctx, opts, reqURL, newToken)
			if retryErr != nil {
				return nil, retryErr
			}
			return retryResp, nil
		}
	}

	return resp, nil
}

// doRequest executes the HTTP request.
func (c *Client) doRequest(ctx context.Context, opts CallOptions, reqURL, token string) (*Response, error) {
	var bodyReader io.Reader
	if opts.Body != "" {
		bodyReader = strings.NewReader(opts.Body)
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set standard headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json, text/plain;q=0.9")
	if opts.Body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Set custom headers
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(respBody),
	}, nil
}

// buildURL constructs the full Graph API URL with query parameters.
func (c *Client) buildURL(opts CallOptions) (string, error) {
	rawURL := opts.URL

	// If it's a relative path, prepend the Graph base URL
	if strings.HasPrefix(rawURL, "/") {
		apiVersion := c.cfg.APIVersion
		if opts.APIVersion != "" {
			apiVersion = opts.APIVersion
		}
		rawURL = c.cfg.GraphURL(apiVersion) + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Add OData query parameters
	q := u.Query()
	if opts.Select != "" {
		q.Set("$select", opts.Select)
	}
	if opts.Filter != "" {
		q.Set("$filter", opts.Filter)
	}
	if opts.Top > 0 {
		q.Set("$top", fmt.Sprintf("%d", opts.Top))
	}
	if opts.Expand != "" {
		q.Set("$expand", opts.Expand)
	}
	if opts.OrderBy != "" {
		q.Set("$orderby", opts.OrderBy)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}
