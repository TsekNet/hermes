// Package urischeme validates and classifies URIs that hermes is allowed to
// open. Each platform has its own settings URI scheme (ms-settings: on Windows,
// x-apple.systempreferences: on macOS) plus the universal http/https. The
// package centralises the allow-list so callers don't scatter platform checks.
package urischeme

import (
	"net/url"
	"runtime"
	"strings"
)

// Kind describes what category a URI falls into.
type Kind int

const (
	KindUnknown  Kind = iota
	KindWeb           // http, https
	KindSettings      // platform settings panel
)

// Scheme is a registered URI scheme with its kind and platform constraint.
type Scheme struct {
	Prefix   string // scheme prefix including ":" (e.g. "ms-settings:")
	Kind     Kind
	Platform string // GOOS value, or "" for all platforms
	Example  string // documentation example
}

// registry is the canonical list of allowed URI schemes.
var registry = []Scheme{
	{Prefix: "https:", Kind: KindWeb, Example: "https://example.com"},
	{Prefix: "http:", Kind: KindWeb, Example: "http://intranet.corp/kb"},
	{Prefix: "ms-settings:", Kind: KindSettings, Platform: "windows", Example: "ms-settings:windowsupdate"},
	{Prefix: "x-apple.systempreferences:", Kind: KindSettings, Platform: "darwin", Example: "x-apple.systempreferences:com.apple.preference.security"},
}

// Allowed reports whether rawURL is permitted on the current OS.
func Allowed(rawURL string) bool {
	return AllowedOn(rawURL, runtime.GOOS)
}

// AllowedOn reports whether rawURL is permitted on the given OS.
// Exported for testing without build tags.
func AllowedOn(rawURL, goos string) bool {
	lower := strings.ToLower(rawURL)
	for _, s := range registry {
		if !strings.HasPrefix(lower, s.Prefix) {
			continue
		}
		if s.Platform != "" && s.Platform != goos {
			return false
		}
		_, err := url.Parse(rawURL)
		return err == nil
	}
	return false
}

// Classify returns the Kind of a URI on the current OS, or KindUnknown.
func Classify(rawURL string) Kind {
	return ClassifyOn(rawURL, runtime.GOOS)
}

// ClassifyOn returns the Kind of a URI on the given OS.
func ClassifyOn(rawURL, goos string) Kind {
	lower := strings.ToLower(rawURL)
	for _, s := range registry {
		if strings.HasPrefix(lower, s.Prefix) {
			if s.Platform != "" && s.Platform != goos {
				return KindUnknown
			}
			return s.Kind
		}
	}
	return KindUnknown
}

// SettingsSchemes returns the settings URI schemes available on the given OS.
// Useful for documentation and help text.
func SettingsSchemes(goos string) []Scheme {
	var out []Scheme
	for _, s := range registry {
		if s.Kind == KindSettings && (s.Platform == "" || s.Platform == goos) {
			out = append(out, s)
		}
	}
	return out
}

// AllSchemes returns every registered scheme available on the given OS.
func AllSchemes(goos string) []Scheme {
	var out []Scheme
	for _, s := range registry {
		if s.Platform == "" || s.Platform == goos {
			out = append(out, s)
		}
	}
	return out
}
