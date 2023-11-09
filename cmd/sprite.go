package cmd

import (
	"log"
	"path/filepath"

	"github.com/blead/wfax/pkg/wf"
	"github.com/spf13/cobra"
)

var spritePath string
var spriteScale float32
var spriteConcurrency int
var spriteEliyabot bool

var spriteCmd = &cobra.Command{
	Use:   "sprite [src] [dest]",
	Short: "Extract and crop sprites from src into images at dest",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		config := wf.SpriterConfig{
			SrcPath:     filepath.Clean(args[0]),
			DestPath:    filepath.Clean(args[1]),
			SpritePath:  spritePath,
			Scale:       spriteScale,
			Concurrency: spriteConcurrency,
			Eliyabot:    spriteEliyabot,
		}

		spriter, err := wf.NewSpriter(&config)
		if err != nil {
			log.Fatalln(err)
		}

		err = spriter.ExtractSprite()
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(spriteCmd)
	spriteCmd.Flags().StringVarP(&spritePath, "path", "p", "item/sprite_sheet", "Internal path of target sprite sheet (default \"item/sprite_sheet\")")
	spriteCmd.Flags().Float32VarP(&spriteScale, "scale", "s", 4, "Sprite size scaling (default 4.0)")
	spriteCmd.Flags().IntVarP(&spriteConcurrency, "concurrency", "c", 5, "Maximum number of concurrent sprite processing")
	spriteCmd.Flags().BoolVarP(&spriteEliyabot, "eliyabot", "e", false, "Extract weapon sprites with rarity backgrounds for eliyabot")
}
