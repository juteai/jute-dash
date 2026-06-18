package voice

import (
	"context"
	"errors"
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
		WakeWordPhrase:          vs.WakeWordPhrase,
		WakeSensitivity:         vs.WakeSensitivity,
		STTProviderID:           vs.STTProviderID,
		TTSProviderID:           vs.TTSProviderID,
		STTModelID:              vs.STTModelID,
		TTSModelID:              vs.TTSModelID,
		TTSVoiceID:              vs.TTSVoiceID,
		TTSEnabled:              vs.TTSEnabled == 1,
		TTSLocale:               vs.TTSLocale,
		TTSSpeed:                vs.TTSSpeed,
		TTSVolume:               vs.TTSVolume,
		PreferredAgentID:        vs.PreferredAgentID,
		CloudOptIn:              vs.CloudOptIn == 1,
		CommandProvidersEnabled: vs.CommandProvidersEnabled == 1,
		SensitiveOutputPolicy:   vs.SensitiveOutputPolicy,
		FollowupWindowSeconds:   vs.FollowupWindowSeconds,
		MicrophoneProfile:       vs.MicrophoneProfile,
		UpdatedAt:               vs.UpdatedAt,
	}, nil
}

func (r *Repository) SaveVoiceSettings(ctx context.Context, req SettingsUpdateRequest) (Settings, error) {
	deviceProfileID := strings.TrimSpace(req.DeviceProfileID)
	if deviceProfileID == "" {
		deviceProfileID = DefaultDeviceProfileID
	}

	current, err := r.VoiceSettings(ctx, deviceProfileID)
	if err != nil {
		return Settings{}, err
	}
	next := applySettingsUpdate(current, req)
	if problems := validateSettings(next); len(problems) > 0 {
		return Settings{}, fmt.Errorf("invalid voice settings: %s", strings.Join(problems, "; "))
	}

	now := nowUTC()
	err = r.db.WithContext(ctx).
		Model(&SettingsDB{}).
		Where("device_profile_id = ?", deviceProfileID).
		Updates(map[string]any{
			"enabled":                   boolToInt(next.Enabled),
			"wake_word_model_id":        next.WakeWordModelID,
			"wake_word_phrase":          next.WakeWordPhrase,
			"wake_sensitivity":          next.WakeSensitivity,
			"stt_provider_id":           next.STTProviderID,
			"tts_provider_id":           next.TTSProviderID,
			"stt_model_id":              next.STTModelID,
			"tts_model_id":              next.TTSModelID,
			"tts_voice_id":              next.TTSVoiceID,
			"tts_enabled":               boolToInt(next.TTSEnabled),
			"tts_locale":                next.TTSLocale,
			"tts_speed":                 next.TTSSpeed,
			"tts_volume":                next.TTSVolume,
			"preferred_agent_id":        next.PreferredAgentID,
			"cloud_opt_in":              boolToInt(next.CloudOptIn),
			"command_providers_enabled": boolToInt(next.CommandProvidersEnabled),
			"sensitive_output_policy":   next.SensitiveOutputPolicy,
			"followup_window_seconds":   next.FollowupWindowSeconds,
			"microphone_profile":        next.MicrophoneProfile,
			"updated_at":                now,
		}).
		Error
	if err != nil {
		return Settings{}, fmt.Errorf("save voice settings: %w", err)
	}
	return r.VoiceSettings(ctx, deviceProfileID)
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
	settings, err := r.VoiceSettings(ctx, DefaultDeviceProfileID)
	if err != nil {
		return nil, err
	}
	var vpps []ProviderPackDB
	if err := r.db.WithContext(ctx).Order("name, id").Find(&vpps).Error; err != nil {
		return nil, fmt.Errorf("load voice providers: %w", err)
	}

	providers := make([]ProviderPack, len(vpps))
	for i, v := range vpps {
		providers[i] = providerPackFromDB(v, settings.CloudOptIn)
	}
	return providers, nil
}

//nolint:nilnil // nil provider with nil error means voice has no usable wake provider.
func (r *Repository) ActiveWakeProvider(ctx context.Context, deviceProfileID, deviceID string) (WakeProvider, error) {
	_ = deviceID
	settings, err := r.VoiceSettings(ctx, deviceProfileID)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, nil
	}

	var providers []ProviderPackDB
	if err := r.db.WithContext(ctx).
		Where("kind = ?", ProviderKindWakeWord).
		Order("name, id").
		Find(&providers).
		Error; err != nil {
		return nil, fmt.Errorf("load active wake provider: %w", err)
	}

	for _, provider := range providers {
		if provider.HealthStatus != "available" && provider.HealthStatus != "degraded" {
			continue
		}
		manifest, err := DecodeProviderManifest(provider.ManifestJSON)
		if err != nil || len(ValidateProviderManifest(manifest)) > 0 {
			continue
		}
		if manifest.Kind != ProviderKindWakeWord {
			continue
		}
		if !manifest.Capabilities.Offline || missingRequiredCredential(manifest) {
			continue
		}
		if manifest.Transport.Type != "command" || !settings.CommandProvidersEnabled ||
			strings.TrimSpace(manifest.Transport.Command) == "" {
			continue
		}

		modelID := selectedWakeModelID(settings.WakeWordModelID, manifest.WakeWord)
		if modelID == "" {
			continue
		}
		return CommandWakeProvider{
			ProviderID: provider.ID,
			Command:    manifest.Transport.Command,
			Args:       append([]string(nil), manifest.Transport.Args...),
			ModelID:    modelID,
		}, nil
	}
	return nil, nil
}

//nolint:nilnil,nilerr // nil provider with nil error means voice has no usable STT provider.
func (r *Repository) ActiveSTTProvider(ctx context.Context, deviceProfileID string) (STTProvider, error) {
	settings, err := r.VoiceSettings(ctx, deviceProfileID)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled || strings.TrimSpace(settings.STTProviderID) == "" {
		return nil, nil
	}

	var provider ProviderPackDB
	if err := r.db.WithContext(ctx).First(&provider, "id = ?", settings.STTProviderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load active STT provider: %w", err)
	}
	if provider.HealthStatus != "available" && provider.HealthStatus != "degraded" {
		return nil, nil
	}

	manifest, err := DecodeProviderManifest(provider.ManifestJSON)
	if err != nil || len(ValidateProviderManifest(manifest)) > 0 {
		return nil, nil
	}
	if manifest.Kind != ProviderKindSTT && manifest.Kind != ProviderKindSTTTTS {
		return nil, nil
	}
	if !manifest.Capabilities.Offline || missingRequiredCredential(manifest) {
		return nil, nil
	}
	if manifest.Transport.Type == "command" {
		if !settings.CommandProvidersEnabled || strings.TrimSpace(manifest.Transport.Command) == "" {
			return nil, nil
		}
		return CommandSTTProvider{
			ProviderID: provider.ID,
			Command:    manifest.Transport.Command,
			Args:       append([]string(nil), manifest.Transport.Args...),
			ModelID:    strings.TrimSpace(settings.STTModelID),
			Language:   firstLanguage(manifest.Capabilities.Languages),
		}, nil
	}
	return nil, nil
}

func (r *Repository) TTSVoices(ctx context.Context, providerID, deviceProfileID string) (TTSVoicesResponse, error) {
	settings, settingsErr := r.VoiceSettings(ctx, deviceProfileID)
	if settingsErr != nil {
		return TTSVoicesResponse{}, settingsErr
	}
	providerID = strings.TrimSpace(providerID)
	if providerID == "" {
		providerID = settings.TTSProviderID
	}
	response := TTSVoicesResponse{
		ProviderID:      providerID,
		HealthStatus:    "disabled",
		SetupStatus:     "disabled",
		SelectedVoiceID: settings.TTSVoiceID,
		SelectedModelID: settings.TTSModelID,
		Locale:          settings.TTSLocale,
		Speed:           settings.TTSSpeed,
		Volume:          settings.TTSVolume,
		Voices:          []TTSVoice{},
	}
	if providerID == "" {
		return response, nil
	}

	var provider ProviderPackDB
	if r.db.WithContext(ctx).First(&provider, "id = ?", providerID).Error != nil {
		response.HealthStatus = "missing"
		response.SetupStatus = "missing"
		return response, nil //nolint:nilerr // missing voice providers are represented in response status.
	}
	response.ProviderName = provider.Name
	response.HealthStatus = provider.HealthStatus

	manifest, manifestValid := decodeValidProviderManifest(provider.ManifestJSON)
	if !manifestValid {
		response.SetupStatus = "misconfigured"
		return response, nil
	}
	if manifest.Kind != ProviderKindTTS && manifest.Kind != ProviderKindSTTTTS {
		response.SetupStatus = "disabled"
		return response, nil
	}
	response.CloudProvider = !manifest.Capabilities.Offline
	if response.CloudProvider && !settings.CloudOptIn {
		response.SetupStatus = "disabled"
		response.HealthStatus = "disabled"
		return response, nil
	}
	if missingRequiredCredential(manifest) {
		response.SetupStatus = "misconfigured"
		response.HealthStatus = "misconfigured"
		return response, nil
	}
	if provider.HealthStatus == "disabled" {
		response.SetupStatus = "disabled"
		return response, nil
	}
	if provider.HealthStatus == "misconfigured" ||
		provider.HealthStatus == "offline" ||
		provider.HealthStatus == "degraded" ||
		provider.HealthStatus == "available" {
		response.SetupStatus = provider.HealthStatus
	} else {
		response.SetupStatus = "misconfigured"
	}
	response.SelectedVoiceID = ttsSelectedVoiceID(manifest, response.SelectedVoiceID)
	if response.SelectedModelID == "" {
		response.SelectedModelID = manifest.TTS.DefaultModelID
	}
	if response.SetupStatus == "available" || response.SetupStatus == "degraded" {
		response.Voices = ttsVoicesFromManifest(manifest)
	}
	return response, nil
}

func decodeValidProviderManifest(raw string) (ProviderManifest, bool) {
	manifest, err := DecodeProviderManifest(raw)
	return manifest, err == nil && len(ValidateProviderManifest(manifest)) == 0
}

//nolint:nilnil,nilerr // nil provider with nil error means voice has no usable TTS provider.
func (r *Repository) ActiveTTSProvider(ctx context.Context, deviceProfileID string) (TTSProvider, error) {
	settings, err := r.VoiceSettings(ctx, deviceProfileID)
	if err != nil {
		return nil, err
	}
	if !settings.TTSEnabled || strings.TrimSpace(settings.TTSProviderID) == "" {
		return nil, nil
	}

	var provider ProviderPackDB
	if err := r.db.WithContext(ctx).First(&provider, "id = ?", settings.TTSProviderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load active TTS provider: %w", err)
	}
	if provider.HealthStatus != "available" && provider.HealthStatus != "degraded" {
		return nil, nil
	}

	manifest, err := DecodeProviderManifest(provider.ManifestJSON)
	if err != nil || len(ValidateProviderManifest(manifest)) > 0 {
		return nil, nil
	}
	if manifest.Kind != ProviderKindTTS && manifest.Kind != ProviderKindSTTTTS {
		return nil, nil
	}
	if !manifest.Capabilities.Offline || missingRequiredCredential(manifest) {
		return nil, nil
	}
	voiceID := ttsSelectedVoiceID(manifest, settings.TTSVoiceID)
	locale := strings.TrimSpace(settings.TTSLocale)
	if locale == "" {
		locale = ttsVoiceLocale(manifest, voiceID)
	}
	modelID := strings.TrimSpace(settings.TTSModelID)
	if modelID == "" {
		modelID = manifest.TTS.DefaultModelID
	}
	switch manifest.Transport.Type {
	case "command":
		if !settings.CommandProvidersEnabled || strings.TrimSpace(manifest.Transport.Command) == "" {
			return nil, nil
		}
		return CommandTTSProvider{
			ProviderID: provider.ID,
			Command:    manifest.Transport.Command,
			Args:       append([]string(nil), manifest.Transport.Args...),
			ModelID:    modelID,
			VoiceID:    voiceID,
			Locale:     locale,
		}, nil
	default:
		return nil, nil
	}
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
		WakeWordPhrase:          voice.WakeWordPhrase,
		WakeSensitivity:         voice.WakeSensitivity,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		TTSEnabled:              boolToInt(voice.TTSEnabled),
		TTSLocale:               voice.TTSLocale,
		TTSSpeed:                voice.TTSSpeed,
		TTSVolume:               voice.TTSVolume,
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

func providerPackFromDB(v ProviderPackDB, cloudOptIn bool) ProviderPack {
	provider := ProviderPack{
		ID:               v.ID,
		Name:             v.Name,
		Version:          v.Version,
		Kind:             v.Kind,
		TransportType:    v.TransportType,
		HealthStatus:     v.HealthStatus,
		LastActivationAt: v.LastActivationAt,
		LastError:        sanitizeText(v.LastError),
		UpdatedAt:        v.UpdatedAt,
	}
	if strings.TrimSpace(v.ManifestJSON) == "" {
		return provider
	}
	manifest, err := DecodeProviderManifest(v.ManifestJSON)
	if err != nil || len(ValidateProviderManifest(manifest)) > 0 {
		return provider
	}
	provider.Capabilities = manifest.Capabilities
	provider.WakeWord = wakeWordSummary(manifest)
	if !manifest.Capabilities.Offline && !cloudOptIn {
		provider.HealthStatus = "disabled"
		provider.LastError = ""
	}
	return provider
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func selectedWakeModelID(configured string, wake WakeWordManifest) string {
	configured = strings.TrimSpace(configured)
	if configured != "" && wakeModelDeclared(configured, wake.Models) {
		return configured
	}
	defaultModelID := strings.TrimSpace(wake.DefaultModelID)
	if configured == "" && wakeModelDeclared(defaultModelID, wake.Models) {
		return defaultModelID
	}
	return ""
}

func wakeModelDeclared(modelID string, models []WakeWordModelManifest) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	for _, model := range models {
		if strings.TrimSpace(model.ID) == modelID {
			return true
		}
	}
	return false
}

func applySettingsUpdate(current Settings, req SettingsUpdateRequest) Settings {
	next := current
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.WakeWordModelID != nil {
		next.WakeWordModelID = strings.TrimSpace(*req.WakeWordModelID)
	}
	if req.WakeWordPhrase != nil {
		next.WakeWordPhrase = strings.TrimSpace(*req.WakeWordPhrase)
	}
	if req.WakeSensitivity != nil {
		next.WakeSensitivity = *req.WakeSensitivity
	}
	if req.STTProviderID != nil {
		next.STTProviderID = strings.TrimSpace(*req.STTProviderID)
	}
	if req.TTSProviderID != nil {
		next.TTSProviderID = strings.TrimSpace(*req.TTSProviderID)
	}
	if req.STTModelID != nil {
		next.STTModelID = strings.TrimSpace(*req.STTModelID)
	}
	if req.TTSModelID != nil {
		next.TTSModelID = strings.TrimSpace(*req.TTSModelID)
	}
	if req.TTSVoiceID != nil {
		next.TTSVoiceID = strings.TrimSpace(*req.TTSVoiceID)
	}
	if req.TTSEnabled != nil {
		next.TTSEnabled = *req.TTSEnabled
	}
	if req.TTSLocale != nil {
		next.TTSLocale = strings.TrimSpace(*req.TTSLocale)
	}
	if req.TTSSpeed != nil {
		next.TTSSpeed = *req.TTSSpeed
	}
	if req.TTSVolume != nil {
		next.TTSVolume = *req.TTSVolume
	}
	if req.PreferredAgentID != nil {
		next.PreferredAgentID = strings.TrimSpace(*req.PreferredAgentID)
	}
	if req.CloudOptIn != nil {
		next.CloudOptIn = *req.CloudOptIn
	}
	if req.CommandProvidersEnabled != nil {
		next.CommandProvidersEnabled = *req.CommandProvidersEnabled
	}
	if req.SensitiveOutputPolicy != nil {
		next.SensitiveOutputPolicy = strings.TrimSpace(*req.SensitiveOutputPolicy)
	}
	if req.FollowupWindowSeconds != nil {
		next.FollowupWindowSeconds = *req.FollowupWindowSeconds
	}
	if req.MicrophoneProfile != nil {
		next.MicrophoneProfile = strings.TrimSpace(*req.MicrophoneProfile)
	}
	defaults := DefaultConfig()
	if strings.TrimSpace(next.SensitiveOutputPolicy) == "" {
		next.SensitiveOutputPolicy = defaults.SensitiveOutputPolicy
	}
	if next.FollowupWindowSeconds == 0 {
		next.FollowupWindowSeconds = defaults.FollowupWindowSeconds
	}
	if next.WakeSensitivity == 0 && req.WakeSensitivity == nil {
		next.WakeSensitivity = defaults.WakeSensitivity
	}
	if strings.TrimSpace(next.TTSLocale) == "" {
		next.TTSLocale = defaults.TTSLocale
	}
	if next.TTSSpeed == 0 {
		next.TTSSpeed = defaults.TTSSpeed
	}
	if next.TTSVolume == 0 && req.TTSVolume == nil {
		next.TTSVolume = defaults.TTSVolume
	}
	return next
}

func validateSettings(settings Settings) []string {
	cfg := Config{
		Enabled:                 settings.Enabled,
		MutedByDefault:          settings.Muted,
		WakeWordModelID:         settings.WakeWordModelID,
		WakeWordPhrase:          settings.WakeWordPhrase,
		WakeSensitivity:         settings.WakeSensitivity,
		STTProviderID:           settings.STTProviderID,
		TTSProviderID:           settings.TTSProviderID,
		STTModelID:              settings.STTModelID,
		TTSModelID:              settings.TTSModelID,
		TTSVoiceID:              settings.TTSVoiceID,
		TTSEnabled:              settings.TTSEnabled,
		TTSLocale:               settings.TTSLocale,
		TTSSpeed:                settings.TTSSpeed,
		TTSVolume:               settings.TTSVolume,
		PreferredAgentID:        settings.PreferredAgentID,
		CloudOptIn:              settings.CloudOptIn,
		CommandProvidersEnabled: settings.CommandProvidersEnabled,
		SensitiveOutputPolicy:   settings.SensitiveOutputPolicy,
		FollowupWindowSeconds:   settings.FollowupWindowSeconds,
		MicrophoneProfile:       settings.MicrophoneProfile,
	}
	return Validate(cfg)
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
