//go:build !windows

// Package logging provides platform-specific log backend initialization.
package logging

import (
	"fmt"
	"os"

	"github.com/google/deck"
	"github.com/google/deck/backends/syslog"
)

// Init adds the syslog backend to deck on macOS and Linux.
func Init() {
	sl, err := syslog.Init("hermes", syslog.LOG_USER)
	if err != nil {
		fmt.Fprintf(os.Stderr, "syslog init: %v (stderr only)\n", err)
		return
	}
	deck.Add(sl)
}
