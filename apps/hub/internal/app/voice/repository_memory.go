package voice

import (
	"context"
	"fmt"
	"strings"
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
			WakeSensitivity: 0.5,
			TTSLocale:       "en",
			TTSSpeed:        1,
			TTSVolume:       1,
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
			WakeWordPhrase:          cfg.WakeWordPhrase,
			WakeSensitivity:         cfg.WakeSensitivity,
			STTProviderID:           cfg.STTProviderID,
			TTSProviderID:           cfg.TTSProviderID,
			STTModelID:              cfg.STTModelID,
			TTSModelID:              cfg.TTSModelID,
			TTSVoiceID:              cfg.TTSVoiceID,
			TTSEnabled:              cfg.TTSEnabled,
			TTSLocale:               cfg.TTSLocale,
			TTSSpeed:                cfg.TTSSpeed,
			TTSVolume:               cfg.TTSVolume,
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

func (m *MemoryRepository) SaveVoiceSettings(_ context.Context, req SettingsUpdateRequest) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := applySettingsUpdate(m.settings, req)
	if problems := validateSettings(next); len(problems) > 0 {
		return Settings{}, fmt.Errorf("invalid voice settings: %s", strings.Join(problems, "; "))
	}
	next.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.settings = next
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

func (m *MemoryRepository) TTSVoices(_ context.Context, providerID, _ string) (TTSVoicesResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if providerID == "" {
		providerID = m.settings.TTSProviderID
	}
	return TTSVoicesResponse{
		ProviderID:      providerID,
		HealthStatus:    "disabled",
		SetupStatus:     "disabled",
		SelectedVoiceID: m.settings.TTSVoiceID,
		SelectedModelID: m.settings.TTSModelID,
		Locale:          m.settings.TTSLocale,
		Speed:           m.settings.TTSSpeed,
		Volume:          m.settings.TTSVolume,
		Voices:          []TTSVoice{},
	}, nil
}
