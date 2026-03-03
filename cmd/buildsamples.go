package cmd

import (
	"fmt"

	"github.com/merill/msgraph/internal/samples"
	"github.com/spf13/cobra"
)

var buildSamplesCmd = &cobra.Command{
	Use:    "build-samples-index",
	Short:  "Build the samples index JSON from YAML source files",
	Long:   "Walks the samples directory, parses all YAML files, and outputs a compiled samples-index.json. Used by CI — not intended for end users.",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		samplesDir, _ := cmd.Flags().GetString("samples-dir")
		outputPath, _ := cmd.Flags().GetString("output")

		idx, err := samples.BuildIndex(samplesDir)
		if err != nil {
			return err
		}

		if err := samples.WriteIndex(idx, outputPath); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStderr(), "Built samples index: %d samples → %s\n", idx.Count, outputPath)
		return nil
	},
}

func init() {
	buildSamplesCmd.Flags().String("samples-dir", "skills/msgraph/samples", "Path to the samples source directory")
	buildSamplesCmd.Flags().String("output", "skills/msgraph/references/samples-index.json", "Path to write the compiled index JSON")

	rootCmd.AddCommand(buildSamplesCmd)
}
