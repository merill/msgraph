package openapi

import (
	"os"
	"testing"
)

func TestBuildAndSearchFTS(t *testing.T) {
	// Load the JSON index.
	jsonPath := "../../skills/msgraph/references/graph-api-index.json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Skip("graph-api-index.json not found, skipping")
	}

	idx, err := LoadIndex(jsonPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	// Convert to FullEndpoints (using only the fields available in JSON).
	var fullEndpoints []FullEndpoint
	for _, ep := range idx.Endpoints {
		fullEndpoints = append(fullEndpoints, FullEndpoint{
			Path:        ep.Path,
			Method:      ep.Method,
			Summary:     ep.Summary,
			Description: ep.Description,
			Resource:    ep.Resource,
			Scopes:      ep.Scopes,
		})
	}

	// Build FTS DB.
	tmpFile, err := os.CreateTemp("", "openapi-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	if err := BuildFTSDatabase(fullEndpoints, dbPath); err != nil {
		t.Fatalf("BuildFTSDatabase: %v", err)
	}

	// Open and search.
	ftsIdx, err := LoadFTSIndex(dbPath)
	if err != nil {
		t.Fatalf("LoadFTSIndex: %v", err)
	}
	defer ftsIdx.Close()

	// Test the original bug: "subscribedSkus licenses" should match
	// endpoints mentioning "license" (singular) via Porter stemming.
	results := ftsIdx.Search("subscribedSkus licenses", "", "", 10)
	if len(results) == 0 {
		t.Error("Expected results for 'subscribedSkus licenses', got none (Porter stemming should match 'license' to 'licenses')")
	}
	for _, r := range results {
		t.Logf("Result: %s %s - %s (reason: %s)", r.Method, r.Path, r.Summary, r.MatchReason)
	}

	// Test resource filter.
	userResults := ftsIdx.Search("", "users", "GET", 5)
	if len(userResults) == 0 {
		t.Error("Expected results for resource 'users' GET, got none")
	}
	t.Logf("Found %d results for resource=users, method=GET", len(userResults))

	// Test basic keyword search.
	mailResults := ftsIdx.Search("send mail", "", "", 5)
	if len(mailResults) == 0 {
		t.Error("Expected results for 'send mail', got none")
	}
	for _, r := range mailResults {
		t.Logf("Mail result: %s %s - %s", r.Method, r.Path, r.Summary)
	}
}
