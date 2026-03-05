// Package app provides the Wails application backend exposed to frontend JS.
package app

import (
	"context"
	goRuntime "runtime"
	"strings"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/action"
	"github.com/TsekNet/hermes/internal/watch"
	"github.com/google/deck"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	WindowWidth        = 380
	WindowHeight       = 220
	WindowHeightImages = 480
)

// grpcReporter is the interface the App uses to report choices back to the
// gRPC service. Satisfied by *client.Client.
type grpcReporter interface {
	ReportChoice(ctx context.Context, notificationID, value string) (bool, error)
	Close() error
}

// App is the Wails application backend exposed to frontend JS.
type App struct {
	ctx            context.Context
	cfg            *config.NotificationConfig
	Result         string
	grpcClient     grpcReporter
	notificationID string
	deferAllowed   bool
	watcher        *watch.Watcher
}

// New creates the App with the parsed config (local mode).
func New(cfg *config.NotificationConfig) *App {
	if cfg.Platform == "" {
		cfg.Platform = goRuntime.GOOS
	}
	return &App{cfg: cfg, deferAllowed: true}
}

// NewWithGRPC creates the App in service mode. The gRPC client is used to
// report the user's choice back to the service daemon.
func NewWithGRPC(cfg *config.NotificationConfig, gc grpcReporter, notifID string, deferAllowed bool) *App {
	if cfg.Platform == "" {
		cfg.Platform = goRuntime.GOOS
	}
	return &App{
		cfg:            cfg,
		grpcClient:     gc,
		notificationID: notifID,
		deferAllowed:   deferAllowed,
	}
}

// Height returns the effective window height based on config (taller for images).
func Height(cfg *config.NotificationConfig) int {
	if len(cfg.Images) > 0 {
		return WindowHeightImages
	}
	return WindowHeight
}

// Startup is the Wails OnStartup callback.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if len(a.cfg.WatchPaths) > 0 {
		a.watcher = watch.New(func(ev watch.Event) {
			wailsRuntime.EventsEmit(ctx, "fs:event", ev)
		})
		if err := a.watcher.Start(a.cfg.WatchPaths); err != nil {
			deck.Errorf("watch start: %v", err)
		}
	}
}

// Shutdown is the Wails OnShutdown callback.
func (a *App) Shutdown(_ context.Context) {
	if a.watcher != nil {
		a.watcher.Stop()
	}
}

// GetConfig returns the notification config for the frontend.
func (a *App) GetConfig() *config.NotificationConfig { return a.cfg }

// DeferralAllowed returns whether the user can still defer this notification.
// Called by the frontend to hide/show defer buttons.
func (a *App) DeferralAllowed() bool { return a.deferAllowed }

// Ready is called by the frontend after it has populated all UI elements.
// It positions the window in the platform's notification corner and reveals it.
func (a *App) Ready() {
	const margin = 12

	wailsRuntime.WindowCenter(a.ctx)
	cx, cy := wailsRuntime.WindowGetPosition(a.ctx)

	wailsRuntime.WindowSetPosition(a.ctx, 0, 0)
	ox, oy := wailsRuntime.WindowGetPosition(a.ctx)

	x := 2*(cx-ox) - margin

	var y int
	switch goRuntime.GOOS {
	case "windows":
		// Bottom-right: matches Action Center / native toasts.
		// Cocoa-style y math: large y = bottom of work area.
		y = 2*(cy-oy) - margin
	default:
		// macOS top-right (Cocoa y-axis: 0 = bottom, menu bar = top).
		// WKWebView reports origin (0,0) at bottom-left; adding margin
		// from the origin places the window just below the menu bar.
		// Linux top-right (GTK y-down): margin from top edge.
		y = oy + margin
	}

	deck.Infof("positioning: center=(%d,%d) origin=(%d,%d) -> (%d,%d)", cx, cy, ox, oy, x, y)
	wailsRuntime.WindowSetPosition(a.ctx, x, y)
	wailsRuntime.WindowShow(a.ctx)
}

// Respond handles the user's choice. Opens URLs, runs commands, sends gRPC, quits.
func (a *App) Respond(value string) {
	if strings.HasPrefix(value, "url:") {
		a.openURL(strings.TrimPrefix(value, "url:"))
		return
	}

	if action.IsCommand(value) {
		if !action.Allowed(value) {
			deck.Warningf("blocked command: %s", value)
		} else if out, err := action.RunCommand(value); err != nil {
			deck.Errorf("cmd: %v: %s", err, out)
		}
	}

	a.Result = value

	// gRPC mode: report choice to service.
	if a.grpcClient != nil && a.notificationID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := a.grpcClient.ReportChoice(ctx, a.notificationID, value); err != nil {
			deck.Errorf("grpc report choice: %v", err)
		}
		a.grpcClient.Close()
		wailsRuntime.Quit(a.ctx)
		return
	}

	// Local mode: just quit (result is read from a.Result by the caller).
	wailsRuntime.Quit(a.ctx)
}

// OpenHelp opens the help URL in the default browser.
func (a *App) OpenHelp() {
	if a.cfg.HelpURL != "" {
		a.openURL(a.cfg.HelpURL)
	}
}

func (a *App) openURL(rawURL string) {
	if !action.Allowed(rawURL) {
		deck.Warningf("blocked URI: %s", rawURL)
		return
	}
	wailsRuntime.BrowserOpenURL(a.ctx, rawURL)
}
