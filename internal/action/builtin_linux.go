//go:build linux

// Linux builtins use absolute paths to systemd binaries. Pure syscalls
// (unix.Reboot) require CAP_SYS_BOOT which a per-user daemon does not have.
// The alternative is D-Bus IPC to systemd-logind, but that adds a dependency
// (godbus/dbus). These paths are package-managed and root-owned.

package action

import (
	"fmt"
	"os/exec"
)

func platformRunBuiltin(verb string) error {
	switch verb {
	case "reboot":
		return exec.Command("/usr/bin/systemctl", "reboot").Run()
	case "shutdown":
		return exec.Command("/usr/bin/systemctl", "poweroff").Run()
	case "lock":
		// lock-session without args locks the calling process's session,
		// which requires hermes to run as a user-session service (not system).
		return exec.Command("/usr/bin/loginctl", "lock-session").Run()
	default:
		return fmt.Errorf("unknown verb: %q", verb)
	}
}
