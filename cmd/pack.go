package cmd

import (
	"log"
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var packConcurrency int

var packCmd = &cobra.Command{
	Use:   "pack [src] [dest]",
	Short: "Pack files from src into game assets at dest (currently only supports default directory structure)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.PackerConfig{
			SrcPath:     filepath.Clean(args[0]),
			DestPath:    filepath.Clean(args[1]),
			Concurrency: packConcurrency,
		}

		packer, err := wf.NewPacker(&config)
		if err != nil {
			log.Fatalln(err)
		}

		err = packer.PackAssets()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(packCmd)
	packCmd.Flags().IntVarP(&packConcurrency, "concurrency", "c", 5, "Maximum number of concurrent file extractions")
}
