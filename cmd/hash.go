package cmd

import (
	"fmt"
	"log"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var hashPath string

var hashCmd = &cobra.Command{
	Use:   "hash [path]",
	Short: "Return hashed wf asset path",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		hasher := &wf.Hasher{}

		hash, err := hasher.HashAssetPath(args[0])
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(hash)
	},
}

func init() {
	rootCmd.AddCommand(hashCmd)
}
