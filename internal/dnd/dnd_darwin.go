//go:build darwin

package dnd

import (
	"os/exec"
	"strings"
)

func active() bool {
	// macOS Monterey+ exposes Focus mode via the Control Center status item.
	// Returns "1" when any Focus mode (DND, Work, Sleep, etc.) is active.
	out, err := exec.Command("defaults", "read", "com.apple.controlcenter", "NSStatusItem Visible FocusModes").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		return true
	}

	// Fallback for older macOS (pre-Monterey): classic DND toggle.
	out, err = exec.Command("defaults", "-currentHost", "read", "com.apple.notificationcenterui", "doNotDisturb").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "1" {
		return true
	}

	return false
}
