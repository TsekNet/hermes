package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TsekNet/hermes/internal/app"
	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/google/deck"
	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wopts "github.com/wailsapp/wails/v2/pkg/options/windows"
)

func historyCmd() *cobra.Command {
	var (
		asJSON bool
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "history",
		Short: "View notification history",
		Long: `Opens the notification history UI showing past notifications, or prints
history as JSON. Action buttons in the history view re-execute the original
action (uri: opens the URI, action: runs the built-in verb).`,
		Example: `  hermes history
  hermes history --json`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if asJSON {
				return printHistoryJSON(dbPath)
			}
			return runHistoryUI(dbPath)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print history as JSON to stdout")
	cmd.Flags().StringVar(&dbPath, "db", "", "read directly from bolt DB (skip service)")
	return cmd
}

func printHistoryJSON(dbPath string) error {
	entries, err := fetchHistory(dbPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func runHistoryUI(dbPath string) error {
	var historyApp *app.HistoryApp

	if dbPath != "" {
		s, err := store.OpenReadOnly(dbPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		historyApp = app.NewHistoryLocal(s)
	} else {
		c, err := client.Dial()
		if err != nil {
			deck.Warningf("service not reachable, falling back to direct DB read")
			s, err := store.OpenReadOnly("")
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			historyApp = app.NewHistoryLocal(s)
		} else {
			historyApp = app.NewHistory(c)
		}
	}

	return wails.Run(&options.App{
		Title:         "Notification History",
		Width:         app.HistoryWidth,
		Height:        app.HistoryHeight,
		Frameless:     false,
		DisableResize: false,
		StartHidden:   false,
		AssetServer:   &assetserver.Options{Assets: frontendAssets},
		OnStartup:     historyApp.Startup,
		OnShutdown:    historyApp.Shutdown,
		Bind:          []interface{}{historyApp},
		Windows: &wopts.Options{IsZoomControlEnabled: false, DisableWindowIcon: false},
	})
}

func fetchHistory(dbPath string) ([]app.HistoryEntry, error) {
	if dbPath != "" {
		return fetchHistoryFromDB(dbPath)
	}
	entries, err := fetchHistoryFromService()
	if err != nil {
		deck.Warningf("service not reachable (%v), falling back to direct DB read", err)
		return fetchHistoryFromDB("")
	}
	return entries, nil
}

func fetchHistoryFromDB(dbPath string) ([]app.HistoryEntry, error) {
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	defer s.Close()
	records, err := s.LoadHistory()
	if err != nil {
		return nil, err
	}
	out := make([]app.HistoryEntry, 0, len(records))
	for _, r := range records {
		if r.Config == nil {
			continue
		}
		out = append(out, app.HistoryEntryFromRecord(r))
	}
	return out, nil
}

func fetchHistoryFromService() ([]app.HistoryEntry, error) {
	c, err := client.Dial()
	if err != nil {
		return nil, fmt.Errorf("connect to service: %w", err)
	}
	defer c.Close()
	entries, err := c.ListHistory(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]app.HistoryEntry, len(entries))
	for i, e := range entries {
		out[i] = app.HistoryEntryFromClientEntry(e)
	}
	return out, nil
}
