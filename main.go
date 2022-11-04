package main

import (
	"log"
	"os"

	"github.com/blead/wfax/cmd"
	"github.com/hashicorp/logutils"
)

func main() {
	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("INFO"),
		Writer:   os.Stderr,
	})

	cmd.Execute()
}
