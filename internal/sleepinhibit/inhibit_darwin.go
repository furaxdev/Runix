//go:build darwin

package sleepinhibit

import (
	"fmt"
	"os"
	"os/exec"
)

// darwinInhibitor holds a `caffeinate -i -w <pid>` process waiting on
// our own process ID, so it exits automatically if Runyx itself ever
// dies uncleanly -- `-i` asserts PreventUserIdleSystemSleep (the
// brief's primary recommendation), not `-d` (display sleep), matching
// "laisse l'écran s'éteindre". A direct IOPMAssertionCreateWithName
// call via cgo was considered and rejected: it would make this the
// only cgo-dependent package in a project that otherwise builds
// CGO_ENABLED=0 everywhere (see docs/DECISIONS.md's SQLite driver
// entry for the same reasoning applied elsewhere), and `caffeinate`
// is Apple's own supported wrapper around the identical assertion.
type darwinInhibitor struct {
	run func(reason string) (*exec.Cmd, error)
}

func newPlatformInhibitor() Inhibitor {
	return &darwinInhibitor{run: startCaffeinate}
}

func startCaffeinate(reason string) (*exec.Cmd, error) {
	if _, err := exec.LookPath("caffeinate"); err != nil {
		return nil, fmt.Errorf("caffeinate not found on $PATH: %w", err)
	}
	// caffeinate doesn't take a "why" string the way systemd-inhibit
	// does; reason is accepted for interface symmetry and logged by
	// the caller (internal/sleepinhibit.Manager), not passed to the OS.
	_ = reason
	cmd := exec.Command("caffeinate", "-i", "-w", fmt.Sprint(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start caffeinate: %w", err)
	}
	return cmd, nil
}

func (d *darwinInhibitor) Acquire(reason string) (func(), error) {
	cmd, err := d.run(reason)
	if err != nil {
		return nil, err
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
	}
	return release, nil
}
