package filesync

import (
	"context"
	"encoding/json"
	"sync"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"
)

// InMemorySyncer is an in-memory Syncer adapter for testing.
type InMemorySyncer struct {
	mu     sync.Mutex
	config config.Config
}

// NewInMemorySyncer returns an in-memory Syncer adapter for testing.
func NewInMemorySyncer(initial config.Config) *InMemorySyncer {
	return &InMemorySyncer{
		config: initial,
	}
}

// Sync is a no-op for in-memory storage.
func (s *InMemorySyncer) Sync(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

// SyncWith applies a mutating closure to the in-memory config.
func (s *InMemorySyncer) SyncWith(
	_ context.Context,
	fn func(cfg *config.Config) error,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return fn(&s.config)
}

// Current returns a snapshot of the in-memory config.
func (s *InMemorySyncer) Current() config.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// DashboardConfig returns the current dashboard configuration.
func (s *InMemorySyncer) DashboardConfig(
	_ context.Context,
) (dashboard.DashboardConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Dashboard, nil
}

// SyncDashboard persists dashboard configuration in memory.
func (s *InMemorySyncer) SyncDashboard(
	_ context.Context,
	dCfg dashboard.DashboardConfig,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Dashboard = dCfg
	return nil
}

// HomeConfig returns home, display, weather, rooms, and tiles.
func (s *InMemorySyncer) HomeConfig(
	_ context.Context,
) (homestate.HomeConfig, any, homestate.WeatherConfig,
	[]homestate.RoomConfig, []homestate.TileConfig, error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Home, s.config.Display, s.config.Weather,
		s.config.Rooms, s.config.Tiles, nil
}

// SyncHome persists home configuration in memory.
func (s *InMemorySyncer) SyncHome(
	_ context.Context,
	home homestate.HomeConfig,
	display any,
	weather homestate.WeatherConfig,
	rooms []homestate.RoomConfig,
	tiles []homestate.TileConfig,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Home = home
	if display != nil {
		var disp dashboard.DisplayConfig
		dispBytes, err := json.Marshal(display)
		if err == nil {
			_ = json.Unmarshal(dispBytes, &disp)
			s.config.Display = disp
		}
	}
	s.config.Weather = weather
	s.config.Rooms = rooms
	s.config.Tiles = tiles
	return nil
}

// VoiceConfig returns the current voice configuration.
func (s *InMemorySyncer) VoiceConfig(
	_ context.Context,
) (voice.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Voice, nil
}

// SyncVoice persists voice configuration in memory.
func (s *InMemorySyncer) SyncVoice(
	_ context.Context,
	vCfg voice.Config,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Voice = vCfg
	return nil
}

// AgentsConfig returns the current agent configurations.
func (s *InMemorySyncer) AgentsConfig(
	_ context.Context,
) ([]agents.AgentConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.Agents, nil
}

// SyncAgents persists agent configurations in memory.
func (s *InMemorySyncer) SyncAgents(
	_ context.Context,
	configs []agents.AgentConfig,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config.Agents = configs
	return nil
}
