package voice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const DefaultDeviceProfileID = "default-display"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) VoiceSettings(ctx context.Context, deviceProfileID string) (Settings, error) {
	deviceProfileID = strings.TrimSpace(deviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = DefaultDeviceProfileID
	}

	var vs SettingsDB
	if err := r.db.WithContext(ctx).First(&vs, "device_profile_id = ?", deviceProfileID).Error; err != nil {
		return Settings{}, fmt.Errorf("load voice settings: %w", err)
	}

	return Settings{
		DeviceProfileID:         vs.DeviceProfileID,
		Enabled:                 vs.Enabled == 1,
		Muted:                   vs.Muted == 1,
		WakeWordModelID:         vs.WakeWordModelID,
		STTProviderID:           vs.STTProviderID,
		TTSProviderID:           vs.TTSProviderID,
		STTModelID:              vs.STTModelID,
		TTSModelID:              vs.TTSModelID,
		TTSVoiceID:              vs.TTSVoiceID,
		PreferredAgentID:        vs.PreferredAgentID,
		CloudOptIn:              vs.CloudOptIn == 1,
		CommandProvidersEnabled: vs.CommandProvidersEnabled == 1,
		SensitiveOutputPolicy:   vs.SensitiveOutputPolicy,
		FollowupWindowSeconds:   vs.FollowupWindowSeconds,
		MicrophoneProfile:       vs.MicrophoneProfile,
		UpdatedAt:               vs.UpdatedAt,
	}, nil
}

func (r *Repository) SetVoiceMuted(ctx context.Context, deviceProfileID string, muted bool) (Settings, error) {
	deviceProfileID = strings.TrimSpace(deviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = DefaultDeviceProfileID
	}
	now := nowUTC()

	err := r.db.WithContext(ctx).
		Model(&SettingsDB{}).
		Where("device_profile_id = ?", deviceProfileID).
		Updates(map[string]any{
			"muted":                 boolToInt(muted),
			"last_state_updated_at": now,
			"updated_at":            now,
		}).
		Error
	if err != nil {
		return Settings{}, fmt.Errorf("update voice mute state: %w", err)
	}
	return r.VoiceSettings(ctx, deviceProfileID)
}

func (r *Repository) CancelVoice(ctx context.Context, deviceProfileID string) (Settings, error) {
	deviceProfileID = strings.TrimSpace(deviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = DefaultDeviceProfileID
	}
	now := nowUTC()

	err := r.db.WithContext(ctx).
		Model(&SettingsDB{}).
		Where("device_profile_id = ?", deviceProfileID).
		Updates(map[string]any{
			"last_state_updated_at": now,
			"updated_at":            now,
		}).
		Error
	if err != nil {
		return Settings{}, fmt.Errorf("cancel voice state: %w", err)
	}
	return r.VoiceSettings(ctx, deviceProfileID)
}

func (r *Repository) VoiceProviders(ctx context.Context) ([]ProviderPack, error) {
	var vpps []ProviderPackDB
	if err := r.db.WithContext(ctx).Order("name, id").Find(&vpps).Error; err != nil {
		return nil, fmt.Errorf("load voice providers: %w", err)
	}

	providers := make([]ProviderPack, len(vpps))
	for i, v := range vpps {
		providers[i] = ProviderPack{
			ID:            v.ID,
			Name:          v.Name,
			Version:       v.Version,
			Kind:          v.Kind,
			TransportType: v.TransportType,
			HealthStatus:  v.HealthStatus,
			UpdatedAt:     v.UpdatedAt,
		}
	}
	return providers, nil
}

func (r *Repository) EnsureDefaultVoiceSettings(ctx context.Context, voice Config) error {
	now := nowUTC()
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&SettingsDB{}).
		Where("device_profile_id = ?", DefaultDeviceProfileID).
		Count(&count).
		Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	vDB := SettingsDB{
		DeviceProfileID:         DefaultDeviceProfileID,
		Enabled:                 boolToInt(voice.Enabled),
		Muted:                   boolToInt(voice.MutedByDefault),
		WakeWordModelID:         voice.WakeWordModelID,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		PreferredAgentID:        voice.PreferredAgentID,
		CloudOptIn:              boolToInt(voice.CloudOptIn),
		CommandProvidersEnabled: boolToInt(voice.CommandProvidersEnabled),
		SensitiveOutputPolicy:   voice.SensitiveOutputPolicy,
		FollowupWindowSeconds:   voice.FollowupWindowSeconds,
		MicrophoneProfile:       voice.MicrophoneProfile,
		UpdatedAt:               now,
	}
	return r.db.WithContext(ctx).Create(&vDB).Error
}

// Helpers

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
