package main

import (
	"fmt"

	"github.com/furaxdev/runyx/internal/config"
	"github.com/furaxdev/runyx/internal/modelstore"
)

// loadStore loads the config and opens the model store, creating
// directories as needed. Shared by every subcommand that touches
// installed models.
func loadStore() (config.Config, *modelstore.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		return config.Config{}, nil, err
	}
	store, err := modelstore.New(cfg.ModelsDir)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, store, nil
}
