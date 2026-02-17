// Package store persists notification state to disk using bbolt.
// One file, one bucket, JSON values. That's it.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("notifications")

// Record is the on-disk representation of a notification.
type Record struct {
	ID         string                       `json:"id"`
	Config     *config.NotificationConfig   `json:"config"`
	State      string                       `json:"state"`
	DeferCount int                          `json:"deferCount"`
	CreatedAt  time.Time                    `json:"createdAt"`
	Deadline   time.Time                    `json:"deadline,omitempty"`
}

// Store wraps a bbolt database.
type Store struct {
	db *bolt.DB
}

// Open opens (or creates) the bolt database at the default platform path.
// Pass "" for path to use the default.
func Open(path string) (*Store, error) {
	if path == "" {
		path = defaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}
	// Ensure bucket exists.
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	}); err != nil {
		db.Close()
		return nil, err
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
		return tx.Bucket(bucketName).ForEach(func(k, v []byte) error {
			var r Record
			if err := json.Unmarshal(v, &r); err != nil {
				return nil // skip corrupt records
			}
			records = append(records, &r)
			return nil
		})
	})
	return records, err
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
