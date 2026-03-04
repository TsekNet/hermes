// Package dnd detects whether the user has Do Not Disturb / Focus mode
// enabled on macOS, Windows, or Linux. The Active function is safe to call
// from any goroutine and returns quickly (no network, no blocking I/O).
package dnd

// Active reports whether the operating system's Do Not Disturb or Focus
// mode is currently enabled. Returns false on unsupported platforms or
// if detection fails (fail-open: show the notification).
func Active() bool {
	return active()
}
