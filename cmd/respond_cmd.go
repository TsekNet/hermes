package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/respond"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const maxInvalidAttempts = 5

func respondCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "respond <#> [value]",
		Short: "Respond to a notification",
		Long: `Submit a response for an active notification. With a value argument,
sends the response directly (works in any context including scripts).
Without a value argument in a TTY, shows an interactive picker.`,
		Example: `  hermes respond 1              # interactive picker (TTY only)
  hermes respond 1 restart     # direct value (scripting)
  hermes respond 1 defer_1h`,
		Args: cobra.RangeArgs(1, 2),
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
			value := ""
			if len(args) > 1 {
				value = args[1]
			}
			code, err := runRespond(os.Stdout, os.Stdin, c, id, value, term.IsTerminal(int(os.Stdin.Fd())))
			if err != nil {
				return err
			}
			os.Exit(int(code))
			return nil
		},
	}
	return cmd
}

func runRespond(out io.Writer, in io.Reader, c respond.Client, id, value string, isTTY bool) (int32, error) {
	if value != "" {
		return submitValue(out, c, id, value)
	}

	if !isTTY {
		_, buttons, err := respond.FetchConfig(context.Background(), c, id)
		if err != nil {
			return 0, err
		}
		fmt.Fprintln(out, "Error: no value provided. Usage: hermes respond <#> <value>")
		fmt.Fprintf(out, "Available actions: %s\n", buttonValues(buttons))
		return 1, fmt.Errorf("no value provided and not a TTY")
	}

	return interactivePick(out, in, c, id)
}

func submitValue(out io.Writer, c respond.Client, id, value string) (int32, error) {
	result, err := respond.Submit(context.Background(), c, id, value)
	if err != nil {
		return 1, err
	}
	fmt.Fprintf(out, "Sent: %s\n", result.Value)
	return result.ExitCode, nil
}

func interactivePick(out io.Writer, in io.Reader, c respond.Client, id string) (int32, error) {
	cfg, buttons, err := respond.FetchConfig(context.Background(), c, id)
	if err != nil {
		return 0, err
	}
	actions := config.FlattenActions(buttons)
	if len(actions) == 0 {
		return 1, fmt.Errorf("no actions available for %s", id)
	}

	fmt.Fprintf(out, "%s: choose action\n", sanitize(cfg.Heading))
	for i, a := range actions {
		fmt.Fprintf(out, "  [%d] %s\n", i+1, a.Label)
	}

	scanner := bufio.NewScanner(in)
	for attempt := 0; attempt < maxInvalidAttempts; attempt++ {
		fmt.Fprint(out, "> ")
		if !scanner.Scan() {
			return 1, nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(actions) {
			fmt.Fprintln(out, "Invalid choice, try again")
			continue
		}
		chosen := actions[n-1].Value
		return submitValue(out, c, id, chosen)
	}
	fmt.Fprintln(out, "Too many invalid attempts")
	return 1, fmt.Errorf("too many invalid attempts")
}

func buttonValues(buttons []config.Button) string {
	actions := config.FlattenActions(buttons)
	vals := make([]string, 0, len(actions))
	for _, a := range actions {
		vals = append(vals, a.Value)
	}
	return strings.Join(vals, ", ")
}
