package client

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/manager"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/TsekNet/hermes/internal/store"
)

func startTestService(t *testing.T) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "hermes.sock")
	mgr := manager.New(func(n *manager.Notification) {}, nil)
	srv := server.New(mgr)

	lis, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return sock
}

func testCfg(id string) *config.NotificationConfig {
	return &config.NotificationConfig{
		Heading:        "Test",
		Message:        "body",
		TimeoutSeconds: 10,
		TimeoutValue:   "auto",
		ID:             id,
		Buttons: []config.Button{
			{Label: "OK", Value: "ok"},
		},
	}
}

func TestDialAndPing(t *testing.T) {
	t.Parallel()
	sock := startTestService(t)
	c, err := DialPath(sock)
	if err != nil {
		t.Fatalf("DialPath: %v", err)
	}
	defer c.Close()

	if _, err := c.List(context.Background()); err != nil {
		t.Errorf("List: %v", err)
	}
}

func TestNotifyAndReportChoice(t *testing.T) {
	t.Parallel()
	sock := startTestService(t)

	c1, err := DialPath(sock)
	if err != nil {
		t.Fatalf("DialPath: %v", err)
	}
	defer c1.Close()

	type result struct {
		r   *NotifyResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		r, err := c1.Notify(context.Background(), testCfg("notify-test-1"))
		ch <- result{r, err}
	}()
	time.Sleep(200 * time.Millisecond)

	c2, _ := DialPath(sock)
	defer c2.Close()
	ok, err := c2.ReportChoice(context.Background(), "notify-test-1", "ok")
	if err != nil {
		t.Fatalf("ReportChoice: %v", err)
	}
	if !ok {
		t.Error("expected accepted")
	}

	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("Notify: %v", r.err)
		}
		if r.r.Value != "ok" {
			t.Errorf("value = %q, want ok", r.r.Value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestListAndCancel(t *testing.T) {
	t.Parallel()
	sock := startTestService(t)

	c1, _ := DialPath(sock)
	defer c1.Close()

	go func() {
		c1.Notify(context.Background(), testCfg("lc-1"))
	}()
	time.Sleep(200 * time.Millisecond)

	c2, _ := DialPath(sock)
	defer c2.Close()

	entries, err := c2.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) < 1 {
		t.Fatalf("expected >= 1 entry, got %d", len(entries))
	}

	found, err := c2.Cancel(context.Background(), "lc-1")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Error("expected found")
	}
}

func TestGetUIConfig(t *testing.T) {
	t.Parallel()
	sock := startTestService(t)

	c1, _ := DialPath(sock)
	defer c1.Close()

	id := "ui-test"
	go func() {
		c1.Notify(context.Background(), testCfg(id))
	}()
	time.Sleep(200 * time.Millisecond)

	c2, _ := DialPath(sock)
	defer c2.Close()

	cfg, deferAllowed, err := c2.GetUIConfig(context.Background(), id)
	if err != nil {
		t.Fatalf("GetUIConfig: %v", err)
	}
	if cfg.Heading != "Test" {
		t.Errorf("heading = %q", cfg.Heading)
	}
	if !deferAllowed {
		t.Error("expected deferAllowed = true")
	}
}

func TestListHistory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s, err := store.Open(filepath.Join(tmpDir, "history.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	sock := filepath.Join(tmpDir, "hermes.sock")
	mgr := manager.New(func(n *manager.Notification) {}, s)
	srv := server.New(mgr)

	lis, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	c1, err := DialPath(sock)
	if err != nil {
		t.Fatalf("DialPath: %v", err)
	}
	defer c1.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c1.Notify(context.Background(), testCfg("hist-1"))
	}()
	time.Sleep(200 * time.Millisecond)

	c2, _ := DialPath(sock)
	defer c2.Close()
	c2.ReportChoice(context.Background(), "hist-1", "ok")

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Notify did not return")
	}

	entries, err := c2.ListHistory(context.Background())
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(entries))
	}
	if entries[0].Heading != "Test" {
		t.Errorf("Heading = %q, want Test", entries[0].Heading)
	}
	if entries[0].ResponseValue != "ok" {
		t.Errorf("ResponseValue = %q, want ok", entries[0].ResponseValue)
	}
	if entries[0].CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
}

func TestDial_NoServer(t *testing.T) {
	t.Parallel()
	c, err := Dial()
	if err == nil {
		c.Close()
	}
}
