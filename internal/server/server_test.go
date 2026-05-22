package server

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/manager"
	"github.com/TsekNet/hermes/internal/socket"
	"github.com/TsekNet/hermes/internal/store"
	pb "github.com/TsekNet/hermes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// startTestServer starts a gRPC server on a UDS in a temp dir and returns a client.
func startTestServer(t *testing.T) (pb.HermesServiceClient, *Server) {
	t.Helper()

	mgr := manager.New(func(n *manager.Notification) {}, nil)
	sock := filepath.Join(t.TempDir(), "hermes.sock")
	srv := New(mgr)

	lis, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		socket.DialTarget(sock),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return pb.NewHermesServiceClient(conn), srv
}

func testConfigJSON(heading string) []byte {
	cfg := &config.NotificationConfig{
		Heading:        heading,
		Message:        "Test message",
		TimeoutSeconds: 10,
		TimeoutValue:   "auto",
		Buttons: []config.Button{
			{Label: "OK", Value: "ok"},
			{Label: "Restart", Value: "restart"},
		},
	}
	data, _ := json.Marshal(cfg)
	return data
}

func TestNotify_FullCycle(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Notify in background (blocks until choice is reported).
	type result struct {
		resp *pb.NotifyResponse
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := client.Notify(ctx, &pb.NotifyRequest{
			Id:         "test-1",
			ConfigJson: testConfigJSON("Full Cycle"),
		})
		ch <- result{resp, err}
	}()

	// Give Notify time to register.
	time.Sleep(200 * time.Millisecond)

	// Simulate UI reporting a choice.
	resp, err := client.ReportChoice(ctx, &pb.ReportChoiceRequest{
		NotificationId: "test-1",
		Value:          "restart",
	})
	if err != nil {
		t.Fatalf("ReportChoice: %v", err)
	}
	if !resp.Accepted {
		t.Error("expected accepted")
	}

	// Notify should now return.
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("Notify: %v", r.err)
		}
		if r.resp.Value != "restart" {
			t.Errorf("value = %q, want restart", r.resp.Value)
		}
		if r.resp.ExitCode != 0 {
			t.Errorf("exitCode = %d, want 0", r.resp.ExitCode)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Notify did not return")
	}
}

func TestGetUIConfig(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)
	ctx := context.Background()

	// Submit a notification first.
	go func() {
		client.Notify(ctx, &pb.NotifyRequest{
			Id:         "ui-test",
			ConfigJson: testConfigJSON("UI Config"),
		})
	}()
	time.Sleep(200 * time.Millisecond)

	resp, err := client.GetUIConfig(ctx, &pb.GetUIConfigRequest{
		NotificationId: "ui-test",
	})
	if err != nil {
		t.Fatalf("GetUIConfig: %v", err)
	}
	if resp.ConfigJson == nil {
		t.Fatal("expected non-nil config JSON")
	}

	var cfg config.NotificationConfig
	if err := json.Unmarshal(resp.ConfigJson, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.Heading != "UI Config" {
		t.Errorf("heading = %q", cfg.Heading)
	}
}

func TestCancel(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)
	ctx := context.Background()

	ch := make(chan *pb.NotifyResponse, 1)
	go func() {
		resp, _ := client.Notify(ctx, &pb.NotifyRequest{
			Id:         "cancel-test",
			ConfigJson: testConfigJSON("Cancel"),
		})
		ch <- resp
	}()
	time.Sleep(200 * time.Millisecond)

	resp, err := client.Cancel(ctx, &pb.CancelRequest{NotificationId: "cancel-test"})
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !resp.Found {
		t.Error("expected found")
	}

	select {
	case r := <-ch:
		if r.Value != "cancelled" {
			t.Errorf("value = %q, want cancelled", r.Value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Notify did not return after cancel")
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)
	ctx := context.Background()

	// Start two notifications.
	go func() {
		client.Notify(ctx, &pb.NotifyRequest{Id: "list-1", ConfigJson: testConfigJSON("One")})
	}()
	go func() {
		client.Notify(ctx, &pb.NotifyRequest{Id: "list-2", ConfigJson: testConfigJSON("Two")})
	}()
	time.Sleep(200 * time.Millisecond)

	resp, err := client.List(ctx, &pb.ListRequest{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(resp.Notifications) < 2 {
		t.Errorf("notifications = %d, want >= 2", len(resp.Notifications))
	}
}

func TestListHistory_AfterCompletion(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	s, err := store.Open(filepath.Join(tmpDir, "history.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	mgr := manager.New(func(n *manager.Notification) {}, s)
	sock := filepath.Join(tmpDir, "hermes.sock")
	srv := New(mgr)

	lis, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		socket.DialTarget(sock),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	client := pb.NewHermesServiceClient(conn)
	ctx := context.Background()

	// Submit and complete a notification.
	ch := make(chan *pb.NotifyResponse, 1)
	go func() {
		resp, _ := client.Notify(ctx, &pb.NotifyRequest{
			Id:         "hist-1",
			ConfigJson: testConfigJSON("History Test"),
		})
		ch <- resp
	}()
	time.Sleep(200 * time.Millisecond)

	client.ReportChoice(ctx, &pb.ReportChoiceRequest{
		NotificationId: "hist-1",
		Value:          "restart",
	})

	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("Notify did not return")
	}

	// ListHistory should return the completed notification.
	resp, err := client.ListHistory(ctx, &pb.ListHistoryRequest{})
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(resp.Records) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(resp.Records))
	}
	r := resp.Records[0]
	if r.Id != "hist-1" {
		t.Errorf("ID = %q, want hist-1", r.Id)
	}
	if r.Heading != "History Test" {
		t.Errorf("Heading = %q, want %q", r.Heading, "History Test")
	}
	if r.ResponseValue != "restart" {
		t.Errorf("ResponseValue = %q, want restart", r.ResponseValue)
	}
	if r.CompletedUnix == 0 {
		t.Error("CompletedUnix should be non-zero")
	}
}

func TestListHistory_IncludesActiveItems(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)
	ctx := context.Background()

	// Submit two notifications without completing them.
	go func() {
		client.Notify(ctx, &pb.NotifyRequest{Id: "active-1", ConfigJson: testConfigJSON("Active One")})
	}()
	go func() {
		client.Notify(ctx, &pb.NotifyRequest{Id: "active-2", ConfigJson: testConfigJSON("Active Two")})
	}()
	time.Sleep(200 * time.Millisecond)

	resp, err := client.ListHistory(ctx, &pb.ListHistoryRequest{})
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}

	// Should have 2 active items with config_json set and empty response_value.
	var activeCount int
	for _, r := range resp.Records {
		if r.ResponseValue == "" && r.CompletedUnix == 0 {
			activeCount++
			if r.ConfigJson == nil {
				t.Errorf("active record %s should have config_json", r.Id)
			}
		}
	}
	if activeCount != 2 {
		t.Errorf("active records = %d, want 2", activeCount)
	}
}

func TestListHistory_Empty(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)
	ctx := context.Background()

	resp, err := client.ListHistory(ctx, &pb.ListHistoryRequest{})
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(resp.Records) != 0 {
		t.Errorf("expected 0 history records, got %d", len(resp.Records))
	}
}

func TestNotify_ValidationErrors(t *testing.T) {
	t.Parallel()
	client, _ := startTestServer(t)
	ctx := context.Background()

	tests := []struct {
		name       string
		configJSON []byte
	}{
		{
			name:       "bad JSON",
			configJSON: []byte("{invalid json}"),
		},
		{
			name: "missing required fields",
			configJSON: func() []byte {
				data, _ := json.Marshal(&config.NotificationConfig{})
				return data
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp, err := client.Notify(ctx, &pb.NotifyRequest{ConfigJson: tt.configJSON})
			if err != nil {
				t.Fatalf("Notify: %v", err)
			}
			if resp.ExitCode != 1 {
				t.Errorf("exitCode = %d, want 1", resp.ExitCode)
			}
			if resp.Error == "" {
				t.Error("expected error message")
			}
		})
	}
}
