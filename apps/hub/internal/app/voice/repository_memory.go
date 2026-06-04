package voice

import (
	"context"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu       sync.RWMutex
	settings Settings
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		settings: Settings{
			DeviceProfileID: DefaultDeviceProfileID,
			Muted:           true,
			UpdatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
}

func NewMemoryRepositoryFromConfig(cfg Config) *MemoryRepository {
	return &MemoryRepository{
		settings: Settings{
			DeviceProfileID:         DefaultDeviceProfileID,
			Enabled:                 cfg.Enabled,
			Muted:                   cfg.MutedByDefault,
			WakeWordModelID:         cfg.WakeWordModelID,
			STTProviderID:           cfg.STTProviderID,
			TTSProviderID:           cfg.TTSProviderID,
			STTModelID:              cfg.STTModelID,
			TTSModelID:              cfg.TTSModelID,
			TTSVoiceID:              cfg.TTSVoiceID,
			PreferredAgentID:        cfg.PreferredAgentID,
			CloudOptIn:              cfg.CloudOptIn,
			CommandProvidersEnabled: cfg.CommandProvidersEnabled,
			SensitiveOutputPolicy:   cfg.SensitiveOutputPolicy,
			FollowupWindowSeconds:   cfg.FollowupWindowSeconds,
			MicrophoneProfile:       cfg.MicrophoneProfile,
			UpdatedAt:               time.Now().UTC().Format(time.RFC3339Nano),
		},
	}
}

func (m *MemoryRepository) VoiceSettings(_ context.Context, _ string) (Settings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings, nil
}

func (m *MemoryRepository) SetVoiceMuted(_ context.Context, _ string, muted bool) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.Muted = muted
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.settings, nil
}

func (m *MemoryRepository) CancelVoice(_ context.Context, _ string) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.settings, nil
}

func (m *MemoryRepository) VoiceProviders(_ context.Context) ([]ProviderPack, error) {
	return []ProviderPack{}, nil
}
