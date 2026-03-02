package graph

import (
	"encoding/json"
	"regexp"
	"strings"
)

// GraphError represents a Microsoft Graph API error response.
type GraphError struct {
	Error struct {
		Code       string `json:"code"`
		Message    string `json:"message"`
		InnerError *struct {
			Date            string `json:"date"`
			RequestID       string `json:"request-id"`
			ClientRequestID string `json:"client-request-id"`
		} `json:"innerError,omitempty"`
	} `json:"error"`
}

// ParseGraphError parses a Graph API error response body.
func ParseGraphError(body string) *GraphError {
	var graphErr GraphError
	if err := json.Unmarshal([]byte(body), &graphErr); err != nil {
		return nil
	}
	if graphErr.Error.Code == "" {
		return nil
	}
	return &graphErr
}

// scopeRegex matches permission/scope names in Graph 403 error messages.
// Common patterns: "Mail.Read", "User.ReadWrite.All", "Directory.Read.All"
var scopeRegex = regexp.MustCompile(`[A-Z][a-zA-Z]+\.[A-Z][a-zA-Z]+(?:\.[A-Z][a-zA-Z]+)?`)

// ParseRequiredScopes attempts to extract required permission scopes from
// a 403 Forbidden response body. Graph API error messages often contain
// the required scopes in the error message text.
func ParseRequiredScopes(body string) []string {
	graphErr := ParseGraphError(body)
	if graphErr == nil {
		return nil
	}

	msg := graphErr.Error.Message

	// Look for scope names in the error message
	matches := scopeRegex.FindAllString(msg, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate and filter out common false positives
	seen := make(map[string]bool)
	var scopes []string
	for _, m := range matches {
		lower := strings.ToLower(m)
		// Skip common false positives that aren't scopes
		if lower == "microsoft.graph" || lower == "access.denied" {
			continue
		}
		if !seen[m] {
			seen[m] = true
			scopes = append(scopes, m)
		}
	}

	return scopes
}

// FormatError returns a user-friendly error message from a Graph API response.
func FormatError(resp *Response) string {
	graphErr := ParseGraphError(resp.Body)
	if graphErr != nil {
		return graphErr.Error.Code + ": " + graphErr.Error.Message
	}
	return resp.Body
}
