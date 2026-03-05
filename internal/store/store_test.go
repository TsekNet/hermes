package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSaveAndLoadAll(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	r := &Record{
		ID:         "abc123",
		Config:     &config.NotificationConfig{Heading: "Test", Message: "body"},
		State:      "deferred",
		DeferCount: 2,
		CreatedAt:  time.Now().Truncate(time.Millisecond),
		Deadline:   time.Now().Add(1 * time.Hour).Truncate(time.Millisecond),
	}
	if err := s.Save(r); err != nil {
		t.Fatalf("Save: %v", err)
	}

	records, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	got := records[0]
	if got.ID != r.ID {
		t.Errorf("ID: got %q, want %q", got.ID, r.ID)
	}
	if got.DeferCount != 2 {
		t.Errorf("DeferCount: got %d, want 2", got.DeferCount)
	}
	if got.Config.Heading != "Test" {
		t.Errorf("Config.Heading: got %q, want %q", got.Config.Heading, "Test")
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	r := &Record{
		ID:     "del-me",
		Config: &config.NotificationConfig{Heading: "Gone"},
		State:  "pending",
	}
	s.Save(r)
	s.Delete("del-me")

	records, _ := s.LoadAll()
	if len(records) != 0 {
		t.Fatalf("expected 0 records after delete, got %d", len(records))
	}
}

func TestMultipleRecords(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	for i := 0; i < 5; i++ {
		s.Save(&Record{
			ID:     time.Now().Format("150405.000") + string(rune('a'+i)),
			Config: &config.NotificationConfig{Heading: "N"},
			State:  "pending",
		})
	}

	records, _ := s.LoadAll()
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}
}

func TestSaveHistory_And_LoadHistory(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now().Truncate(time.Millisecond)
	records := []*HistoryRecord{
		{
			ID:            "h-1",
			Config:        &config.NotificationConfig{Heading: "Reboot", Message: "Please reboot", Title: "IT"},
			ResponseValue: "restart",
			CreatedAt:     now.Add(-2 * time.Hour),
			CompletedAt:   now.Add(-1 * time.Hour),
		},
		{
			ID:            "h-2",
			Config:        &config.NotificationConfig{Heading: "Update", Message: "Install update"},
			ResponseValue: "timeout:auto",
			CreatedAt:     now.Add(-3 * time.Hour),
			CompletedAt:   now, // newest
		},
	}
	for _, r := range records {
		if err := s.SaveHistory(r); err != nil {
			t.Fatalf("SaveHistory(%s): %v", r.ID, err)
		}
	}

	got, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 history records, got %d", len(got))
	}
	// LoadHistory sorts newest first.
	if got[0].ID != "h-2" {
		t.Errorf("first record ID = %q, want h-2 (newest)", got[0].ID)
	}
	if got[1].ID != "h-1" {
		t.Errorf("second record ID = %q, want h-1", got[1].ID)
	}
	if got[0].ResponseValue != "timeout:auto" {
		t.Errorf("ResponseValue = %q, want timeout:auto", got[0].ResponseValue)
	}
	if got[0].Config.Heading != "Update" {
		t.Errorf("Config.Heading = %q, want Update", got[0].Config.Heading)
	}
}

func TestLoadHistory_Empty(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	got, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 history records, got %d", len(got))
	}
}

func TestPruneHistory_ByAge(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	s.SaveHistory(&HistoryRecord{
		ID: "old", Config: &config.NotificationConfig{Heading: "Old"},
		CompletedAt: now.Add(-48 * time.Hour),
	})
	s.SaveHistory(&HistoryRecord{
		ID: "recent", Config: &config.NotificationConfig{Heading: "Recent"},
		CompletedAt: now.Add(-1 * time.Hour),
	})

	if err := s.PruneHistory(24*time.Hour, 1000); err != nil {
		t.Fatalf("PruneHistory: %v", err)
	}

	got, _ := s.LoadHistory()
	if len(got) != 1 {
		t.Fatalf("expected 1 record after prune, got %d", len(got))
	}
	if got[0].ID != "recent" {
		t.Errorf("surviving record ID = %q, want recent", got[0].ID)
	}
}

func TestPruneHistory_ByCount(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	for i := 0; i < 5; i++ {
		s.SaveHistory(&HistoryRecord{
			ID:          fmt.Sprintf("h-%d", i),
			Config:      &config.NotificationConfig{Heading: fmt.Sprintf("N%d", i)},
			CompletedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}

	if err := s.PruneHistory(999*time.Hour, 2); err != nil {
		t.Fatalf("PruneHistory: %v", err)
	}

	got, _ := s.LoadHistory()
	if len(got) != 2 {
		t.Fatalf("expected 2 records after prune by count, got %d", len(got))
	}
}

func TestSaveHistory_Isolation(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	s.Save(&Record{
		ID: "active-1", Config: &config.NotificationConfig{Heading: "Active"}, State: "pending",
	})
	s.SaveHistory(&HistoryRecord{
		ID: "done-1", Config: &config.NotificationConfig{Heading: "Done"},
		CompletedAt: time.Now(),
	})

	active, _ := s.LoadAll()
	history, _ := s.LoadHistory()
	if len(active) != 1 || active[0].ID != "active-1" {
		t.Errorf("active records: got %d, want 1 with ID active-1", len(active))
	}
	if len(history) != 1 || history[0].ID != "done-1" {
		t.Errorf("history records: got %d, want 1 with ID done-1", len(history))
	}
}

func TestReopenPersists(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "persist.db")

	// Open, write, close.
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s1.Save(&Record{
		ID:     "survive",
		Config: &config.NotificationConfig{Heading: "Persistent"},
		State:  "deferred",
	})
	s1.Close()

	// Reopen — data should still be there.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	defer s2.Close()

	records, _ := s2.LoadAll()
	if len(records) != 1 || records[0].ID != "survive" {
		t.Fatalf("expected 1 record 'survive' after reopen, got %v", records)
	}
}
