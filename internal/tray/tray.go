// Package tray provides the system tray icon for the hermes service daemon.
// Pure formatting and detection functions are testable without a display server.
// The Run function (platform-specific) requires fyne.io/systray at runtime.
package tray

import (
	"fmt"
)

// iconData is set by platform-specific files:
// icon_windows.go embeds icon.ico, icon_unix.go embeds icon.png.

// MaxActiveNotifications matches manager.MaxActiveNotifications.
const MaxActiveNotifications = 10

// IconBytes returns the embedded tray icon bytes (ICO on Windows, PNG elsewhere).
func IconBytes() []byte { return iconData }

// FormatTooltip returns the tooltip text for the given pending count.
func FormatTooltip(count int) string {
	if count < 0 {
		count = 0
	}
	if count >= MaxActiveNotifications {
		return fmt.Sprintf("Hermes — at capacity (%d pending)", count)
	}
	switch count {
	case 0:
		return "Hermes — no pending notifications"
	case 1:
		return "Hermes — 1 pending notification"
	default:
		return fmt.Sprintf("Hermes — %d pending notifications", count)
	}
}

// FormatInboxLabel returns the "Open Inbox" menu label, appending the
// pending count when there are active notifications.
func FormatInboxLabel(count int) string {
	if count <= 0 {
		return "Open Inbox"
	}
	return fmt.Sprintf("Open Inbox (Pending: %d)", count)
}

// ShouldEnable reports whether the tray icon should be started on the
// given OS. On Linux/FreeBSD, requires a display server ($DISPLAY or
// $WAYLAND_DISPLAY). Windows and macOS always enable.
// The envLookup function reads environment variables (allows testing
// without modifying the real environment).
func ShouldEnable(goos string, envLookup func(string) string) bool {
	switch goos {
	case "windows", "darwin":
		return true
	case "linux", "freebsd", "openbsd", "netbsd":
		return envLookup("DISPLAY") != "" || envLookup("WAYLAND_DISPLAY") != ""
	default:
		return false
	}
}

// InboxArgs returns the command-line arguments for launching the inbox
// subprocess. The caller provides the port to connect to the service.
func InboxArgs(port int) []string {
	return []string{"inbox", "--port", fmt.Sprintf("%d", port)}
}
