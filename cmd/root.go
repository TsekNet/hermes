// Package cmd defines the Cobra CLI commands for hermes.
package cmd

import (
	"context"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/TsekNet/hermes/internal/app"
	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/dnd"
	"github.com/TsekNet/hermes/internal/exitcodes"
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

With no arguments, renders a demo notification locally.

Use 'hermes serve' to start the service daemon.
Use 'hermes notify' to send a notification via the service.
Use '--config' or '--local' to render directly without the service.`,
		Example: `  hermes                                # demo (local)
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

	// Hidden flag — set by the service when launching UI subprocesses.
	f.StringVar(&flagNotificationID, "notification-id", "", "notification ID (set by service)")
	f.MarkHidden("notification-id")

	root.AddCommand(demoCmd())
	root.AddCommand(versionCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(notifyCmd())
	root.AddCommand(listCmd())
	root.AddCommand(cancelCmd())
	root.AddCommand(inboxCmd())
	root.AddCommand(stopCmd())
	root.AddCommand(motdCmd())
	root.AddCommand(installCmd())
	root.AddCommand(uninstallCmd())

	return root
}

func runRoot(_ *cobra.Command, args []string) error {
	// Mode 1: UI subprocess launched by the service (gRPC).
	if flagNotificationID != "" {
		return runServiceUI(flagNotificationID)
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
		deck.Infof("notification: mode=local heading=%q buttons=%d dnd=%s", cfg.Heading, len(cfg.Buttons), cfg.DND)
		runUI(cfg)
		return nil
	}

	// Mode 3: Config found → send to service (or broadcast if privileged).
	if cfg != nil {
		prepareConfig(cfg)
		if isPrivileged() {
			return broadcastNotify(cfg, args)
		}
		return sendToService(cfg)
	}

	// Mode 4: No config → demo (local).
	return runDemo()
}

// runDemo renders a demo notification locally. Always uses local mode so
// the UI appears reliably regardless of how the binary was launched
// (double-click, terminal, service context). Also enables Wails binding
// generation (which runs the binary with no args).
func runDemo() error {
	cfg := demoConfig()
	waitForDND(cfg)
	deck.Infof("notification: mode=demo heading=%q timeout=%ds", cfg.Heading, cfg.TimeoutSeconds)
	runUI(cfg)
	return nil
}

// sendToService sends a config to the gRPC service and blocks for the result.
// If the service is unreachable, it falls back to the offline queue.
func sendToService(cfg *config.NotificationConfig) error {
	c, err := client.Dial()
	if err != nil {
		if tryEnqueue(cfg, err) {
			return nil
		}
		return fmt.Errorf("connect to service: %w (is 'hermes serve' running?)", err)
	}
	defer c.Close()

	deck.Infof("notification: mode=service heading=%q buttons=%d dnd=%s", cfg.Heading, len(cfg.Buttons), cfg.DND)
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
		deck.Warningf("notification: queue fallback skipped, invalid config: %v", err)
		return false
	}
	if err := store.EnqueueOffline("", cfg, store.DefaultQueueTTL); err != nil {
		deck.Infof("notification: queue fallback failed (service may hold DB lock): %v", err)
		return false
	}
	deck.Infof("notification: queued heading=%q reason=%q", cfg.Heading, originalErr)
	fmt.Print("queued")
	os.Stdout.Sync()
	os.Exit(int(exitcodes.Queued))
	return true // unreachable, but keeps the signature clean
}

// printResultAndExit prints the service result to stdout and exits with
// the appropriate code. Shared by sendToService and runNotify.
func printResultAndExit(r *client.NotifyResult) {
	if r.Error != "" {
		deck.Errorf("notification: result=error error=%q exit=%d", r.Error, r.ExitCode)
		os.Exit(int(r.ExitCode))
	}
	deck.Infof("notification: result=%q exit=%d", r.Value, r.ExitCode)
	fmt.Print(r.Value)
	os.Stdout.Sync()
	os.Exit(int(r.ExitCode))
}

// runServiceUI is Mode 1: the service spawned this process with a notification ID.
// It fetches config from the service via gRPC, renders the UI, and reports back.
func runServiceUI(notifID string) error {
	c, err := client.Dial()
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

	deck.Infof("notification: mode=service-ui id=%s heading=%q defer_allowed=%v", notifID, cfg.Heading, deferAllowed)
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
		Windows: &wopts.Options{IsZoomControlEnabled: false},
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
		return config.Load([]byte(trimmed))
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
		return config.Load(data)
	}

	trimmed := strings.TrimSpace(arg)
	if strings.HasPrefix(trimmed, "{") || strings.Contains(trimmed, ":") {
		cfg, err := config.Load([]byte(trimmed))
		if err == nil {
			return cfg, nil
		}
		return nil, fmt.Errorf("not a file or valid config: %s", arg)
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
			deck.Infof("notification: dnd=skip, suppressed heading=%q", cfg.Heading)
			fmt.Print("dnd_active")
			os.Stdout.Sync()
			os.Exit(int(exitcodes.OK))
		}
		return nil
	default: // "respect"
		const maxDNDWait = 24 * time.Hour
		deadline := time.Now().Add(maxDNDWait)
		for dnd.Active() {
			if time.Now().After(deadline) {
				deck.Warningf("notification: dnd=respect, max wait (%s) exceeded heading=%q", maxDNDWait, cfg.Heading)
				fmt.Print("dnd_timeout")
				os.Stdout.Sync()
				os.Exit(int(exitcodes.Timeout))
			}
			deck.Infof("notification: dnd=respect, waiting 60s heading=%q", cfg.Heading)
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

	wv2Path := webview2DataPath()

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
		Windows: &wopts.Options{
			IsZoomControlEnabled: false,
			WebviewUserDataPath:  wv2Path,
		},
	})
	if err != nil {
		deck.Errorf("wails: %v", err)
		os.Exit(int(exitcodes.Error))
	}

	respond(a.Result)
}

// webview2DataPath returns a dedicated WebView2 user data directory for
// local/demo mode so it doesn't conflict with the service daemon's lock on
// the default path. Creates the directory if it doesn't exist.
// Returns "" on non-Windows (Wails uses a sensible default).
func webview2DataPath() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	d := os.Getenv("LOCALAPPDATA")
	if d == "" {
		return ""
	}
	p := filepath.Join(d, "hermes", "webview2-local")
	if err := os.MkdirAll(p, 0700); err != nil {
		deck.Warningf("webview2 data dir: %v", err)
		return ""
	}
	return p
}

// prepareConfig applies defaults, locale resolution, and HTML escaping to a config.
func prepareConfig(cfg *config.NotificationConfig) {
	cfg.ApplyDefaults()
	locale := flagLocale
	if locale == "" {
		locale = config.DetectLocale()
	}
	cfg.ApplyLocale(locale)
	cfg.SanitizeText()
}

// respond prints the value to stdout and exits with the appropriate code.
// The JS frontend prefixes timeout responses with "timeout:" so we can
// distinguish "user clicked restart" (exit 0) from "countdown expired" (exit 202).
func respond(value string) {
	if value == "" {
		deck.Infof("notification: result=dismissed exit=0")
		os.Exit(0)
	}
	code := exitcodes.ForValue(value)
	clean := strings.TrimPrefix(value, "timeout:")
	deck.Infof("notification: result=%q exit=%d", clean, code)
	fmt.Print(clean)
	os.Stdout.Sync()
	os.Exit(int(code))
}
