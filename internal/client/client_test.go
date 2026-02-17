package client

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/manager"
	"github.com/TsekNet/hermes/internal/server"
)

// startTestService starts a gRPC server on a random port and returns the port.
func startTestService(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	lis.Close()

	mgr := manager.New(func(n *manager.Notification) {}, nil)
	srv := server.New(mgr, port)
	go func() { srv.Serve() }()
	t.Cleanup(srv.Stop)
	time.Sleep(100 * time.Millisecond)
	return port
}

func testCfg(id string) *config.NotificationConfig {
	return &config.NotificationConfig{
		Heading:        "Test",
		Message:        "body",
		TimeoutSeconds: 10,
		TimeoutValue:   "auto",
		ID:             id,
	}
}

func TestDialAndPing(t *testing.T) {
	t.Parallel()
	port := startTestService(t)
	c, err := Dial(port)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

func TestNotifyAndReportChoice(t *testing.T) {
	t.Parallel()
	port := startTestService(t)

	// Submit in background via one client.
	c1, err := Dial(port)
	if err != nil {
		t.Fatalf("Dial: %v", err)
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

	// Report choice via a second client.
	c2, _ := Dial(port)
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
	port := startTestService(t)

	// Submit a notification in background.
	c1, _ := Dial(port)
	defer c1.Close()

	go func() {
		c1.Notify(context.Background(), testCfg(fmt.Sprintf("lc-%d", port)))
	}()
	time.Sleep(200 * time.Millisecond)

	c2, _ := Dial(port)
	defer c2.Close()

	// List should find it.
	entries, err := c2.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) < 1 {
		t.Fatalf("expected >= 1 entry, got %d", len(entries))
	}

	// Cancel it.
	found, err := c2.Cancel(context.Background(), fmt.Sprintf("lc-%d", port))
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !found {
		t.Error("expected found")
	}
}

func TestGetUIConfig(t *testing.T) {
	t.Parallel()
	port := startTestService(t)

	c1, _ := Dial(port)
	defer c1.Close()

	id := fmt.Sprintf("ui-%d", port)
	go func() {
		c1.Notify(context.Background(), testCfg(id))
	}()
	time.Sleep(200 * time.Millisecond)

	c2, _ := Dial(port)
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

func TestDialDefault(t *testing.T) {
	t.Parallel()
	// DialDefault tries the default port — just confirm it doesn't panic.
	// Connection may fail (no server), that's fine.
	c, err := DialDefault()
	if err == nil {
		c.Close()
	}
}
