package config

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// NotificationConfig holds all parameters for a notification dialog.
type NotificationConfig struct {
	Heading        string   `json:"heading" yaml:"heading"`
	Message        string   `json:"message" yaml:"message"`
	Buttons        []Button `json:"buttons" yaml:"buttons"`
	TimeoutSeconds int      `json:"timeout" yaml:"timeout"`
	TimeoutValue   string   `json:"timeout_value" yaml:"timeout_value"`
	EscValue       string   `json:"esc_value" yaml:"esc_value"`
	Title          string   `json:"title" yaml:"title"`
	AccentColor    string   `json:"accent_color" yaml:"accent_color"`
	HelpURL        string   `json:"help_url" yaml:"help_url"`
	Platform       string   `json:"platform" yaml:"platform"`
	// ID uniquely identifies this notification for the service daemon.
	// Auto-generated if empty when submitted via gRPC.
	ID string `json:"id,omitempty" yaml:"id,omitempty"`

	// DeferDeadline is the maximum duration (e.g. "24h", "7d") after the
	// first notification within which the user may defer. After this
	// deadline the notification auto-actions with TimeoutValue.
	DeferDeadline string `json:"defer_deadline,omitempty" yaml:"defer_deadline,omitempty"`

	// MaxDefers is the maximum number of times the user may defer.
	// 0 means unlimited (until deadline).
	MaxDefers int `json:"max_defers,omitempty" yaml:"max_defers,omitempty"`

	// Images is an ordered list of image URLs or base64 data URIs to display
	// in a carousel above the message. When present the window auto-sizes
	// taller to accommodate the images.
	Images []string `json:"images,omitempty" yaml:"images,omitempty"`

	// WatchPaths is a list of filesystem paths to monitor for changes.
	// When a watched path is created, modified, or deleted, the frontend
	// receives an event via the Wails binding. Useful for validating
	// installations (e.g. watch for a receipt file to appear).
	WatchPaths []string `json:"watch_paths,omitempty" yaml:"watch_paths,omitempty"`

	// DND controls behavior when the OS Do Not Disturb / Focus mode is active.
	//   "respect" (default) — wait and retry every 60s until DND is off.
	//   "ignore"  — show the notification regardless (use for critical alerts).
	//   "skip"    — silently complete with value "dnd_active" when DND is on.
	DND string `json:"dnd,omitempty" yaml:"dnd,omitempty"`

	// Priority controls delivery order when multiple notifications are pending.
	// Range: 0 (low) to 10 (critical). Default: 5 (applied by ApplyDefaults
	// when the field is zero/omitted). Higher priority notifications are shown
	// first during queue drain.
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`

	// Escalation defines progressive urgency steps applied as the user
	// repeatedly defers. Each threshold mutates the notification's appearance
	// and timing when the defer count meets or exceeds AfterDefers.
	Escalation []EscalationStep `json:"escalation,omitempty" yaml:"escalation,omitempty"`

	// ResultActions maps response values to automatic follow-up actions.
	// Keys are button values (e.g. "restart"), values use the same prefix
	// scheme as buttons: "cmd:shutdown /r /t 60", "url:https://...".
	// The action is dispatched server-side after the notification completes.
	ResultActions map[string]string `json:"result_actions,omitempty" yaml:"result_actions,omitempty"`

	// QuietHours suppresses notification delivery during specified hours.
	// The manager delays delivery until the quiet window ends (like DND).
	QuietHours *QuietHours `json:"quiet_hours,omitempty" yaml:"quiet_hours,omitempty"`

	// HeadingLocalized maps BCP-47 language codes to localized heading text.
	// At runtime, the resolved locale (--locale flag or OS detection)
	// selects the best match. Falls back to Heading if no match.
	HeadingLocalized map[string]string `json:"heading_localized,omitempty" yaml:"heading_localized,omitempty"`

	// MessageLocalized maps BCP-47 language codes to localized message text.
	MessageLocalized map[string]string `json:"message_localized,omitempty" yaml:"message_localized,omitempty"`

	// DependsOn is the ID of a notification that must complete before this
	// one is shown. The manager holds this notification in a waiting state
	// until the dependency is satisfied.
	DependsOn string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
}

// EscalationStep defines a mutation applied to the notification after
// a certain number of deferrals.
type EscalationStep struct {
	// AfterDefers is the minimum defer count that activates this step.
	AfterDefers int `json:"after_defers" yaml:"after_defers"`
	// Timeout overrides TimeoutSeconds (shorter = more urgent).
	Timeout int `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// AccentColor overrides the notification's accent color.
	AccentColor string `json:"accent_color,omitempty" yaml:"accent_color,omitempty"`
	// MessageSuffix is appended to the message (e.g. urgency warning).
	MessageSuffix string `json:"message_suffix,omitempty" yaml:"message_suffix,omitempty"`
}

// QuietHours defines a daily window during which notifications are delayed.
type QuietHours struct {
	// Start is the beginning of the quiet window in "HH:MM" 24-hour format.
	Start string `json:"start" yaml:"start"`
	// End is the end of the quiet window in "HH:MM" 24-hour format.
	End string `json:"end" yaml:"end"`
	// Timezone is an IANA timezone string (e.g. "America/Los_Angeles").
	// Defaults to local time if empty.
	Timezone string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
}

// IsActive reports whether the current time falls within the quiet window.
func (q *QuietHours) IsActive() bool {
	if q == nil {
		return false
	}
	loc := time.Local
	if q.Timezone != "" {
		var err error
		loc, err = time.LoadLocation(q.Timezone)
		if err != nil {
			return false
		}
	}
	now := time.Now().In(loc)
	start, sok := parseTimeOfDay(q.Start)
	end, eok := parseTimeOfDay(q.End)
	if !sok || !eok {
		return false
	}
	cur := now.Hour()*60 + now.Minute()
	if start <= end {
		return cur >= start && cur < end
	}
	// Overnight range (e.g. 22:00–07:00).
	return cur >= start || cur < end
}

// UntilEnd returns how long until the quiet window ends.
// Returns 0 if quiet hours are not active.
func (q *QuietHours) UntilEnd() time.Duration {
	if q == nil || !q.IsActive() {
		return 0
	}
	loc := time.Local
	if q.Timezone != "" {
		if l, err := time.LoadLocation(q.Timezone); err == nil {
			loc = l
		}
	}
	now := time.Now().In(loc)
	end, ok := parseTimeOfDay(q.End)
	if !ok {
		return 0
	}
	endTime := time.Date(now.Year(), now.Month(), now.Day(), end/60, end%60, 0, 0, loc)
	if endTime.Before(now) {
		endTime = endTime.Add(24 * time.Hour)
	}
	return endTime.Sub(now)
}

func parseTimeOfDay(s string) (int, bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, false
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// DND mode constants.
const (
	DNDRespect = "respect"
	DNDIgnore  = "ignore"
	DNDSkip    = "skip"
)

// ResolvedDND returns the effective DND mode, defaulting to DNDRespect.
func (c *NotificationConfig) ResolvedDND() string {
	if c.DND == "" {
		return DNDRespect
	}
	return c.DND
}

// Button represents a clickable action in the notification.
type Button struct {
	Label    string           `json:"label" yaml:"label"`
	Value    string           `json:"value,omitempty" yaml:"value,omitempty"`
	Style    string           `json:"style" yaml:"style"`
	Dropdown []DropdownOption `json:"dropdown,omitempty" yaml:"dropdown,omitempty"`
}

// DropdownOption is one item in a button's dropdown menu.
type DropdownOption struct {
	Label string `json:"label" yaml:"label"`
	Value string `json:"value" yaml:"value"`
}

// MaxConfigSize is the maximum allowed config payload (64 KB).
const MaxConfigSize = 64 * 1024

// Load parses raw JSON or YAML bytes into a NotificationConfig.
func Load(data []byte) (*NotificationConfig, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty config")
	}
	if len(data) > MaxConfigSize {
		return nil, fmt.Errorf("config too large: %d bytes (max %d)", len(data), MaxConfigSize)
	}
	var cfg NotificationConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// LoadJSON is a backward-compatible alias for Load.
// Deprecated: Use Load instead.
func LoadJSON(data []byte) (*NotificationConfig, error) { return Load(data) }

// ApplyDefaults fills in zero-value fields with sensible defaults.
func (c *NotificationConfig) ApplyDefaults() {
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 300
	}
	if c.Title == "" {
		c.Title = "IT Department"
	}
	if c.AccentColor == "" {
		c.AccentColor = "#D4A843"
	}
	if c.EscValue == "" && c.TimeoutValue != "" {
		c.EscValue = c.TimeoutValue
	}
	if c.DND == "" {
		c.DND = DNDRespect
	}
	if c.Priority == 0 {
		c.Priority = 5
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

// ApplyLocale replaces Heading and Message with localized versions if
// a match exists for the given locale. Tries exact match first ("ja"),
// then prefix match ("ja" matches "ja-JP" key), then falls back to
// the original text.
func (c *NotificationConfig) ApplyLocale(locale string) {
	if locale == "" {
		return
	}
	locale = strings.ToLower(locale)
	if v := matchLocale(c.HeadingLocalized, locale); v != "" {
		c.Heading = v
	}
	if v := matchLocale(c.MessageLocalized, locale); v != "" {
		c.Message = v
	}
}

func matchLocale(m map[string]string, locale string) string {
	if len(m) == 0 {
		return ""
	}
	// Exact match.
	if v, ok := m[locale]; ok {
		return v
	}
	// Prefix match: "ja" matches "ja-jp", "ja_JP".
	for k, v := range m {
		if strings.HasPrefix(strings.ToLower(k), locale) {
			return v
		}
	}
	// Reverse prefix: "ja-JP" locale matches "ja" key.
	prefix := strings.SplitN(locale, "-", 2)[0]
	prefix = strings.SplitN(prefix, "_", 2)[0]
	if prefix != locale {
		if v, ok := m[prefix]; ok {
			return v
		}
	}
	return ""
}

// ApplyEscalation mutates the config based on the current defer count.
// The highest matching threshold wins. This is called by the manager
// before re-showing a deferred notification.
func (c *NotificationConfig) ApplyEscalation(deferCount int) {
	if len(c.Escalation) == 0 || deferCount == 0 {
		return
	}
	var active *EscalationStep
	for i := range c.Escalation {
		if deferCount >= c.Escalation[i].AfterDefers {
			active = &c.Escalation[i]
		}
	}
	if active == nil {
		return
	}
	if active.Timeout > 0 {
		c.TimeoutSeconds = active.Timeout
	}
	if active.AccentColor != "" {
		c.AccentColor = active.AccentColor
	}
	if active.MessageSuffix != "" && !strings.HasSuffix(c.Message, active.MessageSuffix) {
		c.Message += active.MessageSuffix
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
	if c.DND != "" && c.DND != DNDRespect && c.DND != DNDIgnore && c.DND != DNDSkip {
		errs = append(errs, fmt.Sprintf(`"dnd" must be %q, %q, or %q`, DNDRespect, DNDIgnore, DNDSkip))
	}
	if len(c.Images) > 20 {
		errs = append(errs, fmt.Sprintf("images: %d exceeds maximum of 20", len(c.Images)))
	}
	for i, img := range c.Images {
		lower := strings.ToLower(img)
		switch {
		case strings.HasPrefix(lower, "data:image/svg"):
			errs = append(errs, fmt.Sprintf("images[%d]: SVG data URIs are not allowed", i))
		case strings.HasPrefix(lower, "data:image/"):
			// valid raster data URI
		default:
			u, err := url.Parse(img)
			if err != nil || u.Scheme != "https" {
				errs = append(errs, fmt.Sprintf("images[%d]: must be an https URL or data:image/ URI (http is not allowed)", i))
			}
		}
	}
	if len(c.WatchPaths) > 10 {
		errs = append(errs, fmt.Sprintf("watch_paths: %d exceeds maximum of 10", len(c.WatchPaths)))
	}
	for i, p := range c.WatchPaths {
		if strings.Contains(p, "..") {
			errs = append(errs, fmt.Sprintf("watch_paths[%d]: path traversal (..) is not allowed", i))
		}
	}
	if c.Priority < 0 || c.Priority > 10 {
		errs = append(errs, fmt.Sprintf(`"priority" must be 0-10, got %d`, c.Priority))
	}
	for i, step := range c.Escalation {
		if step.AfterDefers < 1 {
			errs = append(errs, fmt.Sprintf("escalation[%d]: after_defers must be >= 1", i))
		}
	}
	if q := c.QuietHours; q != nil {
		if _, ok := parseTimeOfDay(q.Start); !ok {
			errs = append(errs, `quiet_hours.start must be "HH:MM" format`)
		}
		if _, ok := parseTimeOfDay(q.End); !ok {
			errs = append(errs, `quiet_hours.end must be "HH:MM" format`)
		}
		if q.Timezone != "" {
			if _, err := time.LoadLocation(q.Timezone); err != nil {
				errs = append(errs, fmt.Sprintf("quiet_hours.timezone: %v", err))
			}
		}
	}
	if len(c.ResultActions) > 20 {
		errs = append(errs, fmt.Sprintf("result_actions: %d entries exceeds maximum of 20", len(c.ResultActions)))
	}
	for k, v := range c.ResultActions {
		if strings.ContainsAny(k, "\n\r") {
			errs = append(errs, fmt.Sprintf("result_actions key %q: must not contain newlines", k))
		}
		lower := strings.ToLower(v)
		if !strings.HasPrefix(lower, "cmd:") && !strings.HasPrefix(lower, "url:") &&
			!strings.HasPrefix(lower, "https:") && !strings.HasPrefix(lower, "http:") {
			errs = append(errs, fmt.Sprintf("result_actions[%q]: value must start with cmd:, url:, https:, or http:", k))
		}
	}
	if c.DependsOn != "" && c.DependsOn == c.ID {
		errs = append(errs, `"depends_on" must not reference the notification's own ID`)
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
