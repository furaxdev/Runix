// Package daemon wires config, the model store, hardware detection and the
// backend process manager together behind an HTTP server exposing the
// Ollama- and OpenAI-compatible APIs (see internal/api).
package daemon

import (
	"context"
	"log"
	"os/exec"
	"time"

	"github.com/furaxdev/runyx/internal/agent"
	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/history"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/interpreter"
	"github.com/furaxdev/runyx/internal/modelstore"
	"github.com/furaxdev/runyx/internal/remote"
	"github.com/furaxdev/runyx/internal/vision"
	"github.com/furaxdev/runyx/internal/voice"
)

// Daemon holds everything the HTTP API needs to serve requests.
type Daemon struct {
	Config  config.Config
	HW      hwdetect.Info
	Store   *modelstore.Store
	Manager *backend.Manager
	Auth    *TokenStore
	History *history.Store

	// Transcriber and Vision are optional: nil when whisper.cpp / OCR
	// aren't available, in which case the corresponding API endpoints
	// degrade to a clear error instead of being wired up at all.
	Transcriber *voice.Transcriber
	Vision      *vision.Bridge

	// Agent and Interpreter are nil unless Config.AgentWorkDir /
	// InterpreterWorkDir are explicitly set — both are "off by
	// default, explicit opt-in" per §7.
	Agent       *agent.Agent
	Interpreter *interpreter.Interpreter

	// Synthesizer is nil unless Piper is found on PATH.
	Synthesizer *voice.Synthesizer

	// Remote holds any configured remote-API fallback providers (item
	// 28); empty unless the user added one in config.toml. Never used
	// automatically — see internal/remote's package doc.
	Remote *remote.Router

	stopEviction context.CancelFunc
}

// New constructs a Daemon from config, ensuring directories exist and the
// llama-server backend binary is resolved. backendBinary is the path
// returned by backend.Fetcher.Ensure (resolved by the caller so New stays
// synchronous and testable without network access).
func New(cfg config.Config, backendBinary string) (*Daemon, error) {
	if err := cfg.EnsureDirs(); err != nil {
		return nil, err
	}
	store, err := modelstore.New(cfg.ModelsDir)
	if err != nil {
		return nil, err
	}
	auth, err := NewTokenStore(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	hw := hwdetect.Detect()
	mgr := backend.NewManager(backendBinary, time.Duration(cfg.IdleUnloadSeconds)*time.Second, cfg.VRAMBudgetMB)

	hist, err := history.Open(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	d := &Daemon{
		Config:  cfg,
		HW:      hw,
		Store:   store,
		Manager: mgr,
		Auth:    auth,
		History: hist,
	}

	if vision.Available() {
		d.Vision = &vision.Bridge{OCR: vision.OCR{Languages: "fra+eng"}, CachePath: cfg.DataDir + "/vision_cache.json"}
	}
	if whisperBin, err := exec.LookPath("whisper-cli"); err == nil {
		d.Transcriber = &voice.Transcriber{BinaryPath: whisperBin}
	}
	if piperBin, err := exec.LookPath("piper"); err == nil {
		d.Synthesizer = &voice.Synthesizer{BinaryPath: piperBin}
	}
	d.Remote = &remote.Router{Providers: map[string]remote.Provider{}}

	if cfg.AgentWorkDir != "" {
		// No ApprovalFunc is wired here: the daemon has no synchronous
		// way to prompt a human over HTTP yet (that needs a real
		// pending-approval flow — see docs/BLOCKED.md). Until that
		// lands, every command fails closed (refused) rather than ever
		// defaulting to allowed; approvals can be granted by
		// constructing *agent.Agent directly with a real ApprovalFunc
		// (e.g. from a CLI prompt) instead of through this daemon path.
		d.Agent = agent.New(agent.Policy{WorkDir: cfg.AgentWorkDir}, nil)
		d.Agent.AuditLogDir = cfg.DataDir
	}
	if cfg.InterpreterWorkDir != "" {
		d.Interpreter = &interpreter.Interpreter{WorkDir: cfg.InterpreterWorkDir}
	}

	return d, nil
}

// StartEvictionLoop begins periodically evicting idle models until ctx is
// cancelled or Stop is called.
func (d *Daemon) StartEvictionLoop(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	d.stopEviction = cancel
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if evicted := d.Manager.EvictIdle(); len(evicted) > 0 {
					log.Printf("runyx: evicted idle models: %v", evicted)
				}
			}
		}
	}()
}

// Shutdown stops the eviction loop, every running model backend, and
// closes the history database.
func (d *Daemon) Shutdown() {
	if d.stopEviction != nil {
		d.stopEviction()
	}
	d.Manager.StopAll()
	if d.History != nil {
		d.History.Close()
	}
}
