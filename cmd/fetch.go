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
var fetchRegion string
var fetchComics int
var fetchCustomAPI string
var fetchCustomCDN string

var fetchCmd = &cobra.Command{
	Use:   "fetch [target dir]",
	Short: "Fetch assets from API to the target directory and print latest version number to stdout",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.ClientConfig{
			Version:     fetchVersion,
			Workdir:     filepath.Clean(args[0]),
			Concurrency: fetchConcurrency,
			CustomAPI:   fetchCustomAPI,
			CustomCDN:   fetchCustomCDN,
		}
		if fetchDiff {
			config.Mode = wf.DiffAssets
		} else {
			config.Mode = wf.FullAssets
		}

		switch fetchRegion {
		case "jp":
			config.Region = wf.RegionJP
		case "gl":
			config.Region = wf.RegionGL
		case "th":
			config.Region = wf.RegionTH
		case "kr":
			config.Region = wf.RegionKR
		case "cn":
			config.Region = wf.RegionCN
		case "tw":
			config.Region = wf.RegionTW
		default:
			log.Printf("[WARN] Unknown service region %s, using default (jp)", fetchRegion)
			config.Region = wf.RegionJP
		}

		client, err := wf.NewClient(&config)
		if err != nil {
			log.Fatalln(err)
		}

		err = client.FetchAssetsFromAPI(fetchComics)
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
	fetchCmd.Flags().StringVarP(&fetchRegion, "region", "r", "jp", "Service region/language: jp, gl, th, kr, cn, tw")
	fetchCmd.Flags().IntVarP(&fetchComics, "comics", "m", 0, "Fetch comics instead (1: character comics, 2: tutorial comics)")
	fetchCmd.Flags().StringVarP(&fetchCustomAPI, "custom-api", "A", "", "Set custom API endpoint for asset metadata (file URIs also supported)")
	fetchCmd.Flags().StringVarP(&fetchCustomCDN, "custom-cdn", "C", "", "Set custom CDN endpoint for assets (file URIs also supported)")
}
