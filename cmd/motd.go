package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/spf13/cobra"
)

func motdCmd() *cobra.Command {
	var (
		dbPath  string
		oneline bool
	)
	cmd := &cobra.Command{
		Use:   "motd",
		Short: "Print pending notification summary for SSH login banners",
		Long: `Prints a summary of pending notifications to stdout, intended to be
called from a shell profile on SSH login. Exits silently if not in an
SSH session or if there are no pending notifications.

With --oneline, prints "N pending" or nothing (for shell prompt integration).
The --oneline path has a 100ms hard deadline and never fails.`,
		Example: `  hermes motd
  hermes motd --oneline`,
		RunE: func(_ *cobra.Command, _ []string) error {
			if oneline {
				runMotdOneline()
				return nil
			}
			return runMotd(dbPath)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "read directly from bolt DB (skip service)")
	cmd.Flags().BoolVar(&oneline, "oneline", false, "print \"N pending\" or nothing (for shell prompt)")
	return cmd
}

type oneliner interface {
	List(ctx context.Context) ([]client.ListEntry, error)
	Close() error
}

type onelineDialer func() (oneliner, error)

func defaultOnelineDial() (oneliner, error) { return client.Dial() }

func runMotdOneline() {
	runMotdOnelineWith(os.Stdout, defaultOnelineDial, 100*time.Millisecond)
}

func runMotdOnelineWith(out io.Writer, dial onelineDialer, deadline time.Duration) {
	type result struct {
		count int
	}
	ch := make(chan result, 1)
	go func() {
		c, err := dial()
		if err != nil {
			ch <- result{}
			return
		}
		defer c.Close()
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()
		entries, err := c.List(ctx)
		if err != nil {
			ch <- result{}
			return
		}
		ch <- result{count: len(entries)}
	}()

	timer := time.NewTimer(deadline)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.count > 0 {
			fmt.Fprintf(out, "%d pending", r.count)
		}
	case <-timer.C:
	}
}

func runMotd(dbPath string) error {
	if !isSSHSession() {
		return nil
	}

	entries, err := fetchHistory(dbPath)
	if err != nil {
		return nil
	}
	if len(entries) == 0 {
		return nil
	}

	fmt.Printf("\n-- Hermes: %d pending notification(s) --\n", len(entries))
	shown := 0
	for _, e := range entries {
		if shown >= 5 {
			break
		}
		heading := sanitize(e.Heading)
		fmt.Printf("  * %s\n", heading)
		shown++
	}
	if len(entries) > 5 {
		fmt.Printf("  ... and %d more\n", len(entries)-5)
	}
	fmt.Println("Run 'hermes history' for details.")
	fmt.Println("----------------------------------------")
	fmt.Println()
	return nil
}

func isSSHSession() bool {
	for _, key := range []string{"SSH_CLIENT", "SSH_CONNECTION", "SSH_TTY"} {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
