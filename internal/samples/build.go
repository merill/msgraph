package samples

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// BuildIndex walks a samples directory, parses all YAML files, and
// returns a compiled Index. This is used by CI to produce samples-index.json.
func BuildIndex(samplesDir string) (*Index, error) {
	var allSamples []Sample

	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read samples directory %s: %w", samplesDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		product := entry.Name()
		productDir := filepath.Join(samplesDir, product)

		files, err := os.ReadDir(productDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read product directory %s: %w", productDir, err)
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}

			filePath := filepath.Join(productDir, name)
			sample, err := parseSampleFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
			}

			sample.Product = product
			sample.File = filepath.Join(product, name)
			allSamples = append(allSamples, *sample)
		}
	}

	return &Index{
		Generated: time.Now().UTC().Format(time.RFC3339),
		Count:     len(allSamples),
		Samples:   allSamples,
	}, nil
}

// parseSampleFile reads a single YAML sample file and returns a Sample.
func parseSampleFile(path string) (*Sample, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Sample
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	if s.Intent == "" {
		return nil, fmt.Errorf("missing required field 'intent'")
	}
	if s.Query == nil {
		return nil, fmt.Errorf("missing required field 'query'")
	}

	return &s, nil
}

// WriteIndex serializes an Index to JSON and writes it to the given path.
func WriteIndex(idx *Index, path string) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("failed to write index to %s: %w", path, err)
	}

	return nil
}
