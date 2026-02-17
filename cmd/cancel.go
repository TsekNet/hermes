package cmd

import (
	"context"
	"fmt"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/spf13/cobra"
)

func cancelCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "cancel <notification-id>",
		Short: "Cancel an active notification",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := client.Dial(port)
			if err != nil {
				return fmt.Errorf("connect to service: %w", err)
			}
			defer c.Close()

			found, err := c.Cancel(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("cancel: %w", err)
			}
			if !found {
				fmt.Printf("Notification %s not found or already completed.\n", args[0])
			} else {
				fmt.Printf("Notification %s cancelled.\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", server.DefaultPort, "service gRPC port")
	return cmd
}
