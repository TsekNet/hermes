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
	input := `{"heading":"H","message":"M","watchPaths":["/tmp/receipt","/var/log/install.log"]}`
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

