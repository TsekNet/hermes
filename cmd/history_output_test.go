package cmd

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/TsekNet/hermes/internal/app"
)

func TestWriteHistorySummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entries  []app.HistoryEntry
		contains []string
		absent   []string
	}{
		{
			name:     "empty entries",
			entries:  nil,
			contains: []string{"No notification history."},
		},
		{
			name: "action required entry shows star marker",
			entries: []app.HistoryEntry{
				{ID: "abc123", Heading: "Reboot Required", ActionRequired: true},
			},
			contains: []string{"*", "Reboot Required", "action required", "abc123"},
		},
		{
			name: "completed entry shows response and date",
			entries: []app.HistoryEntry{
				{ID: "def456", Heading: "VPN Update", ResponseValue: "ok", CreatedAt: "2025-06-15T14:30:00Z"},
			},
			contains: []string{"def456", "VPN Update", "ok"},
		},
		{
			name: "mixed entries show footer hint",
			entries: []app.HistoryEntry{
				{ID: "abc123", Heading: "Reboot Required", ActionRequired: true},
				{ID: "def456", Heading: "VPN Update", ResponseValue: "ok", CreatedAt: "2025-06-15T14:30:00Z"},
			},
			contains: []string{"2 notification(s)", "hermes list", "hermes respond"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := writeHistorySummary(&buf, tt.entries)
			if err != nil {
				t.Fatalf("writeHistorySummary: %v", err)
			}
			out := buf.String()
			for _, s := range tt.contains {
				if !strings.Contains(out, s) {
					t.Errorf("output missing %q\ngot:\n%s", s, out)
				}
			}
			for _, s := range tt.absent {
				if strings.Contains(out, s) {
					t.Errorf("output should not contain %q\ngot:\n%s", s, out)
				}
			}
		})
	}
}

func TestWriteHistoryJSON(t *testing.T) {
	t.Parallel()

	t.Run("empty entries", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		err := writeHistoryJSON(&buf, []app.HistoryEntry{})
		if err != nil {
			t.Fatalf("writeHistoryJSON: %v", err)
		}
		var got []app.HistoryEntry
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v\nraw: %s", err, buf.String())
		}
		if len(got) != 0 {
			t.Errorf("expected 0 entries, got %d", len(got))
		}
	})

	t.Run("populated entries round-trip", func(t *testing.T) {
		t.Parallel()
		entries := []app.HistoryEntry{
			{ID: "1", Heading: "Reboot", Message: "Restart now", Source: "IT", ResponseValue: "ok", CreatedAt: "2025-06-15T14:30:00Z"},
			{ID: "2", Heading: "Update", Message: "Install v2", Source: "Fleet", ResponseValue: "deferred", CreatedAt: "2025-06-15T13:00:00Z"},
		}
		var buf bytes.Buffer
		err := writeHistoryJSON(&buf, entries)
		if err != nil {
			t.Fatalf("writeHistoryJSON: %v", err)
		}
		var got []app.HistoryEntry
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2, got %d", len(got))
		}
		if got[0].Heading != "Reboot" {
			t.Errorf("first heading = %q, want Reboot", got[0].Heading)
		}
		if got[1].ResponseValue != "deferred" {
			t.Errorf("second response = %q, want deferred", got[1].ResponseValue)
		}
	})

	t.Run("action_required and buttons serialize", func(t *testing.T) {
		t.Parallel()
		entries := []app.HistoryEntry{
			{
				ID: "3", Heading: "Pending", ActionRequired: true,
				Buttons: []app.HistoryButton{
					{Label: "Restart", Value: "action:reboot", Style: "danger"},
					{Label: "Defer", Value: "defer:1h", Style: "secondary"},
				},
			},
		}
		var buf bytes.Buffer
		if err := writeHistoryJSON(&buf, entries); err != nil {
			t.Fatalf("writeHistoryJSON: %v", err)
		}
		var got []app.HistoryEntry
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if !got[0].ActionRequired {
			t.Error("action_required should be true")
		}
		if len(got[0].Buttons) != 2 {
			t.Fatalf("expected 2 buttons, got %d", len(got[0].Buttons))
		}
		if got[0].Buttons[0].Label != "Restart" {
			t.Errorf("button 0 label = %q, want Restart", got[0].Buttons[0].Label)
		}
	})
}

func TestHasDisplay(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hasDisplay always returns true on windows, env vars irrelevant")
	}

	envVars := []string{"DISPLAY", "WAYLAND_DISPLAY", "TERM_PROGRAM"}

	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no display vars", map[string]string{}, false},
		{"DISPLAY set", map[string]string{"DISPLAY": ":0"}, true},
		{"WAYLAND_DISPLAY set", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, true},
	}

	if runtime.GOOS == "darwin" {
		tests = append(tests, struct {
			name string
			env  map[string]string
			want bool
		}{"TERM_PROGRAM set (macOS)", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, true})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range envVars {
				t.Setenv(key, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := hasDisplay(); got != tt.want {
				t.Errorf("hasDisplay() = %v, want %v (env: %v)", got, tt.want, tt.env)
			}
		})
	}
}
