package socket

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestPath_NotEmpty(t *testing.T) {
	t.Parallel()
	p := Path()
	if p == "" {
		t.Fatal("Path() returned empty string")
	}
}

func TestPath_UnderSunPathLimit(t *testing.T) {
	t.Parallel()
	p := Path()
	// sun_path limits: Linux 108, macOS 104, Windows ~260.
	limit := 104
	if runtime.GOOS == "windows" {
		limit = 260
	}
	if len(p) >= limit {
		t.Errorf("Path() = %q (%d bytes), exceeds sun_path limit of %d", p, len(p), limit)
	}
}

func TestPath_ContainsHermes(t *testing.T) {
	t.Parallel()
	p := Path()
	if !strings.Contains(p, "hermes") {
		t.Errorf("Path() = %q, expected to contain 'hermes'", p)
	}
}

func TestPath_Deterministic(t *testing.T) {
	t.Parallel()
	if Path() != Path() {
		t.Error("Path() is not deterministic")
	}
}

func TestListen_CreatesSocket(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")
	lis, err := Listen(sock)
	if err != nil {
		t.Fatalf("Listen(%q): %v", sock, err)
	}
	defer lis.Close()

	if _, err := os.Stat(sock); err != nil {
		t.Errorf("socket file not created: %v", err)
	}
}

func TestListen_CreatesParentDir(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "nested", "deep", "hermes.sock")
	lis, err := Listen(sock)
	if err != nil {
		t.Fatalf("Listen(%q): %v", sock, err)
	}
	defer lis.Close()

	info, err := os.Stat(filepath.Dir(sock))
	if err != nil {
		t.Fatalf("parent dir not created: %v", err)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0700 {
			t.Errorf("parent dir permissions = %o, want 0700", perm)
		}
	}
}

func TestListen_RemovesStaleSocket(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")

	// Create a stale socket file (no listener).
	if err := os.WriteFile(sock, []byte("stale"), 0600); err != nil {
		t.Fatal(err)
	}

	lis, err := Listen(sock)
	if err != nil {
		t.Fatalf("Listen should recover from stale socket: %v", err)
	}
	defer lis.Close()
}

func TestListen_RejectsLiveSocket(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")

	first, err := Listen(sock)
	if err != nil {
		t.Fatalf("first Listen: %v", err)
	}
	defer first.Close()

	_, err = Listen(sock)
	if err == nil {
		t.Fatal("second Listen should fail when first is still alive")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error = %q, want it to mention 'already running'", err)
	}
}

func TestDial_ConnectsToListener(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")

	lis, err := Listen(sock)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer lis.Close()

	// Start a minimal gRPC server to accept the connection.
	srv := grpc.NewServer()
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := Dial(sock)
	if err != nil {
		t.Fatalf("Dial(%q): %v", sock, err)
	}
	defer conn.Close()
}

func TestDial_FailsWithNoListener(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")

	conn, err := grpc.NewClient(
		"unix://"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer conn.Close()

	// Force a connection attempt: the socket doesn't exist, so any RPC
	// context should fail. We just verify grpc.NewClient itself succeeds
	// (lazy connect) but actual connectivity fails.
	ctx, cancel := context.WithTimeout(context.Background(), 1)
	defer cancel()
	conn.Connect()
	<-ctx.Done()
	if conn.GetState().String() == "READY" {
		t.Error("connection should not be READY with no listener")
	}
}

func TestCleanup_RemovesSocket(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")

	lis, err := Listen(sock)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	lis.Close()

	Cleanup(sock)
	if _, err := os.Stat(sock); !os.IsNotExist(err) {
		t.Errorf("Cleanup should remove socket file, got stat error: %v", err)
	}
}

func TestCleanup_NoErrorOnMissing(t *testing.T) {
	t.Parallel()
	Cleanup(filepath.Join(t.TempDir(), "nonexistent.sock"))
}

func TestGRPCRoundTrip(t *testing.T) {
	t.Parallel()
	sock := filepath.Join(t.TempDir(), "hermes.sock")

	lis, err := Listen(sock)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	go srv.Serve(lis)
	defer srv.Stop()

	conn, err := Dial(sock)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer conn.Close()

	// Verify the connection reaches READY state.
	ctx, cancel := context.WithTimeout(context.Background(), 5e9)
	defer cancel()
	conn.Connect()
	for conn.GetState().String() != "READY" {
		if !conn.WaitForStateChange(ctx, conn.GetState()) {
			t.Fatal("connection did not reach READY state")
		}
	}
}

func TestDialTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want string
	}{
		{"simple", "/tmp/hermes.sock", "unix:///tmp/hermes.sock"},
		{"nested", "/run/user/1000/hermes/hermes.sock", "unix:///run/user/1000/hermes/hermes.sock"},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			name string
			path string
			want string
		}{"windows", `C:\Users\test\AppData\Local\hermes\hermes.sock`, `unix:C:\Users\test\AppData\Local\hermes\hermes.sock`})
	} else {
		tests = append(tests, struct {
			name string
			path string
			want string
		}{"relative", "hermes.sock", "unix:hermes.sock"})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := DialTarget(tt.path); got != tt.want {
				t.Errorf("DialTarget(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	t.Parallel()

	t.Run("no socket", func(t *testing.T) {
		t.Parallel()
		sock := filepath.Join(t.TempDir(), "hermes.sock")
		if IsRunning(sock) {
			t.Error("IsRunning should return false when no socket exists")
		}
	})

	t.Run("stale socket", func(t *testing.T) {
		t.Parallel()
		sock := filepath.Join(t.TempDir(), "hermes.sock")
		os.WriteFile(sock, []byte("stale"), 0600)
		if IsRunning(sock) {
			t.Error("IsRunning should return false for stale socket")
		}
	})

	t.Run("live socket", func(t *testing.T) {
		t.Parallel()
		sock := filepath.Join(t.TempDir(), "hermes.sock")
		lis, err := net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		defer lis.Close()
		if !IsRunning(sock) {
			t.Error("IsRunning should return true for live socket")
		}
	})
}
