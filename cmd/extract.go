package cmd

import (
	"log"
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var extractPathList string
var extractConcurrency int
var extractIndent int
var extractFlattenCSV bool
var extractNoDefaultPaths bool
var extractEliyabot bool

var extractCmd = &cobra.Command{
	Use:   "extract [src] [dest]",
	Short: "Extract files from src into readable format at dest",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.ExtractorConfig{
			SrcPath:        filepath.Clean(args[0]),
			DestPath:       filepath.Clean(args[1]),
			PathList:       filepath.Clean(extractPathList),
			NoDefaultPaths: extractNoDefaultPaths,
			Concurrency:    extractConcurrency,
			Indent:         extractIndent,
			FlattenCSV:     extractFlattenCSV,
			Eliyabot:       extractEliyabot,
		}

		extractor, err := wf.NewExtractor(&config)
		if err != nil {
			log.Fatalln(err)
		}

		err = extractor.ExtractAssets()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
	extractCmd.Flags().StringVarP(&extractPathList, "path-list", "p", "", "Path to newline delimited file containing possible asset paths (default \"[dest]/.pathlist\")")
	extractCmd.Flags().IntVarP(&extractConcurrency, "concurrency", "c", 5, "Maximum number of concurrent file extractions")
	extractCmd.Flags().IntVarP(&extractIndent, "indent", "i", 0, "Number of spaces used as indentation in extracted JSON (default 0)")
	extractCmd.Flags().BoolVarP(&extractFlattenCSV, "flatten-csv", "f", false, "Ignore newlines in multi-line CSVs")
	extractCmd.Flags().BoolVarP(&extractNoDefaultPaths, "no-default-paths", "n", false, "Ignore default paths and only extract from supplied path list")
	extractCmd.Flags().BoolVarP(&extractEliyabot, "eliyabot", "e", false, "Extract and resize image assets for eliyabot (requires a path list with internal names)")
}
