package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NotificationConfig holds all parameters for a notification dialog.
type NotificationConfig struct {
	Heading        string   `json:"heading"`
	Message        string   `json:"message"`
	Buttons        []Button `json:"buttons"`
	TimeoutSeconds int      `json:"timeout"`
	TimeoutValue   string   `json:"timeoutValue"`
	EscValue       string   `json:"escValue"`
	Title          string   `json:"title"`
	AccentColor    string   `json:"accentColor"`
	HelpURL        string   `json:"helpUrl"`
	Platform       string   `json:"platform"`
	// ID uniquely identifies this notification for the service daemon.
	// Auto-generated if empty when submitted via gRPC.
	ID string `json:"id,omitempty"`

	// DeferDeadline is the maximum duration (e.g. "24h", "7d") after the
	// first notification within which the user may defer. After this
	// deadline the notification auto-actions with TimeoutValue.
	DeferDeadline string `json:"deferDeadline,omitempty"`

	// MaxDefers is the maximum number of times the user may defer.
	// 0 means unlimited (until deadline).
	MaxDefers int `json:"maxDefers,omitempty"`
}

// Button represents a clickable action in the notification.
type Button struct {
	Label    string           `json:"label"`
	Value    string           `json:"value,omitempty"`
	Style    string           `json:"style"`
	Dropdown []DropdownOption `json:"dropdown,omitempty"`
}

// DropdownOption is one item in a button's dropdown menu.
type DropdownOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// MaxConfigSize is the maximum allowed JSON config payload (64 KB).
const MaxConfigSize = 64 * 1024

// LoadJSON parses raw JSON bytes into a NotificationConfig.
func LoadJSON(data []byte) (*NotificationConfig, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty config")
	}
	if len(data) > MaxConfigSize {
		return nil, fmt.Errorf("config too large: %d bytes (max %d)", len(data), MaxConfigSize)
	}
	var cfg NotificationConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}
	return &cfg, nil
}

// ApplyDefaults fills in zero-value fields with sensible defaults.
func (c *NotificationConfig) ApplyDefaults() {
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 300
	}
	if c.Title == "" {
		c.Title = "Notification"
	}
	if c.AccentColor == "" {
		c.AccentColor = "#D4AF37"
	}
	if c.EscValue == "" && c.TimeoutValue != "" {
		c.EscValue = c.TimeoutValue
	}
	for i := range c.Buttons {
		if c.Buttons[i].Style == "" {
			c.Buttons[i].Style = "secondary"
		}
		if c.Buttons[i].Value == "" && len(c.Buttons[i].Dropdown) == 0 {
			c.Buttons[i].Value = strings.ToLower(strings.ReplaceAll(c.Buttons[i].Label, " ", "_"))
		}
	}
}

// Validate checks that required fields are present and values are safe.
// All user-visible text fields are HTML-escaped as defense-in-depth
// (the frontend uses textContent, but this guards against regressions).
func (c *NotificationConfig) Validate() error {
	c.Heading = html.EscapeString(c.Heading)
	c.Message = html.EscapeString(c.Message)
	c.Title = html.EscapeString(c.Title)

	var errs []string
	if strings.TrimSpace(c.Heading) == "" {
		errs = append(errs, `"heading" is required`)
	}
	if strings.TrimSpace(c.Message) == "" {
		errs = append(errs, `"message" is required`)
	}
	for i := range c.Buttons {
		c.Buttons[i].Label = html.EscapeString(c.Buttons[i].Label)
		if strings.ContainsAny(c.Buttons[i].Value, "\n\r") {
			errs = append(errs, "button values must not contain newlines")
		}
		for j := range c.Buttons[i].Dropdown {
			c.Buttons[i].Dropdown[j].Label = html.EscapeString(c.Buttons[i].Dropdown[j].Label)
			if strings.ContainsAny(c.Buttons[i].Dropdown[j].Value, "\n\r") {
				errs = append(errs, "dropdown values must not contain newlines")
			}
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// deferRe matches "defer_Xh", "defer_Xd", "defer_Xm", "defer_Xs" where X is an integer.
var deferRe = regexp.MustCompile(`^defer_(\d+)([hdms])$`)

// ParseDeferValue extracts the duration from a defer response value like
// "defer_4h", "defer_1d", "defer_30m", "defer_30s". Returns 0 if the value is not a
// recognized defer pattern (e.g. plain "defer").
func ParseDeferValue(value string) time.Duration {
	m := deferRe.FindStringSubmatch(value)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])

	// Per-unit overflow limits for time.Duration (int64 nanoseconds, max ~292 years).
	maxSafe := map[string]int{
		"s": 9_000_000_000, // ~285 years
		"m": 150_000_000,   // ~285 years
		"h": 2_500_000,     // ~285 years
		"d": 100_000,       // ~273 years
	}
	if n > maxSafe[m[2]] {
		return 0
	}

	switch m[2] {
	case "h":
		return time.Duration(n) * time.Hour
	case "d":
		return time.Duration(n) * 24 * time.Hour
	case "m":
		return time.Duration(n) * time.Minute
	case "s":
		return time.Duration(n) * time.Second
	default:
		return 0
	}
}

// ParseDeadline parses a DeferDeadline string like "24h", "7d", "30m"
// into a time.Duration. Returns 0 if empty or unparseable.
func ParseDeadline(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Try Go stdlib first (handles "24h", "30m", "1h30m", etc.)
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	// Handle "Nd" shorthand for days.
	m := regexp.MustCompile(`^(\d+)d$`).FindStringSubmatch(s)
	if m != nil {
		n, _ := strconv.Atoi(m[1])
		// Check for overflow
		if n > 100000 { // 100k days is ~273 years
			return 0
		}
		return time.Duration(n) * 24 * time.Hour
	}
	return 0
}
