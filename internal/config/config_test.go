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
		{name: "invalid syntax", input: "heading: [\ninvalid", wantErr: true, errSubstr: "parse config"},
		{name: "array instead of object", input: `[{"heading":"H"}]`, wantErr: true, errSubstr: "parse config"},
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
				if c.Title != "IT Department" {
					t.Errorf("title = %q, want IT Department", c.Title)
				}
				if c.AccentColor != "#D4A843" {
					t.Errorf("accent = %q, want #D4A843", c.AccentColor)
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

func TestValidate_DND(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dnd     string
		wantErr bool
	}{
		{"empty (valid, defaults to respect)", "", false},
		{"respect", DNDRespect, false},
		{"ignore", DNDIgnore, false},
		{"skip", DNDSkip, false},
		{"invalid value", "yolo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{Heading: "H", Message: "M", DND: tt.dnd}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestApplyDefaults_DND(t *testing.T) {
	t.Parallel()

	t.Run("empty defaults to respect", func(t *testing.T) {
		t.Parallel()
		cfg := NotificationConfig{}
		cfg.ApplyDefaults()
		if cfg.DND != DNDRespect {
			t.Errorf("dnd = %q, want %q", cfg.DND, DNDRespect)
		}
	})

	t.Run("explicit value preserved", func(t *testing.T) {
		t.Parallel()
		cfg := NotificationConfig{DND: DNDIgnore}
		cfg.ApplyDefaults()
		if cfg.DND != DNDIgnore {
			t.Errorf("dnd = %q, want %q", cfg.DND, DNDIgnore)
		}
	})
}

func TestLoadJSON_DND(t *testing.T) {
	t.Parallel()
	input := `{"heading":"H","message":"M","dnd":"ignore"}`
	cfg, err := LoadJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DND != DNDIgnore {
		t.Errorf("dnd = %q, want %q", cfg.DND, DNDIgnore)
	}
}

func TestValidate_Images(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		images    []string
		wantErr   bool
		errSubstr string
	}{
		{name: "no images", images: nil},
		{name: "https URL", images: []string{"https://example.com/slide.png"}},
		{name: "http rejected", images: []string{"http://intranet/img.jpg"}, wantErr: true, errSubstr: "http is not allowed"},
		{name: "data URI", images: []string{"data:image/png;base64,iVBOR..."}},
		{name: "data URI case insensitive", images: []string{"Data:Image/PNG;base64,iVBOR..."}},
		{name: "svg data URI blocked", images: []string{"data:image/svg+xml;base64,PHN2Zz4="}, wantErr: true, errSubstr: "SVG data URIs"},
		{name: "svg data URI case insensitive", images: []string{"DATA:IMAGE/SVG+XML;base64,PHN2Zz4="}, wantErr: true, errSubstr: "SVG data URIs"},
		{name: "multiple valid", images: []string{
			"https://a.com/1.png",
			"data:image/jpeg;base64,/9j/4AAQ...",
			"https://b.com/2.png",
		}},
		{name: "invalid scheme", images: []string{"ftp://bad.com/x.png"}, wantErr: true, errSubstr: "images[0]"},
		{name: "bare path", images: []string{"/tmp/slide.png"}, wantErr: true, errSubstr: "images[0]"},
		{name: "mixed valid and invalid", images: []string{
			"https://ok.com/1.png",
			"file:///etc/passwd",
		}, wantErr: true, errSubstr: "images[1]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{Heading: "H", Message: "M", Images: tt.images}
			err := cfg.Validate()
			assertError(t, err, tt.wantErr, tt.errSubstr)
		})
	}
}

func TestValidate_WatchPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		paths     []string
		wantErr   bool
		errSubstr string
	}{
		{name: "nil", paths: nil},
		{name: "valid paths", paths: []string{"/tmp/receipt", "/var/log/install.log"}},
		{name: "path traversal blocked", paths: []string{"/tmp/../etc/passwd"}, wantErr: true, errSubstr: "path traversal"},
		{name: "too many paths", paths: make([]string, 11), wantErr: true, errSubstr: "exceeds maximum"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.name == "too many paths" {
				for i := range tt.paths {
					tt.paths[i] = "/tmp/watch"
				}
			}
			cfg := NotificationConfig{Heading: "H", Message: "M", WatchPaths: tt.paths}
			err := cfg.Validate()
			assertError(t, err, tt.wantErr, tt.errSubstr)
		})
	}
}

func TestResolvedDND(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dnd  string
		want string
	}{
		{"empty defaults to respect", "", DNDRespect},
		{"respect", DNDRespect, DNDRespect},
		{"ignore", DNDIgnore, DNDIgnore},
		{"skip", DNDSkip, DNDSkip},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{DND: tt.dnd}
			if got := cfg.ResolvedDND(); got != tt.want {
				t.Errorf("ResolvedDND() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidate_ImageLimit(t *testing.T) {
	t.Parallel()
	imgs := make([]string, 21)
	for i := range imgs {
		imgs[i] = "https://example.com/slide.png"
	}
	cfg := NotificationConfig{Heading: "H", Message: "M", Images: imgs}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for >20 images")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("error %q missing 'exceeds maximum'", err)
	}
}

func TestLoadJSON_Images(t *testing.T) {
	t.Parallel()
	input := `{"heading":"H","message":"M","images":["https://a.com/1.png","https://b.com/2.png"]}`
	cfg, err := LoadJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Images) != 2 {
		t.Fatalf("images len = %d, want 2", len(cfg.Images))
	}
	if cfg.Images[0] != "https://a.com/1.png" {
		t.Errorf("images[0] = %q", cfg.Images[0])
	}
}

func TestLoadJSON_WatchPaths(t *testing.T) {
	t.Parallel()
	input := `{"heading":"H","message":"M","watch_paths":["/tmp/receipt","/var/log/install.log"]}`
	cfg, err := LoadJSON([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.WatchPaths) != 2 {
		t.Fatalf("watchPaths len = %d, want 2", len(cfg.WatchPaths))
	}
	if cfg.WatchPaths[0] != "/tmp/receipt" {
		t.Errorf("watchPaths[0] = %q", cfg.WatchPaths[0])
	}
}

func TestApplyLocale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		locale      string
		headingMap  map[string]string
		messageMap  map[string]string
		wantHeading string
		wantMessage string
	}{
		{
			name:        "exact match",
			locale:      "ja",
			headingMap:  map[string]string{"ja": "再起動"},
			wantHeading: "再起動",
			wantMessage: "original",
		},
		{
			name:        "prefix match",
			locale:      "ja",
			headingMap:  map[string]string{"ja-JP": "日本語"},
			wantHeading: "日本語",
			wantMessage: "original",
		},
		{
			name:        "reverse prefix",
			locale:      "ja-jp",
			headingMap:  map[string]string{"ja": "日本語"},
			wantHeading: "日本語",
			wantMessage: "original",
		},
		{
			name:        "no match falls back",
			locale:      "fr",
			headingMap:  map[string]string{"ja": "日本語"},
			wantHeading: "original",
			wantMessage: "original",
		},
		{
			name:        "empty locale is no-op",
			locale:      "",
			headingMap:  map[string]string{"ja": "日本語"},
			wantHeading: "original",
			wantMessage: "original",
		},
		{
			name:        "both heading and message",
			locale:      "de",
			headingMap:  map[string]string{"de": "Neustart"},
			messageMap:  map[string]string{"de": "Bitte neu starten"},
			wantHeading: "Neustart",
			wantMessage: "Bitte neu starten",
		},
		{
			name:        "nil maps",
			locale:      "ja",
			headingMap:  nil,
			messageMap:  nil,
			wantHeading: "original",
			wantMessage: "original",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{
				Heading:          "original",
				Message:          "original",
				HeadingLocalized: tt.headingMap,
				MessageLocalized: tt.messageMap,
			}
			cfg.ApplyLocale(tt.locale)
			if cfg.Heading != tt.wantHeading {
				t.Errorf("heading = %q, want %q", cfg.Heading, tt.wantHeading)
			}
			if cfg.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", cfg.Message, tt.wantMessage)
			}
		})
	}
}

func TestApplyEscalation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deferCount  int
		steps       []EscalationStep
		wantTimeout int
		wantAccent  string
		wantSuffix  bool
	}{
		{
			name:        "no escalation steps",
			deferCount:  5,
			steps:       nil,
			wantTimeout: 300,
			wantAccent:  "#D4A843",
		},
		{
			name:       "zero defer count is no-op",
			deferCount: 0,
			steps: []EscalationStep{
				{AfterDefers: 1, Timeout: 60},
			},
			wantTimeout: 300,
			wantAccent:  "#D4A843",
		},
		{
			name:       "first threshold",
			deferCount: 2,
			steps: []EscalationStep{
				{AfterDefers: 2, Timeout: 120, AccentColor: "#FF6600"},
			},
			wantTimeout: 120,
			wantAccent:  "#FF6600",
		},
		{
			name:       "highest matching threshold wins",
			deferCount: 5,
			steps: []EscalationStep{
				{AfterDefers: 2, Timeout: 120, AccentColor: "#FF6600"},
				{AfterDefers: 4, Timeout: 60, AccentColor: "#FF0000", MessageSuffix: "\n\nFINAL"},
			},
			wantTimeout: 60,
			wantAccent:  "#FF0000",
			wantSuffix:  true,
		},
		{
			name:       "below all thresholds",
			deferCount: 1,
			steps: []EscalationStep{
				{AfterDefers: 2, Timeout: 120},
			},
			wantTimeout: 300,
			wantAccent:  "#D4A843",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{
				TimeoutSeconds: 300,
				AccentColor:    "#D4A843",
				Message:        "base",
				Escalation:     tt.steps,
			}
			cfg.ApplyEscalation(tt.deferCount)
			if cfg.TimeoutSeconds != tt.wantTimeout {
				t.Errorf("timeout = %d, want %d", cfg.TimeoutSeconds, tt.wantTimeout)
			}
			if cfg.AccentColor != tt.wantAccent {
				t.Errorf("accent = %q, want %q", cfg.AccentColor, tt.wantAccent)
			}
			if tt.wantSuffix && !strings.Contains(cfg.Message, "FINAL") {
				t.Errorf("message = %q, want suffix containing FINAL", cfg.Message)
			}
		})
	}
}

func TestApplyEscalation_SuffixNotDuplicated(t *testing.T) {
	t.Parallel()
	cfg := NotificationConfig{
		Message:    "base",
		Escalation: []EscalationStep{{AfterDefers: 1, MessageSuffix: " [URGENT]"}},
	}
	cfg.ApplyEscalation(2)
	cfg.ApplyEscalation(3)
	count := strings.Count(cfg.Message, "[URGENT]")
	if count != 1 {
		t.Errorf("suffix appeared %d times, want 1", count)
	}
}

func TestQuietHours_ParseTimeOfDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  int
		ok    bool
	}{
		{"00:00", 0, true},
		{"23:59", 23*60 + 59, true},
		{"07:30", 7*60 + 30, true},
		{"24:00", 0, false},
		{"12:60", 0, false},
		{"-1:00", 0, false},
		{"abc", 0, false},
		{"12", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got, ok := parseTimeOfDay(tt.input)
			if ok != tt.ok {
				t.Errorf("parseTimeOfDay(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("parseTimeOfDay(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestQuietHours_IsActive_Nil(t *testing.T) {
	t.Parallel()
	var q *QuietHours
	if q.IsActive() {
		t.Error("nil QuietHours should not be active")
	}
}

func TestQuietHours_UntilEnd_Nil(t *testing.T) {
	t.Parallel()
	var q *QuietHours
	if d := q.UntilEnd(); d != 0 {
		t.Errorf("nil QuietHours UntilEnd = %v, want 0", d)
	}
}

func TestValidate_Priority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		priority int
		wantErr  bool
	}{
		{"valid 0", 0, false},
		{"valid 5", 5, false},
		{"valid 10", 10, false},
		{"too low", -1, true},
		{"too high", 11, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{Heading: "H", Message: "M", Priority: tt.priority}
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidate_Escalation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		steps     []EscalationStep
		wantErr   bool
		errSubstr string
	}{
		{"valid", []EscalationStep{{AfterDefers: 2}}, false, ""},
		{"afterDefers 0", []EscalationStep{{AfterDefers: 0}}, true, "after_defers must be >= 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{Heading: "H", Message: "M", Escalation: tt.steps}
			err := cfg.Validate()
			assertError(t, err, tt.wantErr, tt.errSubstr)
		})
	}
}

func TestValidate_QuietHours(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		qh        *QuietHours
		wantErr   bool
		errSubstr string
	}{
		{"nil is valid", nil, false, ""},
		{"valid", &QuietHours{Start: "22:00", End: "07:00"}, false, ""},
		{"bad start", &QuietHours{Start: "bad", End: "07:00"}, true, "quiet_hours.start"},
		{"bad end", &QuietHours{Start: "22:00", End: "bad"}, true, "quiet_hours.end"},
		{"bad timezone", &QuietHours{Start: "22:00", End: "07:00", Timezone: "Fake/Zone"}, true, "quiet_hours.timezone"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{Heading: "H", Message: "M", QuietHours: tt.qh}
			err := cfg.Validate()
			assertError(t, err, tt.wantErr, tt.errSubstr)
		})
	}
}

func TestValidate_ResultActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		actions   map[string]string
		wantErr   bool
		errSubstr string
	}{
		{"nil is valid", nil, false, ""},
		{"valid cmd", map[string]string{"restart": "cmd:shutdown /r /t 60"}, false, ""},
		{"valid url", map[string]string{"wiki": "url:https://wiki.example.com"}, false, ""},
		{"valid https", map[string]string{"wiki": "https://wiki.example.com"}, false, ""},
		{"bad prefix", map[string]string{"restart": "ftp://evil.com"}, true, "result_actions"},
		{"key with newline", map[string]string{"bad\nkey": "cmd:echo"}, true, "newlines"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := NotificationConfig{Heading: "H", Message: "M", ResultActions: tt.actions}
			err := cfg.Validate()
			assertError(t, err, tt.wantErr, tt.errSubstr)
		})
	}
}

func TestValidate_DependsOnSelfRef(t *testing.T) {
	t.Parallel()
	cfg := NotificationConfig{
		Heading:   "H",
		Message:   "M",
		ID:        "my-id",
		DependsOn: "my-id",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for self-referencing dependsOn")
	}
	if !strings.Contains(err.Error(), "own ID") {
		t.Errorf("error %q missing 'own ID'", err)
	}
}

func TestApplyDefaults_Priority(t *testing.T) {
	t.Parallel()

	t.Run("zero gets default 5", func(t *testing.T) {
		t.Parallel()
		cfg := NotificationConfig{}
		cfg.ApplyDefaults()
		if cfg.Priority != 5 {
			t.Errorf("priority = %d, want 5", cfg.Priority)
		}
	})

	t.Run("explicit value preserved", func(t *testing.T) {
		t.Parallel()
		cfg := NotificationConfig{Priority: 8}
		cfg.ApplyDefaults()
		if cfg.Priority != 8 {
			t.Errorf("priority = %d, want 8", cfg.Priority)
		}
	})
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

func TestLoad_YAML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
		checkFn   func(t *testing.T, cfg *NotificationConfig)
	}{
		{
			name: "full YAML config",
			input: `
heading: Reboot Required
message: Your device requires a reboot to apply security updates.
title: IT Security
accent_color: "#E53E3E"
buttons:
  - label: Reboot Now
    value: reboot
    style: primary
  - label: Remind Me Later
    value: defer_4h
    style: secondary
timeout: 300
timeout_value: timeout
max_defers: 3
defer_deadline: 24h
`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.Heading != "Reboot Required" {
					t.Errorf("heading = %q", cfg.Heading)
				}
				if cfg.Message != "Your device requires a reboot to apply security updates." {
					t.Errorf("message = %q", cfg.Message)
				}
				if cfg.Title != "IT Security" {
					t.Errorf("title = %q", cfg.Title)
				}
				if cfg.AccentColor != "#E53E3E" {
					t.Errorf("accent_color = %q", cfg.AccentColor)
				}
				if len(cfg.Buttons) != 2 {
					t.Fatalf("buttons len = %d", len(cfg.Buttons))
				}
				if cfg.Buttons[0].Label != "Reboot Now" || cfg.Buttons[0].Value != "reboot" {
					t.Errorf("button[0] = %+v", cfg.Buttons[0])
				}
				if cfg.Buttons[1].Style != "secondary" {
					t.Errorf("button[1].style = %q", cfg.Buttons[1].Style)
				}
				if cfg.TimeoutSeconds != 300 {
					t.Errorf("timeout = %d", cfg.TimeoutSeconds)
				}
				if cfg.MaxDefers != 3 {
					t.Errorf("max_defers = %d", cfg.MaxDefers)
				}
				if cfg.DeferDeadline != "24h" {
					t.Errorf("defer_deadline = %q", cfg.DeferDeadline)
				}
			},
		},
		{
			name: "minimal YAML",
			input: `
heading: Test
message: Hello
`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.Heading != "Test" {
					t.Errorf("heading = %q", cfg.Heading)
				}
				if cfg.Message != "Hello" {
					t.Errorf("message = %q", cfg.Message)
				}
			},
		},
		{
			name: "YAML with dropdown",
			input: `
heading: Update
message: A new update is available.
buttons:
  - label: Defer
    style: secondary
    dropdown:
      - label: 1 Hour
        value: defer_1h
      - label: 4 Hours
        value: defer_4h
`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if len(cfg.Buttons) != 1 {
					t.Fatalf("buttons len = %d", len(cfg.Buttons))
				}
				if len(cfg.Buttons[0].Dropdown) != 2 {
					t.Fatalf("dropdown len = %d", len(cfg.Buttons[0].Dropdown))
				}
				if cfg.Buttons[0].Dropdown[1].Value != "defer_4h" {
					t.Errorf("dropdown[1].value = %q", cfg.Buttons[0].Dropdown[1].Value)
				}
			},
		},
		{
			name: "YAML with escalation",
			input: `
heading: Restart
message: Please restart.
escalation:
  - after_defers: 2
    timeout: 120
    accent_color: "#FF6600"
  - after_defers: 4
    timeout: 60
    accent_color: "#FF0000"
    message_suffix: " FINAL WARNING"
`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if len(cfg.Escalation) != 2 {
					t.Fatalf("escalation len = %d", len(cfg.Escalation))
				}
				if cfg.Escalation[0].AfterDefers != 2 || cfg.Escalation[0].Timeout != 120 {
					t.Errorf("escalation[0] = %+v", cfg.Escalation[0])
				}
				if cfg.Escalation[1].MessageSuffix != " FINAL WARNING" {
					t.Errorf("escalation[1].message_suffix = %q", cfg.Escalation[1].MessageSuffix)
				}
			},
		},
		{
			name: "YAML with quiet hours",
			input: `
heading: H
message: M
quiet_hours:
  start: "22:00"
  end: "07:00"
  timezone: America/Los_Angeles
`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.QuietHours == nil {
					t.Fatal("quiet_hours is nil")
				}
				if cfg.QuietHours.Start != "22:00" {
					t.Errorf("start = %q", cfg.QuietHours.Start)
				}
				if cfg.QuietHours.Timezone != "America/Los_Angeles" {
					t.Errorf("timezone = %q", cfg.QuietHours.Timezone)
				}
			},
		},
		{
			name: "JSON still works through Load",
			input: `{"heading":"H","message":"M","timeout":60}`,
			checkFn: func(t *testing.T, cfg *NotificationConfig) {
				t.Helper()
				if cfg.Heading != "H" {
					t.Errorf("heading = %q", cfg.Heading)
				}
				if cfg.TimeoutSeconds != 60 {
					t.Errorf("timeout = %d", cfg.TimeoutSeconds)
				}
			},
		},
		{name: "invalid YAML", input: "heading: [\ninvalid", wantErr: true, errSubstr: "parse config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := Load([]byte(tt.input))
			if assertError(t, err, tt.wantErr, tt.errSubstr) {
				return
			}
			if tt.checkFn != nil {
				tt.checkFn(t, cfg)
			}
		})
	}
}

