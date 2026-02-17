// Package exitcodes defines the exit code contract for hermes.
// Both the CLI (cmd/) and the service (manager/) import this
// to avoid duplicating the value→code mapping.
package exitcodes

import "strings"

// Exit code constants. Scripts should switch on these, not parse stdout.
const (
	OK       int32 = 0   // User chose an action (or dismissed — empty stdout means dismissed)
	Error    int32 = 1   // Bad config, validation, or launch failure
	Deferred int32 = 200 // User deferred
	Timeout  int32 = 202 // Countdown expired, auto-actioned
)

// ForValue maps a user response string to the appropriate exit code.
func ForValue(value string) int32 {
	if strings.HasPrefix(value, "timeout:") || value == "timeout" {
		return Timeout
	}
	if strings.HasPrefix(value, "defer") {
		return Deferred
	}
	if value == "cancelled" {
		return Error
	}
	return OK
}
