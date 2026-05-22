package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/google/deck"
	"github.com/spf13/cobra"
)

func notifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify [config]",
		Short: "Send a notification via the hermes service",
		Long: `Sends a notification config to the running hermes service and blocks
until the user responds or the notification times out.

When run as SYSTEM (Windows) or root (macOS/Linux), hermes automatically
broadcasts the notification to all active user sessions. No wrapper scripts
or user-context switching is needed.

Config can be a file path (JSON or YAML), inline JSON string, or piped via stdin.`,
		Example: `  hermes notify '{"heading":"Restart","message":"Please restart."}'
  hermes notify notification.json
  echo '{"heading":"..."}' | hermes notify
  $config | hermes notify`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runNotify,
	}
	return cmd
}

func runNotify(_ *cobra.Command, args []string) error {
	cfg, err := resolveConfig(flagConfig, args)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no config provided: pass as argument or pipe via stdin")
	}
	prepareConfig(cfg)

	if isPrivileged() {
		return broadcastNotify(cfg, args)
	}

	c, err := client.Dial()
	if err != nil {
		if tryEnqueue(cfg, err) {
			return nil
		}
		return fmt.Errorf("connect to service: %w", err)
	}
	defer c.Close()

	deck.Infof("notification: mode=notify heading=%q buttons=%d", cfg.Heading, len(cfg.Buttons))
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

// broadcastNotify re-launches "hermes notify <args>" in every active user
// session so each child runs in user context with access to the per-user
// auth token and daemon. Reuses the same session launch machinery as
// hermes install.
//
// When config came from stdin (args empty, flagConfig empty), cfg is
// serialized to a temp file so children can read it.
func broadcastNotify(cfg *config.NotificationConfig, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	notifyArgs := []string{"notify"}

	switch {
	case flagConfig != "":
		notifyArgs = append(notifyArgs, "--config", flagConfig)
	case len(args) > 0:
		notifyArgs = append(notifyArgs, args...)
	default:
		f, err := writeTempConfig(cfg)
		if err != nil {
			return err
		}
		defer os.Remove(f)
		notifyArgs = append(notifyArgs, "--config", f)
	}

	deck.Infof("notification: mode=broadcast (privileged), relaunching in user sessions")
	launchInUserSessions(exe, notifyArgs)
	return nil
}

func writeTempConfig(cfg *config.NotificationConfig) (string, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config for broadcast: %w", err)
	}
	f, err := os.CreateTemp("", "hermes-broadcast-*.json")
	if err != nil {
		return "", fmt.Errorf("create temp config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp config: %w", err)
	}
	f.Close()
	// Make readable by all users so child processes in other sessions
	// (launched via CreateProcessAsUser / setuid) can read the config.
	os.Chmod(f.Name(), 0644)
	return f.Name(), nil
}
