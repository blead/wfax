package cmd

import (
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var extractConcurrency int

var extractCmd = &cobra.Command{
	Use:   "extract [target dir]",
	Short: "Extract files into readable format.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		config := wf.ExtractorConfig{
			Workdir:     filepath.Clean(args[0]),
			Concurrency: extractConcurrency,
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
	extractCmd.Flags().IntVarP(&extractConcurrency, "concurrency", "c", 5, "Maximum number of concurrent file extractions.")
}
