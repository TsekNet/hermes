//go:build e2e

package e2e

import (
	"testing"

	"github.com/TsekNet/hermes/internal/config"
)

func TestEncoding_HTMLEntities(t *testing.T) {
	tests := []struct {
		name    string
		heading string
		want    string
	}{
		{"ampersand", "Save &amp; Continue", "Save & Continue"},
		{"nbsp", "Hello&nbsp;World", "Hello World"},
		{"lt_gt", "Version &lt;2.0&gt;", "Version <2.0>"},
		{"quot", "Click &quot;OK&quot;", `Click "OK"`},
		{"apos", "It&#39;s ready", "It's ready"},
		{"numeric", "Copyright &#169; 2025", "Copyright © 2025"},
		{"no_entities", "Plain text heading", "Plain text heading"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.NotificationConfig{
				Heading:        tt.heading,
				Message:        "test",
				Title:          "Test",
				TimeoutSeconds: 300,
				TimeoutValue:   "dismiss",
				Buttons:        []config.Button{{Label: "OK", Value: "ok", Style: "primary"}},
			}
			cfg.ApplyDefaults()
			h := Start(t, cfg)

			if got := h.TextContent("#heading"); got != tt.want {
				t.Errorf("heading = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncoding_MessageEntities(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "Test",
		Message:        "Update from IT &amp; Security: version &lt;3.0&gt; is &quot;mandatory&quot;.",
		Title:          "Test",
		TimeoutSeconds: 300,
		TimeoutValue:   "dismiss",
		Buttons:        []config.Button{{Label: "OK", Value: "ok", Style: "primary"}},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	want := `Update from IT & Security: version <3.0> is "mandatory".`
	if got := h.TextContent("#message"); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestEncoding_ButtonLabelEntities(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "Test",
		Message:        "Test",
		Title:          "Test",
		TimeoutSeconds: 300,
		TimeoutValue:   "dismiss",
		Buttons: []config.Button{
			{Label: "Accept &amp; Continue", Value: "accept", Style: "primary"},
		},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	labels := h.ButtonLabels()
	if len(labels) != 1 || labels[0] != "Accept & Continue" {
		t.Errorf("button = %v, want [Accept & Continue]", labels)
	}
}

func TestEncoding_Unicode(t *testing.T) {
	tests := []struct {
		name    string
		heading string
	}{
		{"japanese", "再起動が必要です"},
		{"korean", "재부팅이 필요합니다"},
		{"chinese", "需要重新启动"},
		{"german", "Neustart erforderlich"},
		{"french", "Redémarrage requis"},
		{"spanish", "Reinicio requerido"},
		{"emoji", "⚠️ Critical Update 🛡️"},
		{"rtl_arabic", "إعادة التشغيل مطلوبة"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.NotificationConfig{
				Heading:        tt.heading,
				Message:        "test",
				Title:          "Test",
				TimeoutSeconds: 300,
				TimeoutValue:   "dismiss",
				Buttons:        []config.Button{{Label: "OK", Value: "ok", Style: "primary"}},
			}
			cfg.ApplyDefaults()
			h := Start(t, cfg)

			if got := h.TextContent("#heading"); got != tt.heading {
				t.Errorf("heading = %q, want %q", got, tt.heading)
			}
		})
	}
}

func TestEncoding_XSSPrevention(t *testing.T) {
	tests := []struct {
		name    string
		heading string
	}{
		{"script_tag", "<script>alert('xss')</script>"},
		{"img_onerror", "<img src=x onerror=alert(1)>"},
		{"event_handler", "<div onmouseover=alert(1)>hover</div>"},
		{"javascript_uri", "<a href='javascript:alert(1)'>click</a>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.NotificationConfig{
				Heading:        tt.heading,
				Message:        "safe message",
				Title:          "Test",
				TimeoutSeconds: 300,
				TimeoutValue:   "dismiss",
				Buttons:        []config.Button{{Label: "OK", Value: "ok", Style: "primary"}},
			}
			cfg.ApplyDefaults()
			h := Start(t, cfg)

			// The heading should render as literal text, not as HTML
			got := h.TextContent("#heading")
			if got != tt.heading {
				t.Errorf("heading = %q, want literal %q (XSS should not execute)", got, tt.heading)
			}

			// Verify no alert dialogs were triggered
			alertFired, _ := h.Page.Evaluate(`window.__xss_alert_fired || false`)
			if fired, ok := alertFired.(bool); ok && fired {
				t.Error("XSS alert was triggered")
			}
		})
	}
}

func TestEncoding_LongText(t *testing.T) {
	long := ""
	for i := 0; i < 50; i++ {
		long += "This is a very long message that should not break the layout."
		if i < 49 {
			long += " "
		}
	}

	cfg := &config.NotificationConfig{
		Heading:        "Long Message Test",
		Message:        long,
		Title:          "Test",
		TimeoutSeconds: 300,
		TimeoutValue:   "dismiss",
		Buttons:        []config.Button{{Label: "OK", Value: "ok", Style: "primary"}},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	got := h.TextContent("#message")
	if got != long {
		t.Errorf("long message length: got %d chars, want %d", len(got), len(long))
	}

	if !h.ReadyCalled() {
		t.Error("Ready() not called with long text")
	}
}

func TestEncoding_EmptyFields(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "",
		Message:        "",
		Title:          "",
		TimeoutSeconds: 60,
		TimeoutValue:   "dismiss",
		Buttons:        []config.Button{{Label: "OK", Value: "ok", Style: "primary"}},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "" {
		t.Errorf("empty heading = %q", got)
	}
	if got := h.TextContent("#message"); got != "" {
		t.Errorf("empty message = %q", got)
	}

	if !h.ReadyCalled() {
		t.Error("Ready() not called with empty fields")
	}
}

func TestEncoding_DropdownLabelEntities(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "Test",
		Message:        "Test",
		Title:          "Test",
		TimeoutSeconds: 300,
		TimeoutValue:   "dismiss",
		Buttons: []config.Button{
			{
				Label: "Options",
				Style: "secondary",
				Dropdown: []config.DropdownOption{
					{Label: "Save &amp; Quit", Value: "save_quit"},
					{Label: "Don&#39;t Save", Value: "no_save"},
				},
			},
		},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	items, err := h.Page.Locator(".dropdown-item").All()
	if err != nil {
		t.Fatalf("dropdown items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	text0, _ := items[0].TextContent()
	if text0 != "Save & Quit" {
		t.Errorf("item[0] = %q, want 'Save & Quit'", text0)
	}

	text1, _ := items[1].TextContent()
	if text1 != "Don't Save" {
		t.Errorf("item[1] = %q, want \"Don't Save\"", text1)
	}
}
