package filesync

import (
	"context"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"
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

	// SyncWith allows passing a mutating closure to perform programmatic changes
	// directly on the configuration structure before atomic serialization.
	SyncWith(ctx context.Context, fn func(cfg *config.Config) error) error

	// SyncDashboard persists dashboard widget configuration.
	SyncDashboard(ctx context.Context, cfg dashboard.DashboardConfig) error

	// DashboardConfig returns the current dashboard configuration.
	DashboardConfig(ctx context.Context) (dashboard.DashboardConfig, error)

	// SyncHome persists home, display, weather, rooms, and tiles configuration.
	SyncHome(
		ctx context.Context,
		home homestate.HomeConfig,
		display any,
		weather homestate.WeatherConfig,
		rooms []homestate.RoomConfig,
		tiles []homestate.TileConfig,
	) error

	// HomeConfig returns the current home configuration components.
	HomeConfig(ctx context.Context) (
		homestate.HomeConfig, any, homestate.WeatherConfig,
		[]homestate.RoomConfig, []homestate.TileConfig, error,
	)

	// SyncVoice persists voice configuration.
	SyncVoice(ctx context.Context, cfg voice.Config) error

	// VoiceConfig returns the current voice configuration.
	VoiceConfig(ctx context.Context) (voice.Config, error)

	// SyncAgents persists agent configurations.
	SyncAgents(ctx context.Context, configs []agents.AgentConfig) error

	// AgentsConfig returns the current agent configurations.
	AgentsConfig(ctx context.Context) ([]agents.AgentConfig, error)
}
