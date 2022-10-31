package cmd

import (
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var version string
var diff bool
var fetchConcurrency int

var fetchCmd = &cobra.Command{
	Use:   "fetch [target dir]",
	Short: "Fetch assets from API to the target directory.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.ClientConfig{
			Version:     version,
			Workdir:     filepath.Clean(args[0]),
			Concurrency: fetchConcurrency,
		}
		if diff {
			config.Mode = wf.DiffAssets
		} else {
			config.Mode = wf.FullAssets
		}

		client, err := wf.NewClient(&config)
		if err != nil {
			panic(err)
		}

		err = client.FetchAssetsFromAPI()
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)
	fetchCmd.Flags().StringVarP(&version, "version", "v", "0.0.0", "Game version of existing assets.")
	fetchCmd.Flags().BoolVarP(&diff, "diff-only", "d", false, "Fetch only new assets. (used with --version)")
	fetchCmd.Flags().IntVarP(&fetchConcurrency, "concurrency", "c", 5, "Maximum number of concurrent asset downloads.")
}
