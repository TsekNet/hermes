//go:build windows

package logging

import (
	"fmt"
	"os"

	"github.com/google/deck"
	"github.com/google/deck/backends/eventlog"
)

// Init registers hermes as a Windows Event Log source and adds the eventlog
// backend to deck. Falls back to stderr if registration fails.
func Init() {
	evt, err := eventlog.InitWithDefaultInstall("hermes")
	if err != nil {
		evt, err = eventlog.Init("hermes")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "eventlog init: %v (stderr only)\n", err)
		return
	}
	deck.Add(evt)
}
