// Package action validates, classifies, and executes notification response values.
// Values use a prefix scheme: uri: opens a URI via the OS default handler,
// action: runs a built-in verb (reboot, shutdown, lock), and plain values
// are returned as-is to the calling script via the manager layer.
//
// The cmd: prefix is explicitly rejected. Shell execution from config input
// is not permitted. URI schemes use an allowlist, not a denylist.
package action

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/google/deck"
	"github.com/pkg/browser"
)

// Kind describes what category a response value falls into.
type Kind int

const (
	KindUnknown Kind = iota
	KindURI          // uri: prefix, opened by OS default handler
	KindBuiltin      // action: prefix, compiled verb (reboot, shutdown, lock)
)

var allowedSchemes = []string{
	"https:",
	"http:",
	"ms-settings:",
	"x-apple.systempreferences:",
	"slack:",
	"msteams:",
	"companyportal:",
	"codex:",
}

var validVerbs = []string{"reboot", "shutdown", "lock"}

// ValidVerbs returns the list of supported action: verbs.
func ValidVerbs() []string {
	out := make([]string, len(validVerbs))
	copy(out, validVerbs)
	return out
}

// AllowedSchemes returns the list of URI schemes permitted by the allowlist.
func AllowedSchemes() []string {
	out := make([]string, len(allowedSchemes))
	copy(out, allowedSchemes)
	return out
}

// IsURI reports whether value uses the uri: prefix.
func IsURI(value string) bool {
	return strings.HasPrefix(strings.ToLower(value), "uri:")
}

// IsBuiltin reports whether value uses the action: prefix.
func IsBuiltin(value string) bool {
	return strings.HasPrefix(strings.ToLower(value), "action:")
}

// Allowed reports whether value is permitted on the current OS.
func Allowed(value string) bool {
	return AllowedOn(value, runtime.GOOS)
}

// AllowedOn reports whether value is permitted on the given OS.
// The goos parameter is accepted for test-time cross-platform verification
// but does not currently gate any scheme or verb by platform.
func AllowedOn(value, goos string) bool {
	lower := strings.ToLower(value)

	if strings.HasPrefix(lower, "cmd:") {
		return false
	}

	if strings.HasPrefix(lower, "uri:") {
		body := uriBody(value)
		if body == "" {
			return false
		}
		return isAllowedScheme(body)
	}

	if strings.HasPrefix(lower, "action:") {
		return isValidVerb(actionVerb(value))
	}

	return false
}

// ClassifyOn returns the Kind of a value on the given OS.
func ClassifyOn(value, goos string) Kind {
	lower := strings.ToLower(value)

	if strings.HasPrefix(lower, "uri:") {
		body := uriBody(value)
		if body == "" || !isAllowedScheme(body) {
			return KindUnknown
		}
		return KindURI
	}

	if strings.HasPrefix(lower, "action:") {
		if isValidVerb(actionVerb(value)) {
			return KindBuiltin
		}
		return KindUnknown
	}

	return KindUnknown
}

// OpenURI opens a URI via the OS default handler after checking the allowlist.
func OpenURI(uri string) error {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("empty URI")
	}
	if !isAllowedScheme(uri) {
		return fmt.Errorf("URI scheme not allowed: %s", uri)
	}
	deck.Infof("action: open URI %q", uri)
	return browser.OpenURL(uri)
}

// RunBuiltin executes a built-in action verb on the current OS.
func RunBuiltin(verb string) error {
	verb = strings.ToLower(strings.TrimSpace(verb))
	if !isValidVerb(verb) {
		return fmt.Errorf("unknown action verb: %q", verb)
	}
	deck.Infof("action: builtin %q (os=%s)", verb, runtime.GOOS)
	return platformRunBuiltin(verb)
}

// Dispatch executes an action string from the server side (no Wails context).
// Handles uri: and action: prefixes. Returns nil for plain values (the manager
// layer reads these from the Notify response). Rejects cmd: with an error.
func Dispatch(value string) error {
	lower := strings.ToLower(value)

	if strings.HasPrefix(lower, "cmd:") {
		return fmt.Errorf("cmd: prefix is not supported, use action: or uri: instead")
	}

	if strings.HasPrefix(lower, "uri:") {
		return OpenURI(uriBody(value))
	}

	if strings.HasPrefix(lower, "action:") {
		return RunBuiltin(actionVerb(value))
	}

	return nil
}

func uriBody(value string) string {
	return strings.TrimSpace(value[len("uri:"):])
}

func actionVerb(value string) string {
	return strings.ToLower(strings.TrimSpace(value[len("action:"):]))
}

func isAllowedScheme(uri string) bool {
	lower := strings.ToLower(uri)
	for _, s := range allowedSchemes {
		if strings.HasPrefix(lower, s) {
			return true
		}
	}
	return false
}

func isValidVerb(verb string) bool {
	for _, v := range validVerbs {
		if v == verb {
			return true
		}
	}
	return false
}
