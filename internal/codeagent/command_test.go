package codeagent

import (
	"context"
	"runtime"
	"testing"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/approval"
)

func TestCommandToolRunsApprovedCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix shell command")
	}
	root := t.TempDir()
	q := approval.NewQueue()
	ct := NewCommandTool(root, agent.Policy{}, q)
	go approveNextRequest(t, q)
	result, err := ct.Run(context.Background(), "echo", []string{"hello"}, "test")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d", result.ExitCode)
	}
}

func TestCommandToolRefusesWithoutApproval(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a unix shell command")
	}
	root := t.TempDir()
	q := approval.NewQueue()
	ct := NewCommandTool(root, agent.Policy{}, q)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ct.Run(ctx, "echo", []string{"hello"}, "test"); err == nil {
		t.Error("expected run_command to be refused when nobody approves it")
	}
}

func TestCommandToolRejectsDeniedPrefix(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	ct := NewCommandTool(root, agent.Policy{DenyPrefixes: []string{"sudo"}}, q)
	// No approver running -- policy rejection must happen before
	// approval is ever requested, or this test would hang.
	if _, err := ct.Run(context.Background(), "sudo", []string{"ls"}, "test"); err == nil {
		t.Error("expected a denied command prefix to be refused by policy, before approval")
	}
}
