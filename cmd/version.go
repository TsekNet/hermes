package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version and BuildDate are set at build time via -ldflags.
var (
	Version   = "dev"
	BuildDate = "unknown"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("hermes %s\nBuilt: %s\nGo:    %s\nOS:    %s/%s\n",
				Version, BuildDate, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		},
	}
}
