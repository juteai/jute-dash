package app

import (
	"errors"
)

const (
	defaultHouseholdID     = "default"
	defaultDeviceProfileID = "default-display"
	defaultLayoutProfileID = "default-dashboard"
)

var (
	ErrInvalidLayout   = errors.New("invalid widget layout")
	ErrInvalidSettings = errors.New("invalid settings")
)

// Domain Models

type WidgetLayout struct {
	ProfileID string           `json:"profileId"`
	Widgets   []WidgetInstance `json:"widgets"`
}

type WidgetCatalogItem struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultTitle  string `json:"defaultTitle"`
	DefaultW      int    `json:"defaultW"`
	DefaultH      int    `json:"defaultH"`
	MinW          int    `json:"minW"`
	MinH          int    `json:"minH"`
	DefaultSize   string `json:"defaultSize"`
	Overflow      string `json:"overflow"`
	AllowMultiple bool   `json:"allowMultiple"`
}

type WidgetInstance struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	X        int            `json:"x"`
	Y        int            `json:"y"`
	W        int            `json:"w"`
	H        int            `json:"h"`
	MinW     int            `json:"minW"`
	MinH     int            `json:"minH"`
	Size     string         `json:"size"`
	Overflow string         `json:"overflow"`
	Settings map[string]any `json:"settings"`
	Visible  bool           `json:"visible"`
	Data     any            `json:"data,omitempty"`
}

type VoiceSettings struct {
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

type VoiceProviderPack struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Kind          string `json:"kind"`
	TransportType string `json:"transportType"`
	HealthStatus  string `json:"healthStatus"`
	UpdatedAt     string `json:"updatedAt"`
}

type HouseholdSettings struct {
	Home    HomeConfig    `json:"home"`
	Display DisplayConfig `json:"display"`
	Weather WeatherConfig `json:"weather"`
	Setup   SetupStatus   `json:"setup"`
}

type InitResult struct {
	Config Config
	Setup  SetupStatus
	Seeded bool
}

type SetupStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing"`
}

// GORM DB Models mapping exactly to the existing database schema.

type HouseholdSettingsDB struct {
	ID                      string `gorm:"primaryKey;column:id"`
	Name                    string `gorm:"column:name"`
	Timezone                string `gorm:"column:timezone"`
	Locale                  string `gorm:"column:locale"`
	DisplayTheme            string `gorm:"column:display_theme"`
	DisplayAccentColor      string `gorm:"column:display_accent_color"`
	DisplayIdleMode         string `gorm:"column:display_idle_mode"`
	SetupCompleted          int    `gorm:"column:setup_completed;default:0"`
	CreatedAt               string `gorm:"column:created_at"`
	UpdatedAt               string `gorm:"column:updated_at"`
	DisplayColorMode        string `gorm:"column:display_color_mode;default:'system'"`
	DisplayThemeID          string `gorm:"column:display_theme_id;default:'jute-mono'"`
	DisplayDensity          string `gorm:"column:display_density;default:'comfortable'"`
	DisplayMotion           string `gorm:"column:display_motion;default:'full'"`
	DisplayBackgroundJSON   string `gorm:"column:display_background_json;default:'{}'"`
	DisplayWidgetChromeJSON string `gorm:"column:display_widget_chrome_json;default:'{}'"`
}

func (HouseholdSettingsDB) TableName() string {
	return "household_settings"
}

type WeatherSettingsDB struct {
	ID              string  `gorm:"primaryKey;column:id"`
	Enabled         int     `gorm:"column:enabled"`
	Provider        string  `gorm:"column:provider"`
	LocationName    string  `gorm:"column:location_name"`
	Latitude        float64 `gorm:"column:latitude"`
	Longitude       float64 `gorm:"column:longitude"`
	TemperatureUnit string  `gorm:"column:temperature_unit"`
	WindSpeedUnit   string  `gorm:"column:wind_speed_unit"`
	UpdatedAt       string  `gorm:"column:updated_at"`
}

func (WeatherSettingsDB) TableName() string {
	return "weather_settings"
}

type DeviceProfileDB struct {
	ID              string `gorm:"primaryKey;column:id"`
	Name            string `gorm:"column:name"`
	InteractionMode string `gorm:"column:interaction_mode"`
	LayoutProfileID string `gorm:"column:layout_profile_id"`
	SettingsJSON    string `gorm:"column:settings_json"`
	CreatedAt       string `gorm:"column:created_at"`
	UpdatedAt       string `gorm:"column:updated_at"`
}

func (DeviceProfileDB) TableName() string {
	return "device_profiles"
}

type LayoutProfileDB struct {
	ID              string `gorm:"primaryKey;column:id"`
	DeviceProfileID string `gorm:"column:device_profile_id"`
	Name            string `gorm:"column:name"`
	SettingsJSON    string `gorm:"column:settings_json"`
	CreatedAt       string `gorm:"column:created_at"`
	UpdatedAt       string `gorm:"column:updated_at"`
}

func (LayoutProfileDB) TableName() string {
	return "layout_profiles"
}

type RoomDB struct {
	ID        string `gorm:"primaryKey;column:id"`
	Name      string `gorm:"column:name"`
	Summary   string `gorm:"column:summary"`
	Status    string `gorm:"column:status"`
	SortOrder int    `gorm:"column:sort_order"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (RoomDB) TableName() string {
	return "rooms"
}

type TileDB struct {
	ID        string `gorm:"primaryKey;column:id"`
	Kind      string `gorm:"column:kind"`
	Label     string `gorm:"column:label"`
	Value     string `gorm:"column:value"`
	Detail    string `gorm:"column:detail"`
	SortOrder int    `gorm:"column:sort_order"`
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (TileDB) TableName() string {
	return "tiles"
}

type WidgetPackDB struct {
	ID           string `gorm:"primaryKey;column:id"`
	Name         string `gorm:"column:name"`
	Version      string `gorm:"column:version"`
	ManifestJSON string `gorm:"column:manifest_json"`
	InstalledAt  string `gorm:"column:installed_at"`
	UpdatedAt    string `gorm:"column:updated_at"`
}

func (WidgetPackDB) TableName() string {
	return "widget_packs"
}

type WidgetInstanceDB struct {
	ID              string `gorm:"primaryKey;column:id"`
	Kind            string `gorm:"column:kind"`
	Title           string `gorm:"column:title"`
	LayoutProfileID string `gorm:"column:layout_profile_id"`
	X               int    `gorm:"column:x"`
	Y               int    `gorm:"column:y"`
	W               int    `gorm:"column:w"`
	H               int    `gorm:"column:h"`
	MinW            int    `gorm:"column:min_w"`
	MinH            int    `gorm:"column:min_h"`
	Size            string `gorm:"column:size"`
	SettingsJSON    string `gorm:"column:settings_json"`
	Visible         int    `gorm:"column:visible"`
	SortOrder       int    `gorm:"column:sort_order"`
	CreatedAt       string `gorm:"column:created_at"`
	UpdatedAt       string `gorm:"column:updated_at"`
}

func (WidgetInstanceDB) TableName() string {
	return "widget_instances"
}

type WidgetPermissionDB struct {
	WidgetInstanceID string `gorm:"primaryKey;column:widget_instance_id"`
	Permission       string `gorm:"primaryKey;column:permission"`
	Granted          int    `gorm:"column:granted"`
	UpdatedAt        string `gorm:"column:updated_at"`
}

func (WidgetPermissionDB) TableName() string {
	return "widget_permissions"
}

type VoiceProviderPackDB struct {
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

func (VoiceProviderPackDB) TableName() string {
	return "voice_provider_packs"
}

type VoiceSettingsDB struct {
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
	Muted                   int    `gorm:"column:muted;default:1"`
	PreferredAgentID        string `gorm:"column:preferred_agent_id;default:''"`
	LastStateUpdatedAt      string `gorm:"column:last_state_updated_at;default:''"`
}

func (VoiceSettingsDB) TableName() string {
	return "voice_settings"
}

type AdapterConnectionDB struct {
	ID            string `gorm:"primaryKey;column:id"`
	Kind          string `gorm:"column:kind"`
	Name          string `gorm:"column:name"`
	SettingsJSON  string `gorm:"column:settings_json"`
	SecretRefJSON string `gorm:"column:secret_ref_json"`
	Enabled       int    `gorm:"column:enabled"`
	CreatedAt     string `gorm:"column:created_at"`
	UpdatedAt     string `gorm:"column:updated_at"`
}

func (AdapterConnectionDB) TableName() string {
	return "adapter_connections"
}

type SettingAuditLogDB struct {
	ID           uint   `gorm:"primaryKey;autoIncrement;column:id"`
	Actor        string `gorm:"column:actor"`
	Action       string `gorm:"column:action"`
	Target       string `gorm:"column:target"`
	MetadataJSON string `gorm:"column:metadata_json"`
	CreatedAt    string `gorm:"column:created_at"`
}

func (SettingAuditLogDB) TableName() string {
	return "setting_audit_log"
}
