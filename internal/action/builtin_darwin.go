//go:build darwin

// macOS builtins use absolute SIP-protected paths. Pure Go cannot call
// Carbon/Cocoa APIs (IOKit, AppKit) without cgo. These binaries are in
// paths protected by System Integrity Protection and cannot be tampered with.

package action

import (
	"fmt"
	"os/exec"
)

func platformRunBuiltin(verb string) error {
	switch verb {
	case "reboot":
		return exec.Command("/usr/bin/osascript", "-e",
			`tell application "System Events" to restart`).Run()
	case "shutdown":
		return exec.Command("/usr/bin/osascript", "-e",
			`tell application "System Events" to shut down`).Run()
	case "lock":
		return exec.Command(
			"/System/Library/CoreServices/Menu Extras/User.menu/Contents/Resources/CGSession",
			"-suspend").Run()
	default:
		return fmt.Errorf("unknown verb: %q", verb)
	}
}
