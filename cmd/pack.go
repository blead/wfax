package cmd

import (
	"log"
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var packPathList string
var packConcurrency int

var packCmd = &cobra.Command{
	Use:   "pack [src] [dest]",
	Short: "Pack files from src into game format at dest",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.ExtractorConfig{
			SrcPath:        filepath.Clean(args[1]),
			DestPath:       filepath.Clean(args[0]),
			PathList:       filepath.Clean(packPathList),
			NoDefaultPaths: false,
			Concurrency:    packConcurrency,
			Indent:         0,
			FlattenCSV:     false,
			Eliyabot:       false,
		}

		extractor, err := wf.NewExtractor(&config)
		if err != nil {
			log.Fatalln(err)
		}

		err = extractor.PackAssets()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(packCmd)
	packCmd.Flags().StringVarP(&packPathList, "path-list", "p", "", "Path to newline delimited file containing possible asset paths (default \"[dest]/.pathlist\")")
	packCmd.Flags().IntVarP(&packConcurrency, "concurrency", "c", 5, "Maximum number of concurrent file extractions")
}
