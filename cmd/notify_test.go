package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
)

func TestWriteTempConfig(t *testing.T) {
	t.Parallel()

	t.Run("roundtrips config through JSON", func(t *testing.T) {
		t.Parallel()
		cfg := &config.NotificationConfig{
			Heading: "Reboot Required",
			Message: "Please save your work and restart.",
			Buttons: []config.Button{
				{Label: "Restart Now", Value: "restart", Style: "primary"},
				{Label: "Later", Value: "defer:1h"},
			},
		}

		path, err := writeTempConfig(cfg)
		if err != nil {
			t.Fatalf("writeTempConfig: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(filepath.Dir(path)) })

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read temp config: %v", err)
		}

		var got config.NotificationConfig
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if got.Heading != cfg.Heading {
			t.Errorf("heading = %q, want %q", got.Heading, cfg.Heading)
		}
		if len(got.Buttons) != 2 {
			t.Errorf("buttons = %d, want 2", len(got.Buttons))
		}
	})

	t.Run("file has expected name", func(t *testing.T) {
		t.Parallel()
		cfg := &config.NotificationConfig{Heading: "H", Message: "M"}
		path, err := writeTempConfig(cfg)
		if err != nil {
			t.Fatalf("writeTempConfig: %v", err)
		}
		t.Cleanup(func() { os.RemoveAll(filepath.Dir(path)) })

		if filepath.Base(path) != "config.json" {
			t.Errorf("filename = %q, want config.json", filepath.Base(path))
		}
	})

	t.Run("directory permissions allow traversal", func(t *testing.T) {
		t.Parallel()
		cfg := &config.NotificationConfig{Heading: "H", Message: "M"}
		path, err := writeTempConfig(cfg)
		if err != nil {
			t.Fatalf("writeTempConfig: %v", err)
		}
		dir := filepath.Dir(path)
		t.Cleanup(func() { os.RemoveAll(dir) })

		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat dir: %v", err)
		}
		perm := info.Mode().Perm()
		if perm&0111 == 0 {
			t.Errorf("dir perm = %o, want execute bits set for traversal", perm)
		}
	})
}
