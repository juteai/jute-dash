package repository

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
