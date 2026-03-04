// Package watch monitors filesystem paths and emits events via a callback.
package watch

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/google/deck"
)

// Event describes a single filesystem change.
type Event struct {
	Path string `json:"path"`
	Op   string `json:"op"` // "create", "write", "remove", "rename", "chmod"
}

// Callback is invoked on the main goroutine for each filesystem event.
type Callback func(Event)

// Watcher monitors a set of paths and delivers events via a callback.
type Watcher struct {
	mu       sync.Mutex
	fsw      *fsnotify.Watcher
	cancel   context.CancelFunc
	callback Callback
}

// New creates a Watcher that delivers events to cb. Call Start to begin.
func New(cb Callback) *Watcher {
	return &Watcher{callback: cb}
}

func opName(op fsnotify.Op) string {
	switch {
	case op.Has(fsnotify.Create):
		return "create"
	case op.Has(fsnotify.Write):
		return "write"
	case op.Has(fsnotify.Remove):
		return "remove"
	case op.Has(fsnotify.Rename):
		return "rename"
	case op.Has(fsnotify.Chmod):
		return "chmod"
	default:
		return "unknown"
	}
}

// Start begins watching the given paths. Safe to call multiple times
// (previous watcher is stopped first). Directories are watched
// non-recursively; files are watched directly.
func (w *Watcher) Start(paths []string) error {
	w.Stop()

	w.mu.Lock()
	defer w.mu.Unlock()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			fsw.Close()
			return err
		}
		if err := fsw.Add(abs); err != nil {
			// Path may not exist yet; watch the parent directory instead
			// so we catch creation events.
			parent := filepath.Dir(abs)
			if err2 := fsw.Add(parent); err2 != nil {
				deck.Warningf("watch: cannot watch %s or parent %s: %v / %v", abs, parent, err, err2)
			} else {
				deck.Infof("watch: path %s not found, watching parent %s", abs, parent)
			}
		} else {
			deck.Infof("watch: monitoring %s", abs)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.fsw = fsw
	w.cancel = cancel

	go w.loop(ctx, fsw)
	return nil
}

func (w *Watcher) loop(ctx context.Context, fsw *fsnotify.Watcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-fsw.Events:
			if !ok {
				return
			}
			w.callback(Event{Path: ev.Name, Op: opName(ev.Op)})
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			deck.Errorf("watch: %v", err)
		}
	}
}

// Stop tears down the watcher. Safe to call if not started or already stopped.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	if w.fsw != nil {
		w.fsw.Close()
		w.fsw = nil
	}
}
