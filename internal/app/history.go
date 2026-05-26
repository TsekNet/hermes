package app

import (
	"context"
	"encoding/json"
	"time"

	"github.com/TsekNet/hermes/internal/action"
	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/respond"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/google/deck"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	HistoryWidth  = 480
	HistoryHeight = 520
)

// HistoryButton is the JSON shape for an inline action button.
type HistoryButton struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Style string `json:"style,omitempty"`
}

// HistoryEntry is the JSON shape sent to the history frontend.
type HistoryEntry struct {
	ID             string          `json:"id"`
	Heading        string          `json:"heading"`
	Message        string          `json:"message"`
	Source         string          `json:"source"`
	ResponseValue  string          `json:"response_value"`
	CreatedAt      string          `json:"created_at"`
	CompletedAt    string          `json:"completed_at"`
	ActionRequired bool            `json:"action_required"`
	Buttons        []HistoryButton `json:"buttons,omitempty"`
}

// HistoryApp is the Wails backend for the history view.
type HistoryApp struct {
	ctx        context.Context
	grpcClient *client.Client
	localStore *store.Store
}

// NewHistory creates a HistoryApp that reads history from the gRPC service.
func NewHistory(c *client.Client) *HistoryApp {
	return &HistoryApp{grpcClient: c}
}

// NewHistoryLocal creates a HistoryApp that reads history directly from the store.
func NewHistoryLocal(s *store.Store) *HistoryApp {
	return &HistoryApp{localStore: s}
}

func (a *HistoryApp) Startup(ctx context.Context) { a.ctx = ctx }

func (a *HistoryApp) Shutdown(_ context.Context) {
	if a.grpcClient != nil {
		a.grpcClient.Close()
	}
	if a.localStore != nil {
		a.localStore.Close()
	}
}

func (a *HistoryApp) Ready() {
	wailsRuntime.WindowCenter(a.ctx)
	wailsRuntime.WindowShow(a.ctx)
}

// GetHistory returns history entries for the frontend.
func (a *HistoryApp) GetHistory() []HistoryEntry {
	if a.grpcClient != nil {
		return a.historyFromGRPC()
	}
	return a.historyFromStore()
}

// HistoryEntryFromRecord converts a store.HistoryRecord to a HistoryEntry.
func HistoryEntryFromRecord(r *store.HistoryRecord) HistoryEntry {
	return HistoryEntry{
		ID:            r.ID,
		Heading:       r.Config.Heading,
		Message:       r.Config.Message,
		Source:        r.Config.Title,
		ResponseValue: r.ResponseValue,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		CompletedAt:   r.CompletedAt.Format(time.RFC3339),
	}
}

// HistoryEntryFromClientEntry converts a client.HistoryEntry to a HistoryEntry.
func HistoryEntryFromClientEntry(e client.HistoryEntry) HistoryEntry {
	entry := HistoryEntry{
		ID:             e.ID,
		Heading:        e.Heading,
		Message:        e.Message,
		Source:         e.Source,
		ResponseValue:  e.ResponseValue,
		CreatedAt:      e.CreatedAt.Format(time.RFC3339),
		CompletedAt:    e.CompletedAt.Format(time.RFC3339),
		ActionRequired: e.ActionRequired,
	}
	if e.ActionRequired && len(e.ConfigJSON) > 0 {
		var cfg config.NotificationConfig
		if err := json.Unmarshal(e.ConfigJSON, &cfg); err == nil {
			for _, b := range cfg.Buttons {
				entry.Buttons = append(entry.Buttons, HistoryButton{
					Label: b.Label,
					Value: b.Value,
					Style: b.Style,
				})
			}
		}
	}
	return entry
}

// RespondToNotification sends a user choice for an active notification.
// Uses the shared respond.Submit path for validation and defer filtering.
// Returns "ok" on success or an error string.
func (a *HistoryApp) RespondToNotification(id, value string) string {
	if a.grpcClient == nil {
		return "error: no service connection"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := respond.Submit(ctx, a.grpcClient, id, value)
	if err != nil {
		deck.Errorf("history: report choice %s=%s: %v", id, value, err)
		return "error: " + err.Error()
	}
	deck.Infof("history: responded to %s with %s", id, value)
	return "ok"
}

// RunAction re-opens a uri:-prefixed value from history. Only uri: is
// supported from history: re-running action:reboot from history would
// be a footgun.
func (a *HistoryApp) RunAction(id, value string) string {
	deck.Infof("history: action %s -> %s", id, value)
	if action.IsURI(value) {
		uri := value[len("uri:"):]
		if err := action.OpenURI(uri); err != nil {
			deck.Errorf("history: open URI failed: %v", err)
			return "error: " + err.Error()
		}
		return "ok"
	}
	return value
}

func (a *HistoryApp) historyFromGRPC() []HistoryEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, err := a.grpcClient.ListHistory(ctx)
	if err != nil {
		deck.Errorf("history: list history rpc: %v", err)
		return nil
	}
	out := make([]HistoryEntry, len(entries))
	for i, e := range entries {
		out[i] = HistoryEntryFromClientEntry(e)
	}
	return out
}

func (a *HistoryApp) historyFromStore() []HistoryEntry {
	if a.localStore == nil {
		return nil
	}
	records, err := a.localStore.LoadHistory()
	if err != nil {
		deck.Errorf("history: load history: %v", err)
		return nil
	}
	out := make([]HistoryEntry, 0, len(records))
	for _, r := range records {
		if r.Config == nil {
			continue
		}
		out = append(out, HistoryEntryFromRecord(r))
	}
	return out
}
