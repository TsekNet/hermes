package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/store"
)

func TestFetchHistoryFromDB(t *testing.T) {
	t.Parallel()

	t.Run("returns entries from store", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "test.db")
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}

		now := time.Now()
		s.SaveHistory(&store.HistoryRecord{
			ID:            "h-1",
			Config:        &config.NotificationConfig{Heading: "Reboot", Message: "Please reboot"},
			ResponseValue: "restart",
			CreatedAt:     now.Add(-1 * time.Hour),
			CompletedAt:   now,
		})
		s.SaveHistory(&store.HistoryRecord{
			ID:            "h-2",
			Config:        &config.NotificationConfig{Heading: "Update", Message: "Install update"},
			ResponseValue: "ok",
			CreatedAt:     now.Add(-2 * time.Hour),
			CompletedAt:   now.Add(-30 * time.Minute),
		})
		s.Close()

		entries, err := fetchHistoryFromDB(dbPath)
		if err != nil {
			t.Fatalf("fetchHistoryFromDB: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].Heading != "Reboot" {
			t.Errorf("first entry heading = %q, want Reboot (newest first)", entries[0].Heading)
		}
	})

	t.Run("empty database returns empty slice", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "empty.db")
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		s.Close()

		entries, err := fetchHistoryFromDB(dbPath)
		if err != nil {
			t.Fatalf("fetchHistoryFromDB: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(entries))
		}
	})

	t.Run("skips records with nil config", func(t *testing.T) {
		t.Parallel()
		dbPath := filepath.Join(t.TempDir(), "nilcfg.db")
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}

		now := time.Now()
		s.SaveHistory(&store.HistoryRecord{
			ID:          "h-ok",
			Config:      &config.NotificationConfig{Heading: "Valid"},
			CompletedAt: now,
		})
		// Manually save a record with nil config (corrupt data scenario).
		s.SaveHistory(&store.HistoryRecord{
			ID:          "h-nil",
			Config:      nil,
			CompletedAt: now.Add(-time.Hour),
		})
		s.Close()

		entries, err := fetchHistoryFromDB(dbPath)
		if err != nil {
			t.Fatalf("fetchHistoryFromDB: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry (nil config skipped), got %d", len(entries))
		}
		if entries[0].Heading != "Valid" {
			t.Errorf("heading = %q, want Valid", entries[0].Heading)
		}
	})

	t.Run("invalid db path returns error", func(t *testing.T) {
		t.Parallel()
		_, err := fetchHistoryFromDB("/nonexistent/path/hermes.db")
		if err == nil {
			t.Fatal("expected error for invalid DB path")
		}
	})
}

func TestPrintHistoryJSON(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "history.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.SaveHistory(&store.HistoryRecord{
		ID:            "json-1",
		Config:        &config.NotificationConfig{Heading: "JSON Test"},
		ResponseValue: "ok",
		CompletedAt:   time.Now(),
	})
	s.Close()

	// printHistoryJSON writes to os.Stdout. We verify it doesn't error.
	// A full stdout capture would require os.Pipe; verifying no error is sufficient.
	err = printHistoryJSON(dbPath)
	if err != nil {
		t.Fatalf("printHistoryJSON: %v", err)
	}
}

func TestFetchHistory_DirectDB(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "fetch.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.SaveHistory(&store.HistoryRecord{
		ID:          "fh-1",
		Config:      &config.NotificationConfig{Heading: "Fetch"},
		CompletedAt: time.Now(),
	})
	s.Close()

	entries, err := fetchHistory(dbPath)
	if err != nil {
		t.Fatalf("fetchHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}
