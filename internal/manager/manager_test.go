package manager

import (
	"sync"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/exitcodes"
)

func testConfig(heading string) *config.NotificationConfig {
	return &config.NotificationConfig{
		Heading:        heading,
		Message:        "Test message",
		TimeoutSeconds: 10,
		TimeoutValue:   "auto_action",
	}
}

func TestSubmit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		cfgID  string
		wantID string // empty = any non-empty generated ID
	}{
		{name: "generates ID", cfgID: "", wantID: ""},
		{name: "uses provided ID", cfgID: "custom-123", wantID: "custom-123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr := New(nil, nil)
			cfg := testConfig("Submit")
			cfg.ID = tt.cfgID
			id, ch := mgr.Submit(cfg)

			if tt.wantID != "" && id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
			if id == "" {
				t.Error("expected non-empty ID")
			}
			if ch == nil {
				t.Error("expected non-nil result channel")
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      string
		wantOK  bool
		heading string
	}{
		{name: "found", id: "", wantOK: true, heading: "GetConfig"},
		{name: "not found", id: "nonexistent", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr := New(nil, nil)

			lookupID := tt.id
			if tt.wantOK {
				cfg := testConfig(tt.heading)
				lookupID, _ = mgr.Submit(cfg)
			}

			got, ok := mgr.GetConfig(lookupID)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && got.Heading != tt.heading {
				t.Errorf("heading = %q, want %q", got.Heading, tt.heading)
			}
		})
	}
}

func TestReportChoice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		wantVal  string
		wantCode int32
	}{
		{name: "ok action", value: "restart", wantVal: "restart", wantCode: exitcodes.OK},
		{name: "timeout", value: "timeout:restart", wantVal: "timeout:restart", wantCode: exitcodes.Timeout},
		{name: "bare timeout", value: "timeout", wantVal: "timeout", wantCode: exitcodes.Timeout},
		{name: "defer", value: "defer_4h", wantVal: "defer_4h", wantCode: exitcodes.Deferred},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			onReshow := func(n *Notification) {} // no-op
			if tt.wantCode == exitcodes.Deferred {
				// Deferred: don't complete, just verify state
				mgr := New(onReshow, nil)
				cfg := testConfig(tt.name)
				cfg.DeferDeadline = "24h"
				id, _ := mgr.Submit(cfg)

				ok := mgr.ReportChoice(id, tt.value)
				if !ok {
					t.Fatal("ReportChoice returned false")
				}
				infos := mgr.List()
				for _, info := range infos {
					if info.ID == id && info.State != StateDeferred {
						t.Errorf("state = %q, want deferred", info.State)
					}
				}
				return
			}

			mgr := New(nil, nil)
			id, ch := mgr.Submit(testConfig(tt.name))
			ok := mgr.ReportChoice(id, tt.value)
			if !ok {
				t.Fatal("ReportChoice returned false")
			}
			select {
			case r := <-ch:
				if r.Value != tt.wantVal {
					t.Errorf("value = %q, want %q", r.Value, tt.wantVal)
				}
				if r.ExitCode != tt.wantCode {
					t.Errorf("exitCode = %d, want %d", r.ExitCode, tt.wantCode)
				}
			case <-time.After(time.Second):
				t.Fatal("timeout waiting for result")
			}
		})
	}
}

func TestReportChoice_MaxDefers(t *testing.T) {
	t.Parallel()
	mgr := New(func(n *Notification) {}, nil)

	cfg := testConfig("MaxDefers")
	cfg.DeferDeadline = "24h"
	cfg.MaxDefers = 2
	id, ch := mgr.Submit(cfg)

	mgr.ReportChoice(id, "defer_1m")
	mgr.ReportChoice(id, "defer_1m")
	mgr.ReportChoice(id, "defer_1m") // third triggers auto-action

	select {
	case result := <-ch:
		if result.Value != "auto_action" {
			t.Errorf("value = %q, want auto_action", result.Value)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for auto-action after max defers")
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		exists bool
		want   bool
	}{
		{name: "found", exists: true, want: true},
		{name: "not found", exists: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr := New(nil, nil)

			id := "nonexistent"
			var ch <-chan Result
			if tt.exists {
				id, ch = mgr.Submit(testConfig("Cancel"))
			}

			got := mgr.Cancel(id)
			if got != tt.want {
				t.Errorf("Cancel = %v, want %v", got, tt.want)
			}

			if tt.exists {
				select {
				case r := <-ch:
					if r.Value != "cancelled" {
						t.Errorf("value = %q, want cancelled", r.Value)
					}
				case <-time.After(time.Second):
					t.Fatal("timeout")
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
	}{
		{name: "empty", count: 0},
		{name: "two items", count: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr := New(nil, nil)
			for i := 0; i < tt.count; i++ {
				mgr.Submit(testConfig("List"))
			}
			if got := len(mgr.List()); got != tt.count {
				t.Errorf("list len = %d, want %d", got, tt.count)
			}
		})
	}
}

func TestDeferralAllowed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		maxDefers int
		defer1st  bool // send one defer before checking
		want      bool
	}{
		{name: "no deadline", maxDefers: 0, want: true},
		{name: "max reached", maxDefers: 1, defer1st: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mgr := New(func(n *Notification) {}, nil)

			cfg := testConfig(tt.name)
			cfg.MaxDefers = tt.maxDefers
			cfg.DeferDeadline = "24h"
			id, _ := mgr.Submit(cfg)

			if tt.defer1st {
				mgr.ReportChoice(id, "defer_1h")
			}

			if got := mgr.DeferralAllowed(id); got != tt.want {
				t.Errorf("DeferralAllowed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrentReportChoice(t *testing.T) {
	t.Parallel()
	mgr := New(nil, nil)
	id, ch := mgr.Submit(testConfig("Concurrent"))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.ReportChoice(id, "ok")
		}()
	}
	wg.Wait()

	select {
	case result := <-ch:
		if result.Value != "ok" {
			t.Errorf("value = %q", result.Value)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestSubmit_MaxActive(t *testing.T) {
	t.Parallel()
	mgr := New(nil, nil)

	// Fill to capacity.
	for i := 0; i < MaxActiveNotifications; i++ {
		id, ch := mgr.Submit(testConfig("fill"))
		if id == "" || ch == nil {
			t.Fatalf("submit %d failed unexpectedly", i)
		}
	}

	// Next submit should be rejected.
	id, ch := mgr.Submit(testConfig("overflow"))
	if id != "" {
		t.Errorf("expected empty ID, got %q", id)
	}
	select {
	case r := <-ch:
		if r.ExitCode != exitcodes.Error {
			t.Errorf("exit code = %d, want %d", r.ExitCode, exitcodes.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestSubmit_DuplicateID(t *testing.T) {
	t.Parallel()
	mgr := New(nil, nil)

	cfg := testConfig("dup")
	cfg.ID = "dup-id-1"
	id1, _ := mgr.Submit(cfg)
	if id1 != "dup-id-1" {
		t.Fatalf("first id = %q", id1)
	}

	// Second submit with same ID should be rejected.
	cfg2 := testConfig("dup2")
	cfg2.ID = "dup-id-1"
	id2, ch := mgr.Submit(cfg2)
	if id2 != "" {
		t.Errorf("expected empty ID for dup, got %q", id2)
	}
	select {
	case r := <-ch:
		if r.ExitCode != exitcodes.Error {
			t.Errorf("exit code = %d, want %d", r.ExitCode, exitcodes.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestDND_IgnoreShowsImmediately(t *testing.T) {
	t.Parallel()
	var shown bool
	mgr := New(func(n *Notification) { shown = true }, nil)
	mgr.dndChecker = func() bool { return true }

	cfg := testConfig("DND Ignore")
	cfg.DND = config.DNDIgnore
	mgr.Submit(cfg)

	time.Sleep(200 * time.Millisecond)
	if !shown {
		t.Error("notification should have been shown immediately with dnd=ignore")
	}
}

func TestDND_RespectDefaultApplied(t *testing.T) {
	t.Parallel()
	cfg := testConfig("DND Default")
	cfg.ApplyDefaults()
	if cfg.DND != config.DNDRespect {
		t.Errorf("dnd = %q, want %q", cfg.DND, config.DNDRespect)
	}
}

func TestDND_SkipWhenDNDInactive(t *testing.T) {
	t.Parallel()
	var shown bool
	mgr := New(func(n *Notification) { shown = true }, nil)
	mgr.dndChecker = func() bool { return false }

	cfg := testConfig("DND Skip")
	cfg.DND = config.DNDSkip
	mgr.Submit(cfg)

	time.Sleep(200 * time.Millisecond)
	if !shown {
		t.Error("notification should have been shown when DND is inactive (skip mode)")
	}
}

func TestDND_SkipWhenDNDActive(t *testing.T) {
	t.Parallel()
	var shown bool
	mgr := New(func(n *Notification) { shown = true }, nil)
	mgr.dndChecker = func() bool { return true }

	cfg := testConfig("DND Skip Active")
	cfg.DND = config.DNDSkip
	_, ch := mgr.Submit(cfg)

	select {
	case r := <-ch:
		if r.Value != "dnd_active" {
			t.Errorf("value = %q, want dnd_active", r.Value)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dnd_active result")
	}
	if shown {
		t.Error("notification should NOT have been shown when DND is active (skip mode)")
	}
}

func TestDND_RespectWaitsForDNDClear(t *testing.T) {
	t.Parallel()
	DNDPollInterval = 10 * time.Millisecond

	var calls int
	var mu sync.Mutex
	mgr := New(func(n *Notification) {
		mu.Lock()
		calls++
		mu.Unlock()
	}, nil)

	pollCount := 0
	mgr.dndChecker = func() bool {
		mu.Lock()
		defer mu.Unlock()
		pollCount++
		return pollCount <= 3
	}

	cfg := testConfig("DND Respect Wait")
	cfg.DND = config.DNDRespect
	mgr.Submit(cfg)

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("onReshow called %d times, want 1", calls)
	}
	if pollCount < 4 {
		t.Errorf("dndChecker polled %d times, want >= 4", pollCount)
	}
}

func TestExitCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  int32
	}{
		{"restart", exitcodes.OK},
		{"ok", exitcodes.OK},
		{"defer_4h", exitcodes.Deferred},
		{"defer", exitcodes.Deferred},
		{"timeout:restart", exitcodes.Timeout},
		{"timeout", exitcodes.Timeout},
		{"cancelled", exitcodes.Error},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()
			got := exitcodes.ForValue(tt.value)
			if got != tt.want {
				t.Errorf("ForValue(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}
