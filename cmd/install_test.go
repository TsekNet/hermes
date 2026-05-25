package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendLineIfMissing(t *testing.T) {
	t.Parallel()

	t.Run("creates file when missing", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "sub", "profile.ps1")
		err := appendLineIfMissing(path, "hermes motd # Hermes-MOTD", motdMarker)
		if err != nil {
			t.Fatalf("appendLineIfMissing: %v", err)
		}
		got, _ := os.ReadFile(path)
		if !strings.Contains(string(got), "hermes motd") {
			t.Errorf("file content = %q, want contains 'hermes motd'", got)
		}
	})

	t.Run("idempotent when marker present", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.sh")
		os.WriteFile(path, []byte("existing content\nhermes motd # Hermes-MOTD\n"), 0644)

		err := appendLineIfMissing(path, "hermes motd # Hermes-MOTD", motdMarker)
		if err != nil {
			t.Fatalf("appendLineIfMissing: %v", err)
		}

		got, _ := os.ReadFile(path)
		if count := strings.Count(string(got), motdMarker); count != 1 {
			t.Errorf("marker appears %d times, want 1", count)
		}
	})

	t.Run("appends newline when file lacks trailing newline", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.sh")
		os.WriteFile(path, []byte("existing content"), 0644)

		err := appendLineIfMissing(path, "hermes motd # Hermes-MOTD", motdMarker)
		if err != nil {
			t.Fatalf("appendLineIfMissing: %v", err)
		}

		got, _ := os.ReadFile(path)
		lines := strings.Split(string(got), "\n")
		if len(lines) < 3 {
			t.Fatalf("expected at least 3 lines (original, blank, hook), got %d", len(lines))
		}
	})

	t.Run("appends to empty file", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.sh")
		os.WriteFile(path, []byte(""), 0644)

		err := appendLineIfMissing(path, "hermes motd # Hermes-MOTD", motdMarker)
		if err != nil {
			t.Fatalf("appendLineIfMissing: %v", err)
		}

		got, _ := os.ReadFile(path)
		if !strings.Contains(string(got), motdMarker) {
			t.Errorf("file content = %q, want contains marker", got)
		}
	})
}

func TestWriteFileIfMissing(t *testing.T) {
	t.Parallel()

	t.Run("creates file when absent", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "sub", "hermes-motd.sh")
		content := "command -v hermes >/dev/null 2>&1 && hermes motd # Hermes-MOTD\n"

		err := writeFileIfMissing(path, content, 0644)
		if err != nil {
			t.Fatalf("writeFileIfMissing: %v", err)
		}

		got, _ := os.ReadFile(path)
		if string(got) != content {
			t.Errorf("file content = %q, want %q", got, content)
		}
	})

	t.Run("idempotent when marker present", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "hermes-motd.sh")
		original := "old content # Hermes-MOTD\n"
		os.WriteFile(path, []byte(original), 0644)

		err := writeFileIfMissing(path, "new content # Hermes-MOTD\n", 0644)
		if err != nil {
			t.Fatalf("writeFileIfMissing: %v", err)
		}

		got, _ := os.ReadFile(path)
		if string(got) != original {
			t.Errorf("file was overwritten: got %q, want %q", got, original)
		}
	})

	t.Run("overwrites when no marker", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "hermes-motd.sh")
		os.WriteFile(path, []byte("unrelated content\n"), 0644)

		content := "hermes motd # Hermes-MOTD\n"
		err := writeFileIfMissing(path, content, 0644)
		if err != nil {
			t.Fatalf("writeFileIfMissing: %v", err)
		}

		got, _ := os.ReadFile(path)
		if string(got) != content {
			t.Errorf("file content = %q, want %q", got, content)
		}
	})
}
