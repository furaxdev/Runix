// This file implements the run_command tool from docs/RUNYX_CODE.md
// §2 as a thin adapter onto the existing internal/agent.Agent: Runyx
// Code brings no second command-execution path, policy, or audit
// log -- it reuses the one already built and tested for the legacy
// shell agent, bridged to this package's own approval.Queue the same
// way internal/daemon.approvalAdapter already bridges it for that
// legacy agent.
package codeagent

import (
	"context"
	"fmt"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/approval"
)

// CommandTool wraps an *agent.Agent pinned to this sandbox's project
// root, so Runyx Code's loop calls one method instead of reaching into
// internal/agent directly.
type CommandTool struct {
	agent *agent.Agent
}

// NewCommandTool builds the shell-command tool for one project root,
// policed by internal/agent.Policy and gated by approvals.
func NewCommandTool(projectRoot string, policy agent.Policy, approvals *approval.Queue) *CommandTool {
	policy.WorkDir = projectRoot
	a := agent.New(policy, approvalAdapter(approvals))
	return &CommandTool{agent: a}
}

// Run executes one approved, policy-checked command and returns its
// result. See internal/agent.Agent.Execute for what this guarantees:
// policy validation before approval is even asked, a git snapshot
// before running, a diff after, and an audit entry on every path.
func (c *CommandTool) Run(ctx context.Context, command string, args []string, reason string) (agent.Result, error) {
	return c.agent.Execute(ctx, command, args, reason)
}

// approvalAdapter bridges internal/agent's ApprovalFunc to this
// package's approval.Queue, identically to internal/daemon's own
// approvalAdapter (duplicated rather than imported across packages to
// avoid internal/daemon depending on internal/codeagent or vice versa
// for a four-line function).
func approvalAdapter(q *approval.Queue) agent.ApprovalFunc {
	return func(ctx context.Context, req agent.Request) (bool, agent.ApprovalScope) {
		if q == nil {
			return false, agent.ApprovalDenied
		}
		detail := fmt.Sprintf("command: %s\nargs: %v\nworkdir: %s\nreason: %s", req.Command, req.Args, req.WorkDir, req.Reason)
		areq := approval.NewRequest("command", req.String(), detail)
		decision, err := q.Submit(ctx, areq)
		if err != nil || !decision.Allow {
			return false, agent.ApprovalDenied
		}
		return true, agent.ApprovalOnce
	}
}
