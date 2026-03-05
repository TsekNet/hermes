// Package cmd defines the Cobra CLI commands for hermes.
package cmd

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/TsekNet/hermes/internal/app"
	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/dnd"
	"github.com/TsekNet/hermes/internal/exitcodes"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/google/deck"
	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wopts "github.com/wailsapp/wails/v2/pkg/options/windows"
)

// frontendAssets holds the embedded frontend FS, injected by Execute.
var frontendAssets embed.FS

// Flags populated by Cobra.
var (
	flagLocal          bool
	flagConfig         string
	flagLocale         string
	flagNotificationID string
	flagServicePort    int
)

// Execute is the single entry point called from main.
func Execute(assets embed.FS) error {
	frontendAssets = assets
	return buildRootCmd().Execute()
}

// RunBindings launches the UI with a demo config for Wails binding generation.
func RunBindings(assets embed.FS) {
	frontendAssets = assets
	runUI(demoConfig())
}

func buildRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "hermes [config]",
		Short: "Cross-platform web-based notification CLI",
		Long: `hermes renders a web UI (HTML/CSS/JS) inside a frameless webview.
The same notification looks identical on Windows, macOS, and Linux.

With no arguments, sends a demo notification to the service daemon
(falls back to local display if the service is not running).

Use 'hermes serve' to start the service daemon.
Use 'hermes notify' to send a notification via the service.
Use '--config' or '--local' to render directly without the service.`,
		Example: `  hermes                                # demo (via service or local)
  hermes --config notification.json     # send to service
  hermes --local '{"heading":"Test"}'   # render locally
  hermes notify '{"heading":"..."}'     # explicit service RPC
  hermes serve                          # start service daemon`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runRoot,
	}

	f := root.Flags()
	f.BoolVar(&flagLocal, "local", false, "render locally in current session (skip service)")
	f.StringVar(&flagConfig, "config", "", "config file or inline JSON/YAML")
	f.StringVar(&flagLocale, "locale", "", "override locale for localized notifications (e.g. ja, de, es)")

	// Hidden flags — set by the service when launching UI subprocesses.
	f.StringVar(&flagNotificationID, "notification-id", "", "notification ID (set by service)")
	f.IntVar(&flagServicePort, "service-port", server.DefaultPort, "gRPC service port (set by service)")
	f.MarkHidden("notification-id")
	f.MarkHidden("service-port")

	root.AddCommand(demoCmd())
	root.AddCommand(versionCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(notifyCmd())
	root.AddCommand(listCmd())
	root.AddCommand(cancelCmd())
	root.AddCommand(inboxCmd())

	return root
}

func runRoot(_ *cobra.Command, args []string) error {
	// Mode 1: UI subprocess launched by the service (gRPC).
	if flagNotificationID != "" {
		return runServiceUI(flagNotificationID, flagServicePort)
	}

	// Resolve config from --config flag, positional arg, or stdin.
	cfg, err := resolveConfig(flagConfig, args)
	if err != nil {
		return err
	}

	// Mode 2: --local renders directly without the service.
	if flagLocal {
		if cfg == nil {
			cfg = demoConfig()
		}
		prepareConfig(cfg)
		if err := waitForDND(cfg); err != nil {
			return err
		}
		deck.Infof("local mode: heading=%q buttons=%d", cfg.Heading, len(cfg.Buttons))
		runUI(cfg)
		return nil
	}

	// Mode 3: Config found → send to service.
	if cfg != nil {
		prepareConfig(cfg)
		return sendToService(cfg)
	}

	// Mode 4: No config → demo (service or local fallback).
	return runDemo()
}

// runDemo sends a demo notification to the service. If the service is
// unreachable, falls back to local UI. This path also supports Wails
// binding generation (which runs the binary with no args).
func runDemo() error {
	c, err := client.DialDefault()
	if err == nil {
		defer c.Close()
		if err := c.Ping(context.Background()); err == nil {
			deck.Info("service reachable, sending demo via gRPC")
			result, err := c.Notify(context.Background(), demoConfig())
			if err != nil {
				return fmt.Errorf("demo via service: %w", err)
			}
			printResultAndExit(result)
		}
	}

	// Service not running — local fallback (also enables Wails binding gen).
	deck.Info("service not reachable, showing demo locally")
	cfg := demoConfig()
	waitForDND(cfg)
	runUI(cfg)
	return nil
}

// sendToService sends a config to the gRPC service and blocks for the result.
// If the service is unreachable, it falls back to the offline queue.
func sendToService(cfg *config.NotificationConfig) error {
	c, err := client.DialDefault()
	if err != nil {
		if tryEnqueue(cfg, err) {
			return nil
		}
		return fmt.Errorf("connect to service: %w (is 'hermes serve' running?)", err)
	}
	defer c.Close()

	result, err := c.Notify(context.Background(), cfg)
	if err != nil {
		if tryEnqueue(cfg, err) {
			return nil
		}
		return fmt.Errorf("notify: %w", err)
	}
	printResultAndExit(result)
	return nil
}

// tryEnqueue attempts to write a notification to the offline queue (bbolt).
// The DB lock acts as a mutex: if the daemon holds it, Open fails and we
// return false (the service IS running, so the gRPC error is "real").
// If Open succeeds, the daemon is down and we persist for later delivery.
func tryEnqueue(cfg *config.NotificationConfig, originalErr error) bool {
	if err := cfg.Validate(); err != nil {
		deck.Infof("queue fallback skipped (invalid config): %v", err)
		return false
	}
	if err := store.EnqueueOffline("", cfg, store.DefaultQueueTTL); err != nil {
		deck.Infof("queue fallback failed (service may be running): %v", err)
		return false
	}
	deck.Infof("service unavailable (%v), notification queued for next startup", originalErr)
	fmt.Print("queued")
	os.Stdout.Sync()
	os.Exit(int(exitcodes.Queued))
	return true // unreachable, but keeps the signature clean
}

// printResultAndExit prints the service result to stdout and exits with
// the appropriate code. Shared by sendToService, runNotify, and runDemo.
func printResultAndExit(r *client.NotifyResult) {
	if r.Error != "" {
		deck.Errorf("service: %s", r.Error)
		os.Exit(int(r.ExitCode))
	}
	fmt.Print(r.Value)
	os.Stdout.Sync()
	os.Exit(int(r.ExitCode))
}

// runServiceUI is Mode 1: the service spawned this process with a notification ID.
// It fetches config from the service via gRPC, renders the UI, and reports back.
func runServiceUI(notifID string, port int) error {
	c, err := client.Dial(port)
	if err != nil {
		return fmt.Errorf("connect to service: %w", err)
	}

	cfg, deferAllowed, err := c.GetUIConfig(context.Background(), notifID)
	if err != nil {
		c.Close()
		return fmt.Errorf("get ui config: %w", err)
	}
	cfg.ApplyDefaults()
	cfg.ApplyLocale(config.DetectLocale())

	// If deferrals are exhausted or deadline passed, filter out defer buttons.
	if !deferAllowed {
		cfg.Buttons = filterDeferButtons(cfg.Buttons)
	}

	a := app.NewWithGRPC(cfg, c, notifID, deferAllowed)

	err = wails.Run(&options.App{
		Title:         cfg.Title,
		Width:         app.WindowWidth,
		Height:        app.Height(cfg),
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		StartHidden:   true,
		AssetServer:   &assetserver.Options{Assets: frontendAssets},
		OnStartup:     a.Startup,
		OnShutdown:    a.Shutdown,
		Bind:          []interface{}{a},
		Windows:       &wopts.Options{IsZoomControlEnabled: false},
	})
	if err != nil {
		deck.Errorf("wails: %v", err)
		os.Exit(int(exitcodes.Error))
	}
	os.Exit(int(exitcodes.OK))
	return nil
}

// filterDeferButtons removes defer-valued buttons and dropdown options.
func filterDeferButtons(buttons []config.Button) []config.Button {
	var out []config.Button
	for _, btn := range buttons {
		if strings.HasPrefix(btn.Value, "defer") {
			continue
		}
		if len(btn.Dropdown) > 0 {
			var filtered []config.DropdownOption
			for _, opt := range btn.Dropdown {
				if !strings.HasPrefix(opt.Value, "defer") {
					filtered = append(filtered, opt)
				}
			}
			if len(filtered) == 0 && btn.Value == "" {
				continue
			}
			btn.Dropdown = filtered
		}
		out = append(out, btn)
	}
	return out
}

// resolveConfig loads config from a flag value, positional arg, or stdin.
// Returns (nil, nil) if no config source is available.
func resolveConfig(configFlag string, args []string) (*config.NotificationConfig, error) {
	if configFlag != "" {
		return loadFromArg(configFlag)
	}
	if len(args) > 0 {
		return loadFromArg(args[0])
	}
	info, _ := os.Stdin.Stat()
	if info != nil && (info.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, int64(config.MaxConfigSize)+1))
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		if len(data) > config.MaxConfigSize {
			return nil, fmt.Errorf("stdin too large: %d bytes (max %d)", len(data), config.MaxConfigSize)
		}
		trimmed := strings.TrimSpace(string(data))
		if trimmed == "" {
			return nil, nil
		}
		return config.LoadJSON([]byte(trimmed))
	}
	return nil, nil
}

// loadFromArg tries the arg as a file path first, then as inline JSON/YAML.
func loadFromArg(arg string) (*config.NotificationConfig, error) {
	if _, err := os.Stat(arg); err == nil {
		data, err := os.ReadFile(arg)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", arg, err)
		}
		deck.Infof("loaded config from file: %s", arg)
		return config.LoadJSON(data)
	}

	trimmed := strings.TrimSpace(arg)
	if strings.HasPrefix(trimmed, "{") {
		return config.LoadJSON([]byte(trimmed))
	}

	return nil, fmt.Errorf("not a file or valid config: %s", arg)
}

// waitForDND handles DND policy for local mode. Returns nil when it's safe
// to show the notification, or exits the process for "skip" mode.
func waitForDND(cfg *config.NotificationConfig) error {
	switch cfg.ResolvedDND() {
	case config.DNDIgnore:
		return nil
	case config.DNDSkip:
		if dnd.Active() {
			deck.Infof("DND active, skipping notification (dnd=skip)")
			fmt.Print("dnd_active")
			os.Stdout.Sync()
			os.Exit(int(exitcodes.OK))
		}
		return nil
	default: // "respect"
		for dnd.Active() {
			deck.Infof("DND active, waiting 60s to show notification (dnd=respect)")
			time.Sleep(60 * time.Second)
		}
		return nil
	}
}

// runUI opens the Wails webview with the given config. On error it exits
// with exitError; on success it prints the user's response to stdout.
func runUI(cfg *config.NotificationConfig) {
	if runtime.GOOS == "linux" {
		if os.Getenv("XDG_SESSION_TYPE") == "wayland" || os.Getenv("WAYLAND_DISPLAY") != "" {
			deck.Info("wayland session detected, forcing GDK_BACKEND=x11 for window positioning")
			os.Setenv("GDK_BACKEND", "x11")
		}
	}

	a := app.New(cfg)

	err := wails.Run(&options.App{
		Title:         cfg.Title,
		Width:         app.WindowWidth,
		Height:        app.Height(cfg),
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,
		StartHidden:   true,
		AssetServer:   &assetserver.Options{Assets: frontendAssets},
		OnStartup:     a.Startup,
		OnShutdown:    a.Shutdown,
		Bind:          []interface{}{a},
		Windows:       &wopts.Options{IsZoomControlEnabled: false},
	})
	if err != nil {
		deck.Errorf("wails: %v", err)
		os.Exit(int(exitcodes.Error))
	}

	respond(a.Result)
}

// prepareConfig applies defaults and locale resolution to a config.
func prepareConfig(cfg *config.NotificationConfig) {
	cfg.ApplyDefaults()
	locale := flagLocale
	if locale == "" {
		locale = config.DetectLocale()
	}
	cfg.ApplyLocale(locale)
}

// respond prints the value to stdout and exits with the appropriate code.
// The JS frontend prefixes timeout responses with "timeout:" so we can
// distinguish "user clicked restart" (exit 0) from "countdown expired" (exit 202).
func respond(value string) {
	if value == "" {
		// No interaction (dismissed or binding gen). Empty stdout signals "dismissed"
		// to callers. Exit 0 so Wails binding generation succeeds in CI.
		os.Exit(0)
	}
	clean := strings.TrimPrefix(value, "timeout:")
	fmt.Print(clean)
	os.Stdout.Sync()
	os.Exit(int(exitcodes.ForValue(value)))
}
