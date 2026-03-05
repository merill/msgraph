package cmd

import (
	"fmt"
	"strings"

	"github.com/merill/msgraph/internal/samples"
	"github.com/spf13/cobra"
)

var buildSamplesCmd = &cobra.Command{
	Use:    "build-samples-index",
	Short:  "Build the samples index JSON from YAML source files",
	Long:   "Walks the samples directory, parses all YAML files, and outputs a compiled samples-index.json and samples-index.db. Used by CI — not intended for end users.",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		samplesDir, _ := cmd.Flags().GetString("samples-dir")
		outputPath, _ := cmd.Flags().GetString("output")
		dbOutput, _ := cmd.Flags().GetString("db-output")

		idx, err := samples.BuildIndex(samplesDir)
		if err != nil {
			return err
		}

		// Write JSON index (backward compat).
		if err := samples.WriteIndex(idx, outputPath); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStderr(), "Built samples index: %d samples -> %s\n", idx.Count, outputPath)

		// Write FTS SQLite database.
		if dbOutput == "" {
			dbOutput = strings.TrimSuffix(outputPath, ".json") + ".db"
		}
		if err := samples.BuildFTSDatabase(idx, dbOutput); err != nil {
			return fmt.Errorf("failed to build FTS database: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStderr(), "Built samples FTS database: %d samples -> %s\n", idx.Count, dbOutput)

		return nil
	},
}

func init() {
	buildSamplesCmd.Flags().String("samples-dir", "samples", "Path to the samples source directory")
	buildSamplesCmd.Flags().String("output", "skills/msgraph/references/samples-index.json", "Path to write the compiled index JSON")
	buildSamplesCmd.Flags().String("db-output", "", "Path to write the FTS SQLite database (defaults to .db alongside JSON)")

	rootCmd.AddCommand(buildSamplesCmd)
}
