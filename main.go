package main

import (
	"embed"
	"os"

	"github.com/TsekNet/hermes/cmd"
	"github.com/TsekNet/hermes/internal/logging"
	"github.com/google/deck"
	"github.com/google/deck/backends/logger"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	deck.Add(logger.Init(os.Stderr, 0))
	logging.Init()

	// Wails calls the binary with -generate-bindings during build.
	// Handle it before Cobra since it's a single-dash flag.
	if len(os.Args) > 1 && os.Args[1] == "-generate-bindings" {
		cmd.RunBindings(assets)
		return
	}

	if err := cmd.Execute(assets); err != nil {
		deck.Errorf("%v", err)
		os.Exit(1)
	}
}
