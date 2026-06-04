package voice

import (
	"context"
	"errors"
	"sync"
	"time"
)

type YAMLRepository struct {
	mu         sync.RWMutex
	configPath string
	loadFn     func(path string) (Config, error)
	saveFn     func(path string, cfg Config) error
}

func NewYAMLRepository(
	configPath string,
	loadFn func(path string) (Config, error),
	saveFn func(path string, cfg Config) error,
) *YAMLRepository {
	return &YAMLRepository{
		configPath: configPath,
		loadFn:     loadFn,
		saveFn:     saveFn,
	}
}

func (y *YAMLRepository) load() (Config, error) {
	if y.configPath == "" {
		return Config{}, errors.New("config path is empty")
	}
	return y.loadFn(y.configPath)
}

func (y *YAMLRepository) save(cfg Config) error {
	if y.configPath == "" {
		return errors.New("cannot save: config path is empty")
	}
	return y.saveFn(y.configPath, cfg)
}

func (y *YAMLRepository) VoiceSettings(_ context.Context, _ string) (Settings, error) {
	y.mu.RLock()
	defer y.mu.RUnlock()
	cfg, err := y.load()
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
	_ context.Context,
	_ string,
	muted bool,
) (Settings, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	cfg, err := y.load()
	if err != nil {
		return Settings{}, err
	}
	cfg.MutedByDefault = muted
	if err := y.save(cfg); err != nil {
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
