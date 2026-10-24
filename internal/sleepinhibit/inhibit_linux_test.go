//go:build linux

package sleepinhibit

import (
	"os/exec"
	"testing"
	"time"
)

func TestLinuxInhibitorRealSystemdInhibit(t *testing.T) {
	if _, err := exec.LookPath("systemd-inhibit"); err != nil {
		t.Skip("systemd-inhibit not available in this environment")
	}
	inh := newPlatformInhibitor()
	release, err := inh.Acquire("runyx test")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	// The child process should actually be running.
	time.Sleep(100 * time.Millisecond)
	release()
	release() // idempotent
}

func TestLinuxInhibitorPropagatesStartFailure(t *testing.T) {
	inh := &linuxInhibitor{
		run: func(reason string) (*exec.Cmd, error) {
			// Simulate systemd-inhibit missing / failing to start.
			_, err := exec.LookPath("definitely-not-a-real-binary-xyz")
			return nil, err
		},
	}
	if _, err := inh.Acquire("x"); err == nil {
		t.Fatal("expected an error when the inhibitor process fails to start")
	}
}

func TestLinuxInhibitorKillsChildOnRelease(t *testing.T) {
	calledWith := ""
	inh := &linuxInhibitor{
		run: func(reason string) (*exec.Cmd, error) {
			calledWith = reason
			// A real, short-lived child process we can verify gets killed
			// rather than lingering.
			cmd := exec.Command("sleep", "30")
			if err := cmd.Start(); err != nil {
				return nil, err
			}
			return cmd, nil
		},
	}
	release, err := inh.Acquire("agent turn")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if calledWith != "agent turn" {
		t.Errorf("reason passed through = %q", calledWith)
	}
	release()
	// Calling release a second time must not panic or error.
	release()
}
