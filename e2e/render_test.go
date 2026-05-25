//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestRender_AllConfigs(t *testing.T) {
	configs := AllTestdataConfigs(t)
	if len(configs) == 0 {
		t.Fatal("no testdata configs found")
	}

	for _, name := range configs {
		t.Run(name, func(t *testing.T) {
			cfg := LoadConfig(t, name)

			var h *Harness
			if len(cfg.Images) > 0 {
				h = StartTall(t, cfg)
			} else {
				h = Start(t, cfg)
			}

			if got := h.TextContent("#heading"); got != cfg.Heading {
				t.Errorf("heading = %q, want %q", got, cfg.Heading)
			}

			if got := h.TextContent("#message"); got != cfg.Message {
				t.Errorf("message = %q, want %q", got, cfg.Message)
			}

			if got := h.TextContent("#title"); got != cfg.Title {
				t.Errorf("title = %q, want %q", got, cfg.Title)
			}

			if !h.ReadyCalled() {
				t.Error("Ready() was not called by frontend")
			}

			wantButtons := 0
			for _, btn := range cfg.Buttons {
				if len(btn.Dropdown) > 0 {
					wantButtons++
				} else {
					wantButtons++
				}
			}
			if got := h.ButtonCount(); got != wantButtons {
				t.Errorf("button count = %d, want %d", got, wantButtons)
			}
		})
	}
}

func TestRender_SimpleNotification(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Maintenance Window" {
		t.Errorf("heading = %q", got)
	}
	if got := h.TextContent("#message"); got != "Scheduled maintenance begins in 30 minutes. Please save your work." {
		t.Errorf("message = %q", got)
	}

	labels := h.ButtonLabels()
	if len(labels) != 1 || labels[0] != "Got it" {
		t.Errorf("buttons = %v, want [Got it]", labels)
	}

	if h.IsVisible("#carousel") {
		t.Error("carousel should be hidden for simple notification")
	}
	if h.IsVisible("#help-link") {
		t.Error("help link should be hidden when no help_url")
	}
}

func TestRender_RestartNotification(t *testing.T) {
	cfg := LoadConfig(t, "restart-notification.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "System Restart Required" {
		t.Errorf("heading = %q", got)
	}

	if !h.IsVisible("#help-link") {
		t.Error("help link should be visible when help_url is set")
	}

	labels := h.ButtonLabels()
	if len(labels) != 2 {
		t.Fatalf("buttons = %v, want 2", labels)
	}
	if !strings.Contains(labels[0], "Defer") {
		t.Errorf("first button = %q, want Defer dropdown", labels[0])
	}
	if labels[1] != "Restart Now" {
		t.Errorf("second button = %q, want Restart Now", labels[1])
	}
}

func TestRender_ActionChaining(t *testing.T) {
	cfg := LoadConfig(t, "action-chaining.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "VPN Disconnected" {
		t.Errorf("heading = %q", got)
	}

	labels := h.ButtonLabels()
	want := []string{"Reconnect VPN", "Open IT Wiki", "Dismiss"}
	if len(labels) != len(want) {
		t.Fatalf("buttons = %v, want %v", labels, want)
	}
	for i, w := range want {
		if labels[i] != w {
			t.Errorf("button[%d] = %q, want %q", i, labels[i], w)
		}
	}
}

func TestRender_DeferWithDropdown(t *testing.T) {
	cfg := LoadConfig(t, "defer-with-dropdown.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "VPN Disconnecting" {
		t.Errorf("heading = %q", got)
	}

	dropdowns := h.Page.Locator(".dropdown-wrapper")
	count, err := dropdowns.Count()
	if err != nil {
		t.Fatalf("dropdown count: %v", err)
	}
	if count != 1 {
		t.Errorf("dropdown count = %d, want 1", count)
	}

	items := h.Page.Locator(".dropdown-item")
	itemCount, _ := items.Count()
	if itemCount != 3 {
		t.Errorf("dropdown items = %d, want 3", itemCount)
	}
}

func TestRender_EscalationRestart(t *testing.T) {
	cfg := LoadConfig(t, "escalation-restart.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Restart Required" {
		t.Errorf("heading = %q", got)
	}
	if !h.IsVisible("#help-link") {
		t.Error("help link should be visible")
	}
}

func TestRender_ImageCarousel(t *testing.T) {
	cfg := LoadConfig(t, "image-carousel.json")
	h := StartTall(t, cfg)

	if got := h.TextContent("#heading"); got != "macOS 15.3 Update Available" {
		t.Errorf("heading = %q", got)
	}

	if !h.IsVisible("#carousel") {
		t.Error("carousel should be visible with images")
	}

	indicator := h.TextContent("#carousel-indicator")
	if indicator != "1 / 3" {
		t.Errorf("indicator = %q, want %q", indicator, "1 / 3")
	}

	images := h.Page.Locator("#carousel-track img")
	imgCount, _ := images.Count()
	if imgCount != 3 {
		t.Errorf("carousel images = %d, want 3", imgCount)
	}
}

func TestRender_InstallWithWatch(t *testing.T) {
	cfg := LoadConfig(t, "install-with-watch.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Security Agent Installation" {
		t.Errorf("heading = %q", got)
	}

	if !h.IsVisible("#watch-status") {
		t.Error("watch status should be visible when watch_paths set")
	}
	if got := h.TextContent("#watch-status"); got != "Monitoring filesystem..." {
		t.Errorf("watch status = %q, want %q", got, "Monitoring filesystem...")
	}
}

func TestRender_LocalizedRestart(t *testing.T) {
	cfg := LoadConfig(t, "localized-restart.json")
	cfg.ApplyLocale("ja")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "再起動が必要です" {
		t.Errorf("heading = %q, want Japanese heading", got)
	}
	if got := h.TextContent("#message"); got != "セキュリティアップデートを適用するため、コンピューターを再起動してください。" {
		t.Errorf("message = %q, want Japanese message", got)
	}
}

func TestRender_PriorityCritical(t *testing.T) {
	cfg := LoadConfig(t, "priority-critical.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Critical Security Patch" {
		t.Errorf("heading = %q", got)
	}
	if got := h.TextContent("#title"); got != "IT Department - CRITICAL" {
		t.Errorf("title = %q", got)
	}

	labels := h.ButtonLabels()
	if len(labels) != 1 || labels[0] != "Restart Now" {
		t.Errorf("buttons = %v, want [Restart Now]", labels)
	}

	// Verify danger button style
	btnClass, err := h.Page.Locator("#buttons .btn").First().GetAttribute("class")
	if err != nil {
		t.Fatalf("button class: %v", err)
	}
	if !strings.Contains(btnClass, "btn-danger") {
		t.Errorf("button class = %q, want btn-danger", btnClass)
	}
}

func TestRender_QuietHours(t *testing.T) {
	cfg := LoadConfig(t, "quiet-hours.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Software Update Available" {
		t.Errorf("heading = %q", got)
	}

	labels := h.ButtonLabels()
	if len(labels) != 2 {
		t.Fatalf("buttons = %v, want 2", labels)
	}
	if labels[0] != "Update Now" {
		t.Errorf("button[0] = %q, want Update Now", labels[0])
	}
}

func TestRender_ShortDeferDeadline(t *testing.T) {
	cfg := LoadConfig(t, "short-defer-deadline.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "VPN Maintenance" {
		t.Errorf("heading = %q", got)
	}
	if !h.IsVisible("#help-link") {
		t.Error("help link should be visible")
	}

	labels := h.ButtonLabels()
	if len(labels) != 2 {
		t.Fatalf("buttons = %v", labels)
	}
	if labels[0] != "Defer 15s" {
		t.Errorf("button[0] = %q", labels[0])
	}

	btnClass, _ := h.Page.Locator("#buttons .btn").Nth(1).GetAttribute("class")
	if !strings.Contains(btnClass, "btn-danger") {
		t.Errorf("Disconnect Now should be danger style, got %q", btnClass)
	}
}

func TestRender_ShortDeferRestart(t *testing.T) {
	cfg := LoadConfig(t, "short-defer-restart.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Restart Required" {
		t.Errorf("heading = %q", got)
	}

	dropdowns := h.Page.Locator(".dropdown-wrapper")
	count, _ := dropdowns.Count()
	if count != 1 {
		t.Errorf("dropdown count = %d, want 1", count)
	}

	items := h.Page.Locator(".dropdown-item")
	itemCount, _ := items.Count()
	if itemCount != 3 {
		t.Errorf("dropdown items = %d, want 3 (10s, 30s, 1m)", itemCount)
	}
}

func TestRender_UpdateNotification(t *testing.T) {
	cfg := LoadConfig(t, "update-notification.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "Software Update Available" {
		t.Errorf("heading = %q", got)
	}
	if h.IsVisible("#carousel") {
		t.Error("carousel should be hidden (no images in update-notification)")
	}

	labels := h.ButtonLabels()
	if len(labels) != 2 {
		t.Fatalf("buttons = %v, want 2", labels)
	}
	if labels[0] != "Remind Me Later" {
		t.Errorf("button[0] = %q, want Remind Me Later", labels[0])
	}
}

func TestRender_WorkflowStep1EULA(t *testing.T) {
	cfg := LoadConfig(t, "workflow-step1-eula.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "End User License Agreement" {
		t.Errorf("heading = %q", got)
	}

	labels := h.ButtonLabels()
	if len(labels) != 2 {
		t.Fatalf("buttons = %v, want 2", labels)
	}
	if labels[0] != "Accept" {
		t.Errorf("button[0] = %q, want Accept", labels[0])
	}
}

func TestRender_WorkflowStep2Update(t *testing.T) {
	cfg := LoadConfig(t, "workflow-step2-update.json")
	h := Start(t, cfg)

	if got := h.TextContent("#heading"); got != "System Update Ready" {
		t.Errorf("heading = %q", got)
	}

	labels := h.ButtonLabels()
	if len(labels) != 2 {
		t.Fatalf("buttons = %v, want 2", labels)
	}
	if labels[0] != "Install & Restart" {
		t.Errorf("button[0] = %q, want Install & Restart", labels[0])
	}
}

func TestRender_AccentColor(t *testing.T) {
	cfg := LoadConfig(t, "priority-critical.json")
	h := Start(t, cfg)

	// Red accent (#FF0000) should be applied as CSS variable
	val, err := h.Page.Evaluate(`getComputedStyle(document.documentElement).getPropertyValue('--accent').trim()`)
	if err != nil {
		t.Fatalf("get accent: %v", err)
	}
	accent, ok := val.(string)
	if !ok || accent == "" {
		t.Fatal("accent color not set")
	}
	// Chromium normalizes hex to rgb
	if accent != "#FF0000" && accent != "rgb(255, 0, 0)" {
		t.Errorf("accent = %q, want #FF0000 or rgb(255, 0, 0)", accent)
	}
}

func TestRender_CountdownFormat(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	countdown := h.TextContent("#countdown")
	if !strings.HasPrefix(countdown, "Auto-action in ") {
		t.Errorf("countdown = %q, want prefix 'Auto-action in '", countdown)
	}
	// 60s timeout should show 0:59 (one tick has passed by render time)
	if !strings.Contains(countdown, "0:5") && !strings.Contains(countdown, "1:00") {
		t.Errorf("countdown = %q, want ~0:59 or 1:00 for 60s timeout", countdown)
	}
}

func TestRender_CountdownAccessibility(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	cd := h.Page.Locator("#countdown")

	role, err := cd.GetAttribute("role")
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	if role != "progressbar" {
		t.Errorf("role = %q, want progressbar", role)
	}

	ariaMin, _ := cd.GetAttribute("aria-valuemin")
	if ariaMin != "0" {
		t.Errorf("aria-valuemin = %q, want 0", ariaMin)
	}

	ariaMax, _ := cd.GetAttribute("aria-valuemax")
	if ariaMax != "60" {
		t.Errorf("aria-valuemax = %q, want 60 (timeout seconds)", ariaMax)
	}

	ariaText, _ := cd.GetAttribute("aria-valuetext")
	if !strings.HasPrefix(ariaText, "Auto-action in ") {
		t.Errorf("aria-valuetext = %q", ariaText)
	}
}
