// Package daemon wires config, the model store, hardware detection and the
// backend process manager together behind an HTTP server exposing the
// Ollama- and OpenAI-compatible APIs (see internal/api).
package daemon

import (
	"context"
	"log"
	"time"

	"github.com/furaxdev/runyx/internal/backend"
	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/hwdetect"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// Daemon holds everything the HTTP API needs to serve requests.
type Daemon struct {
	Config  config.Config
	HW      hwdetect.Info
	Store   *modelstore.Store
	Manager *backend.Manager
	Auth    *TokenStore

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

	d := &Daemon{
		Config:  cfg,
		HW:      hw,
		Store:   store,
		Manager: mgr,
		Auth:    auth,
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

// Shutdown stops the eviction loop and every running model backend.
func (d *Daemon) Shutdown() {
	if d.stopEviction != nil {
		d.stopEviction()
	}
	d.Manager.StopAll()
}
