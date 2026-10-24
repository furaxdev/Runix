package sleepinhibit

import (
	"sync"
	"time"
)

// DefaultMaxHold is the mission's example cap ("par exemple 4 h") on
// how long one inhibit can be held continuously, after which it is
// force-released and the caller is expected to surface a notice --
// not a permanent lock, per docs/RUNYX_CODE.md section 8.
const DefaultMaxHold = 4 * time.Hour

// BatteryStatus is read periodically by a Manager with battery-aware
// pausing enabled. percent is 0-100; onBattery is false when on AC
// power (in which case percent/threshold don't matter).
type BatteryStatus struct {
	Percent   int
	OnBattery bool
}

// Event is published on a Manager's Events channel whenever its held
// state changes, for a UI to show "Keeping this PC awake" / explain
// why it stopped.
type Event struct {
	Held   bool
	Reason string // "acquired" | "released" | "max-hold-exceeded" | "low-battery" | "error:<msg>"
}

// Manager wraps a raw Inhibitor with the policy layer the mission
// calls for: reference counting (several concurrent tool calls/turns
// each wanting the system kept awake don't fight over one hold),
// a maximum continuous hold duration, and an optional battery-level
// check that releases and pauses below a threshold while on battery.
// A Manager is created once per daemon and reused across sessions.
type Manager struct {
	inhibitor Inhibitor
	maxHold   time.Duration

	mu             sync.Mutex
	refCount       int
	release        func()
	acquiredAt     time.Time
	reason         string
	maxHoldTimer   *time.Timer
	battery        func() (BatteryStatus, bool) // bool: whether a real reading was obtained
	batteryTicker  *time.Ticker
	batteryStop    chan struct{}
	keepAwakeOnBat bool
	lowBatteryPct  int

	Events chan Event
}

// NewManager wraps inhibitor with default policy (4h max hold, no
// battery awareness -- see WithBatteryCheck). Events is a buffered
// channel; a Manager never blocks on a slow/absent consumer.
func NewManager(inhibitor Inhibitor) *Manager {
	return &Manager{
		inhibitor:     inhibitor,
		maxHold:       DefaultMaxHold,
		lowBatteryPct: 20,
		Events:        make(chan Event, 16),
	}
}

// WithMaxHold overrides the default 4h cap -- exposed for the "délai
// maximal configurable" setting and for tests.
func (m *Manager) WithMaxHold(d time.Duration) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxHold = d
	return m
}

// WithBatteryCheck enables periodic battery-level checking:
// below lowBatteryPct while on battery, the inhibit is released, the
// caller is expected to pause the session (Events fires "low-battery"
// so the daemon can do that), and nothing re-acquires until Acquire is
// called again. keepAwakeOnBattery mirrors the "Keep awake on
// battery" setting (off by default per the mission) -- when true, the
// battery check is a no-op (this method simply isn't called by the
// caller wiring it up). statusFn is injected so this is fully unit
// testable without reading real OS battery state.
func (m *Manager) WithBatteryCheck(statusFn func() (BatteryStatus, bool), interval time.Duration, lowBatteryPct int) *Manager {
	m.mu.Lock()
	m.battery = statusFn
	m.lowBatteryPct = lowBatteryPct
	m.mu.Unlock()

	m.batteryStop = make(chan struct{})
	ticker := time.NewTicker(interval)
	m.batteryTicker = ticker
	go func() {
		for {
			select {
			case <-ticker.C:
				m.checkBattery()
			case <-m.batteryStop:
				return
			}
		}
	}()
	return m
}

func (m *Manager) checkBattery() {
	m.mu.Lock()
	statusFn := m.battery
	keepOnBattery := m.keepAwakeOnBat
	low := m.lowBatteryPct
	held := m.refCount > 0
	m.mu.Unlock()

	if statusFn == nil || !held || keepOnBattery {
		return
	}
	status, ok := statusFn()
	if !ok || !status.OnBattery || status.Percent > low {
		return
	}
	m.ForceRelease("low-battery")
}

// SetKeepAwakeOnBattery mirrors the "Keep awake on battery" setting.
func (m *Manager) SetKeepAwakeOnBattery(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keepAwakeOnBat = v
}

// Acquire increments the hold's reference count, actually calling the
// underlying Inhibitor only on the first (0->1) acquire. Safe to call
// from multiple goroutines (concurrent tool calls in the same agent
// turn, or multiple sessions).
func (m *Manager) Acquire(reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.refCount > 0 {
		m.refCount++
		return nil
	}

	release, err := m.inhibitor.Acquire(reason)
	if err != nil {
		m.emit(Event{Held: false, Reason: "error:" + err.Error()})
		return err
	}
	m.refCount = 1
	m.release = release
	m.acquiredAt = time.Now()
	m.reason = reason
	m.armMaxHoldLocked()
	m.emit(Event{Held: true, Reason: "acquired"})
	return nil
}

// armMaxHoldLocked must be called with m.mu held.
func (m *Manager) armMaxHoldLocked() {
	if m.maxHoldTimer != nil {
		m.maxHoldTimer.Stop()
	}
	if m.maxHold <= 0 {
		return
	}
	m.maxHoldTimer = time.AfterFunc(m.maxHold, func() {
		m.ForceRelease("max-hold-exceeded")
	})
}

// Release decrements the reference count, releasing the underlying
// inhibit once it reaches zero -- "relâché dès que l'agent est
// inactif, a terminé, a échoué ou a été stoppé" means every one of
// those call sites calls Release exactly once per matching Acquire,
// including via a deferred call so a panic mid-turn still releases.
func (m *Manager) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.refCount <= 0 {
		return
	}
	m.refCount--
	if m.refCount > 0 {
		return
	}
	m.releaseLocked("released")
}

// ForceRelease drops the hold immediately regardless of the reference
// count -- "bouton pour relâcher tout de suite", and also used
// internally for the max-hold timeout and low-battery pause.
func (m *Manager) ForceRelease(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.refCount <= 0 {
		return
	}
	m.refCount = 0
	m.releaseLocked(reason)
}

// releaseLocked must be called with m.mu held and refCount already at 0.
func (m *Manager) releaseLocked(reason string) {
	if m.maxHoldTimer != nil {
		m.maxHoldTimer.Stop()
		m.maxHoldTimer = nil
	}
	if m.release != nil {
		m.release()
		m.release = nil
	}
	m.emit(Event{Held: false, Reason: reason})
}

func (m *Manager) emit(e Event) {
	select {
	case m.Events <- e:
	default: // never block the caller on a full/unread channel
	}
}

// Status reports whether the system is currently being kept awake,
// since when, and why -- for the "Keeping this PC awake" indicator in
// the UI/TUI/`runyx doctor`.
func (m *Manager) Status() (held bool, since time.Time, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refCount > 0, m.acquiredAt, m.reason
}

// Close stops the battery-check goroutine, if one was started. Safe
// to call even if WithBatteryCheck was never used.
func (m *Manager) Close() {
	if m.batteryTicker != nil {
		m.batteryTicker.Stop()
	}
	if m.batteryStop != nil {
		close(m.batteryStop)
	}
}
