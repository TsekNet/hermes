package cmd

import (
	"context"
	"fmt"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/google/deck"
	"github.com/spf13/cobra"
)

var flagNotifyPort int

func notifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify [config]",
		Short: "Send a notification via the hermes service",
		Long: `Sends a JSON notification config to the running hermes service and blocks
until the user responds or the notification times out.

Config can be a file path, inline JSON string, or piped via stdin.`,
		Example: `  hermes notify '{"heading":"Restart","message":"Please restart."}'
  hermes notify notification.json
  echo '{"heading":"..."}' | hermes notify
  $config | hermes notify`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runNotify,
	}
	cmd.Flags().IntVar(&flagNotifyPort, "port", server.DefaultPort, "service gRPC port")
	return cmd
}

func runNotify(_ *cobra.Command, args []string) error {
	cfg, err := resolveConfig("", args)
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no config provided: pass as argument or pipe via stdin")
	}
	cfg.ApplyDefaults()

	c, err := client.Dial(flagNotifyPort)
	if err != nil {
		return fmt.Errorf("connect to service: %w", err)
	}
	defer c.Close()

	deck.Infof("sending notification to service on port %d", flagNotifyPort)
	result, err := c.Notify(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("notify: %w", err)
	}
	printResultAndExit(result)
	return nil
}
