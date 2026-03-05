// Package action validates, classifies, and executes button response values.
// Values use a prefix scheme: url: opens a browser, cmd: runs a shell command,
// ms-settings: / x-apple.systempreferences: open platform settings panels,
// and plain values are returned as-is.
package action

import (
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/google/deck"
)

// Kind describes what category a response value falls into.
type Kind int

const (
	KindUnknown  Kind = iota
	KindWeb           // http, https
	KindSettings      // platform settings panel
	KindCommand       // shell command execution
)

// Scheme is a registered action prefix with its kind and platform constraint.
type Scheme struct {
	Prefix   string // prefix including ":" (e.g. "cmd:")
	Kind     Kind
	Platform string // GOOS value, or "" for all platforms
	Example  string
}

var registry = []Scheme{
	{Prefix: "https:", Kind: KindWeb, Example: "https://example.com"},
	{Prefix: "http:", Kind: KindWeb, Example: "http://intranet.corp/kb"},
	{Prefix: "ms-settings:", Kind: KindSettings, Platform: "windows", Example: "ms-settings:windowsupdate"},
	{Prefix: "x-apple.systempreferences:", Kind: KindSettings, Platform: "darwin", Example: "x-apple.systempreferences:com.apple.preference.security"},
	{Prefix: "cmd:", Kind: KindCommand, Example: "cmd:shutdown /r /t 0"},
}

// Allowed reports whether value is permitted on the current OS.
func Allowed(value string) bool {
	return AllowedOn(value, runtime.GOOS)
}

// AllowedOn reports whether value is permitted on the given OS.
func AllowedOn(value, goos string) bool {
	lower := strings.ToLower(value)
	for _, s := range registry {
		if !strings.HasPrefix(lower, s.Prefix) {
			continue
		}
		if s.Platform != "" && s.Platform != goos {
			return false
		}
		if s.Kind == KindCommand {
			return strings.TrimSpace(value[len("cmd:"):]) != ""
		}
		_, err := url.Parse(value)
		return err == nil
	}
	return false
}

// Classify returns the Kind of a value on the current OS.
func Classify(value string) Kind {
	return ClassifyOn(value, runtime.GOOS)
}

// ClassifyOn returns the Kind of a value on the given OS.
func ClassifyOn(value, goos string) Kind {
	lower := strings.ToLower(value)
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

// SettingsSchemes returns the settings schemes available on the given OS.
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

// IsCommand reports whether value uses the cmd: prefix.
func IsCommand(value string) bool {
	return strings.HasPrefix(strings.ToLower(value), "cmd:")
}

// CommandString extracts the shell command from a cmd: value.
func CommandString(value string) string {
	if len(value) <= len("cmd:") {
		return ""
	}
	return strings.TrimSpace(value[len("cmd:"):])
}

// RunCommand executes a cmd:-prefixed value via the platform shell.
func RunCommand(value string) (string, error) {
	return RunCommandOn(value, runtime.GOOS)
}

// RunCommandOn executes a cmd:-prefixed value using the shell for the given OS.
// Exported for cross-platform testing.
func RunCommandOn(value, goos string) (string, error) {
	cmdStr := CommandString(value)
	deck.Infof("action: exec %q (os=%s)", cmdStr, goos)

	var cmd *exec.Cmd
	if goos == "windows" {
		cmd = exec.Command("cmd", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}
