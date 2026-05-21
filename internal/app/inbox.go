package app

import (
	"context"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/TsekNet/hermes/internal/action"
	"github.com/google/deck"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	InboxWidth  = 480
	InboxHeight = 520
)

// InboxEntry is the JSON shape sent to the inbox frontend.
type InboxEntry struct {
	ID            string `json:"id"`
	Heading       string `json:"heading"`
	Message       string `json:"message"`
	Source        string `json:"source"`
	ResponseValue string `json:"response_value"`
	CreatedAt     string `json:"created_at"`
	CompletedAt   string `json:"completed_at"`
}

// InboxApp is the Wails backend for the inbox view.
type InboxApp struct {
	ctx        context.Context
	grpcClient *client.Client
	localStore *store.Store
}

// NewInbox creates an InboxApp that reads history from the gRPC service.
func NewInbox(c *client.Client) *InboxApp {
	return &InboxApp{grpcClient: c}
}

// NewInboxLocal creates an InboxApp that reads history directly from the store.
func NewInboxLocal(s *store.Store) *InboxApp {
	return &InboxApp{localStore: s}
}

func (a *InboxApp) Startup(ctx context.Context) { a.ctx = ctx }

func (a *InboxApp) Shutdown(_ context.Context) {
	if a.grpcClient != nil {
		a.grpcClient.Close()
	}
	if a.localStore != nil {
		a.localStore.Close()
	}
}

func (a *InboxApp) Ready() {
	wailsRuntime.WindowCenter(a.ctx)
	wailsRuntime.WindowShow(a.ctx)
}

// GetHistory returns inbox entries for the frontend.
func (a *InboxApp) GetHistory() []InboxEntry {
	if a.grpcClient != nil {
		return a.historyFromGRPC()
	}
	return a.historyFromStore()
}

// InboxEntryFromRecord converts a store.HistoryRecord to an InboxEntry.
func InboxEntryFromRecord(r *store.HistoryRecord) InboxEntry {
	return InboxEntry{
		ID:            r.ID,
		Heading:       r.Config.Heading,
		Message:       r.Config.Message,
		Source:        r.Config.Title,
		ResponseValue: r.ResponseValue,
		CreatedAt:     r.CreatedAt.Format(time.RFC3339),
		CompletedAt:   r.CompletedAt.Format(time.RFC3339),
	}
}

// InboxEntryFromClientEntry converts a client.HistoryEntry to an InboxEntry.
func InboxEntryFromClientEntry(e client.HistoryEntry) InboxEntry {
	return InboxEntry{
		ID:            e.ID,
		Heading:       e.Heading,
		Message:       e.Message,
		Source:        e.Source,
		ResponseValue: e.ResponseValue,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
		CompletedAt:   e.CompletedAt.Format(time.RFC3339),
	}
}

// RunAction re-opens a uri:-prefixed value from history. Only uri: is
// supported from the inbox: re-running action:reboot from history would
// be a footgun.
func (a *InboxApp) RunAction(id, value string) string {
	deck.Infof("inbox: action %s -> %s", id, value)
	if action.IsURI(value) {
		uri := value[len("uri:"):]
		if err := action.OpenURI(uri); err != nil {
			deck.Errorf("inbox: open URI failed: %v", err)
			return "error: " + err.Error()
		}
		return "ok"
	}
	return value
}

func (a *InboxApp) historyFromGRPC() []InboxEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, err := a.grpcClient.ListHistory(ctx)
	if err != nil {
		deck.Errorf("inbox: list history rpc: %v", err)
		return nil
	}
	out := make([]InboxEntry, len(entries))
	for i, e := range entries {
		out[i] = InboxEntryFromClientEntry(e)
	}
	return out
}

func (a *InboxApp) historyFromStore() []InboxEntry {
	if a.localStore == nil {
		return nil
	}
	records, err := a.localStore.LoadHistory()
	if err != nil {
		deck.Errorf("inbox: load history: %v", err)
		return nil
	}
	out := make([]InboxEntry, 0, len(records))
	for _, r := range records {
		if r.Config == nil {
			continue
		}
		out = append(out, InboxEntryFromRecord(r))
	}
	return out
}
