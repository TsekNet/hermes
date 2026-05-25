package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/exitcodes"
)

func TestBuildRootCmd_SubcommandErrors(t *testing.T) {
	// These tests exercise Cobra command construction, flag parsing, and
	// early error paths. They share package-level flag vars, so they
	// run sequentially within this top-level test.

	t.Run("notify no config", func(t *testing.T) {
		cmd := buildRootCmd()
		cmd.SetArgs([]string{"notify"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("notify with no config should error")
		}
	})

	t.Run("stop no service", func(t *testing.T) {
		cmd := buildRootCmd()
		cmd.SetArgs([]string{"stop"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("stop with no running service should error")
		}
	})

	t.Run("list no service", func(t *testing.T) {
		cmd := buildRootCmd()
		cmd.SetArgs([]string{"list"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("list with no running service should error")
		}
	})

	t.Run("cancel no service", func(t *testing.T) {
		cmd := buildRootCmd()
		cmd.SetArgs([]string{"cancel", "test-id"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("cancel with no running service should error")
		}
	})

	t.Run("history JSON", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "history-cmd.db")
		cmd := buildRootCmd()
		cmd.SetArgs([]string{"history", "--json", "--db", dbPath})
		cmd.Execute()
	})
}

func TestResolveConfig_Stdin(t *testing.T) {
	// resolveConfig reads stdin when no flag and no args, and stdin is a pipe.
	// In test context, stdin is /dev/null (not a char device), so depending
	// on the test runner it may or may not trigger the stdin path.
	// We test the explicit paths that don't depend on stdin state.
	t.Parallel()

	t.Run("oversized stdin arg returns error", func(t *testing.T) {
		t.Parallel()
		// loadFromArg with JSON too large should error at config.Load level.
		huge := `{"heading":"` + strings.Repeat("X", 2*1024*1024) + `"}`
		_, err := loadFromArg(huge)
		if err == nil {
			t.Fatal("expected error for oversized config")
		}
	})
}

func TestBuildRootCmd(t *testing.T) {
	t.Parallel()
	cmd := buildRootCmd()

	if cmd.Use != "hermes [config]" {
		t.Errorf("Use = %q", cmd.Use)
	}

	wantSubs := []string{"demo", "version", "serve", "notify", "list",
		"cancel", "history", "stop", "motd", "install", "uninstall"}
	gotSubs := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		gotSubs[sub.Name()] = true
	}

	for _, want := range wantSubs {
		if !gotSubs[want] {
			t.Errorf("missing subcommand: %s", want)
		}
	}
}

func TestResolveConfig_NoInput(t *testing.T) {
	t.Parallel()
	cfg, err := resolveConfig("", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for no input, got heading=%q", cfg.Heading)
	}
}

func TestResolveConfig_NoArgs_TerminalStdin(t *testing.T) {
	t.Parallel()
	// Simulate: no --config, no positional args, stdin is a terminal (char device).
	// On CI and in tests, stdin is typically /dev/null or NUL, not a char device,
	// so resolveConfig should still return (nil, nil).
	cfg, err := resolveConfig("", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got heading=%q", cfg.Heading)
	}
}

func TestResolveConfig_InlineJSON(t *testing.T) {
	t.Parallel()
	cfg, err := resolveConfig("", []string{`{"heading":"Test","message":"Body"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config from inline JSON")
	}
	if cfg.Heading != "Test" {
		t.Errorf("heading = %q, want %q", cfg.Heading, "Test")
	}
}

func TestResolveConfig_FilePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "test.json")
	os.WriteFile(f, []byte(`{"heading":"FromFile","message":"M"}`), 0644)

	cfg, err := resolveConfig(f, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || cfg.Heading != "FromFile" {
		t.Errorf("heading = %v, want FromFile", cfg)
	}
}

func TestDemoConfig_DND(t *testing.T) {
	t.Parallel()
	cfg := demoConfig()
	if cfg.DND != config.DNDIgnore {
		t.Errorf("dnd = %q, want %q: demo must ignore DND to prevent silent hang", cfg.DND, config.DNDIgnore)
	}
}

func TestDemoConfig_Valid(t *testing.T) {
	t.Parallel()
	cfg := demoConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("demoConfig is invalid: %v", err)
	}
	if cfg.Heading == "" {
		t.Error("heading is empty")
	}
	if len(cfg.Buttons) == 0 {
		t.Error("no buttons")
	}
	if cfg.TimeoutSeconds <= 0 {
		t.Error("timeout must be positive")
	}
}

func TestDemoConfig_DefaultsApplied(t *testing.T) {
	t.Parallel()
	cfg := demoConfig()
	if cfg.Title == "" {
		t.Error("title should be set by ApplyDefaults")
	}
}

func TestWaitForDND_IgnoreReturnsImmediately(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{DND: config.DNDIgnore}
	if err := waitForDND(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUI_WebView2DirCreated(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("WebView2 data path only applies on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	expected := filepath.Join(dir, "hermes", "webview2-local")
	wv2Path := webview2DataPath()
	if wv2Path != expected {
		t.Fatalf("wv2Path = %q, want %q", wv2Path, expected)
	}

	info, err := os.Stat(wv2Path)
	if err != nil {
		t.Fatalf("WebView2 data dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("WebView2 data path is not a directory")
	}
}

func TestPrepareConfig_SetsDefaults(t *testing.T) {
	t.Parallel()
	cfg := &config.NotificationConfig{Heading: "H", Message: "M"}
	prepareConfig(cfg)
	if cfg.Title == "" {
		t.Error("title should be set by ApplyDefaults")
	}
	if cfg.DND == "" {
		t.Error("dnd should be set by ApplyDefaults")
	}
}

func TestDoubleClickPath_ReachesDemo(t *testing.T) {
	t.Parallel()

	// Simulate the exact double-click scenario: no flags, no args, no stdin pipe.
	// resolveConfig must return (nil, nil), causing runRoot to call runDemo.
	cfg, err := resolveConfig("", nil)
	if err != nil {
		t.Fatalf("resolveConfig error: %v", err)
	}
	if cfg != nil {
		t.Fatal("resolveConfig should return nil for double-click (no input)")
	}

	// demoConfig must produce a valid, DND-ignoring config.
	demo := demoConfig()
	if demo.DND != config.DNDIgnore {
		t.Errorf("demo dnd = %q, want %q", demo.DND, config.DNDIgnore)
	}
	if err := demo.Validate(); err != nil {
		t.Fatalf("demo config invalid: %v", err)
	}

	// waitForDND must return immediately for ignore mode.
	if err := waitForDND(demo); err != nil {
		t.Fatalf("waitForDND error: %v", err)
	}
}

func TestLoadFromArg_InvalidPath(t *testing.T) {
	t.Parallel()
	badPath := filepath.Join(t.TempDir(), "nonexistent", "config.json")
	_, err := loadFromArg(badPath)
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "not a file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromArg_InlineYAML(t *testing.T) {
	t.Parallel()
	cfg, err := loadFromArg("heading: YAMLTest\nmessage: body")
	if err != nil {
		t.Fatalf("loadFromArg: %v", err)
	}
	if cfg.Heading != "YAMLTest" {
		t.Errorf("heading = %q, want YAMLTest", cfg.Heading)
	}
}

func TestLoadFromArg_FileOverInline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create a file whose name looks like JSON but is actually a file path.
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"heading":"FromFile"}`), 0644)

	cfg, err := loadFromArg(path)
	if err != nil {
		t.Fatalf("loadFromArg: %v", err)
	}
	if cfg.Heading != "FromFile" {
		t.Errorf("heading = %q, want FromFile (file takes precedence)", cfg.Heading)
	}
}

func TestLoadFromArg_GarbageString(t *testing.T) {
	t.Parallel()
	_, err := loadFromArg("not-json-not-file-not-yaml")
	if err == nil {
		t.Fatal("expected error for garbage string")
	}
}

func TestResolveConfig_FlagOverArg(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	flagFile := filepath.Join(dir, "flag.json")
	os.WriteFile(flagFile, []byte(`{"heading":"Flag"}`), 0644)

	cfg, err := resolveConfig(flagFile, []string{`{"heading":"Arg"}`})
	if err != nil {
		t.Fatalf("resolveConfig: %v", err)
	}
	if cfg.Heading != "Flag" {
		t.Errorf("heading = %q, want Flag (--config flag takes precedence)", cfg.Heading)
	}
}

func TestWebView2DataPath_NonWindowsReturnsEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test only applies on non-Windows")
	}
	t.Setenv("LOCALAPPDATA", t.TempDir())
	if p := webview2DataPath(); p != "" {
		t.Errorf("expected empty string on %s, got %q", runtime.GOOS, p)
	}
}

func TestFilterDeferButtons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		buttons    []config.Button
		wantLabels []string
	}{
		{
			name:       "nil buttons",
			buttons:    nil,
			wantLabels: nil,
		},
		{
			name: "no defers unchanged",
			buttons: []config.Button{
				{Label: "OK", Value: "ok"},
				{Label: "Cancel", Value: "cancel"},
			},
			wantLabels: []string{"OK", "Cancel"},
		},
		{
			name: "top-level defer removed",
			buttons: []config.Button{
				{Label: "OK", Value: "ok"},
				{Label: "Later", Value: "defer"},
				{Label: "Defer 1h", Value: "defer:1h"},
			},
			wantLabels: []string{"OK"},
		},
		{
			name: "dropdown defer options removed",
			buttons: []config.Button{
				{Label: "Schedule", Value: "schedule", Dropdown: []config.DropdownOption{
					{Label: "1 hour", Value: "defer:1h"},
					{Label: "Tomorrow", Value: "defer:24h"},
					{Label: "Custom", Value: "custom"},
				}},
			},
			wantLabels: []string{"Schedule"},
		},
		{
			name: "dropdown-only button with empty value dropped when all options are defers",
			buttons: []config.Button{
				{Label: "Defer", Value: "", Dropdown: []config.DropdownOption{
					{Label: "1 hour", Value: "defer:1h"},
					{Label: "4 hours", Value: "defer:4h"},
				}},
				{Label: "OK", Value: "ok"},
			},
			wantLabels: []string{"OK"},
		},
		{
			name: "dropdown-only button with base value kept when dropdown empties",
			buttons: []config.Button{
				{Label: "Snooze", Value: "snooze", Dropdown: []config.DropdownOption{
					{Label: "1 hour", Value: "defer:1h"},
				}},
			},
			wantLabels: []string{"Snooze"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := filterDeferButtons(tt.buttons)

			if tt.wantLabels == nil {
				if got != nil {
					t.Fatalf("got %d buttons, want nil", len(got))
				}
				return
			}

			if len(got) != len(tt.wantLabels) {
				labels := make([]string, len(got))
				for i, b := range got {
					labels[i] = b.Label
				}
				t.Fatalf("labels = %v, want %v", labels, tt.wantLabels)
			}

			for i, want := range tt.wantLabels {
				if got[i].Label != want {
					t.Errorf("button[%d].Label = %q, want %q", i, got[i].Label, want)
				}
			}
		})
	}
}

func TestFilterDeferButtons_DropdownNonDeferOptionsPreserved(t *testing.T) {
	t.Parallel()
	buttons := []config.Button{
		{Label: "Actions", Value: "", Dropdown: []config.DropdownOption{
			{Label: "Restart", Value: "restart"},
			{Label: "Defer 1h", Value: "defer:1h"},
			{Label: "Shutdown", Value: "shutdown"},
		}},
	}
	got := filterDeferButtons(buttons)
	if len(got) != 1 {
		t.Fatalf("expected 1 button, got %d", len(got))
	}
	if len(got[0].Dropdown) != 2 {
		t.Fatalf("dropdown options = %d, want 2", len(got[0].Dropdown))
	}
	if got[0].Dropdown[0].Value != "restart" {
		t.Errorf("dropdown[0].Value = %q, want restart", got[0].Dropdown[0].Value)
	}
	if got[0].Dropdown[1].Value != "shutdown" {
		t.Errorf("dropdown[1].Value = %q, want shutdown", got[0].Dropdown[1].Value)
	}
}

func TestRespond_ExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		wantCode int
		wantOut  string
	}{
		{"empty_value", "", 0, ""},
		{"timeout_prefix", "timeout:restart", int(exitcodes.Timeout), "restart"},
		{"deferred", "defer", int(exitcodes.Deferred), "defer"},
		{"normal_ok", "ok", int(exitcodes.OK), "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if os.Getenv("TEST_RESPOND_SUBPROCESS") == "1" {
				respond(os.Getenv("TEST_RESPOND_VALUE"))
				return
			}

			cmd := exec.Command(os.Args[0],
				fmt.Sprintf("-test.run=^TestRespond_ExitCodes/%s$", tt.name),
			)
			cmd.Env = append(os.Environ(),
				"TEST_RESPOND_SUBPROCESS=1",
				"TEST_RESPOND_VALUE="+tt.value,
			)
			out, err := cmd.Output()

			gotCode := 0
			if exitErr, ok := err.(*exec.ExitError); ok {
				gotCode = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotCode != tt.wantCode {
				t.Errorf("exit code = %d, want %d", gotCode, tt.wantCode)
			}
			if string(out) != tt.wantOut {
				t.Errorf("stdout = %q, want %q", string(out), tt.wantOut)
			}
		})
	}
}
