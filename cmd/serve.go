package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TsekNet/hermes/internal/auth"
	"github.com/TsekNet/hermes/internal/manager"
	"github.com/TsekNet/hermes/internal/ratelimit"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/google/deck"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	flagPort int
	flagDB   string
)

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the hermes notification service",
		Long: `Starts the per-user gRPC service daemon that manages notification lifecycles.

Run this in the user's desktop session (e.g. via LaunchAgent, systemd user unit,
or HKCU Run key). The service launches webviews directly — no privilege escalation
or session-crossing needed.

Deferral state is persisted to a local bolt database so notifications survive
service restarts.`,
		RunE: runServe,
	}
	cmd.Flags().IntVar(&flagPort, "port", server.DefaultPort, "gRPC listen port")
	cmd.Flags().StringVar(&flagDB, "db", "", "bolt database path (default: platform-specific)")
	return cmd
}

func runServe(_ *cobra.Command, _ []string) error {
	s, err := store.Open(flagDB)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if err := s.PruneHistory(30*24*time.Hour, 200); err != nil {
		deck.Warningf("prune history: %v", err)
	}

	token, err := auth.GenerateToken()
	if err != nil {
		return fmt.Errorf("generate session token: %w", err)
	}
	defer auth.RemoveToken()
	deck.Infof("session token written to %s", auth.TokenPath())

	mgr := manager.New(reshowNotification, s)

	restored := mgr.Restore()
	if restored > 0 {
		deck.Infof("restored %d notification(s) from disk", restored)
	}

	rl := ratelimit.New(10, 2, []string{"/hermes.HermesService/Notify"})

	var interceptors []grpc.UnaryServerInterceptor
	interceptors = append(interceptors, auth.UnaryInterceptor(token))
	interceptors = append(interceptors, rl.UnaryInterceptor())

	srv := server.New(mgr, flagPort, interceptors...)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		deck.Infof("received %s, shutting down", sig)
		srv.Stop()
	}()

	deck.Infof("hermes service starting on port %d (per-user daemon, pid %d)", flagPort, os.Getpid())
	return srv.Serve()
}

// reshowNotification is called by the manager when a deferred notification's
// timer fires. The service runs per-user, so we launch the webview directly.
func reshowNotification(n *manager.Notification) {
	selfPath, err := os.Executable()
	if err != nil {
		deck.Errorf("cannot determine own path: %v", err)
		return
	}

	deck.Infof("re-show %s: launching UI subprocess", n.ID)
	args := []string{"--notification-id", n.ID, "--service-port", fmt.Sprintf("%d", flagPort)}

	if err := launchSubprocess(selfPath, args); err != nil {
		deck.Errorf("re-show %s: launch failed: %v", n.ID, err)
	}
}
