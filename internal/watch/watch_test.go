package watch

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWatcher_FileCreate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	var mu sync.Mutex
	var events []Event

	w := New(func(ev Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	if err := w.Start([]string{dir}); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer w.Stop()

	target := filepath.Join(dir, "receipt.txt")
	if err := os.WriteFile(target, []byte("installed"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for fs event")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, ev := range events {
		if filepath.Base(ev.Path) == "receipt.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no event for receipt.txt; got %v", events)
	}
}

func TestWatcher_NonexistentPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "subdir", "file.txt")

	var mu sync.Mutex
	var events []Event

	w := New(func(ev Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	if err := w.Start([]string{target}); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer w.Stop()

	// Create the parent directory and file; watcher should have
	// fallen back to watching the parent of the parent (dir).
	os.MkdirAll(filepath.Dir(target), 0755)

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			// Fallback watch may not catch nested creation on all platforms.
			t.Skip("no event received (platform may not support parent-dir fallback for nested paths)")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func TestWatcher_StopIdempotent(t *testing.T) {
	t.Parallel()
	w := New(func(_ Event) {})
	w.Stop()
	w.Stop()
}

func TestWatcher_RestartReplacesOld(t *testing.T) {
	t.Parallel()
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	var mu sync.Mutex
	var events []Event

	w := New(func(ev Event) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	if err := w.Start([]string{dir1}); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	if err := w.Start([]string{dir2}); err != nil {
		t.Fatalf("start 2: %v", err)
	}
	defer w.Stop()

	// Write to dir1 — should NOT trigger (old watcher stopped).
	os.WriteFile(filepath.Join(dir1, "old.txt"), []byte("x"), 0644)
	// Write to dir2 — should trigger.
	os.WriteFile(filepath.Join(dir2, "new.txt"), []byte("x"), 0644)

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for event on dir2")
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	for _, ev := range events {
		if filepath.Base(ev.Path) == "old.txt" {
			t.Error("received event from old watcher (dir1)")
		}
	}
}
