package homestate

import (
	"errors"
	"strings"
)

const (
	DefaultHouseholdID     = "default"
	DefaultDeviceProfileID = "default-display"
	DefaultLayoutProfileID = "default-dashboard"
)

var ErrInvalidSettings = errors.New("invalid settings")

// HomeConfig represents the home details config.
type HomeConfig struct {
	Name string `json:"name" yaml:"name"`
}

// RoomConfig defines room settings.
type RoomConfig struct {
	ID      string `json:"id"      yaml:"id"`
	Name    string `json:"name"    yaml:"name"`
	Summary string `json:"summary" yaml:"summary"`
	Status  string `json:"status"  yaml:"status"`
}

// TileConfig defines dashboard tile settings.
type TileConfig struct {
	ID     string `json:"id"     yaml:"id"`
	Kind   string `json:"kind"   yaml:"kind"`
	Label  string `json:"label"  yaml:"label"`
	Value  string `json:"value"  yaml:"value"`
	Detail string `json:"detail" yaml:"detail"`
}

// HouseholdSettings groups home-state related settings for frontend consumption.
type HouseholdSettings struct {
	Home    HomeConfig  `json:"home"`
	Display any         `json:"display"` // type dashboard.DisplayConfig, stored as any to avoid cyclic dependency
	Setup   SetupStatus `json:"setup"`
}

// SetupStatus holds initial configuration completion status.
type SetupStatus struct {
	Complete bool     `json:"complete"`
	Missing  []string `json:"missing"`
}

// InitResult represents the result of dashboard state initialization.
type InitResult struct {
	Config any // composite Config
	Setup  SetupStatus
	Seeded bool
}

// GORM DB Models mapping exactly to the existing database schema.

type HouseholdSettingsDB struct {
	ID                      string `gorm:"primaryKey;column:id"`
	Name                    string `gorm:"column:name"`
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

type AdapterConnection struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Settings   map[string]any    `json:"settings"`
	SecretRefs map[string]string `json:"secretRefs,omitempty"`
	Enabled    bool              `json:"enabled"`
	CreatedAt  string            `json:"createdAt,omitempty"`
	UpdatedAt  string            `json:"updatedAt,omitempty"`
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

// Validation helpers

// ValidateHome validates home details configuration.
func ValidateHome(cfg HomeConfig) []string {
	var problems []string
	if strings.TrimSpace(cfg.Name) == "" {
		problems = append(problems, "home.name is required")
	}
	return problems
}

// DefaultHomeConfig returns the default home configuration.
func DefaultHomeConfig() HomeConfig {
	return HomeConfig{
		Name: "Jute Home",
	}
}

// ApplyHomeDefaults sets default values for HomeConfig if empty.
func ApplyHomeDefaults(cfg *HomeConfig) {
	defaults := DefaultHomeConfig()
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = defaults.Name
	}
}

type DisplaySettings struct {
	Theme        string         `json:"theme"`
	ColorMode    string         `json:"colorMode"`
	ThemeID      string         `json:"themeId"`
	Density      string         `json:"density"`
	Motion       string         `json:"motion"`
	Background   map[string]any `json:"background"`
	WidgetChrome map[string]any `json:"widgetChrome"`
	AccentColor  string         `json:"accentColor"`
	IdleMode     string         `json:"idleMode"`
}
