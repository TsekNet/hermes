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

func TestEnqueue_And_LoadQueue(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	records := []*QueueRecord{
		{ID: "q-low", Config: &config.NotificationConfig{Heading: "Low"}, QueuedAt: now, ExpiresAt: now.Add(24 * time.Hour), Priority: 3},
		{ID: "q-high", Config: &config.NotificationConfig{Heading: "High"}, QueuedAt: now.Add(time.Second), ExpiresAt: now.Add(24 * time.Hour), Priority: 8},
		{ID: "q-mid", Config: &config.NotificationConfig{Heading: "Mid"}, QueuedAt: now.Add(-time.Hour), ExpiresAt: now.Add(24 * time.Hour), Priority: 5},
	}
	for _, r := range records {
		if err := s.Enqueue(r); err != nil {
			t.Fatalf("Enqueue(%s): %v", r.ID, err)
		}
	}

	got, err := s.LoadQueue()
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}
	if got[0].ID != "q-high" {
		t.Errorf("first = %q, want q-high (highest priority)", got[0].ID)
	}
	if got[1].ID != "q-mid" {
		t.Errorf("second = %q, want q-mid", got[1].ID)
	}
	if got[2].ID != "q-low" {
		t.Errorf("third = %q, want q-low", got[2].ID)
	}
}

func TestEnqueue_Dedup(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	r := &QueueRecord{
		ID: "dup-1", Config: &config.NotificationConfig{Heading: "A"},
		QueuedAt: now, ExpiresAt: now.Add(time.Hour), Priority: 5,
	}
	if err := s.Enqueue(r); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	r2 := &QueueRecord{
		ID: "dup-1", Config: &config.NotificationConfig{Heading: "B"},
		QueuedAt: now, ExpiresAt: now.Add(time.Hour), Priority: 9,
	}
	if err := s.Enqueue(r2); err != nil {
		t.Fatalf("Enqueue dup: %v", err)
	}

	got, _ := s.LoadQueue()
	if len(got) != 1 {
		t.Fatalf("expected 1 record (dedup), got %d", len(got))
	}
	if got[0].Config.Heading != "A" {
		t.Errorf("heading = %q, want A (first write wins)", got[0].Config.Heading)
	}
}

func TestDeleteQueued(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	s.Enqueue(&QueueRecord{
		ID: "del-q", Config: &config.NotificationConfig{Heading: "X"},
		QueuedAt: now, ExpiresAt: now.Add(time.Hour), Priority: 5,
	})
	if err := s.DeleteQueued("del-q"); err != nil {
		t.Fatalf("DeleteQueued: %v", err)
	}
	got, _ := s.LoadQueue()
	if len(got) != 0 {
		t.Fatalf("expected 0 after delete, got %d", len(got))
	}
}

func TestEnqueueOffline(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "offline.db")

	cfg := &config.NotificationConfig{
		Heading: "Offline", Message: "test", ID: "off-1", Priority: 7,
	}
	if err := EnqueueOffline(path, cfg, 24*time.Hour); err != nil {
		t.Fatalf("EnqueueOffline: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	got, _ := s.LoadQueue()
	if len(got) != 1 {
		t.Fatalf("expected 1 queued record, got %d", len(got))
	}
	if got[0].ID != "off-1" {
		t.Errorf("ID = %q, want off-1", got[0].ID)
	}
	if got[0].Priority != 7 {
		t.Errorf("priority = %d, want 7", got[0].Priority)
	}
	if got[0].Config.Heading != "Offline" {
		t.Errorf("heading = %q, want Offline", got[0].Config.Heading)
	}
}

func TestEnqueueOffline_DefaultPriority(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "offline-default.db")

	cfg := &config.NotificationConfig{Heading: "Default", Message: "prio"}
	if err := EnqueueOffline(path, cfg, 24*time.Hour); err != nil {
		t.Fatalf("EnqueueOffline: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	got, _ := s.LoadQueue()
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Priority != 5 {
		t.Errorf("default priority = %d, want 5", got[0].Priority)
	}
}

func TestLoadQueue_SamePriority_OldestFirst(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	s.Enqueue(&QueueRecord{ID: "newer", Config: &config.NotificationConfig{Heading: "B"}, QueuedAt: now.Add(time.Hour), ExpiresAt: now.Add(48 * time.Hour), Priority: 5})
	s.Enqueue(&QueueRecord{ID: "older", Config: &config.NotificationConfig{Heading: "A"}, QueuedAt: now, ExpiresAt: now.Add(48 * time.Hour), Priority: 5})

	got, _ := s.LoadQueue()
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].ID != "older" {
		t.Errorf("first = %q, want older (FIFO within same priority)", got[0].ID)
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

func TestOpenReadOnly(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "readonly.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.SaveHistory(&HistoryRecord{
		ID:     "h-ro",
		Config: &config.NotificationConfig{Heading: "ReadOnly"},
	})
	s.Close()

	ro, err := OpenReadOnly(path)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer ro.Close()

	records, err := ro.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(records) != 1 || records[0].ID != "h-ro" {
		t.Fatalf("expected 1 record 'h-ro', got %v", records)
	}
}

func TestOpenReadOnly_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := OpenReadOnly(filepath.Join(t.TempDir(), "missing.db"))
	if err == nil {
		t.Error("expected error for missing DB file")
	}
}

func TestOpen_CreatesParentDir(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "deep", "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Close()
}

func TestPruneHistory_MaxCountZero(t *testing.T) {
	t.Parallel()
	s := tempStore(t)

	now := time.Now()
	for i := range 5 {
		s.SaveHistory(&HistoryRecord{
			ID:          fmt.Sprintf("h-%d", i),
			Config:      &config.NotificationConfig{Heading: fmt.Sprintf("N%d", i)},
			CompletedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}

	// maxCount=0 is clamped to 1 internally.
	if err := s.PruneHistory(999*time.Hour, 0); err != nil {
		t.Fatalf("PruneHistory: %v", err)
	}

	got, _ := s.LoadHistory()
	if len(got) != 1 {
		t.Fatalf("expected 1 record (clamped maxCount=0 to 1), got %d", len(got))
	}
}

func TestDefaultPath_NonEmpty(t *testing.T) {
	t.Parallel()
	p := defaultPath()
	if p == "" {
		t.Error("defaultPath() returned empty string")
	}
	if !filepath.IsAbs(p) {
		t.Errorf("defaultPath() = %q, want absolute path", p)
	}
}

func TestEnqueueOffline_GeneratesID(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "autoid.db")

	cfg := &config.NotificationConfig{Heading: "AutoID", Message: "test"}
	if err := EnqueueOffline(path, cfg, 24*time.Hour); err != nil {
		t.Fatalf("EnqueueOffline: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	records, _ := s.LoadQueue()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ID == "" {
		t.Error("expected auto-generated ID, got empty")
	}
	if records[0].ID[:2] != "q-" {
		t.Errorf("auto-generated ID = %q, want q- prefix", records[0].ID)
	}
}

func TestDeleteNonexistentRecord(t *testing.T) {
	t.Parallel()
	s := tempStore(t)
	if err := s.Delete("does-not-exist"); err != nil {
		t.Errorf("Delete non-existent should not error, got: %v", err)
	}
}

func TestLoadAll_EmptyStore(t *testing.T) {
	t.Parallel()
	s := tempStore(t)
	records, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
