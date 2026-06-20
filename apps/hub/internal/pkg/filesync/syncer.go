package filesync

import (
	"context"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/model"
)

// ConfigStore defines the database/runtime abstraction for querying
// Jute's current active configuration.
type ConfigStore interface {
	Config(ctx context.Context) (config.Config, error)
}

// Syncer coordinates transactional configuration persistence.
type Syncer interface {
	// Sync aggregates active settings from the database store and writes
	// them to the persistent configuration file.
	Sync(ctx context.Context) error

	// Load returns the full configuration loaded from the YAML/JSON file.
	Load(ctx context.Context) (config.Config, error)

	// SyncWith allows passing a mutating closure to perform programmatic changes
	// directly on the configuration structure before atomic serialization.
	SyncWith(ctx context.Context, fn func(cfg *config.Config) error) error

	// SyncAgents persists agent configurations.
	SyncAgents(ctx context.Context, configs []model.AgentConfig) error

	// AgentsConfig returns the current agent configurations.
	AgentsConfig(ctx context.Context) ([]model.AgentConfig, error)
}
