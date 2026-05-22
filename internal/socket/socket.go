// Package socket provides Unix domain socket transport for the hermes gRPC service.
// UDS replaces TCP for all IPC: invisible to port scanners, no port collisions
// in multi-user environments, and OS-level access control via file permissions.
package socket

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const sockName = "hermes.sock"

// Path returns the platform-specific Unix domain socket path.
// macOS uses ~/.hermes/ (not ~/Library/Application Support/hermes/) to stay
// within the 104-byte sun_path limit. The auth token path in internal/auth
// uses the longer Library path because it has no such constraint.
func Path() string {
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "hermes", sockName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "hermes", sockName)
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".hermes", sockName)
	default:
		if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
			return filepath.Join(dir, "hermes", sockName)
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".hermes", sockName)
	}
}

// Listen creates a Unix domain socket listener at the given path.
// It handles stale sockets (leftover from a crash) by probing: if the
// socket file exists but nobody is listening, it removes and retries.
// If another daemon is actively listening, it returns an error.
func Listen(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create socket dir: %w", err)
	}

	lis, err := net.Listen("unix", path)
	if err == nil {
		os.Chmod(path, 0600)
		return lis, nil
	}

	if !isAddrInUse(err) {
		return nil, fmt.Errorf("listen unix %s: %w", path, err)
	}

	// Socket file exists. Probe to distinguish stale from live.
	conn, dialErr := net.DialTimeout("unix", path, 500*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		return nil, fmt.Errorf("hermes is already running (socket %s)", path)
	}

	// Stale socket: remove and retry. The parent directory is 0700, so only
	// same-user processes can race this window.
	os.Remove(path)
	lis, err = net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s (after stale removal): %w", path, err)
	}
	os.Chmod(path, 0600)
	return lis, nil
}

// Dial connects to the hermes gRPC service via Unix domain socket.
func Dial(path string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	target := DialTarget(path)
	defaults := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	opts = append(defaults, opts...)
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", path, err)
	}
	return conn, nil
}

// DialTarget returns the gRPC target string for a Unix domain socket path.
// Unix absolute paths: "unix://" + "/abs/path" = "unix:///abs/path".
// Windows paths (C:\...): "unix:" + path (no authority, no leading slash).
func DialTarget(path string) string {
	if len(path) > 0 && path[0] == '/' {
		return "unix://" + path
	}
	return "unix:" + path
}

// Cleanup removes the socket file. Called on service shutdown.
func Cleanup(path string) {
	os.Remove(path)
}

// IsRunning reports whether a hermes daemon is actively listening on the
// given socket path. Returns false if the socket doesn't exist, is stale,
// or the dial times out.
func IsRunning(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
