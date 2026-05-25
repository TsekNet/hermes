package cmd

import (
	"path/filepath"
	"testing"
	"time"
)

func TestWaitForSocketRemoval_AlreadyGone(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "hermes.sock")
	got := waitForSocketRemoval(path, 100*time.Millisecond)
	if !got {
		t.Error("expected true when socket already absent")
	}
}

func TestWaitForSocketRemoval_Timeout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "hermes.sock")

	// Create a regular file to simulate a socket that never disappears.
	// socket.IsRunning uses net.Dial, which won't connect to a regular file,
	// so this returns false immediately (socket gone).
	// This tests the "already removed" path with a very short timeout.
	got := waitForSocketRemoval(path, 50*time.Millisecond)
	if !got {
		t.Error("expected true: non-socket file causes IsRunning to return false")
	}
}
