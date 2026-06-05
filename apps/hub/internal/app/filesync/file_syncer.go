package filesync

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/voice"
)

type fileSyncer struct {
	mu         sync.RWMutex
	configPath string
	dbStore    ConfigStore
}

// NewFileSyncer returns a filesystem-backed Syncer implementation.
func NewFileSyncer(configPath string, dbStore ConfigStore) Syncer {
	return &fileSyncer{
		configPath: configPath,
		dbStore:    dbStore,
	}
}

func (s *fileSyncer) Sync(ctx context.Context) error {
	if strings.TrimSpace(s.configPath) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dbStore == nil {
		return errors.New("filesync: database ConfigStore is not configured")
	}

	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	dbCfg, err := s.dbStore.Config(ctx)
	if err != nil {
		return err
	}

	cfg.Home = dbCfg.Home
	cfg.Display = dbCfg.Display
	cfg.Weather = dbCfg.Weather
	cfg.Voice = dbCfg.Voice
	cfg.Rooms = dbCfg.Rooms
	cfg.Tiles = dbCfg.Tiles
	cfg.Dashboard = dbCfg.Dashboard

	return config.SaveYAML(s.configPath, cfg)
}

func (s *fileSyncer) SyncWith(
	_ context.Context,
	fn func(cfg *config.Config) error,
) error {
	if strings.TrimSpace(s.configPath) == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return err
	}

	if err := fn(&cfg); err != nil {
		return err
	}

	return config.SaveYAML(s.configPath, cfg)
}

// DashboardConfig returns the current dashboard configuration from the YAML file.
func (s *fileSyncer) DashboardConfig(
	_ context.Context,
) (dashboard.DashboardConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return dashboard.DashboardConfig{}, err
	}
	return cfg.Dashboard, nil
}

func (s *fileSyncer) SyncDashboard(
	ctx context.Context,
	dCfg dashboard.DashboardConfig,
) error {
	return s.SyncWith(ctx, func(cfg *config.Config) error {
		cfg.Dashboard = dCfg
		return nil
	})
}

// HomeConfig returns the current home configuration components from the YAML file.
func (s *fileSyncer) HomeConfig(
	_ context.Context,
) (homestate.HomeConfig, any, homestate.WeatherConfig,
	[]homestate.RoomConfig, []homestate.TileConfig, error,
) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return homestate.HomeConfig{}, nil, homestate.WeatherConfig{}, nil, nil, err
	}
	return cfg.Home, cfg.Display, cfg.Weather, cfg.Rooms, cfg.Tiles, nil
}

func (s *fileSyncer) SyncHome(
	ctx context.Context,
	home homestate.HomeConfig,
	display any,
	weather homestate.WeatherConfig,
	rooms []homestate.RoomConfig,
	tiles []homestate.TileConfig,
) error {
	return s.SyncWith(ctx, func(cfg *config.Config) error {
		cfg.Home = home
		if display != nil {
			var disp dashboard.DisplayConfig
			dispBytes, err := json.Marshal(display)
			if err == nil {
				_ = json.Unmarshal(dispBytes, &disp)
				cfg.Display = disp
			}
		}
		cfg.Weather = weather
		cfg.Rooms = rooms
		cfg.Tiles = tiles
		return nil
	})
}

// VoiceConfig returns the current voice configuration from the YAML file.
func (s *fileSyncer) VoiceConfig(
	_ context.Context,
) (voice.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return voice.Config{}, err
	}
	return cfg.Voice, nil
}

func (s *fileSyncer) SyncVoice(
	ctx context.Context,
	vCfg voice.Config,
) error {
	return s.SyncWith(ctx, func(cfg *config.Config) error {
		cfg.Voice = vCfg
		return nil
	})
}

// AgentsConfig returns the current agent configurations from the YAML file.
func (s *fileSyncer) AgentsConfig(
	_ context.Context,
) ([]agents.AgentConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, err := config.LoadConfig(s.configPath)
	if err != nil {
		return nil, err
	}
	return cfg.Agents, nil
}

func (s *fileSyncer) SyncAgents(
	ctx context.Context,
	configs []agents.AgentConfig,
) error {
	return s.SyncWith(ctx, func(cfg *config.Config) error {
		cfg.Agents = configs
		return nil
	})
}
