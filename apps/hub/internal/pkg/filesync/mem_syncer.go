package filesync

import (
	"context"
	"sync"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/service/agents"
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

// Load returns the current config.
func (s *InMemorySyncer) Load(_ context.Context) (config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config, nil
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
