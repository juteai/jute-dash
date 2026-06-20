package model

import (
	"strings"
	"time"
)

// VoiceConfig represents wake word and STT/TTS provider config.
type VoiceConfig struct {
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

// Config is kept as a compatibility alias while callers migrate to VoiceConfig.
type Config = VoiceConfig

// VoiceSettings holds current voice settings state.
type VoiceSettings struct {
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

// Settings is kept as a compatibility alias while callers migrate to VoiceSettings.
type Settings = VoiceSettings

type VoiceSettingsUpdateRequest struct {
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

// SettingsUpdateRequest is kept as a compatibility alias while callers migrate to VoiceSettingsUpdateRequest.
type SettingsUpdateRequest = VoiceSettingsUpdateRequest

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

// ProviderPackConfig is a bootstrap install record for a voice provider pack.
type ProviderPackConfig struct {
	ID            string               `json:"id"                      yaml:"id"`
	Name          string               `json:"name"                    yaml:"name"`
	Version       string               `json:"version"                 yaml:"version"`
	Kind          string               `json:"kind"                    yaml:"kind"`
	Transport     ProviderTransport    `json:"transport"               yaml:"transport"`
	Caps          ProviderCapabilities `json:"capabilities"            yaml:"capabilities"`
	Credentials   []ProviderCredential `json:"credentials"             yaml:"credentials"`
	Wake          *WakeWordProvider    `json:"wakeWord,omitempty"      yaml:"wake-word,omitempty"`
	TTS           *TTSProvider         `json:"tts,omitempty"           yaml:"tts,omitempty"`
	Health        string               `json:"healthStatus"            yaml:"health-status"`
	Error         string               `json:"lastError,omitempty"     yaml:"last-error,omitempty"`
	InstalledAt   string               `json:"installedAt,omitempty"   yaml:"installed-at,omitempty"`
	UpdatedAt     string               `json:"updatedAt,omitempty"     yaml:"updated-at,omitempty"`
	TransportKind string               `json:"transportType,omitempty" yaml:"transport-type,omitempty"`
}

type ProviderTransport struct {
	Type    string   `json:"type"              yaml:"type"`
	Command string   `json:"command,omitempty" yaml:"command,omitempty"`
	Args    []string `json:"args,omitempty"    yaml:"args,omitempty"`
}

type ProviderCredential struct {
	ID       string `json:"id"       yaml:"id"`
	Label    string `json:"label"    yaml:"label"`
	Source   string `json:"source"   yaml:"source"`
	Env      string `json:"env"      yaml:"env"`
	Required bool   `json:"required" yaml:"required"`
}

type ProviderCapabilities struct {
	Streaming          bool     `json:"streaming"              yaml:"streaming"`
	PartialTranscripts bool     `json:"partialTranscripts"     yaml:"partial-transcripts"`
	Offline            bool     `json:"offline"                yaml:"offline"`
	Languages          []string `json:"languages,omitempty"    yaml:"languages,omitempty"`
	InputFormats       []string `json:"inputFormats,omitempty" yaml:"input-formats,omitempty"`
}

type WakeWordProvider struct {
	DefaultModelID string                `json:"defaultModelId"      yaml:"default-model-id"`
	Phrase         string                `json:"phrase,omitempty"    yaml:"phrase,omitempty"`
	Languages      []string              `json:"languages,omitempty" yaml:"languages,omitempty"`
	Sensitivity    float64               `json:"sensitivity"         yaml:"sensitivity"`
	Models         []WakeWordModelConfig `json:"models,omitempty"    yaml:"models,omitempty"`
}

type WakeWordModelConfig struct {
	ID          string   `json:"id"                  yaml:"id"`
	Path        string   `json:"path"                yaml:"path"`
	Phrase      string   `json:"phrase,omitempty"    yaml:"phrase,omitempty"`
	Languages   []string `json:"languages,omitempty" yaml:"languages,omitempty"`
	Sensitivity float64  `json:"sensitivity"         yaml:"sensitivity"`
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

type TTSProvider struct {
	DefaultVoiceID string           `json:"defaultVoiceId,omitempty" yaml:"default-voice-id,omitempty"`
	DefaultModelID string           `json:"defaultModelId,omitempty" yaml:"default-model-id,omitempty"`
	Voices         []TTSVoiceConfig `json:"voices,omitempty"         yaml:"voices,omitempty"`
}

type TTSVoiceConfig struct {
	ID      string `json:"id"                yaml:"id"`
	Label   string `json:"label"             yaml:"label"`
	Locale  string `json:"locale"            yaml:"locale"`
	ModelID string `json:"modelId,omitempty" yaml:"model-id,omitempty"`
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
		Enabled: settings.Enabled,
		Muted:   settings.Muted,
		State:   State(settings.Enabled, settings.Muted),
		ServiceStatus: ServiceStatus(
			settings.Enabled,
			settings.STTProviderID,
			settings.TTSProviderID,
		),
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
		Enabled: voice.Enabled,
		Muted:   voice.MutedByDefault,
		State:   State(voice.Enabled, voice.MutedByDefault),
		ServiceStatus: ServiceStatus(
			voice.Enabled,
			voice.STTProviderID,
			voice.TTSProviderID,
		),
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
