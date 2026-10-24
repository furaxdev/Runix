// Package sleepinhibit prevents the system (not the display) from
// sleeping while a Runyx Code agent session is actively working, per
// docs/RUNYX_CODE.md section 8: Windows via SetThreadExecutionState
// (ES_SYSTEM_REQUIRED, deliberately never ES_DISPLAY_REQUIRED), macOS
// via `caffeinate`, Linux via a `systemd-inhibit`-held child process
// (the documented fallback in the mission brief -- a raw
// org.freedesktop.login1.Inhibit D-Bus call would need a new
// dependency this project doesn't otherwise carry; `systemd-inhibit`
// wraps the exact same call).
package sleepinhibit

import "fmt"

// Inhibitor is the per-OS primitive: Acquire blocks system idle sleep
// until the returned release func is called. release is always safe
// to call more than once (idempotent) and safe to call from any
// goroutine.
type Inhibitor interface {
	Acquire(reason string) (release func(), err error)
}

// New returns the real inhibitor for the running OS -- see
// inhibit_linux.go / inhibit_darwin.go / inhibit_windows.go /
// inhibit_other.go for GOOS-specific implementations.
func New() Inhibitor {
	return newPlatformInhibitor()
}

// ErrUnsupported is returned by Acquire on a platform with no
// implementation (and documented as such, per the mission's call for
// honest limits rather than a silent no-op that claims the system is
// being kept awake when it isn't).
var ErrUnsupported = fmt.Errorf("sleepinhibit: not implemented on this platform")
