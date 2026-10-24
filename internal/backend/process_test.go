package backend

import (
	"strings"
	"testing"
	"time"

	"github.com/furaxdev/runyx/internal/modelstore"
)

// TestServerArgsPassesFlashAttnAValue guards a real CI failure: a
// current llama-server build rejects a bare "--flash-attn" flag
// ("expected value for argument") because the flag now takes an
// explicit on/off/auto value. Found via internal/backend's new output
// capture in a real CI run after llama-server had started crash-free
// for the first time (past the earlier release-resolution and
// extraction bugs).
func TestServerArgsPassesFlashAttnAValue(t *testing.T) {
	args := serverArgs("/models/foo.gguf", 12345, modelstore.TuningParams{
		CtxSize: 4096, BatchSize: 512, Threads: 4, FlashAttn: true,
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--flash-attn on") {
		t.Errorf("args = %q, want \"--flash-attn on\" (a bare flag is now a parse error)", joined)
	}
}

// TestServerArgsOmitsFlashAttnWhenNotTuned keeps the previous
// behavior for a model tuned without flash attention: no flag at all
// (llama-server defaults to "auto").
func TestServerArgsOmitsFlashAttnWhenNotTuned(t *testing.T) {
	args := serverArgs("/models/foo.gguf", 12345, modelstore.TuningParams{
		CtxSize: 4096, BatchSize: 512, Threads: 4, FlashAttn: false,
	})
	for _, a := range args {
		if a == "--flash-attn" {
			t.Errorf("args = %v, did not expect --flash-attn when tuning.FlashAttn is false", args)
		}
	}
}

// TestRingBufferKeepsOnlyTheTail guards the buffer that captures a
// started llama-server's stdout/stderr for diagnosing startup failures:
// it must cap at maxSize and keep the most recent bytes, not the first
// ones (the useful error is usually near the end of the output).
func TestRingBufferKeepsOnlyTheTail(t *testing.T) {
	rb := newRingBuffer(10)
	rb.Write([]byte("0123456789"))
	rb.Write([]byte("abcde"))
	got := rb.String()
	if len(got) != 10 {
		t.Fatalf("len = %d, want 10", len(got))
	}
	if !strings.HasSuffix("0123456789abcde", got) {
		t.Errorf("String() = %q, want the tail of the written bytes", got)
	}
}

func TestLRUVictimPicksLeastRecentlyUsed(t *testing.T) {
	now := time.Now()
	a := &Instance{ModelName: "a", lastUsed: now.Add(-10 * time.Minute)}
	b := &Instance{ModelName: "b", lastUsed: now.Add(-1 * time.Minute)}
	c := &Instance{ModelName: "c", lastUsed: now}
	victim := lruVictim([]*Instance{b, a, c})
	if victim.ModelName != "a" {
		t.Errorf("victim = %q, want %q (oldest lastUsed)", victim.ModelName, "a")
	}
}

func TestLRUVictimNilOnEmpty(t *testing.T) {
	if v := lruVictim(nil); v != nil {
		t.Errorf("expected nil victim for empty instance list, got %v", v)
	}
}

func TestMakeRoomLockedNoOpWhenUnlimited(t *testing.T) {
	m := NewManager("llama-server", time.Minute, 0)
	if err := m.makeRoomLocked(999999); err != nil {
		t.Errorf("expected no error with unlimited VRAM budget, got %v", err)
	}
}

func TestMakeRoomLockedErrorsWhenNothingToEvict(t *testing.T) {
	m := NewManager("llama-server", time.Minute, 1000)
	// No instances running, and 2000MB doesn't fit in a 1000MB budget.
	if err := m.makeRoomLocked(2000); err == nil {
		t.Error("expected error when model exceeds budget with nothing to evict")
	}
}

func TestInstanceTouchResetsIdle(t *testing.T) {
	inst := &Instance{lastUsed: time.Now().Add(-time.Hour)}
	if inst.IdleFor() < 59*time.Minute {
		t.Fatal("expected instance to appear idle before Touch")
	}
	inst.Touch()
	if inst.IdleFor() > time.Second {
		t.Error("expected IdleFor to be near zero right after Touch")
	}
}
