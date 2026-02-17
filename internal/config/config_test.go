package config

import (
	"strings"
	"testing"
)

// assertError checks err against wantErr. Returns true if the test case is done
// (caller should return). If substr is non-empty, verifies the error message.
func assertError(t *testing.T, err error, wantErr bool, substr string) bool {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if substr != "" && !strings.Contains(err.Error(), substr) {
			t.Errorf("error %q missing substring %q", err, substr)
		}
		return true
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return false
}

func TestLoadJSON(t *testing.T) {
	t.Parallel()

	bigPayload := `{"heading":"H","message":"` + strings.Repeat("x", MaxConfigSize) + `"}`

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
		checkFn   func(t *testing.T, cfg *NotificationConfig)
	}{
		{name: "too large", input: bigPayload, wantErr: true, errSubstr: "config too large"},
		{
			name:  "full config",
			input: `{"heading":"Reboot","message":"Please restart","timeout":120,"buttons":[{"label":"OK","value":"ok","style":"primary"}]}`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.Heading != "Reboot" {
					t.Errorf("heading = %q", cfg.Heading)
				}
				if cfg.TimeoutSeconds != 120 {
					t.Errorf("timeout = %d", cfg.TimeoutSeconds)
				}
				if len(cfg.Buttons) != 1 {
					t.Fatalf("buttons len = %d", len(cfg.Buttons))
				}
				if cfg.Buttons[0].Label != "OK" {
					t.Errorf("button label = %q", cfg.Buttons[0].Label)
				}
			},
		},
		{
			name:  "minimal config",
			input: `{"heading":"H","message":"M"}`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.Heading != "H" {
					t.Errorf("heading = %q", cfg.Heading)
				}
			},
		},
		{
			name:  "with dropdown",
			input: `{"heading":"H","message":"M","buttons":[{"label":"Defer","style":"secondary","dropdown":[{"label":"1 Hour","value":"defer_1h"},{"label":"4 Hours","value":"defer_4h"}]}]}`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if len(cfg.Buttons) != 1 {
					t.Fatalf("buttons len = %d", len(cfg.Buttons))
				}
				if len(cfg.Buttons[0].Dropdown) != 2 {
					t.Fatalf("dropdown len = %d", len(cfg.Buttons[0].Dropdown))
				}
				if cfg.Buttons[0].Dropdown[0].Value != "defer_1h" {
					t.Errorf("dropdown[0].Value = %q", cfg.Buttons[0].Dropdown[0].Value)
				}
			},
		},
		{
			name:  "whitespace padded",
			input: `  {"heading":"H","message":"M"}  `,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.Heading != "H" {
					t.Errorf("heading = %q", cfg.Heading)
				}
			},
		},
		{name: "empty input", input: "", wantErr: true, errSubstr: "empty config"},
		{name: "whitespace only", input: "   ", wantErr: true, errSubstr: "empty config"},
		{name: "invalid JSON", input: "{not valid}", wantErr: true, errSubstr: "parse config JSON"},
		{name: "array instead of object", input: `[{"heading":"H"}]`, wantErr: true, errSubstr: "parse config JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := LoadJSON([]byte(tt.input))
			if assertError(t, err, tt.wantErr, tt.errSubstr) {
				return
			}
			if tt.checkFn != nil {
				tt.checkFn(t, cfg)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       NotificationConfig
		wantErr   bool
		errSubstr string
	}{
		{name: "valid", cfg: NotificationConfig{Heading: "Title", Message: "Body"}},
		{name: "missing heading", cfg: NotificationConfig{Message: "hello"}, wantErr: true, errSubstr: `"heading"`},
		{name: "missing message", cfg: NotificationConfig{Heading: "hi"}, wantErr: true, errSubstr: `"message"`},
		{name: "both missing", cfg: NotificationConfig{}, wantErr: true},
		{name: "whitespace heading", cfg: NotificationConfig{Heading: "   ", Message: "ok"}, wantErr: true},
		{name: "whitespace message", cfg: NotificationConfig{Heading: "ok", Message: "\t"}, wantErr: true},
		{
			name: "button value with newline",
			cfg: NotificationConfig{
				Heading: "H", Message: "M",
				Buttons: []Button{{Label: "Bad", Value: "line1\nline2"}},
			},
			wantErr:   true,
			errSubstr: "button values must not contain newlines",
		},
		{
			name: "dropdown value with carriage return",
			cfg: NotificationConfig{
				Heading: "H", Message: "M",
				Buttons: []Button{{Label: "D", Dropdown: []DropdownOption{{Label: "X", Value: "a\rb"}}}},
			},
			wantErr:   true,
			errSubstr: "dropdown values must not contain newlines",
		},
		{
			name: "valid buttons pass",
			cfg: NotificationConfig{
				Heading: "H", Message: "M",
				Buttons: []Button{{Label: "OK", Value: "ok"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if assertError(t, err, tt.wantErr, tt.errSubstr) {
				return
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   NotificationConfig
		checkFn func(t *testing.T, c *NotificationConfig)
	}{
		{
			name:  "zero values get defaults",
			input: NotificationConfig{},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.TimeoutSeconds != 300 {
					t.Errorf("timeout = %d, want 300", c.TimeoutSeconds)
				}
				if c.Title != "Notification" {
					t.Errorf("title = %q, want Notification", c.Title)
				}
				if c.AccentColor != "#D4AF37" {
					t.Errorf("accent = %q, want #D4AF37", c.AccentColor)
				}
			},
		},
		{
			name:  "existing values preserved",
			input: NotificationConfig{TimeoutSeconds: 60, Title: "IT Dept", AccentColor: "#FF0000"},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.TimeoutSeconds != 60 {
					t.Errorf("timeout = %d, want 60", c.TimeoutSeconds)
				}
				if c.Title != "IT Dept" {
					t.Errorf("title = %q, want IT Dept", c.Title)
				}
				if c.AccentColor != "#FF0000" {
					t.Errorf("accent = %q, want #FF0000", c.AccentColor)
				}
			},
		},
		{
			name:  "esc falls back to timeout value",
			input: NotificationConfig{TimeoutValue: "restart"},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.EscValue != "restart" {
					t.Errorf("esc = %q, want restart", c.EscValue)
				}
			},
		},
		{
			name:  "explicit esc not overridden",
			input: NotificationConfig{TimeoutValue: "restart", EscValue: "dismiss"},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.EscValue != "dismiss" {
					t.Errorf("esc = %q, want dismiss", c.EscValue)
				}
			},
		},
		{
			name: "button style defaults to secondary",
			input: NotificationConfig{
				Buttons: []Button{{Label: "OK"}},
			},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.Buttons[0].Style != "secondary" {
					t.Errorf("style = %q, want secondary", c.Buttons[0].Style)
				}
			},
		},
		{
			name: "button value derived from label",
			input: NotificationConfig{
				Buttons: []Button{{Label: "Restart Now"}},
			},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.Buttons[0].Value != "restart_now" {
					t.Errorf("value = %q, want restart_now", c.Buttons[0].Value)
				}
			},
		},
		{
			name: "explicit button value not overridden",
			input: NotificationConfig{
				Buttons: []Button{{Label: "OK", Value: "confirmed"}},
			},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.Buttons[0].Value != "confirmed" {
					t.Errorf("value = %q, want confirmed", c.Buttons[0].Value)
				}
			},
		},
		{
			name: "dropdown button keeps empty value",
			input: NotificationConfig{
				Buttons: []Button{{Label: "Defer", Dropdown: []DropdownOption{{Label: "1h", Value: "d1"}}}},
			},
			checkFn: func(t *testing.T, c *NotificationConfig) {
				t.Helper()
				if c.Buttons[0].Value != "" {
					t.Errorf("dropdown button value = %q, want empty", c.Buttons[0].Value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := tt.input
			cfg.ApplyDefaults()
			tt.checkFn(t, &cfg)
		})
	}
}

func TestValidate_HTMLEscaping(t *testing.T) {
	t.Parallel()
	cfg := NotificationConfig{
		Heading: `<script>alert("xss")</script>`,
		Message: `<img src=x onerror=alert(1)>`,
		Title:   `<b>bold</b>`,
		Buttons: []Button{
			{
				Label: `<a href="evil">click</a>`,
				Value: "ok",
				Dropdown: []DropdownOption{
					{Label: `<em>defer</em>`, Value: "defer_1h"},
				},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(cfg.Heading, "<script>") {
		t.Error("heading not escaped")
	}
	if strings.Contains(cfg.Message, "<img") {
		t.Error("message not escaped")
	}
	if strings.Contains(cfg.Title, "<b>") {
		t.Error("title not escaped")
	}
	if strings.Contains(cfg.Buttons[0].Label, "<a") {
		t.Error("button label not escaped")
	}
	if strings.Contains(cfg.Buttons[0].Dropdown[0].Label, "<em>") {
		t.Error("dropdown label not escaped")
	}
}

