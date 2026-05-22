package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/TsekNet/hermes/internal/client"
	"github.com/TsekNet/hermes/internal/socket"
	"github.com/google/deck"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running hermes service daemon",
		Long:  `Sends a graceful Shutdown RPC to the hermes daemon via its Unix domain socket.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runStop()
		},
	}
	return cmd
}

func runStop() error {
	sockPath := socket.Path()
	if !socket.IsRunning(sockPath) {
		return fmt.Errorf("no hermes daemon found (socket %s)", sockPath)
	}

	c, err := client.Dial()
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer c.Close()

	deck.Infof("sending Shutdown RPC via %s", sockPath)
	if err := c.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("shutdown RPC: %w", err)
	}

	if waitForSocketRemoval(sockPath, 5*time.Second) {
		fmt.Fprintf(os.Stderr, "hermes daemon stopped\n")
		return nil
	}
	return fmt.Errorf("daemon did not exit within 5s")
}

func waitForSocketRemoval(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !socket.IsRunning(path) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}
