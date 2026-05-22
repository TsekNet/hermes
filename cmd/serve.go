package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/TsekNet/hermes/internal/auth"
	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/exitcodes"
	"github.com/TsekNet/hermes/internal/manager"
	"github.com/TsekNet/hermes/internal/ratelimit"
	"github.com/TsekNet/hermes/internal/server"
	"github.com/TsekNet/hermes/internal/socket"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/TsekNet/hermes/internal/tray"
	"github.com/google/deck"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	flagDB     string
	flagNoTray bool
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
	cmd.Flags().StringVar(&flagDB, "db", "", "bolt database path (default: platform-specific)")
	cmd.Flags().BoolVar(&flagNoTray, "no-tray", false, "disable system tray icon")
	return cmd
}

func runServe(_ *cobra.Command, _ []string) error {
	sockPath := socket.Path()

	lis, err := socket.Listen(sockPath)
	if err != nil {
		return err
	}
	defer lis.Close()
	defer socket.Cleanup(sockPath)

	s, err := store.Open(flagDB)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer s.Close()

	if err := s.PruneHistory(30*24*time.Hour, 50); err != nil {
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

	rl := ratelimit.New(5, 1, []string{"/hermes.HermesService/Notify"})

	var interceptors []grpc.UnaryServerInterceptor
	interceptors = append(interceptors, auth.UnaryInterceptor(token))
	interceptors = append(interceptors, rl.UnaryInterceptor())

	srv := server.New(mgr, interceptors...)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go drainQueue(mgr, s)

	useTray := !flagNoTray && tray.ShouldEnable(runtime.GOOS, os.Getenv)

	if useTray {
		deck.Infof("hermes service starting on %s with tray icon (per-user daemon, pid %d)", sockPath, os.Getpid())

		errCh := make(chan error, 1)
		go func() { errCh <- srv.Serve(lis) }()

		trayReady := make(chan struct{})
		go func() {
			<-trayReady
			select {
			case sig := <-sigCh:
				deck.Infof("received %s, shutting down", sig)
				srv.Stop()
				tray.Quit()
			case err := <-errCh:
				if err != nil {
					deck.Errorf("grpc server: %v", err)
				}
				tray.Quit()
			}
		}()

		tray.Run(tray.Config{
			Count: mgr.ActiveCount,
			Stop:  srv.Stop,
			Ready: trayReady,
		})

		return nil
	}

	if flagNoTray {
		deck.Infof("hermes service starting on %s (tray disabled, per-user daemon, pid %d)", sockPath, os.Getpid())
	} else {
		deck.Infof("hermes service starting on %s (no display server, tray skipped, per-user daemon, pid %d)", sockPath, os.Getpid())
	}

	go func() {
		sig := <-sigCh
		deck.Infof("received %s, shutting down", sig)
		srv.Stop()
	}()

	return srv.Serve(lis)
}

// drainDelay is the breathing room between queued notifications so the
// user isn't overwhelmed after returning from extended absence.
const drainDelay = 30 * time.Second

// drainQueue processes notifications that were submitted while the service
// was offline. Notifications are shown one at a time: submit, wait for
// the user to respond, pause, repeat. Expired records are discarded.
func drainQueue(mgr *manager.Manager, s *store.Store) {
	// Let restored (in-progress) notifications get some airtime first.
	time.Sleep(drainDelay)

	records, err := s.LoadQueue()
	if err != nil {
		deck.Errorf("drain queue: %v", err)
		return
	}
	if len(records) == 0 {
		return
	}

	records = sortByDependencies(records)
	deck.Infof("notification: draining %d queued item(s)", len(records))
	locale := config.DetectLocale()
	now := time.Now()

	for _, r := range records {
		r.Config.ApplyLocale(locale)
		r.Config.ApplyDefaults()
		if err := r.Config.Validate(); err != nil {
			deck.Warningf("notification: drain id=%s result=invalid heading=%q error=%q", r.ID, r.Config.Heading, err)
			s.DeleteQueued(r.ID)
			continue
		}
		if now.After(r.ExpiresAt) {
			deck.Infof("notification: drain id=%s result=expired heading=%q queued=%s expired=%s",
				r.ID, r.Config.Heading, r.QueuedAt.Format(time.DateOnly), r.ExpiresAt.Format(time.DateOnly))
			s.SaveHistory(&store.HistoryRecord{
				ID:            r.ID,
				Config:        r.Config,
				ResponseValue: "expired_while_queued",
				CreatedAt:     r.QueuedAt,
				CompletedAt:   now,
			})
			s.DeleteQueued(r.ID)
			continue
		}

		// Delete before submit: at-most-once > at-least-once for user-facing notifications.
		s.DeleteQueued(r.ID)

		id, resultCh := mgr.Submit(r.Config)
		if id == "" {
			<-resultCh
			deck.Warningf("notification: drain id=%s result=rejected heading=%q", r.ID, r.Config.Heading)
			s.SaveHistory(&store.HistoryRecord{
				ID:            r.ID,
				Config:        r.Config,
				ResponseValue: "rejected_from_queue",
				CreatedAt:     r.QueuedAt,
				CompletedAt:   time.Now(),
			})
			continue
		}

		result := <-resultCh
		deck.Infof("notification: drain id=%s result=%q heading=%q priority=%d exit=%d queued=%s",
			id, result.Value, r.Config.Heading, r.Priority, result.ExitCode, r.QueuedAt.Format(time.DateOnly))

		if result.ExitCode != exitcodes.Deferred {
			time.Sleep(drainDelay)
		}
	}

	deck.Infof("notification: drain complete")
}

// sortByDependencies reorders queue records so that if A depends on B
// and both are in the queue, B is drained before A. Within a dependency
// level, the original priority/time ordering is preserved.
func sortByDependencies(records []*store.QueueRecord) []*store.QueueRecord {
	byConfigID := make(map[string]int, len(records))
	for i, r := range records {
		if r.Config.ID != "" {
			byConfigID[r.Config.ID] = i
		}
	}
	visited := make([]bool, len(records))
	sorted := make([]*store.QueueRecord, 0, len(records))

	var visit func(i int)
	visit = func(i int) {
		if visited[i] {
			return
		}
		visited[i] = true
		if dep := records[i].Config.DependsOn; dep != "" {
			if j, ok := byConfigID[dep]; ok {
				visit(j)
			}
		}
		sorted = append(sorted, records[i])
	}
	for i := range records {
		visit(i)
	}
	return sorted
}

// reshowNotification is called by the manager when a deferred notification's
// timer fires. The service runs per-user, so we launch the webview directly.
func reshowNotification(n *manager.Notification) {
	selfPath, err := os.Executable()
	if err != nil {
		deck.Errorf("notification: reshow id=%s error=%q", n.ID, err)
		return
	}

	args := []string{"--notification-id", n.ID}
	if err := launchSubprocess(selfPath, args); err != nil {
		deck.Errorf("notification: reshow id=%s error=%q", n.ID, err)
		return
	}
	deck.Infof("notification: reshow id=%s heading=%q", n.ID, n.Config.Heading)
}
