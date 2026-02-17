package store

import (
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
