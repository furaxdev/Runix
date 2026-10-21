package api

import (
	"encoding/json"
	"net/http"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/interpreter"
)

// AgentExtras wires the safe shell agent (item 21) and the sandboxed
// code interpreter (item 22) into the HTTP API. Both are nil-safe: if
// not configured, their endpoints return 503 rather than panicking —
// a Runyx install with no agent configured still serves every other
// endpoint normally.
type AgentExtras struct {
	Agent       *agent.Agent
	Interpreter *interpreter.Interpreter
}

// RegisterAgent wires agent-shell and interpreter routes onto mux.
func (a *AgentExtras) RegisterAgent(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/runyx/agent/execute", a.handleExecute)
	mux.HandleFunc("POST /api/runyx/agent/undo", a.handleUndo)
	mux.HandleFunc("POST /api/runyx/interpreter/run", a.handleInterpreterRun)
}

func (a *AgentExtras) handleExecute(w http.ResponseWriter, r *http.Request) {
	if a.Agent == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("the shell agent is not configured (set agent_work_dir in config.toml to enable it)"))
		return
	}
	var req struct {
		Command string
		Args    []string
		Reason  string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.Agent.Execute(r.Context(), req.Command, req.Args, req.Reason)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *AgentExtras) handleUndo(w http.ResponseWriter, r *http.Request) {
	if a.Agent == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("the shell agent is not configured (set agent_work_dir in config.toml to enable it)"))
		return
	}
	var req struct{ SnapshotID string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := a.Agent.Undo(r.Context(), req.SnapshotID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *AgentExtras) handleInterpreterRun(w http.ResponseWriter, r *http.Request) {
	if a.Interpreter == nil {
		writeError(w, http.StatusServiceUnavailable, jsonErr("the code interpreter is not configured (set interpreter_work_dir in config.toml to enable it)"))
		return
	}
	var req struct{ Code string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := a.Interpreter.Run(r.Context(), req.Code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
