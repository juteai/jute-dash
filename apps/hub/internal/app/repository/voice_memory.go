package repository

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type MemoryVoiceRepository struct {
	mu       sync.RWMutex
	settings Settings
}

func NewMemoryVoiceRepository() *MemoryVoiceRepository {
	return &MemoryVoiceRepository{
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

func NewMemoryVoiceRepositoryFromConfig(cfg Config) *MemoryVoiceRepository {
	return &MemoryVoiceRepository{
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

func (m *MemoryVoiceRepository) VoiceSettings(_ context.Context, _ string) (Settings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings, nil
}

func (m *MemoryVoiceRepository) SaveVoiceSettings(_ context.Context, req SettingsUpdateRequest) (Settings, error) {
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

func (m *MemoryVoiceRepository) SetVoiceMuted(_ context.Context, _ string, muted bool) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.Muted = muted
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.settings, nil
}

func (m *MemoryVoiceRepository) CancelVoice(_ context.Context, _ string) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return m.settings, nil
}

func (m *MemoryVoiceRepository) VoiceProviders(_ context.Context) ([]ProviderPack, error) {
	return []ProviderPack{}, nil
}

func (m *MemoryVoiceRepository) TTSVoices(_ context.Context, providerID, _ string) (TTSVoicesResponse, error) {
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
