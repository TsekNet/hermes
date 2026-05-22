// Package app provides the Wails application backend exposed to frontend JS.
package app

import (
	"context"
	goRuntime "runtime"
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
// Caller must call cfg.ApplyDefaults() first (sets Platform if empty).
func New(cfg *config.NotificationConfig) *App {
	return &App{cfg: cfg, deferAllowed: true}
}

// NewWithGRPC creates the App in service mode. The gRPC client is used to
// report the user's choice back to the service daemon.
// Caller must call cfg.ApplyDefaults() first (sets Platform if empty).
func NewWithGRPC(cfg *config.NotificationConfig, gc grpcReporter, notifID string, deferAllowed bool) *App {
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
//
// WindowCenter uses the work area (excludes taskbar/menu bar/dock) on all
// platforms: Win32 MonitorInfo.rcWork, Cocoa visibleFrame, GTK workarea.
// We center, read back the position to learn the work-area center, then
// compute the notification corner from that.
func (a *App) Ready() {
	const margin = 12

	wailsRuntime.WindowCenter(a.ctx)
	cx, cy := wailsRuntime.WindowGetPosition(a.ctx)
	w, h := wailsRuntime.WindowGetSize(a.ctx)

	// cx,cy is the top-left when centered in the work area.
	// Work-area width  ≈ cx + w/2 + (cx + w/2 - workOriginX) → simplified: 2*cx + w
	// This holds because center places the window at (waW-w)/2 from the work-area origin.
	waW := 2*cx + w
	waH := 2*cy + h

	x := waW - w - margin

	var y int
	switch goRuntime.GOOS {
	case "windows":
		y = waH - h - margin
	default:
		y = margin
	}

	deck.Infof("positioning: workarea≈%dx%d window=%dx%d center=(%d,%d) -> (%d,%d)", waW, waH, w, h, cx, cy, x, y)
	wailsRuntime.WindowSetPosition(a.ctx, x, y)
	wailsRuntime.WindowShow(a.ctx)
}

// Respond handles the user's choice. Opens URIs, runs builtins, sends gRPC, quits.
func (a *App) Respond(value string) {
	if !a.cfg.HasValue(value) && !action.IsURI(value) && !action.IsBuiltin(value) {
		deck.Warningf("respond: rejected unknown value %q", value)
		return
	}

	if action.IsURI(value) {
		a.openURI(value[len("uri:"):])
		return
	}

	if action.IsBuiltin(value) {
		verb := value[len("action:"):]
		if err := action.RunBuiltin(verb); err != nil {
			deck.Errorf("action: %v", err)
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
		a.openURI(a.cfg.HelpURL)
	}
}

func (a *App) openURI(rawURI string) {
	if err := action.OpenURI(rawURI); err != nil {
		deck.Warningf("open URI: %v", err)
		return
	}
}
