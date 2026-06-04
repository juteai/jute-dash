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
	Name     string `json:"name"     yaml:"name"`
	Timezone string `json:"timezone" yaml:"timezone"`
	Locale   string `json:"locale"   yaml:"locale"`
}

// WeatherConfig represents weather provider configuration.
type WeatherConfig struct {
	Enabled         bool    `json:"enabled"         yaml:"enabled"`
	Provider        string  `json:"provider"        yaml:"provider"`
	LocationName    string  `json:"locationName"    yaml:"location-name"`
	Latitude        float64 `json:"latitude"        yaml:"latitude"`
	Longitude       float64 `json:"longitude"       yaml:"longitude"`
	TemperatureUnit string  `json:"temperatureUnit" yaml:"temperature-unit"`
	WindSpeedUnit   string  `json:"windSpeedUnit"   yaml:"wind-speed-unit"`
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
	Home    HomeConfig    `json:"home"`
	Display any           `json:"display"` // type dashboard.DisplayConfig, stored as any to avoid cyclic dependency
	Weather WeatherConfig `json:"weather"`
	Setup   SetupStatus   `json:"setup"`
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

// Validation helpers

func isSupportedTemperatureUnit(unit string) bool {
	switch unit {
	case "celsius", "fahrenheit":
		return true
	default:
		return false
	}
}

func isSupportedWindSpeedUnit(unit string) bool {
	switch unit {
	case "kmh", "mph", "ms", "kn":
		return true
	default:
		return false
	}
}

// ValidateHome validates home details configuration.
func ValidateHome(cfg HomeConfig) []string {
	var problems []string
	if strings.TrimSpace(cfg.Name) == "" {
		problems = append(problems, "home.name is required")
	}
	return problems
}

// ValidateWeather validates weather provider configuration.
func ValidateWeather(cfg WeatherConfig) []string {
	var problems []string
	if cfg.Enabled {
		if cfg.Provider != "open-meteo" {
			problems = append(problems, "weather.provider must be open-meteo")
		}
		if strings.TrimSpace(cfg.LocationName) == "" {
			problems = append(problems, "weather.locationName is required")
		}
		if cfg.Latitude < -90 || cfg.Latitude > 90 {
			problems = append(problems, "weather.latitude must be between -90 and 90")
		}
		if cfg.Longitude < -180 || cfg.Longitude > 180 {
			problems = append(problems, "weather.longitude must be between -180 and 180")
		}
		if !isSupportedTemperatureUnit(cfg.TemperatureUnit) {
			problems = append(problems, "weather.temperatureUnit must be celsius or fahrenheit")
		}
		if !isSupportedWindSpeedUnit(cfg.WindSpeedUnit) {
			problems = append(problems, "weather.windSpeedUnit must be kmh, mph, ms, or kn")
		}
	}
	return problems
}

// DefaultHomeConfig returns the default home configuration.
func DefaultHomeConfig() HomeConfig {
	return HomeConfig{
		Name:     "Jute Home",
		Timezone: "UTC",
		Locale:   "en",
	}
}

// DefaultWeatherConfig returns the default weather configuration.
func DefaultWeatherConfig() WeatherConfig {
	return WeatherConfig{
		Enabled:         true,
		Provider:        "open-meteo",
		LocationName:    "London",
		Latitude:        51.5072,
		Longitude:       -0.1276,
		TemperatureUnit: "celsius",
		WindSpeedUnit:   "kmh",
	}
}

// ApplyHomeDefaults sets default values for HomeConfig if empty.
func ApplyHomeDefaults(cfg *HomeConfig) {
	defaults := DefaultHomeConfig()
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = defaults.Name
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		cfg.Timezone = defaults.Timezone
	}
	if strings.TrimSpace(cfg.Locale) == "" {
		cfg.Locale = defaults.Locale
	}
}

// ApplyWeatherDefaults sets default values for WeatherConfig if empty.
func ApplyWeatherDefaults(cfg *WeatherConfig) {
	defaults := DefaultWeatherConfig()
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = defaults.Provider
	}
	if strings.TrimSpace(cfg.LocationName) == "" {
		cfg.LocationName = defaults.LocationName
	}
	if strings.TrimSpace(cfg.TemperatureUnit) == "" {
		cfg.TemperatureUnit = defaults.TemperatureUnit
	}
	if strings.TrimSpace(cfg.WindSpeedUnit) == "" {
		cfg.WindSpeedUnit = defaults.WindSpeedUnit
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
