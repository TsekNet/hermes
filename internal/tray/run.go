package tray

import (
	"os"
	"os/exec"
	"time"

	"fyne.io/systray"
	"github.com/google/deck"
)

// CountFunc returns the current number of active notifications.
type CountFunc func() int

// StopFunc is called when the user clicks Quit in the tray menu.
type StopFunc func()

// Config holds the parameters for running the system tray.
type Config struct {
	Count CountFunc
	Stop  StopFunc
	Ready chan<- struct{} // closed when the tray is initialized; nil = no signal
	done  chan struct{}
}

// Run starts the system tray icon. It blocks until the tray exits.
// Must be called from the main goroutine on macOS (Cocoa run loop).
func Run(cfg Config) {
	cfg.done = make(chan struct{})
	systray.Run(func() { onReady(cfg) }, func() { close(cfg.done) })
}

// Quit requests the systray event loop to exit.
func Quit() {
	systray.Quit()
}

func onReady(cfg Config) {
	systray.SetIcon(iconData)
	systray.SetTitle("")
	systray.SetTooltip(FormatTooltip(0))

	mInbox := systray.AddMenuItem("Notification History", "View notification history")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit Hermes", "Stop the notification service")

	go pollCount(cfg.Count, mInbox, cfg.done)

	if cfg.Ready != nil {
		close(cfg.Ready)
	}

	for {
		select {
		case <-mInbox.ClickedCh:
			launchInbox()
		case <-mQuit.ClickedCh:
			deck.Info("tray: quit requested by user")
			if cfg.Stop != nil {
				cfg.Stop()
			}
			systray.Quit()
			return
		}
	}
}

func pollCount(countFn CountFunc, mInbox *systray.MenuItem, done <-chan struct{}) {
	base := iconData
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			count := countFn()
			systray.SetTooltip(FormatTooltip(count))
			mInbox.SetTitle(FormatInboxLabel(count))
			systray.SetIcon(BadgeIcon(base, count))
		}
	}
}

// launchInbox starts the inbox UI as a detached subprocess.
// Uses os.Executable to avoid PATH injection (red team: attacker-controlled
// PATH could plant a malicious "hermes" binary).
func launchInbox() {
	selfPath, err := os.Executable()
	if err != nil {
		deck.Errorf("tray: cannot determine executable path: %v", err)
		return
	}

	args := InboxArgs()
	cmd := exec.Command(selfPath, args...)
	detachProcess(cmd)

	if err := cmd.Start(); err != nil {
		deck.Errorf("tray: launch inbox: %v", err)
		return
	}
	deck.Infof("tray: launched inbox (pid %d)", cmd.Process.Pid)
}

// detachProcess is implemented in detach_unix.go / detach_windows.go.
