package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/store"
)

func TestHistoryEntryFromRecord(t *testing.T) {
	t.Parallel()

	created := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	completed := time.Date(2025, 6, 15, 10, 5, 0, 0, time.UTC)

	r := &store.HistoryRecord{
		ID: "h-42",
		Config: &config.NotificationConfig{
			Heading: "Reboot Required",
			Message: "Please restart your machine.",
			Title:   "IT Notifications",
		},
		ResponseValue: "restart",
		CreatedAt:     created,
		CompletedAt:   completed,
	}

	got := HistoryEntryFromRecord(r)

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ID", got.ID, "h-42"},
		{"Heading", got.Heading, "Reboot Required"},
		{"Message", got.Message, "Please restart your machine."},
		{"Source", got.Source, "IT Notifications"},
		{"ResponseValue", got.ResponseValue, "restart"},
		{"CreatedAt", got.CreatedAt, created.Format(time.RFC3339)},
		{"CompletedAt", got.CompletedAt, completed.Format(time.RFC3339)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestHistoryEntryFromClientEntry_Completed(t *testing.T) {
	t.Parallel()

	created := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)
	completed := time.Date(2025, 7, 1, 12, 30, 0, 0, time.UTC)

	e := client.HistoryEntry{
		ID:             "ce-1",
		Heading:        "Update Available",
		Message:        "Version 2.0",
		Source:         "Software Updates",
		ResponseValue:  "dismiss",
		CreatedAt:      created,
		CompletedAt:    completed,
		ActionRequired: false,
	}

	got := HistoryEntryFromClientEntry(e)

	if got.ActionRequired {
		t.Error("ActionRequired should be false for completed entry")
	}
	if len(got.Buttons) != 0 {
		t.Errorf("Buttons = %d, want 0 for completed entry", len(got.Buttons))
	}
	if got.Heading != "Update Available" {
		t.Errorf("Heading = %q, want %q", got.Heading, "Update Available")
	}
}

func TestHistoryEntryFromClientEntry_ActiveWithButtons(t *testing.T) {
	t.Parallel()

	cfg := config.NotificationConfig{
		Heading: "Reboot",
		Message: "Reboot needed",
		Buttons: []config.Button{
			{Label: "Restart Now", Value: "restart", Style: "primary"},
			{Label: "Later", Value: "defer:1h"},
		},
	}
	cfgJSON, _ := json.Marshal(cfg)

	e := client.HistoryEntry{
		ID:             "active-1",
		Heading:        "Reboot",
		Message:        "Reboot needed",
		ActionRequired: true,
		ConfigJSON:     cfgJSON,
		CreatedAt:      time.Now(),
	}

	got := HistoryEntryFromClientEntry(e)

	if !got.ActionRequired {
		t.Error("ActionRequired should be true")
	}
	if len(got.Buttons) != 2 {
		t.Fatalf("Buttons = %d, want 2", len(got.Buttons))
	}
	if got.Buttons[0].Label != "Restart Now" {
		t.Errorf("Buttons[0].Label = %q, want %q", got.Buttons[0].Label, "Restart Now")
	}
	if got.Buttons[0].Style != "primary" {
		t.Errorf("Buttons[0].Style = %q, want %q", got.Buttons[0].Style, "primary")
	}
	if got.Buttons[1].Value != "defer:1h" {
		t.Errorf("Buttons[1].Value = %q, want %q", got.Buttons[1].Value, "defer:1h")
	}
}

func TestHistoryEntryFromClientEntry_ActiveEmptyConfigJSON(t *testing.T) {
	t.Parallel()

	e := client.HistoryEntry{
		ID:             "active-2",
		ActionRequired: true,
		ConfigJSON:     nil,
		CreatedAt:      time.Now(),
	}

	got := HistoryEntryFromClientEntry(e)

	if len(got.Buttons) != 0 {
		t.Errorf("Buttons = %d, want 0 when ConfigJSON is nil", len(got.Buttons))
	}
}

func TestHistoryEntryFromClientEntry_ActiveInvalidConfigJSON(t *testing.T) {
	t.Parallel()

	e := client.HistoryEntry{
		ID:             "active-3",
		ActionRequired: true,
		ConfigJSON:     []byte("{invalid json"),
		CreatedAt:      time.Now(),
	}

	got := HistoryEntryFromClientEntry(e)

	if len(got.Buttons) != 0 {
		t.Errorf("Buttons = %d, want 0 when ConfigJSON is invalid", len(got.Buttons))
	}
}

func TestNewHistory(t *testing.T) {
	t.Parallel()
	app := NewHistory(nil)
	if app.grpcClient != nil {
		t.Error("grpcClient should be nil when passed nil")
	}
}

func TestNewHistoryLocal(t *testing.T) {
	t.Parallel()
	app := NewHistoryLocal(nil)
	if app.localStore != nil {
		t.Error("localStore should be nil when passed nil")
	}
}

func TestGetHistory_NilStore(t *testing.T) {
	t.Parallel()
	app := &HistoryApp{}
	got := app.GetHistory()
	if got != nil {
		t.Errorf("GetHistory = %v, want nil with no client or store", got)
	}
}

func TestRespondToNotification_NoClient(t *testing.T) {
	t.Parallel()
	app := &HistoryApp{}
	got := app.RespondToNotification("id", "ok")
	if got != "error: no service connection" {
		t.Errorf("got %q, want 'error: no service connection'", got)
	}
}

func TestHistoryFromStore(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "history-store.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	now := time.Now()
	s.SaveHistory(&store.HistoryRecord{
		ID:            "hs-1",
		Config:        &config.NotificationConfig{Heading: "Valid", Message: "body"},
		ResponseValue: "ok",
		CompletedAt:   now,
	})
	// Record with nil config should be skipped.
	s.SaveHistory(&store.HistoryRecord{
		ID:          "hs-nil",
		Config:      nil,
		CompletedAt: now.Add(-time.Hour),
	})

	app := NewHistoryLocal(s)
	entries := app.GetHistory()

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (nil config skipped), got %d", len(entries))
	}
	if entries[0].Heading != "Valid" {
		t.Errorf("heading = %q, want Valid", entries[0].Heading)
	}
}

func TestHistoryFromStore_NilStore(t *testing.T) {
	t.Parallel()
	app := &HistoryApp{}
	entries := app.historyFromStore()
	if entries != nil {
		t.Errorf("expected nil with no store, got %d entries", len(entries))
	}
}

func TestRunAction_URIPrefix(t *testing.T) {
	t.Parallel()
	app := &HistoryApp{}

	got := app.RunAction("id", "some-non-uri-value")
	if got != "some-non-uri-value" {
		t.Errorf("RunAction non-URI = %q, want passthrough", got)
	}
}

func TestHistoryApp_Startup(t *testing.T) {
	t.Parallel()
	app := &HistoryApp{}
	ctx := context.Background()
	app.Startup(ctx)
	if app.ctx != ctx {
		t.Error("ctx not set")
	}
}

func TestHistoryApp_Shutdown_NilFields(t *testing.T) {
	t.Parallel()
	app := &HistoryApp{}
	app.Shutdown(context.Background())
}
