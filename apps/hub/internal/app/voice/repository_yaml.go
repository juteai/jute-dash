package voice

import (
	"context"
	"sync"
	"time"
)

// Syncer defines the interface needed for voice config persistence.
type Syncer interface {
	SyncVoice(ctx context.Context, cfg Config) error
	VoiceConfig(ctx context.Context) (Config, error)
}

type YAMLRepository struct {
	mu     sync.RWMutex
	syncer Syncer
}

func NewYAMLRepository(syncer Syncer) *YAMLRepository {
	return &YAMLRepository{
		syncer: syncer,
	}
}

func (y *YAMLRepository) VoiceSettings(ctx context.Context, _ string) (Settings, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.syncer.VoiceConfig(ctx)
	if err != nil {
		return Settings{}, err
	}
	return Settings{
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
	}, nil
}

func (y *YAMLRepository) SetVoiceMuted(
	ctx context.Context,
	_ string,
	muted bool,
) (Settings, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.syncer.VoiceConfig(ctx)
	if err != nil {
		return Settings{}, err
	}
	cfg.MutedByDefault = muted
	if err := y.syncer.SyncVoice(ctx, cfg); err != nil {
		return Settings{}, err
	}
	return Settings{
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
	}, nil
}

func (y *YAMLRepository) CancelVoice(ctx context.Context, _ string) (Settings, error) {
	return y.VoiceSettings(ctx, "")
}

func (y *YAMLRepository) VoiceProviders(_ context.Context) ([]ProviderPack, error) {
	return []ProviderPack{}, nil
}
