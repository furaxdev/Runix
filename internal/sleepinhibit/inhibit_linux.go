//go:build linux

package sleepinhibit

import (
	"fmt"
	"os/exec"
)

// linuxInhibitor holds a `systemd-inhibit --what=sleep:idle ... sleep
// infinity` child process alive for the duration of the inhibit: the
// child is what's actually registered with logind (what=sleep:idle,
// mode=block), and killing it is what releases the inhibit lock --
// the same mechanism the systemd-inhibit(1) command line tool itself
// relies on, just driven as our own subprocess instead of wrapping
// another command with it.
type linuxInhibitor struct {
	// run is overridable in tests so they don't need a real logind.
	run func(reason string) (*exec.Cmd, error)
}

func newPlatformInhibitor() Inhibitor {
	return &linuxInhibitor{run: startSystemdInhibit}
}

func startSystemdInhibit(reason string) (*exec.Cmd, error) {
	if _, err := exec.LookPath("systemd-inhibit"); err != nil {
		return nil, fmt.Errorf("systemd-inhibit not found on $PATH: %w", err)
	}
	cmd := exec.Command("systemd-inhibit",
		"--what=sleep:idle",
		"--who=Runyx",
		"--why="+reason,
		"--mode=block",
		"sleep", "infinity",
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start systemd-inhibit: %w", err)
	}
	return cmd, nil
}

func (l *linuxInhibitor) Acquire(reason string) (func(), error) {
	cmd, err := l.run(reason)
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
		cmd.Wait() // reap, avoid a zombie
	}
	return release, nil
}
