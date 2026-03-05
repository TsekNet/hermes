package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TsekNet/hermes/internal/app"
	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/google/deck"
	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wopts "github.com/wailsapp/wails/v2/pkg/options/windows"
)

func inboxCmd() *cobra.Command {
	var (
		port   int
		asJSON bool
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "View notification history",
		Long: `Opens the notification history UI showing past notifications, or prints
history as JSON. Action buttons in the history view execute cmd:-prefixed
response values (e.g. "cmd:shutdown /r /t 0") via the platform shell.`,
		Example: `  hermes inbox
  hermes inbox --json`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if asJSON {
				return printInboxJSON(port, dbPath)
			}
			return runInboxUI(port, dbPath)
		},
	}
	cmd.Flags().IntVar(&port, "port", server.DefaultPort, "service gRPC port")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print history as JSON to stdout")
	cmd.Flags().StringVar(&dbPath, "db", "", "read directly from bolt DB (skip service)")
	return cmd
}

func printInboxJSON(port int, dbPath string) error {
	entries, err := fetchHistory(port, dbPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func runInboxUI(port int, dbPath string) error {
	var inboxApp *app.InboxApp

	if dbPath != "" {
		s, err := store.OpenReadOnly(dbPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		inboxApp = app.NewInboxLocal(s)
	} else {
		c, err := client.Dial(port)
		if err != nil {
			deck.Warningf("service not reachable, falling back to direct DB read")
			s, err := store.OpenReadOnly("")
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			inboxApp = app.NewInboxLocal(s)
		} else {
			inboxApp = app.NewInbox(c)
		}
	}

	return wails.Run(&options.App{
		Title:         "Notification History",
		Width:         app.InboxWidth,
		Height:        app.InboxHeight,
		Frameless:     false,
		DisableResize: false,
		StartHidden:   false,
		AssetServer:   &assetserver.Options{Assets: frontendAssets},
		OnStartup:     inboxApp.Startup,
		OnShutdown:    inboxApp.Shutdown,
		Bind:          []interface{}{inboxApp},
		Windows:       &wopts.Options{IsZoomControlEnabled: false, DisableWindowIcon: false},
	})
}

func fetchHistory(port int, dbPath string) ([]app.InboxEntry, error) {
	if dbPath != "" {
		return fetchHistoryFromDB(dbPath)
	}
	entries, err := fetchHistoryFromService(port)
	if err != nil {
		deck.Warningf("service not reachable (%v), falling back to direct DB read", err)
		return fetchHistoryFromDB("")
	}
	return entries, nil
}

func fetchHistoryFromDB(dbPath string) ([]app.InboxEntry, error) {
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer s.Close()
	records, err := s.LoadHistory()
	if err != nil {
		return nil, err
	}
	out := make([]app.InboxEntry, 0, len(records))
	for _, r := range records {
		if r.Config == nil {
			continue
		}
		out = append(out, app.InboxEntryFromRecord(r))
	}
	return out, nil
}

func fetchHistoryFromService(port int) ([]app.InboxEntry, error) {
	c, err := client.Dial(port)
	if err != nil {
		return nil, fmt.Errorf("connect to service: %w", err)
	}
	defer c.Close()
	entries, err := c.ListHistory(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]app.InboxEntry, len(entries))
	for i, e := range entries {
		out[i] = app.InboxEntryFromClientEntry(e)
	}
	return out, nil
}
