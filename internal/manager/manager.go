// Package manager tracks active notifications, handles deferrals, and
// enforces deadlines. It is the core state machine for the hermes service.
package manager

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/TsekNet/hermes/internal/config"
	"github.com/TsekNet/hermes/internal/exitcodes"
	"github.com/TsekNet/hermes/internal/store"
	"github.com/google/deck"
)

// State describes where a notification is in its lifecycle.
type State string

const (
	StatePending  State = "pending"
	StateDeferred State = "deferred"
	StateActive   State = "awaiting_response"
	StateDone     State = "done"
)

// Notification is the service-side representation of a notification.
type Notification struct {
	ID         string
	Config     *config.NotificationConfig
	State      State
	DeferCount int
	CreatedAt  time.Time
	Deadline   time.Time // zero = no deadline

	// result is sent when the notification reaches a terminal state.
	result chan Result
	// deferTimer fires when a deferral period expires and the
	// notification should be re-shown.
	deferTimer *time.Timer
}

// Result is the terminal outcome of a notification.
type Result struct {
	Value    string
	ExitCode int32
}

// MaxActiveNotifications is the ceiling on concurrent notifications.
// Prevents resource exhaustion from rapid-fire Notify calls.
const MaxActiveNotifications = 50

// Manager holds the set of active notifications.
type Manager struct {
	mu       sync.Mutex
	active   map[string]*Notification
	store    *store.Store // nil = in-memory only
	onReshow func(n *Notification)
}

// New creates a Manager. onReshow is called (in a new goroutine) when a
// deferred notification's timer fires and it should be launched again.
// Pass nil for store to disable persistence (tests, local mode).
func New(onReshow func(n *Notification), s *store.Store) *Manager {
	return &Manager{
		active:   make(map[string]*Notification),
		store:    s,
		onReshow: onReshow,
	}
}

// Restore loads persisted notifications from the store and re-registers them.
// Notifications past their deadline are auto-actioned. Deferred notifications
// are re-shown immediately (the timer was lost on shutdown).
// Returns the number of restored notifications.
func (m *Manager) Restore() int {
	if m.store == nil {
		return 0
	}
	records, err := m.store.LoadAll()
	if err != nil {
		deck.Errorf("manager: restore: %v", err)
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	restored := 0
	for _, r := range records {
		n := &Notification{
			ID:         r.ID,
			Config:     r.Config,
			State:      State(r.State),
			DeferCount: r.DeferCount,
			CreatedAt:  r.CreatedAt,
			Deadline:   r.Deadline,
			result:     make(chan Result, 1),
		}

		// Deadline already passed — auto-action and delete from store.
		if !n.Deadline.IsZero() && time.Now().After(n.Deadline) {
			deck.Infof("manager: restore %s: deadline passed, auto-actioning", n.ID)
			n.State = StateDone
			n.result <- Result{Value: n.Config.TimeoutValue, ExitCode: exitcodes.Timeout}
			close(n.result)
			m.store.Delete(n.ID)
			continue
		}

		// Re-register as pending so the next reshow fires.
		n.State = StatePending
		m.active[n.ID] = n
		restored++
		deck.Infof("manager: restored notification %s heading=%q defers=%d", n.ID, n.Config.Heading, n.DeferCount)

		// Kick a reshow immediately — the user was waiting for this.
		if m.onReshow != nil {
			go m.onReshow(n)
		}
	}
	return restored
}

// Submit registers a new notification and returns its ID and result channel.
// The caller blocks on the result channel until the notification completes.
// Returns ("", ch) with an immediate error result if the notification is
// rejected (too many active, or duplicate ID).
func (m *Manager) Submit(cfg *config.NotificationConfig) (string, <-chan Result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Count non-done notifications toward the cap.
	active := 0
	for _, n := range m.active {
		if n.State != StateDone {
			active++
		}
	}
	if active >= MaxActiveNotifications {
		deck.Warningf("manager: rejecting notification, at capacity (%d)", MaxActiveNotifications)
		ch := make(chan Result, 1)
		ch <- Result{Value: "", ExitCode: exitcodes.Error}
		close(ch)
		return "", ch
	}

	id := cfg.ID
	if id == "" || strings.HasPrefix(id, "-") {
		id = generateID()
	}
	cfg.ID = id

	// Reject duplicate IDs to avoid orphaning the existing caller.
	if existing, ok := m.active[id]; ok && existing.State != StateDone {
		deck.Warningf("manager: rejecting duplicate notification ID %s", id)
		ch := make(chan Result, 1)
		ch <- Result{Value: "", ExitCode: exitcodes.Error}
		close(ch)
		return "", ch
	}

	n := &Notification{
		ID:        id,
		Config:    cfg,
		State:     StatePending,
		CreatedAt: time.Now(),
		result:    make(chan Result, 1),
	}

	if dl := config.ParseDeadline(cfg.DeferDeadline); dl > 0 {
		n.Deadline = n.CreatedAt.Add(dl)
	}

	m.active[id] = n
	m.persist(n)
	deck.Infof("manager: submitted notification %s heading=%q", id, cfg.Heading)

	// Launch the UI immediately.
	if m.onReshow != nil {
		go m.onReshow(n)
	}

	return id, n.result
}

// GetConfig returns the config for a notification (called by UI subprocess).
func (m *Manager) GetConfig(id string) (*config.NotificationConfig, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.active[id]
	if !ok {
		return nil, false
	}
	n.State = StateActive
	return n.Config, true
}

// DeferralAllowed returns true if the notification can still be deferred.
func (m *Manager) DeferralAllowed(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.active[id]
	if !ok {
		return false
	}
	if n.Config.MaxDefers > 0 && n.DeferCount >= n.Config.MaxDefers {
		return false
	}
	if !n.Deadline.IsZero() && time.Now().After(n.Deadline) {
		return false
	}
	return true
}

// ReportChoice handles a user response from the UI.
// Returns true if accepted (first responder wins per notification).
func (m *Manager) ReportChoice(id, value string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	n, ok := m.active[id]
	if !ok || n.State == StateDone {
		return false
	}

	// Deferral handling.
	if strings.HasPrefix(value, "defer") {
		return m.handleDeferLocked(n, value)
	}

	return m.completeLocked(n, value)
}

// Cancel cancels a notification by ID. Returns true if found.
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	n, ok := m.active[id]
	if !ok || n.State == StateDone {
		return false
	}

	m.completeLocked(n, "cancelled")
	return true
}

// List returns info about all non-done notifications.
func (m *Manager) List() []NotificationInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []NotificationInfo
	for _, n := range m.active {
		if n.State == StateDone {
			continue
		}
		info := NotificationInfo{
			ID:         n.ID,
			Heading:    n.Config.Heading,
			State:      n.State,
			DeferCount: n.DeferCount,
			CreatedAt:  n.CreatedAt,
		}
		if !n.Deadline.IsZero() {
			info.Deadline = n.Deadline
		}
		out = append(out, info)
	}
	return out
}

// NotificationInfo is a read-only snapshot for the List RPC.
type NotificationInfo struct {
	ID         string
	Heading    string
	State      State
	DeferCount int
	Deadline   time.Time
	CreatedAt  time.Time
}

// handleDeferLocked processes a defer response. Must be called with mu held.
func (m *Manager) handleDeferLocked(n *Notification, value string) bool {
	if n.Config.MaxDefers > 0 && n.DeferCount >= n.Config.MaxDefers {
		deck.Warningf("manager: max defers reached for %s, auto-actioning", n.ID)
		return m.completeLocked(n, n.Config.TimeoutValue)
	}
	if !n.Deadline.IsZero() && time.Now().After(n.Deadline) {
		deck.Warningf("manager: deadline passed for %s, auto-actioning", n.ID)
		return m.completeLocked(n, n.Config.TimeoutValue)
	}

	n.DeferCount++
	n.State = StateDeferred

	// Determine how long to wait before re-showing.
	wait := config.ParseDeferValue(value)
	if wait == 0 {
		wait = 1 * time.Hour // default deferral
	}

	// Clamp to deadline if set.
	if !n.Deadline.IsZero() {
		remaining := time.Until(n.Deadline)
		if remaining <= 0 {
			return m.completeLocked(n, n.Config.TimeoutValue)
		}
		if wait > remaining {
			wait = remaining
		}
	}

	deck.Infof("manager: notification %s deferred (%d), re-show in %s", n.ID, n.DeferCount, wait)
	m.persist(n)

	if n.deferTimer != nil {
		n.deferTimer.Stop()
	}
	n.deferTimer = time.AfterFunc(wait, func() {
		m.mu.Lock()
		if n.State == StateDeferred {
			n.State = StatePending
			deck.Infof("manager: re-showing notification %s after deferral", n.ID)
		}
		m.mu.Unlock()
		if m.onReshow != nil {
			m.onReshow(n)
		}
	})

	return true
}

// completeLocked resolves a notification. Must be called with mu held.
func (m *Manager) completeLocked(n *Notification, value string) bool {
	if n.deferTimer != nil {
		n.deferTimer.Stop()
	}
	n.State = StateDone

	exitCode := exitcodes.ForValue(value)
	select {
	case n.result <- Result{Value: value, ExitCode: exitCode}:
	default:
	}
	close(n.result)

	// Remove from store immediately.
	if m.store != nil {
		m.store.Delete(n.ID)
	}

	// Clean up in-memory after a short delay to avoid races with late UI reports.
	go func() {
		time.Sleep(30 * time.Second)
		m.mu.Lock()
		// Ensure we don't delete a new notification with the same ID
		if current, ok := m.active[n.ID]; ok && current == n {
			delete(m.active, n.ID)
		}
		m.mu.Unlock()
	}()

	deck.Infof("manager: notification %s completed with %q (exit %d)", n.ID, value, exitCode)
	return true
}

// persist saves the notification to the store (if configured).
func (m *Manager) persist(n *Notification) {
	if m.store == nil {
		return
	}
	r := &store.Record{
		ID:         n.ID,
		Config:     n.Config,
		State:      string(n.State),
		DeferCount: n.DeferCount,
		CreatedAt:  n.CreatedAt,
		Deadline:   n.Deadline,
	}
	if err := m.store.Save(r); err != nil {
		deck.Errorf("manager: persist %s: %v", n.ID, err)
	}
}


func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("n-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
