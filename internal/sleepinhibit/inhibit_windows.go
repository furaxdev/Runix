//go:build windows

package sleepinhibit

import (
	"fmt"
	"sync"
	"syscall"
)

// SetThreadExecutionState flags, from winnt.h. ES_SYSTEM_REQUIRED
// alone (never ES_DISPLAY_REQUIRED) keeps the system awake while
// letting the display sleep normally, per the mission brief.
const (
	esContinuous     = 0x80000000
	esSystemRequired = 0x00000001
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procSetThreadExecState    = kernel32.NewProc("SetThreadExecutionState")
	windowsInhibitMu          sync.Mutex
	windowsInhibitActiveCount int
)

// windowsInhibitor calls SetThreadExecutionState directly via
// syscall, not golang.org/x/sys/windows -- this is the one Win32 call
// this package needs, and the project otherwise has no Windows-
// specific dependency to justify adding x/sys just for this.
type windowsInhibitor struct{}

func newPlatformInhibitor() Inhibitor {
	return &windowsInhibitor{}
}

func (w *windowsInhibitor) Acquire(reason string) (func(), error) {
	_ = reason // SetThreadExecutionState has no "why" parameter
	if err := setExecState(esContinuous | esSystemRequired); err != nil {
		return nil, err
	}
	windowsInhibitMu.Lock()
	windowsInhibitActiveCount++
	windowsInhibitMu.Unlock()

	released := false
	release := func() {
		windowsInhibitMu.Lock()
		defer windowsInhibitMu.Unlock()
		if released {
			return
		}
		released = true
		windowsInhibitActiveCount--
		if windowsInhibitActiveCount <= 0 {
			windowsInhibitActiveCount = 0
			// Clearing ES_SYSTEM_REQUIRED (ES_CONTINUOUS alone) restores
			// normal idle-sleep behavior for this thread.
			setExecState(esContinuous)
		}
	}
	return release, nil
}

func setExecState(flags uint32) error {
	ret, _, callErr := procSetThreadExecState.Call(uintptr(flags))
	if ret == 0 {
		return fmt.Errorf("SetThreadExecutionState failed: %v", callErr)
	}
	return nil
}
