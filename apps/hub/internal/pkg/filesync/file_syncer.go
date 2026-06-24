package filesync

import (
	"context"
	"errors"
	"strings"
	"sync"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/model"
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

// Load returns the full configuration loaded from the YAML/JSON file.
func (s *fileSyncer) Load(_ context.Context) (config.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return config.LoadConfig(s.configPath)
}

// AgentsConfig returns the current agent configurations from the YAML file.
func (s *fileSyncer) AgentsConfig(
	_ context.Context,
) ([]model.AgentConfig, error) {
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
	configs []model.AgentConfig,
) error {
	return s.SyncWith(ctx, func(cfg *config.Config) error {
		cfg.Agents = configs
		return nil
	})
}
