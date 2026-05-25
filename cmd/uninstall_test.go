package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveLinesWithMarker(t *testing.T) {
	t.Parallel()

	t.Run("removes matching lines", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.sh")
		content := "line one\nhermes motd # Hermes-MOTD\nline three\n"
		os.WriteFile(path, []byte(content), 0644)

		err := removeLinesWithMarker(path, motdMarker)
		if err != nil {
			t.Fatalf("removeLinesWithMarker: %v", err)
		}

		got, _ := os.ReadFile(path)
		want := "line one\nline three\n"
		if string(got) != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("no-op when marker absent", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.sh")
		content := "line one\nline two\n"
		os.WriteFile(path, []byte(content), 0644)

		err := removeLinesWithMarker(path, motdMarker)
		if err != nil {
			t.Fatalf("removeLinesWithMarker: %v", err)
		}

		got, _ := os.ReadFile(path)
		if string(got) != content {
			t.Errorf("file was modified: got %q, want %q", got, content)
		}
	})

	t.Run("no-op when file missing", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "nonexistent.sh")
		err := removeLinesWithMarker(path, motdMarker)
		if err != nil {
			t.Fatalf("expected nil for missing file, got: %v", err)
		}
	})

	t.Run("removes multiple matching lines", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "profile.sh")
		content := "first # Hermes-MOTD\nkeep\nsecond # Hermes-MOTD\n"
		os.WriteFile(path, []byte(content), 0644)

		err := removeLinesWithMarker(path, motdMarker)
		if err != nil {
			t.Fatalf("removeLinesWithMarker: %v", err)
		}

		got, _ := os.ReadFile(path)
		want := "keep\n"
		if string(got) != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
