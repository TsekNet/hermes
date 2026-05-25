package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func motdCmd() *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "motd",
		Short: "Print pending notification summary for SSH login banners",
		Long: `Prints a summary of pending notifications to stdout, intended to be
called from a shell profile on SSH login. Exits silently if not in an
SSH session or if there are no pending notifications.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMotd(dbPath)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "read directly from bolt DB (skip service)")
	return cmd
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
