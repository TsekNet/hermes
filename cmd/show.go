package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/respond"
	"github.com/spf13/cobra"
)

func showCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <notification-id or #>",
		Short: "Show full notification details and available actions",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := client.Dial()
			if err != nil {
				return fmt.Errorf("connect to service: %w", err)
			}
			defer c.Close()

			id, err := resolveID(context.Background(), c, args[0])
			if err != nil {
				return err
			}
			return runShow(os.Stdout, c, id)
		},
	}
	return cmd
}

func runShow(out io.Writer, c respond.Client, id string) error {
	cfg, buttons, err := respond.FetchConfig(context.Background(), c, id)
	if err != nil {
		return err
	}
	writeNotification(out, id, cfg, buttons)
	return nil
}

func writeNotification(out io.Writer, id string, cfg *config.NotificationConfig, buttons []config.Button) {
	heading := sanitize(cfg.Heading)
	fmt.Fprintln(out, heading)
	fmt.Fprintln(out, strings.Repeat("-", len(heading)))

	if cfg.Message != "" {
		fmt.Fprintln(out, sanitize(cfg.Message))
	}

	actions := config.FlattenActions(buttons)
	if len(actions) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Actions:")
		for i, a := range actions {
			fmt.Fprintf(out, "  [%d] %-20s (%s)\n", i+1, a.Label, a.Value)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "Respond: hermes respond %s <value>\n", id)
}

