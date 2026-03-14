// Package store persists notification state to disk using bbolt.
// One file, one bucket, JSON values. That's it.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketName  = []byte("notifications")
	historyName = []byte("history")
	queueName   = []byte("queue")
)

// Record is the on-disk representation of a notification.
type Record struct {
	ID         string                       `json:"id"`
	Config     *config.NotificationConfig   `json:"config"`
	State      string                       `json:"state"`
	DeferCount int                          `json:"defer_count"`
	CreatedAt  time.Time                    `json:"created_at"`
	Deadline   time.Time                    `json:"deadline,omitempty"`
}

// Store wraps a bbolt database.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the bolt database at the default platform path.
// Pass "" for path to use the default.
//
// On Windows, MSI upgrades can briefly hold the DB lock while the old daemon
// exits. Open retries with exponential backoff (1s, 2s, 4s) before giving up.
func Open(path string) (*Store, error) {
	if path == "" {
		path = defaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	var db *bolt.DB
	backoff := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	var lastErr error
	for attempt := range len(backoff) + 1 {
		var err error
		db, err = bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
		if err == nil {
			break
		}
		lastErr = err
		if attempt < len(backoff) {
			time.Sleep(backoff[attempt])
		}
	}
	if db == nil {
		return nil, fmt.Errorf("open bolt db after retries: %w", lastErr)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(bucketName); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(historyName); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(queueName)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// OpenReadOnly opens the bolt database in read-only mode for offline use.
// bbolt does not support concurrent multi-process access; this will timeout
// if the service is running and holds the DB lock.
func OpenReadOnly(path string) (*Store, error) {
	if path == "" {
		path = defaultPath()
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout:  1 * time.Second,
		ReadOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open bolt db (read-only): %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Save persists a notification record.
func (s *Store) Save(r *Record) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(r.ID), data)
	})
}

// Delete removes a notification record.
func (s *Store) Delete(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Delete([]byte(id))
	})
}

// LoadAll returns all stored records.
func (s *Store) LoadAll() ([]*Record, error) {
	var records []*Record
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var r Record
			if err := json.Unmarshal(v, &r); err != nil {
				return nil
			}
			records = append(records, &r)
			return nil
		})
	})
	return records, err
}

// HistoryRecord stores a completed notification for the inbox.
type HistoryRecord struct {
	ID            string                     `json:"id"`
	Config        *config.NotificationConfig `json:"config"`
	ResponseValue string                     `json:"response_value"`
	CreatedAt     time.Time                  `json:"created_at"`
	CompletedAt   time.Time                  `json:"completed_at"`
}

// SaveHistory persists a completed notification to the history bucket.
func (s *Store) SaveHistory(r *HistoryRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(historyName).Put([]byte(r.ID), data)
	})
}

// LoadHistory returns all history records sorted newest first.
func (s *Store) LoadHistory() ([]*HistoryRecord, error) {
	var records []*HistoryRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(historyName)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var r HistoryRecord
			if err := json.Unmarshal(v, &r); err != nil {
				fmt.Fprintf(os.Stderr, "store: skip corrupt history key %q: %v\n", k, err)
				return nil
			}
			records = append(records, &r)
			return nil
		})
	})
	sort.Slice(records, func(i, j int) bool {
		return records[i].CompletedAt.After(records[j].CompletedAt)
	})
	return records, err
}

// PruneHistory removes records older than maxAge or exceeding maxCount.
func (s *Store) PruneHistory(maxAge time.Duration, maxCount int) error {
	if maxCount < 1 {
		maxCount = 1
	}
	records, err := s.LoadHistory()
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(historyName)
		if b == nil {
			return nil
		}
		for i, r := range records {
			if i >= maxCount || r.CompletedAt.Before(cutoff) {
				b.Delete([]byte(r.ID))
			}
		}
		return nil
	})
}

// DefaultQueueTTL is how long a queued notification remains valid before
// expiring. Notifications older than this are discarded on drain.
const DefaultQueueTTL = 30 * 24 * time.Hour

// QueueRecord is a notification that was submitted while the service
// was offline. It sits in the queue bucket until the next hermes serve
// startup, at which point notifications are drained one at a time.
type QueueRecord struct {
	ID        string                     `json:"id"`
	Config    *config.NotificationConfig `json:"config"`
	QueuedAt  time.Time                  `json:"queued_at"`
	ExpiresAt time.Time                  `json:"expires_at"`
	Priority  int                        `json:"priority"`
}

// Enqueue adds a notification to the offline queue.
func (s *Store) Enqueue(r *QueueRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(queueName)
		existing := b.Get([]byte(r.ID))
		if existing != nil {
			return nil // dedup: already queued
		}
		return b.Put([]byte(r.ID), data)
	})
}

// LoadQueue returns all queued records sorted by priority (highest first),
// then by queue time (oldest first within the same priority).
func (s *Store) LoadQueue() ([]*QueueRecord, error) {
	var records []*QueueRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(queueName)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var r QueueRecord
			if err := json.Unmarshal(v, &r); err != nil {
				fmt.Fprintf(os.Stderr, "store: skip corrupt queue key %q: %v\n", k, err)
				return nil
			}
			records = append(records, &r)
			return nil
		})
	})
	// Higher priority first, then oldest within same priority.
	sort.Slice(records, func(i, j int) bool {
		if records[i].Priority != records[j].Priority {
			return records[i].Priority > records[j].Priority
		}
		return records[i].QueuedAt.Before(records[j].QueuedAt)
	})
	return records, err
}

// DeleteQueued removes a record from the offline queue.
func (s *Store) DeleteQueued(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(queueName).Delete([]byte(id))
	})
}

// EnqueueOffline opens the database, writes a queue record, and closes it.
// Intended for use by hermes notify when the service daemon is not running.
// If the daemon holds the DB lock, this returns an error (bbolt timeout).
func EnqueueOffline(dbPath string, cfg *config.NotificationConfig, ttl time.Duration) error {
	s, err := Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()

	id := cfg.ID
	if id == "" {
		id = generateQueueID()
	}

	now := time.Now()
	priority := cfg.Priority
	if priority == 0 {
		priority = config.DefaultPriority
	}
	return s.Enqueue(&QueueRecord{
		ID:        id,
		Config:    cfg,
		QueuedAt:  now,
		ExpiresAt: now.Add(ttl),
		Priority:  priority,
	})
}

func generateQueueID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("q-%d", time.Now().UnixNano())
	}
	return "q-" + hex.EncodeToString(b)
}

// defaultPath returns the platform-appropriate database path.
func defaultPath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "hermes", "hermes.db")
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "hermes", "hermes.db")
	default:
		// XDG_DATA_HOME or ~/.local/share
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "hermes", "hermes.db")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".local", "share", "hermes", "hermes.db")
	}
}
