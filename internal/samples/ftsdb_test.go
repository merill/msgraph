package samples

import (
	"os"
	"testing"
)

func TestBuildAndSearchFTS(t *testing.T) {
	// Load the JSON index.
	jsonPath := "../../skills/msgraph/references/samples-index.json"
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Skip("samples-index.json not found, skipping")
	}

	idx, err := LoadIndex(jsonPath)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	// Build FTS DB to a temp file.
	tmpFile, err := os.CreateTemp("", "samples-test-*.db")
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

	// Test FTS search.
	results := ftsIdx.Search("conditional access", "", 5)
	if len(results) == 0 {
		t.Error("Expected results for 'conditional access', got none")
	}
	for _, r := range results {
		t.Logf("Sample: %s (product: %s, reason: %s)", r.Intent, r.Product, r.MatchReason)
	}

	// Test product filter.
	prodResults := ftsIdx.Search("", "entra", 20)
	if len(prodResults) == 0 {
		t.Error("Expected results for product 'entra', got none")
	}
	t.Logf("Found %d entra samples", len(prodResults))
}
