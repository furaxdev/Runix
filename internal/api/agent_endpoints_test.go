package api

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/interpreter"
)

func TestAgentExecuteDisabledByDefault(t *testing.T) {
	mux := http.NewServeMux()
	a := &AgentExtras{}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/agent/execute", map[string]string{"Command": "echo", "Reason": "test"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when no agent is configured", rec.Code)
	}
}

func TestInterpreterRunOverHTTP(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	mux := http.NewServeMux()
	a := &AgentExtras{Interpreter: &interpreter.Interpreter{WorkDir: t.TempDir()}}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/interpreter/run", map[string]string{"Code": "print('ok depuis http')"})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ok depuis http") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestAgentExecuteOverHTTP(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	ag := agent.New(agent.Policy{WorkDir: dir}, func(r agent.Request) (bool, agent.ApprovalScope) {
		return true, agent.ApprovalOnce
	})
	mux := http.NewServeMux()
	a := &AgentExtras{Agent: ag}
	a.RegisterAgent(mux)

	rec := doJSON(t, mux, "POST", "/api/runyx/agent/execute", map[string]any{
		"Command": "echo", "Args": []string{"hello"}, "Reason": "greet",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hello") {
		t.Errorf("body = %s", rec.Body.String())
	}
}

func TestPermissionsCenterReflectsState(t *testing.T) {
	dir := t.TempDir()
	ag := agent.New(agent.Policy{WorkDir: dir, DenyPrefixes: []string{"curl"}}, nil)
	pc := &PermissionsCenter{Agent: ag, VisionEnabled: true}
	mux := http.NewServeMux()
	pc.RegisterPermissions(mux)

	rec := doJSON(t, mux, "GET", "/api/runyx/permissions", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"enabled":true`) || !strings.Contains(body, dir) || !strings.Contains(body, "curl") {
		t.Errorf("body = %s", body)
	}
}
