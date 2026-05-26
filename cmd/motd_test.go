package cmd

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/store"
)

func TestSanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean string unchanged", "Hello World", "Hello World"},
		{"strips null byte", "Hello\x00World", "HelloWorld"},
		{"strips newline", "Hello\nWorld", "HelloWorld"},
		{"strips tab", "Hello\tWorld", "HelloWorld"},
		{"strips carriage return", "Hello\rWorld", "HelloWorld"},
		{"strips DEL 0x7f", "Hello\x7fWorld", "HelloWorld"},
		{"strips C1 U+0085 NEL", "HelloWorld", "HelloWorld"},
		{"strips C1 U+0080", "HelloWorld", "HelloWorld"},
		{"strips C1 U+009F", "HelloWorld", "HelloWorld"},
		{"preserves em dash", "Hermes — notification", "Hermes — notification"},
		{"preserves emoji", "Update ready \U0001F680", "Update ready \U0001F680"},
		{"empty string", "", ""},
		{"only ASCII controls", "\x00\x01\x02\x1f\x7f", ""},
		{"mixed controls and text", "Re\x00boot\x1frequi\x7fred", "Rebootrequired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitize(tt.input); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSSHSession(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"SSH_CLIENT set", map[string]string{"SSH_CLIENT": "192.168.1.1 54321 22"}, true},
		{"SSH_CONNECTION set", map[string]string{"SSH_CONNECTION": "192.168.1.1 54321 10.0.0.1 22"}, true},
		{"SSH_TTY set", map[string]string{"SSH_TTY": "/dev/pts/0"}, true},
		{"no SSH env vars", map[string]string{}, false},
		{"unrelated env vars", map[string]string{"HOME": "/home/user", "TERM": "xterm"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"SSH_CLIENT", "SSH_CONNECTION", "SSH_TTY"} {
				t.Setenv(key, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := isSSHSession(); got != tt.want {
				t.Errorf("isSSHSession() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunMotd_NoSSH(t *testing.T) {
	for _, key := range []string{"SSH_CLIENT", "SSH_CONNECTION", "SSH_TTY"} {
		t.Setenv(key, "")
	}

	err := runMotd(filepath.Join(t.TempDir(), "unused.db"))
	if err != nil {
		t.Fatalf("runMotd should return nil when not SSH: %v", err)
	}
}

func TestRunMotd_SSHWithHistory(t *testing.T) {
	t.Setenv("SSH_CLIENT", "192.168.1.1 54321 22")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")

	dbPath := filepath.Join(t.TempDir(), "motd.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	now := time.Now()
	for i := range 7 {
		s.SaveHistory(&store.HistoryRecord{
			ID:          fmt.Sprintf("h-%d", i),
			Config:      &config.NotificationConfig{Heading: fmt.Sprintf("Notification %d", i)},
			CompletedAt: now.Add(time.Duration(i) * time.Minute),
		})
	}
	s.Close()

	// runMotd prints to stdout; we just verify no error.
	err = runMotd(dbPath)
	if err != nil {
		t.Fatalf("runMotd: %v", err)
	}
}

func TestRunMotd_SSHEmptyHistory(t *testing.T) {
	t.Setenv("SSH_CLIENT", "192.168.1.1 54321 22")
	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("SSH_TTY", "")

	dbPath := filepath.Join(t.TempDir(), "empty.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s.Close()

	err = runMotd(dbPath)
	if err != nil {
		t.Fatalf("runMotd: %v", err)
	}
}

type mockOneliner struct {
	entries []client.ListEntry
	err     error
}

func (m *mockOneliner) List(_ context.Context) ([]client.ListEntry, error) {
	return m.entries, m.err
}

func (m *mockOneliner) Close() error { return nil }

func TestMotdOneline_WithPending(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	dial := func() (oneliner, error) {
		return &mockOneliner{entries: []client.ListEntry{{ID: "a"}, {ID: "b"}, {ID: "c"}}}, nil
	}
	runMotdOnelineWith(&buf, dial, time.Second)
	if got := buf.String(); got != "3 pending" {
		t.Errorf("got %q, want %q", got, "3 pending")
	}
}

func TestMotdOneline_NoPending(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	dial := func() (oneliner, error) {
		return &mockOneliner{entries: nil}, nil
	}
	runMotdOnelineWith(&buf, dial, time.Second)
	if got := buf.String(); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMotdOneline_ServiceDown(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	dial := func() (oneliner, error) {
		return nil, fmt.Errorf("connection refused")
	}
	runMotdOnelineWith(&buf, dial, time.Second)
	if got := buf.String(); got != "" {
		t.Errorf("got %q, want empty on dial error", got)
	}
}

func TestMotdOneline_DaemonHangs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	slowDial := func() (oneliner, error) {
		time.Sleep(500 * time.Millisecond)
		return &mockOneliner{entries: []client.ListEntry{{ID: "a"}}}, nil
	}
	start := time.Now()
	runMotdOnelineWith(&buf, slowDial, 50*time.Millisecond)
	elapsed := time.Since(start)
	if got := buf.String(); got != "" {
		t.Errorf("got %q, want empty on timeout", got)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("took %v, should complete within ~50ms deadline", elapsed)
	}
}

func TestMotdOneline_ListError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	dial := func() (oneliner, error) {
		return &mockOneliner{err: fmt.Errorf("rpc error")}, nil
	}
	runMotdOnelineWith(&buf, dial, time.Second)
	if got := buf.String(); got != "" {
		t.Errorf("got %q, want empty on list error", got)
	}
}
