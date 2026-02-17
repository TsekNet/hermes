package manager

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSubmit_PersistsToDisk(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	mgr := New(nil, s)

	cfg := testConfig("Persisted")
	id, _ := mgr.Submit(cfg)

	records, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID != id {
		t.Errorf("ID: got %q, want %q", records[0].ID, id)
	}
}

func TestComplete_RemovesFromDisk(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	mgr := New(nil, s)

	cfg := testConfig("Will Complete")
	id, _ := mgr.Submit(cfg)
	mgr.ReportChoice(id, "ok")

	// Give the async delete a moment.
	time.Sleep(50 * time.Millisecond)

	records, _ := s.LoadAll()
	if len(records) != 0 {
		t.Fatalf("expected 0 records after completion, got %d", len(records))
	}
}

func TestDefer_UpdatesDisk(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	mgr := New(func(n *Notification) {}, s)

	cfg := testConfig("Deferrable")
	cfg.DeferDeadline = "24h"
	id, _ := mgr.Submit(cfg)
	mgr.ReportChoice(id, "defer:1h")

	records, _ := s.LoadAll()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].DeferCount != 1 {
		t.Errorf("DeferCount: got %d, want 1", records[0].DeferCount)
	}
	if records[0].State != "deferred" {
		t.Errorf("State: got %q, want %q", records[0].State, "deferred")
	}
}

func TestRestore_ReshowsFiredNotifications(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "restore.db")

	// Simulate a prior service run: save a deferred notification directly.
	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s1.Save(&store.Record{
		ID: "old-notif",
		Config: &config.NotificationConfig{
			Heading:      "Reboot",
			Message:      "Please reboot",
			TimeoutValue: "auto_reboot",
		},
		State:      "deferred",
		DeferCount: 1,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
		Deadline:   time.Now().Add(22 * time.Hour),
	})
	s1.Close()

	// Reopen and restore.
	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	reshowIDs := make(chan string, 1)
	mgr := New(func(n *Notification) {
		reshowIDs <- n.ID
	}, s2)

	restored := mgr.Restore()
	if restored != 1 {
		t.Fatalf("expected 1 restored, got %d", restored)
	}

	select {
	case id := <-reshowIDs:
		if id != "old-notif" {
			t.Errorf("reshown ID: got %q, want %q", id, "old-notif")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for reshow callback")
	}
}

func TestRestore_ExpiredDeadline_AutoActions(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "expired.db")

	// Save a notification whose deadline already passed.
	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	s1.Save(&store.Record{
		ID: "expired",
		Config: &config.NotificationConfig{
			Heading:      "Old",
			Message:      "Too late",
			TimeoutValue: "auto_action",
		},
		State:      "deferred",
		DeferCount: 3,
		CreatedAt:  time.Now().Add(-48 * time.Hour),
		Deadline:   time.Now().Add(-1 * time.Hour), // already passed
	})
	s1.Close()

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	mgr := New(nil, s2)
	restored := mgr.Restore()
	if restored != 0 {
		t.Errorf("expected 0 restored (expired), got %d", restored)
	}

	// Should have been deleted from disk.
	records, _ := s2.LoadAll()
	if len(records) != 0 {
		t.Errorf("expected 0 records on disk after expired auto-action, got %d", len(records))
	}
}
