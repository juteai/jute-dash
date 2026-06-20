package model

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
