package cmd

import (
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var extractConcurrency int
var extractIndent int
var extractFlattenCSV bool

var extractCmd = &cobra.Command{
	Use:   "extract [src] [dest]",
	Short: "Extract files from src into readable format at dest",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {

		config := wf.ExtractorConfig{
			SrcPath:     filepath.Clean(args[0]),
			DestPath:    filepath.Clean(args[1]),
			Concurrency: extractConcurrency,
			Indent:      extractIndent,
			FlattenCSV:  extractFlattenCSV,
		}

		extractor, err := wf.NewExtractor(&config)
		if err != nil {
			panic(err)
		}

		err = extractor.ExtractAssets()
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().IntVarP(&extractConcurrency, "concurrency", "c", 5, "Maximum number of concurrent file extractions")
	extractCmd.Flags().IntVarP(&extractIndent, "indent", "i", 0, "Number of spaces used as indentation in extracted JSON")
	extractCmd.Flags().BoolVarP(&extractFlattenCSV, "flatten-csv", "f", false, "Ignore newlines in multi-line CSVs")
}
