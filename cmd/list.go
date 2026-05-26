package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/respond"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var (
		asJSON bool
		full   bool
	)
	cmd := &cobra.Command{
		Use:   "list [#]",
		Short: "List active notifications",
		Long: `List active notifications. With a position number, shows that single
notification in the table. With --full, shows full details and actions.`,
		Example: `  hermes list
  hermes list 1
  hermes list --full 1
  hermes list --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := client.Dial()
			if err != nil {
				return fmt.Errorf("connect to service: %w", err)
			}
			defer c.Close()

			entries, err := c.List(context.Background())
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			sortEntries(entries)

			if len(args) == 1 {
				idx, err := resolveIndex(entries, args[0])
				if err != nil {
					return err
				}
				if full {
					return runShowByEntry(os.Stdout, c, entries[idx])
				}
				writeTable(os.Stdout, entries[idx:idx+1], idx)
				return nil
			}

			if len(entries) == 0 {
				fmt.Println("No active notifications.")
				return nil
			}

			writeTable(os.Stdout, entries, 0)
			fmt.Println()
			fmt.Println("Details: hermes list --full <#>    Respond: hermes respond <#>")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON array")
	cmd.Flags().BoolVar(&full, "full", false, "show full details and actions for a notification")
	return cmd
}

func writeTable(out io.Writer, entries []client.ListEntry, startIdx int) {
	fmt.Fprintf(out, "%-3s %-30s %-18s %-8s %s\n", "#", "HEADING", "STATE", "DEFERS", "DEADLINE")
	for i, e := range entries {
		dl := "none"
		if !e.Deadline.IsZero() {
			dl = e.Deadline.Format("Jan 02 15:04")
		}
		heading := e.Heading
		if len(heading) > 30 {
			heading = heading[:27] + "..."
		}
		fmt.Fprintf(out, "%-3d %-30s %-18s %-8d %s\n",
			startIdx+i+1, heading, e.State, e.DeferCount, dl)
	}
}

func resolveIndex(entries []client.ListEntry, arg string) (int, error) {
	n, isPos := parsePosition(arg)
	if !isPos {
		return 0, fmt.Errorf("invalid position %q", arg)
	}
	if n < 1 {
		return 0, fmt.Errorf("position must be >= 1, got %d", n)
	}
	if n > len(entries) {
		return 0, fmt.Errorf("position %d out of range (have %d active)", n, len(entries))
	}
	return n - 1, nil
}

func runShowByEntry(out io.Writer, c respond.Client, entry client.ListEntry) error {
	cfg, buttons, err := respond.FetchConfig(context.Background(), c, entry.ID)
	if err != nil {
		return err
	}
	writeNotification(out, cfg, buttons)
	return nil
}

func writeNotification(out io.Writer, cfg *config.NotificationConfig, buttons []config.Button) {
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
	fmt.Fprintln(out, "Respond: hermes respond <#>")
}
