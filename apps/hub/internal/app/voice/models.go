package voice

import (
	"strings"
	"time"
)

// Config represents wake word and STT/TTS provider config.
type Config struct {
	Enabled                 bool   `json:"enabled"                 yaml:"enabled"`
	MutedByDefault          bool   `json:"mutedByDefault"          yaml:"muted-by-default"`
	WakeWordModelID         string `json:"wakeWordModelId"         yaml:"wake-word-model-id"`
	STTProviderID           string `json:"sttProviderId"           yaml:"stt-provider-id"`
	TTSProviderID           string `json:"ttsProviderId"           yaml:"tts-provider-id"`
	STTModelID              string `json:"sttModelId"              yaml:"stt-model-id"`
	TTSModelID              string `json:"ttsModelId"              yaml:"tts-model-id"`
	TTSVoiceID              string `json:"ttsVoiceId"              yaml:"tts-voice-id"`
	PreferredAgentID        string `json:"preferredAgentId"        yaml:"preferred-agent-id"`
	CloudOptIn              bool   `json:"cloudOptIn"              yaml:"cloud-opt-in"`
	CommandProvidersEnabled bool   `json:"commandProvidersEnabled" yaml:"command-providers-enabled"`
	SensitiveOutputPolicy   string `json:"sensitiveOutputPolicy"   yaml:"sensitive-output-policy"`
	FollowupWindowSeconds   int    `json:"followupWindowSeconds"   yaml:"followup-window-seconds"`
	MicrophoneProfile       string `json:"microphoneProfile"       yaml:"microphone-profile"`
}

// Settings holds current voice settings state.
type Settings struct {
	DeviceProfileID         string `json:"deviceProfileId"`
	Enabled                 bool   `json:"enabled"`
	Muted                   bool   `json:"muted"`
	WakeWordModelID         string `json:"wakeWordModelId"`
	STTProviderID           string `json:"sttProviderId"`
	TTSProviderID           string `json:"ttsProviderId"`
	STTModelID              string `json:"sttModelId"`
	TTSModelID              string `json:"ttsModelId"`
	TTSVoiceID              string `json:"ttsVoiceId"`
	PreferredAgentID        string `json:"preferredAgentId"`
	CloudOptIn              bool   `json:"cloudOptIn"`
	CommandProvidersEnabled bool   `json:"commandProvidersEnabled"`
	SensitiveOutputPolicy   string `json:"sensitiveOutputPolicy"`
	FollowupWindowSeconds   int    `json:"followupWindowSeconds"`
	MicrophoneProfile       string `json:"microphoneProfile"`
	UpdatedAt               string `json:"updatedAt"`
}

// ProviderPack holds provider metadata.
type ProviderPack struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Kind          string `json:"kind"`
	TransportType string `json:"transportType"`
	HealthStatus  string `json:"healthStatus"`
	UpdatedAt     string `json:"updatedAt"`
}

// StatusResponse is the HTTP status payload for voice settings.
type StatusResponse struct {
	Enabled                 bool   `json:"enabled"`
	Muted                   bool   `json:"muted"`
	State                   string `json:"state"`
	ServiceStatus           string `json:"serviceStatus"`
	DeviceProfileID         string `json:"deviceProfileId"`
	WakeWordModelID         string `json:"wakeWordModelId"`
	STTProviderID           string `json:"sttProviderId"`
	TTSProviderID           string `json:"ttsProviderId"`
	STTModelID              string `json:"sttModelId"`
	TTSModelID              string `json:"ttsModelId"`
	TTSVoiceID              string `json:"ttsVoiceId"`
	PreferredAgentID        string `json:"preferredAgentId"`
	CloudOptIn              bool   `json:"cloudOptIn"`
	CommandProvidersEnabled bool   `json:"commandProvidersEnabled"`
	FollowupWindowSeconds   int    `json:"followupWindowSeconds"`
	MicrophoneProfile       string `json:"microphoneProfile"`
	UpdatedAt               string `json:"updatedAt"`
}

// DB Models

type SettingsDB struct {
	DeviceProfileID         string `gorm:"primaryKey;column:device_profile_id"`
	WakeWordModelID         string `gorm:"column:wake_word_model_id;default:''"`
	STTProviderID           string `gorm:"column:stt_provider_id;default:''"`
	TTSProviderID           string `gorm:"column:tts_provider_id;default:''"`
	STTModelID              string `gorm:"column:stt_model_id;default:''"`
	TTSModelID              string `gorm:"column:tts_model_id;default:''"`
	TTSVoiceID              string `gorm:"column:tts_voice_id;default:''"`
	CloudOptIn              int    `gorm:"column:cloud_opt_in;default:0"`
	CommandProvidersEnabled int    `gorm:"column:command_providers_enabled;default:0"`
	SensitiveOutputPolicy   string `gorm:"column:sensitive_output_policy;default:'visual_only_sensitive'"`
	FollowupWindowSeconds   int    `gorm:"column:followup_window_seconds;default:8"`
	MicrophoneProfile       string `gorm:"column:microphone_profile;default:''"`
	UpdatedAt               string `gorm:"column:updated_at"`
	Enabled                 int    `gorm:"column:enabled;default:0"`
	Muted                   int    `gorm:"column:muted"`
	PreferredAgentID        string `gorm:"column:preferred_agent_id;default:''"`
	LastStateUpdatedAt      string `gorm:"column:last_state_updated_at;default:''"`
}

func (SettingsDB) TableName() string {
	return "voice_settings"
}

type ProviderPackDB struct {
	ID            string `gorm:"primaryKey;column:id"`
	Name          string `gorm:"column:name"`
	Version       string `gorm:"column:version"`
	Kind          string `gorm:"column:kind"`
	TransportType string `gorm:"column:transport_type"`
	ManifestJSON  string `gorm:"column:manifest_json"`
	HealthStatus  string `gorm:"column:health_status"`
	InstalledAt   string `gorm:"column:installed_at"`
	UpdatedAt     string `gorm:"column:updated_at"`
}

func (ProviderPackDB) TableName() string {
	return "voice_provider_packs"
}

// Helpers

func State(enabled, muted bool) string {
	if muted {
		return "muted"
	}
	if enabled {
		return "wake_listening"
	}
	return "idle"
}

func ServiceStatus(enabled bool, sttProviderID, _ string) string {
	if !enabled || strings.TrimSpace(sttProviderID) == "" {
		return "not_configured"
	}
	return "ready"
}

func StatusFromSettings(settings Settings) StatusResponse {
	return StatusResponse{
		Enabled:                 settings.Enabled,
		Muted:                   settings.Muted,
		State:                   State(settings.Enabled, settings.Muted),
		ServiceStatus:           ServiceStatus(settings.Enabled, settings.STTProviderID, settings.TTSProviderID),
		DeviceProfileID:         settings.DeviceProfileID,
		WakeWordModelID:         settings.WakeWordModelID,
		STTProviderID:           settings.STTProviderID,
		TTSProviderID:           settings.TTSProviderID,
		STTModelID:              settings.STTModelID,
		TTSModelID:              settings.TTSModelID,
		TTSVoiceID:              settings.TTSVoiceID,
		PreferredAgentID:        settings.PreferredAgentID,
		CloudOptIn:              settings.CloudOptIn,
		CommandProvidersEnabled: settings.CommandProvidersEnabled,
		FollowupWindowSeconds:   settings.FollowupWindowSeconds,
		MicrophoneProfile:       settings.MicrophoneProfile,
		UpdatedAt:               settings.UpdatedAt,
	}
}

func StatusFromConfig(voice Config, now time.Time) StatusResponse {
	return StatusResponse{
		Enabled:                 voice.Enabled,
		Muted:                   voice.MutedByDefault,
		State:                   State(voice.Enabled, voice.MutedByDefault),
		ServiceStatus:           ServiceStatus(voice.Enabled, voice.STTProviderID, voice.TTSProviderID),
		DeviceProfileID:         "default-display",
		WakeWordModelID:         voice.WakeWordModelID,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		PreferredAgentID:        voice.PreferredAgentID,
		CloudOptIn:              voice.CloudOptIn,
		CommandProvidersEnabled: voice.CommandProvidersEnabled,
		FollowupWindowSeconds:   voice.FollowupWindowSeconds,
		MicrophoneProfile:       voice.MicrophoneProfile,
		UpdatedAt:               now.Format(time.RFC3339Nano),
	}
}

func DefaultConfig() Config {
	return Config{
		Enabled:               false,
		MutedByDefault:        true,
		SensitiveOutputPolicy: "visual_only_sensitive",
		FollowupWindowSeconds: 8,
	}
}

func ApplyDefaults(cfg *Config) {
	defaults := DefaultConfig()
	if strings.TrimSpace(cfg.SensitiveOutputPolicy) == "" {
		cfg.SensitiveOutputPolicy = defaults.SensitiveOutputPolicy
	}
	if cfg.FollowupWindowSeconds == 0 {
		cfg.FollowupWindowSeconds = defaults.FollowupWindowSeconds
	}
}

func Validate(cfg Config) []string {
	var problems []string
	if cfg.FollowupWindowSeconds < 1 || cfg.FollowupWindowSeconds > 30 {
		problems = append(problems, "voice.followupWindowSeconds must be between 1 and 30")
	}
	if strings.TrimSpace(cfg.SensitiveOutputPolicy) == "" {
		problems = append(problems, "voice.sensitiveOutputPolicy is required")
	}
	return problems
}
