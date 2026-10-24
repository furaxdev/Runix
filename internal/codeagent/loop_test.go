package codeagent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/furaxdev/runyx/internal/approval"
	"github.com/furaxdev/runyx/internal/tools"
)

// scriptedClient returns, on each successive Complete call, the next
// Message from replies -- the "fake model, no network" the mission
// explicitly required for testing the loop's control flow.
type scriptedClient struct {
	replies []Message
	calls   int
}

func (c *scriptedClient) Complete(ctx context.Context, messages []Message, defs []tools.Def) (Message, error) {
	if c.calls >= len(c.replies) {
		return Message{}, errors.New("scriptedClient: ran out of scripted replies")
	}
	reply := c.replies[c.calls]
	c.calls++
	return reply, nil
}

func toolCallArgs(name string, args map[string]any) tools.ToolCall {
	raw, _ := json.Marshal(args)
	tc := tools.ToolCall{ID: "call_1", Type: "function"}
	tc.Function.Name = name
	tc.Function.Arguments = string(raw)
	return tc
}

func TestLoopRunsOneToolCallThenStops(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	ftools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	reg := tools.NewRegistry()
	RegisterFileTools(reg, ftools)

	client := &scriptedClient{replies: []Message{
		{Role: "assistant", ToolCalls: []tools.ToolCall{toolCallArgs("write_file", map[string]any{
			"path": "hello.txt", "content": "hi", "reason": "test",
		})}},
		{Role: "assistant", Content: "Done, I created hello.txt."},
	}}
	loop := &Loop{Client: client, Registry: reg}

	go approveNextRequest(t, q)
	transcript, err := loop.Run(context.Background(), "system prompt", "create hello.txt")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, readErr := os.ReadFile(filepath.Join(root, "hello.txt"))
	if readErr != nil || string(data) != "hi" {
		t.Errorf("hello.txt content = %q, %v", data, readErr)
	}

	// user message, assistant tool-call message, tool result message,
	// final assistant message.
	if len(transcript) != 4 {
		t.Fatalf("got %d transcript messages, want 4: %+v", len(transcript), transcript)
	}
	if transcript[len(transcript)-1].Content != "Done, I created hello.txt." {
		t.Errorf("final message = %q", transcript[len(transcript)-1].Content)
	}
}

func TestLoopReportsToolErrorWithoutStoppingTheWholeBatch(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	ftools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	reg := tools.NewRegistry()
	RegisterFileTools(reg, ftools)

	client := &scriptedClient{replies: []Message{
		{Role: "assistant", ToolCalls: []tools.ToolCall{toolCallArgs("read_file", map[string]any{
			"path": "../outside.txt",
		})}},
		{Role: "assistant", Content: "I couldn't read that file."},
	}}
	loop := &Loop{Client: client, Registry: reg}

	transcript, err := loop.Run(context.Background(), "system prompt", "read ../outside.txt")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	toolMsg := transcript[2]
	if toolMsg.Role != "tool" {
		t.Fatalf("expected a tool-role message at index 2, got %+v", toolMsg)
	}
	if toolMsg.Content == "" {
		t.Error("expected the sandbox refusal to be surfaced as the tool result, not silently dropped")
	}
}

func TestLoopStopsAtMaxSteps(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(tools.FunctionSpec{Name: "noop"}, func(ctx context.Context, raw json.RawMessage) (string, error) {
		return "ok", nil
	})
	// Always returns a tool call -- a model that never stops.
	replies := make([]Message, 10)
	for i := range replies {
		replies[i] = Message{Role: "assistant", ToolCalls: []tools.ToolCall{toolCallArgs("noop", map[string]any{})}}
	}
	client := &scriptedClient{replies: replies}
	loop := &Loop{Client: client, Registry: reg, MaxSteps: 3}

	_, err := loop.Run(context.Background(), "system", "go forever")
	if err == nil {
		t.Fatal("expected the loop to stop at MaxSteps with an error")
	}
	if client.calls != 3 {
		t.Errorf("expected exactly MaxSteps (3) model calls, got %d", client.calls)
	}
}

func TestLoopStopsWhenContextIsCancelled(t *testing.T) {
	reg := tools.NewRegistry()
	client := &scriptedClient{replies: []Message{{Role: "assistant", Content: "should never be reached"}}}
	loop := &Loop{Client: client, Registry: reg}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := loop.Run(ctx, "system", "user")
	if err == nil {
		t.Fatal("expected Run to stop immediately when ctx is already cancelled")
	}
	if client.calls != 0 {
		t.Errorf("expected no model call once ctx is cancelled before Run starts, got %d", client.calls)
	}
}

func TestLoopReportsStateTransitions(t *testing.T) {
	root := t.TempDir()
	q := approval.NewQueue()
	ftools, err := NewTools(root, q)
	if err != nil {
		t.Fatalf("NewTools: %v", err)
	}
	reg := tools.NewRegistry()
	RegisterFileTools(reg, ftools)

	client := &scriptedClient{replies: []Message{
		{Role: "assistant", ToolCalls: []tools.ToolCall{toolCallArgs("write_file", map[string]any{
			"path": "f.txt", "content": "x",
		})}},
		{Role: "assistant", Content: "done"},
	}}
	var states []State
	loop := &Loop{Client: client, Registry: reg, OnState: func(s State) { states = append(states, s) }}

	go approveNextRequest(t, q)
	if _, err := loop.Run(context.Background(), "system", "user"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []State{StateThinking, StateWorking, StateThinking, StateDone}
	if len(states) != len(want) {
		t.Fatalf("states = %v, want %v", states, want)
	}
	for i := range want {
		if states[i] != want[i] {
			t.Errorf("states[%d] = %q, want %q", i, states[i], want[i])
		}
	}
}
