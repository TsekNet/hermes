//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
)

func TestInteraction_ButtonClick(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	h.Page.Locator("#buttons .btn").First().Click()
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "dismiss" {
		t.Errorf("respond calls = %v, want [dismiss]", calls)
	}
}

func TestInteraction_MultipleButtonsCorrectValue(t *testing.T) {
	cfg := LoadConfig(t, "action-chaining.json")
	h := Start(t, cfg)

	// Click "Open IT Wiki" (second button, value="wiki")
	h.Page.Locator("#buttons .btn").Nth(1).Click()
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "wiki" {
		t.Errorf("respond calls = %v, want [wiki]", calls)
	}
}

func TestInteraction_DropdownClick(t *testing.T) {
	cfg := LoadConfig(t, "defer-with-dropdown.json")
	h := Start(t, cfg)

	// Dropdown menu should not have 'open' class initially
	isOpen, _ := h.Page.Evaluate(`document.querySelector('.dropdown-menu').classList.contains('open')`)
	if open, ok := isOpen.(bool); ok && open {
		t.Error("dropdown menu should not be open initially")
	}

	// Click the dropdown trigger to open
	h.Page.Locator(".dropdown-wrapper .btn").First().Click()
	time.Sleep(200 * time.Millisecond)

	isOpen, _ = h.Page.Evaluate(`document.querySelector('.dropdown-menu').classList.contains('open')`)
	if open, ok := isOpen.(bool); !ok || !open {
		t.Error("dropdown menu should have 'open' class after click")
	}

	// Click "1 Hour" (value=defer_1h)
	h.Page.Locator(".dropdown-item").Nth(1).Click()
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "defer_1h" {
		t.Errorf("respond calls = %v, want [defer_1h]", calls)
	}
}

func TestInteraction_DropdownDismissOnOutsideClick(t *testing.T) {
	cfg := LoadConfig(t, "defer-with-dropdown.json")
	h := Start(t, cfg)

	// Open dropdown
	h.Page.Locator(".dropdown-wrapper .btn").First().Click()
	time.Sleep(200 * time.Millisecond)

	isOpen, _ := h.Page.Evaluate(`document.querySelector('.dropdown-menu').classList.contains('open')`)
	if open, ok := isOpen.(bool); !ok || !open {
		t.Fatal("dropdown should be open")
	}

	// Click outside to dismiss dropdown
	h.Page.Mouse().Click(10, 10)
	time.Sleep(200 * time.Millisecond)

	isOpen, _ = h.Page.Evaluate(`document.querySelector('.dropdown-menu').classList.contains('open')`)
	if open, ok := isOpen.(bool); ok && open {
		t.Error("dropdown should close on outside click")
	}
}

func TestInteraction_EscapeKey(t *testing.T) {
	cfg := LoadConfig(t, "restart-notification.json")
	h := Start(t, cfg)

	h.Page.Keyboard().Press("Escape")
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "defer_1h" {
		t.Errorf("ESC respond = %v, want [defer_1h] (esc_value)", calls)
	}
}

func TestInteraction_EnterKey(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	h.Page.Keyboard().Press("Enter")
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "dismiss" {
		t.Errorf("Enter respond = %v, want [dismiss] (primary button value)", calls)
	}
}

func TestInteraction_DoubleClickGuard(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	btn := h.Page.Locator("#buttons .btn").First()
	btn.Click()
	btn.Click()
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 {
		t.Errorf("double click produced %d respond calls, want 1 (guard should prevent duplicate)", len(calls))
	}
}

func TestInteraction_CountdownTimeout(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "Short Timeout",
		Message:        "This expires fast.",
		Title:          "Test",
		TimeoutSeconds: 2,
		TimeoutValue:   "auto_dismiss",
		Buttons: []config.Button{
			{Label: "OK", Value: "ok", Style: "primary"},
		},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	// Wait for timeout + buffer
	time.Sleep(3 * time.Second)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "timeout:auto_dismiss" {
		t.Errorf("timeout respond = %v, want [timeout:auto_dismiss]", calls)
	}
}

func TestInteraction_CountdownStopsOnClick(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "Click Before Timeout",
		Message:        "Click the button before it times out.",
		Title:          "Test",
		TimeoutSeconds: 5,
		TimeoutValue:   "auto",
		Buttons: []config.Button{
			{Label: "Act", Value: "acted", Style: "primary"},
		},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	h.Page.Locator("#buttons .btn").First().Click()
	time.Sleep(6 * time.Second)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "acted" {
		t.Errorf("calls = %v, want [acted] (timeout should not fire after click)", calls)
	}
}

func TestInteraction_CarouselNavigation(t *testing.T) {
	cfg := LoadConfig(t, "image-carousel.json")
	h := StartTall(t, cfg)

	if got := h.TextContent("#carousel-indicator"); got != "1 / 3" {
		t.Errorf("initial indicator = %q, want 1 / 3", got)
	}

	// Click next
	h.Page.Locator("#carousel-next").Click()
	time.Sleep(100 * time.Millisecond)

	if got := h.TextContent("#carousel-indicator"); got != "2 / 3" {
		t.Errorf("after next = %q, want 2 / 3", got)
	}

	// Click next again
	h.Page.Locator("#carousel-next").Click()
	time.Sleep(100 * time.Millisecond)

	if got := h.TextContent("#carousel-indicator"); got != "3 / 3" {
		t.Errorf("after next x2 = %q, want 3 / 3", got)
	}

	// Click next wraps to 1
	h.Page.Locator("#carousel-next").Click()
	time.Sleep(100 * time.Millisecond)

	if got := h.TextContent("#carousel-indicator"); got != "1 / 3" {
		t.Errorf("wrap around = %q, want 1 / 3", got)
	}

	// Click prev wraps to 3
	h.Page.Locator("#carousel-prev").Click()
	time.Sleep(100 * time.Millisecond)

	if got := h.TextContent("#carousel-indicator"); got != "3 / 3" {
		t.Errorf("prev wrap = %q, want 3 / 3", got)
	}
}

func TestInteraction_CarouselArrowKeys(t *testing.T) {
	cfg := LoadConfig(t, "image-carousel.json")
	h := StartTall(t, cfg)

	h.Page.Keyboard().Press("ArrowRight")
	time.Sleep(100 * time.Millisecond)

	if got := h.TextContent("#carousel-indicator"); got != "2 / 3" {
		t.Errorf("ArrowRight = %q, want 2 / 3", got)
	}

	h.Page.Keyboard().Press("ArrowLeft")
	time.Sleep(100 * time.Millisecond)

	if got := h.TextContent("#carousel-indicator"); got != "1 / 3" {
		t.Errorf("ArrowLeft = %q, want 1 / 3", got)
	}
}

func TestInteraction_HelpLink(t *testing.T) {
	cfg := LoadConfig(t, "restart-notification.json")
	h := Start(t, cfg)

	h.Page.Locator("#help-link").Click()
	time.Sleep(100 * time.Millisecond)

	if !h.OpenHelpCalled() {
		t.Error("OpenHelp should be called when help link is clicked")
	}
}

func TestInteraction_URIButtonNoClose(t *testing.T) {
	cfg := LoadConfig(t, "install-with-watch.json")
	h := Start(t, cfg)

	// "Install" button has value "uri:https://..." which should call Respond
	// but NOT set responded=true (URI buttons don't close the notification)
	h.Page.Locator("#buttons .btn").First().Click()
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 {
		t.Fatalf("URI button respond calls = %d, want 1", len(calls))
	}
	if calls[0] != "uri:https://intranet.example.com/install-agent" {
		t.Errorf("URI value = %q", calls[0])
	}

	// The notification should still be active (responded guard not set for URI)
	// Click the Dismiss button to verify it still works
	h.Page.Locator("#buttons .btn").Nth(1).Click()
	time.Sleep(100 * time.Millisecond)

	calls = h.RespondCalls()
	if len(calls) != 2 {
		t.Errorf("after dismiss, calls = %d, want 2 (URI + dismiss)", len(calls))
	}
}

func TestInteraction_DangerButtonStyling(t *testing.T) {
	cfg := LoadConfig(t, "defer-with-dropdown.json")
	h := Start(t, cfg)

	// "Disconnect Now" is style: "danger"
	disconnectBtn := h.Page.Locator("#buttons .btn-danger")
	count, _ := disconnectBtn.Count()
	if count != 1 {
		t.Errorf("danger buttons = %d, want 1", count)
	}

	text, _ := disconnectBtn.First().TextContent()
	if text != "Disconnect Now" {
		t.Errorf("danger button text = %q, want Disconnect Now", text)
	}
}

func TestInteraction_ProgressBar(t *testing.T) {
	cfg := LoadConfig(t, "simple-notification.json")
	h := Start(t, cfg)

	// Progress bar should have non-zero scaleX initially
	transform, err := h.Page.Evaluate(`
		getComputedStyle(document.getElementById('bar-fill')).transform
	`)
	if err != nil {
		t.Fatalf("bar-fill transform: %v", err)
	}
	if transform == "none" || transform == "" {
		t.Error("progress bar should have a transform initially")
	}
}

func TestInteraction_EscFallsBackToTimeout(t *testing.T) {
	// When esc_value is not set, ESC should use timeout_value
	cfg := &config.NotificationConfig{
		Heading:        "No ESC Value",
		Message:        "ESC should use timeout value.",
		Title:          "Test",
		TimeoutSeconds: 300,
		TimeoutValue:   "fallback_dismiss",
		Buttons: []config.Button{
			{Label: "OK", Value: "ok", Style: "primary"},
		},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	h.Page.Keyboard().Press("Escape")
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "fallback_dismiss" {
		t.Errorf("ESC with no esc_value = %v, want [fallback_dismiss]", calls)
	}
}

func TestInteraction_EnterSelectsPrimaryNotDanger(t *testing.T) {
	// Enter should select .btn-primary, even when danger buttons exist
	cfg := &config.NotificationConfig{
		Heading:        "Mixed Styles",
		Message:        "Test",
		Title:          "Test",
		TimeoutSeconds: 300,
		TimeoutValue:   "timeout",
		Buttons: []config.Button{
			{Label: "Safe", Value: "safe", Style: "primary"},
			{Label: "Danger", Value: "danger", Style: "danger"},
		},
	}
	cfg.ApplyDefaults()
	h := Start(t, cfg)

	h.Page.Keyboard().Press("Enter")
	time.Sleep(100 * time.Millisecond)

	calls := h.RespondCalls()
	if len(calls) != 1 || calls[0] != "safe" {
		t.Errorf("Enter = %v, want [safe] (primary, not danger)", calls)
	}
}

func TestInteraction_WatchStatusVisible(t *testing.T) {
	cfg := LoadConfig(t, "install-with-watch.json")
	h := Start(t, cfg)

	ws := h.Page.Locator("#watch-status")
	visible, _ := ws.IsVisible()
	if !visible {
		t.Error("watch-status should be visible when watch_paths configured")
	}

	text, _ := ws.TextContent()
	if text != "Monitoring filesystem..." {
		t.Errorf("watch-status = %q, want 'Monitoring filesystem...'", text)
	}
}

func TestInteraction_SingleImageNoControls(t *testing.T) {
	cfg := &config.NotificationConfig{
		Heading:        "Single Image",
		Message:        "Only one image, no controls.",
		Title:          "Test",
		TimeoutSeconds: 60,
		TimeoutValue:   "dismiss",
		Images:         []string{"data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIxMCIgaGVpZ2h0PSIxMCI+PC9zdmc+"},
		Buttons: []config.Button{
			{Label: "OK", Value: "ok", Style: "primary"},
		},
	}
	cfg.ApplyDefaults()
	h := StartTall(t, cfg)

	if !h.IsVisible("#carousel") {
		t.Error("carousel should be visible with one image")
	}

	// Controls (prev/next) should be hidden for single image
	controls := h.Page.Locator("#carousel-controls")
	visible, _ := controls.IsVisible()
	if visible {
		t.Error("carousel controls should be hidden for single image")
	}

	images := h.Page.Locator("#carousel-track img")
	count, _ := images.Count()
	if count != 1 {
		t.Errorf("images = %d, want 1", count)
	}
}
