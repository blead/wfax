package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var fetchVersion string
var fetchDiff bool
var fetchConcurrency int

var fetchCmd = &cobra.Command{
	Use:   "fetch [target dir]",
	Short: "Fetch assets from API to the target directory and print latest version number to stdout",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.ClientConfig{
			Version:     fetchVersion,
			Workdir:     filepath.Clean(args[0]),
			Concurrency: fetchConcurrency,
		}
		if fetchDiff {
			config.Mode = wf.DiffAssets
		} else {
			config.Mode = wf.FullAssets
		}

		client, err := wf.NewClient(&config)
		if err != nil {
			log.Fatalln(err)
		}

		err = client.FetchAssetsFromAPI()
		if err != nil {
			if err == wf.ErrNoNewAssets {
				os.Exit(1)
			}
			log.Fatalln(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().StringVarP(&fetchVersion, "version", "v", "0.0.0", "Game version of existing assets")
	fetchCmd.Flags().BoolVarP(&fetchDiff, "diff-only", "d", false, "Fetch only new assets (used with --version)")
	fetchCmd.Flags().IntVarP(&fetchConcurrency, "concurrency", "c", 5, "Maximum number of concurrent asset downloads")
}
