package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/spf13/cobra"
)

func listCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active notifications",
		RunE: func(_ *cobra.Command, _ []string) error {
			c, err := client.Dial(port)
			if err != nil {
				return fmt.Errorf("connect to service: %w", err)
			}
			defer c.Close()

			entries, err := c.List(context.Background())
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No active notifications.")
				return nil
			}

			for _, e := range entries {
				dl := "none"
				if !e.Deadline.IsZero() {
					dl = time.Until(e.Deadline).Round(time.Second).String()
				}
				fmt.Printf("%-16s %-20s state=%-18s defers=%d deadline=%s\n",
					e.ID, e.Heading, e.State, e.DeferCount, dl)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", server.DefaultPort, "service gRPC port")
	return cmd
}
