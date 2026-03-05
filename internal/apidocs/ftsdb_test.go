package apidocs

import (
	"os"
	"testing"
)

func TestBuildAndSearchFTS(t *testing.T) {
	// Load the JSON index.
	jsonPath := "../../skills/msgraph/references/api-docs-index.json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Skip("api-docs-index.json not found, skipping")
	}

	idx, err := LoadIndex(jsonPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	// Build FTS DB to a temp file.
	tmpFile, err := os.CreateTemp("", "api-docs-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(dbPath)

	if err := BuildFTSDatabase(idx, dbPath); err != nil {
		t.Fatalf("BuildFTSDatabase: %v", err)
	}

	// Open and search.
	ftsIdx, err := LoadFTSIndex(dbPath)
	if err != nil {
		t.Fatalf("LoadFTSIndex: %v", err)
	}
	defer ftsIdx.Close()

	// Test endpoint search.
	results := ftsIdx.SearchEndpoints("/users", "GET", "", 5)
	if len(results) == 0 {
		t.Error("Expected endpoint results for /users GET, got none")
	}
	for _, r := range results {
		t.Logf("Endpoint: %s %s (reason: %s)", r.Method, r.Path, r.MatchReason)
	}

	// Test resource search.
	resResults := ftsIdx.SearchResources("user", "", 5)
	if len(resResults) == 0 {
		t.Error("Expected resource results for 'user', got none")
	}
	for _, r := range resResults {
		t.Logf("Resource: %s (reason: %s)", r.Name, r.MatchReason)
	}

	// Test FTS query search.
	ftsResults := ftsIdx.SearchEndpoints("", "", "ConsistencyLevel", 5)
	if len(ftsResults) == 0 {
		t.Error("Expected results for query 'ConsistencyLevel', got none")
	}
	for _, r := range ftsResults {
		t.Logf("FTS result: %s %s (reason: %s)", r.Method, r.Path, r.MatchReason)
	}
}
