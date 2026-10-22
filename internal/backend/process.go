package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/furaxdev/runyx/internal/modelstore"
)

// Instance is one running llama-server subprocess backing a loaded model.
type Instance struct {
	ModelName string
	Port      int
	BaseURL   string

	cmd      *exec.Cmd
	lastUsed time.Time
	estMB    int
	mu       sync.Mutex
}

// Touch records that the instance was just used, for idle/LRU accounting.
func (i *Instance) Touch() {
	i.mu.Lock()
	i.lastUsed = time.Now()
	i.mu.Unlock()
}

// IdleFor returns how long the instance has gone unused.
func (i *Instance) IdleFor() time.Duration {
	i.mu.Lock()
	defer i.mu.Unlock()
	return time.Since(i.lastUsed)
}

// Manager supervises llama-server subprocesses, one per currently loaded
// model, enforcing an idle timeout and a VRAM budget via LRU eviction.
type Manager struct {
	BinaryPath    string
	IdleTimeout   time.Duration
	VRAMBudgetMB  int // 0 = unlimited
	PortRangeFrom int

	mu        sync.Mutex
	instances map[string]*Instance
	startMu   map[string]*sync.Mutex // per-model lock serializing concurrent cold starts
}

// NewManager returns a Manager that launches BinaryPath for each model.
func NewManager(binaryPath string, idleTimeout time.Duration, vramBudgetMB int) *Manager {
	return &Manager{
		BinaryPath:    binaryPath,
		IdleTimeout:   idleTimeout,
		VRAMBudgetMB:  vramBudgetMB,
		PortRangeFrom: 11540,
		instances:     make(map[string]*Instance),
		startMu:       make(map[string]*sync.Mutex),
	}
}

// startLock returns the per-model mutex used to serialize concurrent
// Start calls for the same model name, creating it on first use.
func (m *Manager) startLock(name string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.startMu[name]
	if !ok {
		l = &sync.Mutex{}
		m.startMu[name] = l
	}
	return l
}

// Get returns the running instance for a model name, if any.
func (m *Manager) Get(modelName string) (*Instance, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	inst, ok := m.instances[modelName]
	return inst, ok
}

// List returns all currently loaded instances.
func (m *Manager) List() []*Instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		out = append(out, inst)
	}
	return out
}

// Start launches llama-server for mdl with the given tuning parameters,
// evicting idle/LRU instances first if the VRAM budget requires it. It
// blocks until the server's /health endpoint responds or ctx is done.
func (m *Manager) Start(ctx context.Context, mdl modelstore.Model, tuning modelstore.TuningParams) (*Instance, error) {
	// Serialize concurrent cold starts for the SAME model name: without
	// this, two concurrent first requests could both see an empty slot,
	// both launch a subprocess, and the later map write would orphan
	// the other's process (never tracked, never stopped, bypassing the
	// VRAM budget too). Different models still start concurrently since
	// each gets its own lock.
	lock := m.startLock(mdl.Name)
	lock.Lock()
	defer lock.Unlock()

	m.mu.Lock()
	if inst, ok := m.instances[mdl.Name]; ok {
		m.mu.Unlock()
		inst.Touch()
		return inst, nil
	}
	m.mu.Unlock()

	estMB := int(mdl.SizeBytes / (1024 * 1024))
	if err := m.makeRoomLocked(estMB); err != nil {
		return nil, err
	}

	port, err := freePort(m.PortRangeFrom)
	if err != nil {
		return nil, fmt.Errorf("find free port: %w", err)
	}

	args := []string{
		"--model", mdl.File,
		"--port", strconv.Itoa(port),
		"--host", "127.0.0.1",
		"--ctx-size", strconv.Itoa(tuning.CtxSize),
		"--batch-size", strconv.Itoa(tuning.BatchSize),
		"--threads", strconv.Itoa(tuning.Threads),
	}
	if tuning.NGPULayers != 0 {
		args = append(args, "--n-gpu-layers", strconv.Itoa(tuning.NGPULayers))
	}
	if tuning.FlashAttn {
		args = append(args, "--flash-attn")
	}

	cmd := exec.CommandContext(context.Background(), m.BinaryPath, args...) // outlives request ctx
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start llama-server: %w", err)
	}

	inst := &Instance{
		ModelName: mdl.Name,
		Port:      port,
		BaseURL:   fmt.Sprintf("http://127.0.0.1:%d", port),
		cmd:       cmd,
		lastUsed:  time.Now(),
		estMB:     estMB,
	}

	if err := waitHealthy(ctx, inst.BaseURL); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // reap; Kill alone leaves a zombie until the daemon exits
		return nil, fmt.Errorf("llama-server did not become healthy: %w", err)
	}

	m.mu.Lock()
	m.instances[mdl.Name] = inst
	m.mu.Unlock()
	return inst, nil
}

// Stop terminates and removes the instance for modelName, if running.
func (m *Manager) Stop(modelName string) error {
	m.mu.Lock()
	inst, ok := m.instances[modelName]
	if ok {
		delete(m.instances, modelName)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return stopInstance(inst)
}

// StopAll terminates every running instance, e.g. on daemon shutdown.
func (m *Manager) StopAll() {
	m.mu.Lock()
	instances := m.instances
	m.instances = make(map[string]*Instance)
	m.mu.Unlock()
	for _, inst := range instances {
		_ = stopInstance(inst)
	}
}

func stopInstance(inst *Instance) error {
	if inst.cmd == nil || inst.cmd.Process == nil {
		return nil
	}
	err := inst.cmd.Process.Kill()
	_ = inst.cmd.Wait() // reap; Kill alone leaves a zombie until the daemon exits
	return err
}

// EvictIdle stops any instance that has been idle longer than
// m.IdleTimeout. Intended to be called periodically by the daemon.
func (m *Manager) EvictIdle() []string {
	if m.IdleTimeout <= 0 {
		return nil
	}
	var evicted []string
	for _, inst := range m.List() {
		if inst.IdleFor() >= m.IdleTimeout {
			if err := m.Stop(inst.ModelName); err == nil {
				evicted = append(evicted, inst.ModelName)
			}
		}
	}
	return evicted
}

// makeRoomLocked evicts least-recently-used instances until adding a model
// of size estMB would fit within the VRAM budget. No-op if VRAMBudgetMB is
// 0 (unlimited) or estMB already fits.
func (m *Manager) makeRoomLocked(estMB int) error {
	if m.VRAMBudgetMB <= 0 {
		return nil
	}
	for {
		instances := m.List()
		used := 0
		for _, inst := range instances {
			used += inst.estMB
		}
		if used+estMB <= m.VRAMBudgetMB {
			return nil
		}
		victim := lruVictim(instances)
		if victim == nil {
			return fmt.Errorf("model needs ~%d MB, exceeds VRAM budget of %d MB with nothing left to evict", estMB, m.VRAMBudgetMB)
		}
		if err := m.Stop(victim.ModelName); err != nil {
			return err
		}
	}
}

// lruVictim returns the least-recently-used instance, or nil if none.
func lruVictim(instances []*Instance) *Instance {
	var victim *Instance
	for _, inst := range instances {
		if victim == nil || inst.IdleFor() > victim.IdleFor() {
			victim = inst
		}
	}
	return victim
}

func freePort(from int) (int, error) {
	for p := from; p < from+1000; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port found starting at %d", from)
}

func waitHealthy(ctx context.Context, baseURL string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s/health", baseURL)
}
