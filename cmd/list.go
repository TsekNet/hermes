package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active notifications",
		Example: `  hermes list
  hermes list --json`,
		RunE: func(_ *cobra.Command, _ []string) error {
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

			if len(entries) == 0 {
				fmt.Println("No active notifications.")
				return nil
			}

			sortEntries(entries)
			fmt.Printf("%-3s %-16s %-20s %-18s %-8s %s\n", "#", "ID", "HEADING", "STATE", "DEFERS", "DEADLINE")
			for i, e := range entries {
				dl := "none"
				if !e.Deadline.IsZero() {
					dl = e.Deadline.Format("Jan 02 15:04")
				}
				fmt.Printf("%-3d %-16s %-20s %-18s %-8d %s\n",
					i+1, e.ID, e.Heading, e.State, e.DeferCount, dl)
			}
			fmt.Println()
			fmt.Println("Details: hermes show <#>    Respond: hermes respond <#> <value>")
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output as JSON array")
	return cmd
}
