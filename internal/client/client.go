// Package client provides a gRPC client for the hermes service.
// Used by CLI commands (hermes notify, list, cancel) and the UI subprocess.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/TsekNet/hermes/internal/auth"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/socket"
	pb "github.com/TsekNet/hermes/proto"
	"google.golang.org/grpc"
)

// Client wraps a gRPC connection to the hermes service.
type Client struct {
	conn *grpc.ClientConn
	svc  pb.HermesServiceClient
}

// Dial connects to the hermes gRPC service via Unix domain socket.
// It auto-loads the session token from disk for authentication.
func Dial() (*Client, error) {
	return DialPath(socket.Path())
}

// DialPath connects to the hermes gRPC service at the given socket path.
func DialPath(sockPath string) (*Client, error) {
	var extraOpts []grpc.DialOption
	if token, err := auth.LoadToken(); err == nil && token != "" {
		extraOpts = append(extraOpts, grpc.WithPerRPCCredentials(&auth.PerRPCCredentials{Token: token}))
	}
	conn, err := socket.Dial(sockPath, extraOpts...)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, svc: pb.NewHermesServiceClient(conn)}, nil
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// NotifyResult is the response from a Notify RPC.
type NotifyResult struct {
	Value    string
	ExitCode int32
	Error    string
}

// Notify sends a notification config and blocks until the user responds.
func (c *Client) Notify(ctx context.Context, cfg *config.NotificationConfig) (*NotifyResult, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	resp, err := c.svc.Notify(ctx, &pb.NotifyRequest{
		Id:         cfg.ID,
		ConfigJson: data,
	})
	if err != nil {
		return nil, fmt.Errorf("notify rpc: %w", err)
	}
	return &NotifyResult{
		Value:    resp.Value,
		ExitCode: resp.ExitCode,
		Error:    resp.Error,
	}, nil
}

// GetUIConfig retrieves the notification config for a UI subprocess.
func (c *Client) GetUIConfig(ctx context.Context, notificationID string) (*config.NotificationConfig, bool, error) {
	resp, err := c.svc.GetUIConfig(ctx, &pb.GetUIConfigRequest{
		NotificationId: notificationID,
	})
	if err != nil {
		return nil, false, fmt.Errorf("get ui config rpc: %w", err)
	}
	cfg, err := config.Load(resp.ConfigJson)
	if err != nil {
		return nil, false, fmt.Errorf("parse config from service: %w", err)
	}
	return cfg, resp.DeferralAllowed, nil
}

// ReportChoice sends a user response to the service.
func (c *Client) ReportChoice(ctx context.Context, notificationID, value string) (bool, error) {
	resp, err := c.svc.ReportChoice(ctx, &pb.ReportChoiceRequest{
		NotificationId: notificationID,
		Value:          value,
	})
	if err != nil {
		return false, fmt.Errorf("report choice rpc: %w", err)
	}
	return resp.Accepted, nil
}

// Cancel cancels an active notification.
func (c *Client) Cancel(ctx context.Context, notificationID string) (bool, error) {
	resp, err := c.svc.Cancel(ctx, &pb.CancelRequest{
		NotificationId: notificationID,
	})
	if err != nil {
		return false, fmt.Errorf("cancel rpc: %w", err)
	}
	return resp.Found, nil
}

// ListEntry is a single notification from the List RPC.
type ListEntry struct {
	ID         string
	Heading    string
	State      string
	DeferCount int
	Deadline   time.Time
	CreatedAt  time.Time
}

// List returns all active notifications from the service.
func (c *Client) List(ctx context.Context) ([]ListEntry, error) {
	resp, err := c.svc.List(ctx, &pb.ListRequest{})
	if err != nil {
		return nil, fmt.Errorf("list rpc: %w", err)
	}
	var out []ListEntry
	for _, ni := range resp.Notifications {
		entry := ListEntry{
			ID:         ni.Id,
			Heading:    ni.Heading,
			State:      ni.State,
			DeferCount: int(ni.DeferCount),
			CreatedAt:  time.Unix(ni.CreatedUnix, 0),
		}
		if ni.DeadlineUnix > 0 {
			entry.Deadline = time.Unix(ni.DeadlineUnix, 0)
		}
		out = append(out, entry)
	}
	return out, nil
}

// HistoryEntry is a notification from the ListHistory RPC.
// ActionRequired is true for active notifications that still need user input.
type HistoryEntry struct {
	ID             string
	Heading        string
	Message        string
	Source         string
	ResponseValue  string
	CreatedAt      time.Time
	CompletedAt    time.Time
	ActionRequired bool
	ConfigJSON     []byte
}

// ListHistory returns completed notification history from the service.
func (c *Client) ListHistory(ctx context.Context) ([]HistoryEntry, error) {
	resp, err := c.svc.ListHistory(ctx, &pb.ListHistoryRequest{})
	if err != nil {
		return nil, fmt.Errorf("list history rpc: %w", err)
	}
	out := make([]HistoryEntry, len(resp.Records))
	for i, r := range resp.Records {
		active := r.ResponseValue == "" && r.CompletedUnix == 0
		out[i] = HistoryEntry{
			ID:             r.Id,
			Heading:        r.Heading,
			Message:        r.Message,
			Source:         r.Source,
			ResponseValue:  r.ResponseValue,
			CreatedAt:      time.Unix(r.CreatedUnix, 0),
			CompletedAt:    time.Unix(r.CompletedUnix, 0),
			ActionRequired: active,
			ConfigJSON:     r.ConfigJson,
		}
	}
	return out, nil
}

// Shutdown requests a graceful daemon shutdown via gRPC.
func (c *Client) Shutdown(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := c.svc.Shutdown(ctx, &pb.ShutdownRequest{})
	return err
}
