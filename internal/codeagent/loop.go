// This file implements the agent loop from docs/RUNYX_CODE.md §1.2:
// plan -> tool call -> observation, repeated up to a step limit or
// until the model stops producing tool calls, with every step
// cancellable (the UI's Stop button cancels ctx, same as Chat's
// existing stall-watchdog Stop). It reuses internal/tools' registry
// and dispatcher rather than inventing a second function-calling
// convention -- Runyx Code's tools are "tools" entries like any other.
package codeagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/furaxdev/runyx/internal/tools"
)

// Message mirrors one OpenAI-shaped chat message, trimmed to what the
// loop needs to pass back and forth. Client implementations translate
// to/from whatever wire format they actually speak (a real
// /v1/chat/completions call, or a scripted fake for tests).
type Message struct {
	Role       string           `json:"role"` // "system" | "user" | "assistant" | "tool"
	Content    string           `json:"content"`
	ToolCalls  []tools.ToolCall `json:"tool_calls,omitempty"`   // set on an assistant message proposing calls
	ToolCallID string           `json:"tool_call_id,omitempty"` // set on a tool-role message
}

// ChatClient is the model boundary: anything that can take the
// conversation so far plus the available tool defs and return the
// model's next message. The real implementation calls Runyx's own
// /v1/chat/completions; tests use a scripted fake, which is the whole
// point of this interface existing -- the loop's control flow (step
// limit, Stop, tool dispatch, mascot-state hooks) is fully testable
// with no network and no real model.
type ChatClient interface {
	Complete(ctx context.Context, messages []Message, toolDefs []tools.Def) (Message, error)
}

// State is the mascot state (docs/RUNYX_CODE.md §1.2 step 3/6) the
// loop is currently in, surfaced via OnState so each client's renderer
// can react without the loop knowing anything about rendering.
type State string

const (
	StateThinking State = "thinking" // a model call is in flight
	StateWorking  State = "working"  // a tool call is executing
	StateDone     State = "done"     // the loop finished without error
	StateError    State = "error"    // the loop finished with an error
)

// DefaultMaxSteps bounds how many model-call/tool-call round trips one
// Run performs, so a model that never stops calling tools can't loop
// forever.
const DefaultMaxSteps = 20

// Loop ties a ChatClient to a tool registry and runs turns.
type Loop struct {
	Client   ChatClient
	Registry *tools.Registry
	MaxSteps int         // 0 means DefaultMaxSteps
	OnState  func(State) // optional; called on every state transition
}

// Run executes one user turn: it seeds the conversation with
// systemPrompt and userMessage, then alternates model calls and tool
// dispatches until the model responds with no tool calls, the step
// limit is hit, or ctx is cancelled (Stop). It returns the full
// transcript appended during this turn (not including systemPrompt),
// so the caller can persist it to session history.
func (l *Loop) Run(ctx context.Context, systemPrompt, userMessage string) ([]Message, error) {
	maxSteps := l.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	if l.Client == nil {
		return nil, fmt.Errorf("codeagent: Loop.Client is nil")
	}
	registry := l.Registry
	if registry == nil {
		registry = tools.NewRegistry()
	}

	messages := []Message{{Role: "system", Content: systemPrompt}, {Role: "user", Content: userMessage}}
	var transcript []Message
	transcript = append(transcript, messages[1]) // the user message, for the caller's record

	for step := 0; step < maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			l.setState(StateError)
			return transcript, fmt.Errorf("codeagent: stopped: %w", err)
		}

		l.setState(StateThinking)
		reply, err := l.Client.Complete(ctx, messages, registry.Defs())
		if err != nil {
			l.setState(StateError)
			return transcript, fmt.Errorf("codeagent: model call failed: %w", err)
		}
		messages = append(messages, reply)
		transcript = append(transcript, reply)

		if len(reply.ToolCalls) == 0 {
			l.setState(StateDone)
			return transcript, nil
		}

		l.setState(StateWorking)
		results := registry.Dispatch(ctx, reply.ToolCalls)
		for _, r := range results {
			msg := Message{Role: r.Role, Content: r.Content, ToolCallID: r.ToolCallID}
			messages = append(messages, msg)
			transcript = append(transcript, msg)
		}
	}

	l.setState(StateError)
	return transcript, fmt.Errorf("codeagent: step limit (%d) reached without the model stopping", maxSteps)
}

func (l *Loop) setState(s State) {
	if l.OnState != nil {
		l.OnState(s)
	}
}

// RegisterFileTools adds read_file, list_dir, search, write_file,
// edit_file, and delete to reg, backed by t. Kept as a free function
// (not a Tools method) so the JSON Schemas live next to the
// registration call, where they're easy to diff against
// docs/RUNYX_CODE.md §2.
func RegisterFileTools(reg *tools.Registry, t *Tools) {
	reg.Register(tools.FunctionSpec{
		Name:        "read_file",
		Description: "Read a file's content, relative to the project root. Optionally restrict to a line range.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string"},
				"start_line": map[string]any{"type": "integer"},
				"end_line":   map[string]any{"type": "integer"},
			},
			"required": []string{"path"},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Path      string `json:"path"`
			StartLine int    `json:"start_line"`
			EndLine   int    `json:"end_line"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("read_file: invalid arguments: %w", err)
		}
		return t.ReadFile(args.Path, args.StartLine, args.EndLine)
	})

	reg.Register(tools.FunctionSpec{
		Name:        "list_dir",
		Description: "List the immediate children of a directory, relative to the project root.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Path string `json:"path"`
		}
		json.Unmarshal(raw, &args)
		entries, err := t.ListDir(args.Path)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(entries)
		return string(out), nil
	})

	reg.Register(tools.FunctionSpec{
		Name:        "search",
		Description: "Search for a regex pattern across files under a directory (default: the whole project).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":     map[string]any{"type": "string"},
				"path":        map[string]any{"type": "string"},
				"max_results": map[string]any{"type": "integer"},
			},
			"required": []string{"pattern"},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Pattern    string `json:"pattern"`
			Path       string `json:"path"`
			MaxResults int    `json:"max_results"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("search: invalid arguments: %w", err)
		}
		matches, err := t.Search(args.Pattern, args.Path, args.MaxResults)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(matches)
		return string(out), nil
	})

	reg.Register(tools.FunctionSpec{
		Name:        "write_file",
		Description: "Create or fully overwrite a file, relative to the project root. Requires human approval.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string"},
				"content": map[string]any{"type": "string"},
				"reason":  map[string]any{"type": "string"},
			},
			"required": []string{"path", "content"},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Path, Content, Reason string
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("write_file: invalid arguments: %w", err)
		}
		if err := t.WriteFile(ctx, args.Path, args.Content, args.Reason); err != nil {
			return "", err
		}
		return "ok", nil
	})

	reg.Register(tools.FunctionSpec{
		Name:        "edit_file",
		Description: "Apply a unified diff to a file, relative to the project root. Requires human approval. Refused if the diff's context no longer matches the file's current content.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string"},
				"diff":   map[string]any{"type": "string"},
				"reason": map[string]any{"type": "string"},
			},
			"required": []string{"path", "diff"},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Path, Diff, Reason string
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("edit_file: invalid arguments: %w", err)
		}
		if err := t.EditFile(ctx, args.Path, args.Diff, args.Reason); err != nil {
			return "", err
		}
		return "ok", nil
	})

	reg.Register(tools.FunctionSpec{
		Name:        "delete",
		Description: "Delete a file or directory, relative to the project root. Snapshotted first so it's undoable. Requires human approval.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string"},
				"reason": map[string]any{"type": "string"},
			},
			"required": []string{"path"},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Path, Reason string
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("delete: invalid arguments: %w", err)
		}
		if err := t.Delete(ctx, args.Path, args.Reason); err != nil {
			return "", err
		}
		return "ok", nil
	})
}

// RegisterCommandTool adds run_command to reg, backed by ct.
func RegisterCommandTool(reg *tools.Registry, ct *CommandTool) {
	reg.Register(tools.FunctionSpec{
		Name:        "run_command",
		Description: "Run a shell command inside the project root. Requires human approval.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command":     map[string]any{"type": "string"},
				"args":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"reason":      map[string]any{"type": "string"},
				"max_seconds": map[string]any{"type": "integer"},
			},
			"required": []string{"command"},
		},
	}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Reason  string   `json:"reason"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return "", fmt.Errorf("run_command: invalid arguments: %w", err)
		}
		result, err := ct.Run(ctx, args.Command, args.Args, args.Reason)
		if err != nil {
			return "", err
		}
		out, _ := json.Marshal(result)
		return string(out), nil
	})
}
