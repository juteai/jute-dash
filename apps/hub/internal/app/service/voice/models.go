package voice

import (
	"strings"
	"time"
)

// Config represents wake word and STT/TTS provider config.
type Config struct {
	Enabled                 bool    `json:"enabled"                 yaml:"enabled"`
	MutedByDefault          bool    `json:"mutedByDefault"          yaml:"muted-by-default"`
	WakeWordModelID         string  `json:"wakeWordModelId"         yaml:"wake-word-model-id"`
	WakeWordPhrase          string  `json:"wakeWordPhrase"          yaml:"wake-word-phrase"`
	WakeSensitivity         float64 `json:"wakeSensitivity"         yaml:"wake-sensitivity"`
	STTProviderID           string  `json:"sttProviderId"           yaml:"stt-provider-id"`
	TTSProviderID           string  `json:"ttsProviderId"           yaml:"tts-provider-id"`
	STTModelID              string  `json:"sttModelId"              yaml:"stt-model-id"`
	TTSModelID              string  `json:"ttsModelId"              yaml:"tts-model-id"`
	TTSVoiceID              string  `json:"ttsVoiceId"              yaml:"tts-voice-id"`
	TTSEnabled              bool    `json:"ttsEnabled"              yaml:"tts-enabled"`
	TTSLocale               string  `json:"ttsLocale"               yaml:"tts-locale"`
	TTSSpeed                float64 `json:"ttsSpeed"                yaml:"tts-speed"`
	TTSVolume               float64 `json:"ttsVolume"               yaml:"tts-volume"`
	PreferredAgentID        string  `json:"preferredAgentId"        yaml:"preferred-agent-id"`
	CloudOptIn              bool    `json:"cloudOptIn"              yaml:"cloud-opt-in"`
	CommandProvidersEnabled bool    `json:"commandProvidersEnabled" yaml:"command-providers-enabled"`
	SensitiveOutputPolicy   string  `json:"sensitiveOutputPolicy"   yaml:"sensitive-output-policy"`
	FollowupWindowSeconds   int     `json:"followupWindowSeconds"   yaml:"followup-window-seconds"`
	MicrophoneProfile       string  `json:"microphoneProfile"       yaml:"microphone-profile"`
}

// Settings holds current voice settings state.
type Settings struct {
	DeviceProfileID         string  `json:"deviceProfileId"`
	Enabled                 bool    `json:"enabled"`
	Muted                   bool    `json:"muted"`
	WakeWordModelID         string  `json:"wakeWordModelId"`
	WakeWordPhrase          string  `json:"wakeWordPhrase"`
	WakeSensitivity         float64 `json:"wakeSensitivity"`
	STTProviderID           string  `json:"sttProviderId"`
	TTSProviderID           string  `json:"ttsProviderId"`
	STTModelID              string  `json:"sttModelId"`
	TTSModelID              string  `json:"ttsModelId"`
	TTSVoiceID              string  `json:"ttsVoiceId"`
	TTSEnabled              bool    `json:"ttsEnabled"`
	TTSLocale               string  `json:"ttsLocale"`
	TTSSpeed                float64 `json:"ttsSpeed"`
	TTSVolume               float64 `json:"ttsVolume"`
	PreferredAgentID        string  `json:"preferredAgentId"`
	CloudOptIn              bool    `json:"cloudOptIn"`
	CommandProvidersEnabled bool    `json:"commandProvidersEnabled"`
	SensitiveOutputPolicy   string  `json:"sensitiveOutputPolicy"`
	FollowupWindowSeconds   int     `json:"followupWindowSeconds"`
	MicrophoneProfile       string  `json:"microphoneProfile"`
	UpdatedAt               string  `json:"updatedAt"`
}

type SettingsUpdateRequest struct {
	DeviceProfileID         string   `json:"deviceProfileId,omitempty"`
	Enabled                 *bool    `json:"enabled,omitempty"`
	WakeWordModelID         *string  `json:"wakeWordModelId,omitempty"`
	WakeWordPhrase          *string  `json:"wakeWordPhrase,omitempty"`
	WakeSensitivity         *float64 `json:"wakeSensitivity,omitempty"`
	STTProviderID           *string  `json:"sttProviderId,omitempty"`
	TTSProviderID           *string  `json:"ttsProviderId,omitempty"`
	STTModelID              *string  `json:"sttModelId,omitempty"`
	TTSModelID              *string  `json:"ttsModelId,omitempty"`
	TTSVoiceID              *string  `json:"ttsVoiceId,omitempty"`
	TTSEnabled              *bool    `json:"ttsEnabled,omitempty"`
	TTSLocale               *string  `json:"ttsLocale,omitempty"`
	TTSSpeed                *float64 `json:"ttsSpeed,omitempty"`
	TTSVolume               *float64 `json:"ttsVolume,omitempty"`
	PreferredAgentID        *string  `json:"preferredAgentId,omitempty"`
	CloudOptIn              *bool    `json:"cloudOptIn,omitempty"`
	CommandProvidersEnabled *bool    `json:"commandProvidersEnabled,omitempty"`
	SensitiveOutputPolicy   *string  `json:"sensitiveOutputPolicy,omitempty"`
	FollowupWindowSeconds   *int     `json:"followupWindowSeconds,omitempty"`
	MicrophoneProfile       *string  `json:"microphoneProfile,omitempty"`
}

// ProviderPack holds provider metadata.
type ProviderPack struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Version          string                   `json:"version"`
	Kind             string                   `json:"kind"`
	TransportType    string                   `json:"transportType"`
	Capabilities     ProviderCapabilities     `json:"capabilities"`
	WakeWord         *WakeWordProviderSummary `json:"wakeWord,omitempty"`
	HealthStatus     string                   `json:"healthStatus"`
	LastActivationAt string                   `json:"lastActivationAt,omitempty"`
	LastError        string                   `json:"lastError,omitempty"`
	UpdatedAt        string                   `json:"updatedAt"`
}

type ProviderCapabilities struct {
	Streaming          bool     `json:"streaming"`
	PartialTranscripts bool     `json:"partialTranscripts"`
	Offline            bool     `json:"offline"`
	Languages          []string `json:"languages,omitempty"`
	InputFormats       []string `json:"inputFormats,omitempty"`
}

type WakeWordProviderSummary struct {
	DefaultModelID string                 `json:"defaultModelId"`
	Phrase         string                 `json:"phrase,omitempty"`
	Languages      []string               `json:"languages,omitempty"`
	Sensitivity    float64                `json:"sensitivity,omitempty"`
	Models         []WakeWordModelSummary `json:"models,omitempty"`
}

type WakeWordModelSummary struct {
	ID          string   `json:"id"`
	Phrase      string   `json:"phrase,omitempty"`
	Languages   []string `json:"languages,omitempty"`
	Sensitivity float64  `json:"sensitivity,omitempty"`
}

type TTSVoicesResponse struct {
	ProviderID      string     `json:"providerId"`
	ProviderName    string     `json:"providerName,omitempty"`
	HealthStatus    string     `json:"healthStatus"`
	SetupStatus     string     `json:"setupStatus"`
	SelectedVoiceID string     `json:"selectedVoiceId,omitempty"`
	SelectedModelID string     `json:"selectedModelId,omitempty"`
	Locale          string     `json:"locale"`
	Speed           float64    `json:"speed"`
	Volume          float64    `json:"volume"`
	CloudProvider   bool       `json:"cloudProvider"`
	Voices          []TTSVoice `json:"voices"`
}

type TTSVoice struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Locale  string `json:"locale"`
	ModelID string `json:"modelId,omitempty"`
}

// StatusResponse is the HTTP status payload for voice settings.
type StatusResponse struct {
	Enabled                 bool    `json:"enabled"`
	Muted                   bool    `json:"muted"`
	State                   string  `json:"state"`
	ServiceStatus           string  `json:"serviceStatus"`
	DeviceProfileID         string  `json:"deviceProfileId"`
	WakeWordModelID         string  `json:"wakeWordModelId"`
	WakeWordPhrase          string  `json:"wakeWordPhrase"`
	WakeSensitivity         float64 `json:"wakeSensitivity"`
	STTProviderID           string  `json:"sttProviderId"`
	TTSProviderID           string  `json:"ttsProviderId"`
	STTModelID              string  `json:"sttModelId"`
	TTSModelID              string  `json:"ttsModelId"`
	TTSVoiceID              string  `json:"ttsVoiceId"`
	TTSEnabled              bool    `json:"ttsEnabled"`
	TTSLocale               string  `json:"ttsLocale"`
	TTSSpeed                float64 `json:"ttsSpeed"`
	TTSVolume               float64 `json:"ttsVolume"`
	PreferredAgentID        string  `json:"preferredAgentId"`
	CloudOptIn              bool    `json:"cloudOptIn"`
	CommandProvidersEnabled bool    `json:"commandProvidersEnabled"`
	FollowupWindowSeconds   int     `json:"followupWindowSeconds"`
	MicrophoneProfile       string  `json:"microphoneProfile"`
	UpdatedAt               string  `json:"updatedAt"`
}

// DB Models

type SettingsDB struct {
	DeviceProfileID         string  `gorm:"primaryKey;column:device_profile_id"`
	WakeWordModelID         string  `gorm:"column:wake_word_model_id;default:''"`
	WakeWordPhrase          string  `gorm:"column:wake_word_phrase;default:''"`
	WakeSensitivity         float64 `gorm:"column:wake_sensitivity;default:0.5"`
	STTProviderID           string  `gorm:"column:stt_provider_id;default:''"`
	TTSProviderID           string  `gorm:"column:tts_provider_id;default:''"`
	STTModelID              string  `gorm:"column:stt_model_id;default:''"`
	TTSModelID              string  `gorm:"column:tts_model_id;default:''"`
	TTSVoiceID              string  `gorm:"column:tts_voice_id;default:''"`
	TTSEnabled              int     `gorm:"column:tts_enabled;default:0"`
	TTSLocale               string  `gorm:"column:tts_locale;default:'en'"`
	TTSSpeed                float64 `gorm:"column:tts_speed;default:1"`
	TTSVolume               float64 `gorm:"column:tts_volume;default:1"`
	CloudOptIn              int     `gorm:"column:cloud_opt_in;default:0"`
	CommandProvidersEnabled int     `gorm:"column:command_providers_enabled;default:0"`
	SensitiveOutputPolicy   string  `gorm:"column:sensitive_output_policy;default:'visual_only_sensitive'"`
	FollowupWindowSeconds   int     `gorm:"column:followup_window_seconds;default:8"`
	MicrophoneProfile       string  `gorm:"column:microphone_profile;default:''"`
	UpdatedAt               string  `gorm:"column:updated_at"`
	Enabled                 int     `gorm:"column:enabled;default:0"`
	Muted                   int     `gorm:"column:muted"`
	PreferredAgentID        string  `gorm:"column:preferred_agent_id;default:''"`
	LastStateUpdatedAt      string  `gorm:"column:last_state_updated_at;default:''"`
}

func (SettingsDB) TableName() string {
	return "voice_settings"
}

type ProviderPackDB struct {
	ID               string `gorm:"primaryKey;column:id"`
	Name             string `gorm:"column:name"`
	Version          string `gorm:"column:version"`
	Kind             string `gorm:"column:kind"`
	TransportType    string `gorm:"column:transport_type"`
	ManifestJSON     string `gorm:"column:manifest_json"`
	HealthStatus     string `gorm:"column:health_status"`
	LastActivationAt string `gorm:"column:last_activation_at;default:''"`
	LastError        string `gorm:"column:last_error;default:''"`
	InstalledAt      string `gorm:"column:installed_at"`
	UpdatedAt        string `gorm:"column:updated_at"`
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
		WakeWordPhrase:          voice.WakeWordPhrase,
		WakeSensitivity:         voice.WakeSensitivity,
		STTProviderID:           voice.STTProviderID,
		TTSProviderID:           voice.TTSProviderID,
		STTModelID:              voice.STTModelID,
		TTSModelID:              voice.TTSModelID,
		TTSVoiceID:              voice.TTSVoiceID,
		TTSEnabled:              voice.TTSEnabled,
		TTSLocale:               voice.TTSLocale,
		TTSSpeed:                voice.TTSSpeed,
		TTSVolume:               voice.TTSVolume,
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
		WakeSensitivity:       0.5,
		TTSLocale:             "en",
		TTSSpeed:              1,
		TTSVolume:             1,
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
	if cfg.WakeSensitivity == 0 {
		cfg.WakeSensitivity = defaults.WakeSensitivity
	}
	if strings.TrimSpace(cfg.TTSLocale) == "" {
		cfg.TTSLocale = defaults.TTSLocale
	}
	if cfg.TTSSpeed == 0 {
		cfg.TTSSpeed = defaults.TTSSpeed
	}
	if cfg.TTSVolume == 0 {
		cfg.TTSVolume = defaults.TTSVolume
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
	if cfg.WakeSensitivity < 0 || cfg.WakeSensitivity > 1 {
		problems = append(problems, "voice.wakeSensitivity must be between 0 and 1")
	}
	if cfg.TTSSpeed < 0.5 || cfg.TTSSpeed > 2 {
		problems = append(problems, "voice.ttsSpeed must be between 0.5 and 2")
	}
	if cfg.TTSVolume < 0 || cfg.TTSVolume > 1 {
		problems = append(problems, "voice.ttsVolume must be between 0 and 1")
	}
	return problems
}
