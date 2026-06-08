package dashboard

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
)

var ErrInvalidLayout = errors.New("invalid widget layout")

// BaseColumns is the number of columns in the authored base grid. Layouts are
// stored at this resolution and proportionally remapped to fewer columns on
// smaller screens by the display.
const BaseColumns = 12

// LegacyColumnScale migrates layouts authored on the original 4-column grid to
// the 12-column base grid by scaling x/w by this factor.
const LegacyColumnScale = 3

// Widget modes.
const (
	WidgetModeUI       = "ui"
	WidgetModeHeadless = "headless"
)

// DisplayConfig represents settings for the dashboard display shell.
type DisplayConfig struct {
	Theme        string              `json:"theme"        yaml:"theme"`
	ColorMode    string              `json:"colorMode"    yaml:"color-mode"`
	ThemeID      string              `json:"themeId"      yaml:"theme-id"`
	Density      string              `json:"density"      yaml:"density"`
	Motion       string              `json:"motion"       yaml:"motion"`
	Background   DisplayBackground   `json:"background"   yaml:"background"`
	WidgetChrome DisplayWidgetChrome `json:"widgetChrome" yaml:"widget-chrome"`
	AccentColor  string              `json:"accentColor"  yaml:"accent-color"`
	IdleMode     string              `json:"idleMode"     yaml:"idle-mode"`
}

// DisplayBackground defines local-first background configurations.
type DisplayBackground struct {
	Kind     string `json:"kind"     yaml:"kind"`
	Value    string `json:"value"    yaml:"value"`
	Fit      string `json:"fit"      yaml:"fit"`
	Position string `json:"position" yaml:"position"`
	Overlay  string `json:"overlay"  yaml:"overlay"`
	// Slideshow configuration. Used when Kind is "slideshow".
	Images          []string `json:"images,omitempty"          yaml:"images,omitempty"`
	IntervalSeconds int      `json:"intervalSeconds,omitempty" yaml:"interval-seconds,omitempty"`
	Transition      string   `json:"transition,omitempty"      yaml:"transition,omitempty"`
	// Properties contains custom configuration parameters for dynamic backgrounds.
	Properties map[string]any `json:"properties,omitempty" yaml:"properties,omitempty"`
}

// DisplayWidgetChrome defines default visual framing rules.
type DisplayWidgetChrome struct {
	Default        string   `json:"default"                  yaml:"default"`
	SmokedOpacity  *float64 `json:"smokedOpacity,omitempty"  yaml:"smoked-opacity,omitempty"`
	FrostedOpacity *float64 `json:"frostedOpacity,omitempty" yaml:"frosted-opacity,omitempty"`
}

// DashboardConfig represents grid configuration defaults.
type DashboardConfig struct {
	Widgets []DashboardWidgetConfig `json:"widgets" yaml:"widgets"`
}

// DashboardWidgetConfig represents basic bootstrap placement config.
type DashboardWidgetConfig struct {
	ID       string         `json:"id"                 yaml:"id"`
	Type     string         `json:"type"               yaml:"type"`
	Title    string         `json:"title"              yaml:"title"`
	X        int            `json:"x"                  yaml:"x"`
	Y        int            `json:"y"                  yaml:"y"`
	W        int            `json:"w"                  yaml:"w"`
	H        int            `json:"h"                  yaml:"h"`
	MinW     int            `json:"minW,omitempty"     yaml:"min-w,omitempty"`
	MinH     int            `json:"minH,omitempty"     yaml:"min-h,omitempty"`
	Size     string         `json:"size,omitempty"     yaml:"size,omitempty"`
	Visible  bool           `json:"visible"            yaml:"visible"`
	Mode     string         `json:"mode,omitempty"     yaml:"mode,omitempty"`
	Settings map[string]any `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// Domain Models

// WidgetLayout represents widget placement on the display grid.
type WidgetLayout struct {
	ProfileID string           `json:"profileId"`
	Widgets   []WidgetInstance `json:"widgets"`
}

type SettingFieldType string

const (
	SettingString     SettingFieldType = "string"
	SettingNumber     SettingFieldType = "number"
	SettingBoolean    SettingFieldType = "boolean"
	SettingEnum       SettingFieldType = "enum"
	SettingStringList SettingFieldType = "string-list"
	SettingObjectList SettingFieldType = "object-list"
)

type SettingField struct {
	ID      string           `json:"id"`
	Type    SettingFieldType `json:"type"`
	Label   string           `json:"label"`
	Help    string           `json:"help,omitempty"`
	Default any              `json:"default,omitempty"`
	Options []string         `json:"options,omitempty"`
	Fields  []SettingField   `json:"fields,omitempty"`
}

// WidgetCatalogItem holds metadata of a widget kind.
type WidgetCatalogItem struct {
	Kind           string         `json:"kind"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	DefaultTitle   string         `json:"defaultTitle"`
	DefaultW       int            `json:"defaultW"`
	DefaultH       int            `json:"defaultH"`
	MinW           int            `json:"minW"`
	MinH           int            `json:"minH"`
	DefaultSize    string         `json:"defaultSize"`
	Overflow       string         `json:"overflow"`
	AllowMultiple  bool           `json:"allowMultiple"`
	SettingsSchema []SettingField `json:"settingsSchema,omitempty"`
}

// WidgetInstance represents an active widget.
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
	Mode     string         `json:"mode"`
	Settings map[string]any `json:"settings"`
	Visible  bool           `json:"visible"`
	Data     any            `json:"data,omitempty"`
}

// GORM DB Models

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
	Mode            string `gorm:"column:mode;default:'ui'"`
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

// Theme helpers

func SupportedThemeIDs() []string {
	return []string{
		"jute-mono",
		"solarized",
		"ayu",
		"one-dark",
		"gruvbox",
		"dracula",
		"catppuccin",
		"nord",
		"tokyo-night",
		"kanagawa",
		"monokai",
		"material",
		"github",
		"everforest",
	}
}

func IsSupportedThemeID(id string) bool {
	id = strings.TrimSpace(id)
	return slices.Contains(SupportedThemeIDs(), id)
}

func SupportedBackgroundIDs() []string {
	return []string{"stardust", "weather-ambient"}
}

func IsSupportedBackgroundID(id string) bool {
	id = strings.TrimSpace(id)
	return slices.Contains(SupportedBackgroundIDs(), id)
}

// Validation

func isSupportedColorMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "system", "light", "dark":
		return true
	default:
		return false
	}
}

func isSupportedDensity(value string) bool {
	switch strings.TrimSpace(value) {
	case "comfortable", "compact", "large-touch":
		return true
	default:
		return false
	}
}

func isSupportedMotion(value string) bool {
	switch strings.TrimSpace(value) {
	case "full", "reduced", "none":
		return true
	default:
		return false
	}
}

func isSupportedWidgetChrome(value string) bool {
	switch strings.TrimSpace(value) {
	case "solid", "clear", "smoked", "frosted", "auto":
		return true
	default:
		return false
	}
}

func containsRemoteReference(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//")
}

func validateDisplayBackground(background DisplayBackground, problems *[]string) {
	switch strings.TrimSpace(background.Kind) {
	case "theme":
		if strings.TrimSpace(background.Value) != "" {
			*problems = append(*problems, "display.background.value must be empty when kind is theme")
		}
	case "color":
		value := strings.TrimSpace(background.Value)
		if value == "" {
			*problems = append(*problems, "display.background.value is required when kind is color")
		}
		if containsRemoteReference(value) || strings.Contains(strings.ToLower(value), "url(") {
			*problems = append(*problems, "display.background.value must not contain remote URLs")
		}
	case "asset":
		value := strings.TrimSpace(background.Value)
		if value == "" || !strings.HasPrefix(value, "/") {
			*problems = append(
				*problems,
				"display.background.value must be an absolute app asset path when kind is asset",
			)
		}
		if containsRemoteReference(value) || strings.Contains(value, "..") {
			*problems = append(
				*problems,
				"display.background.value must not contain remote URLs or parent directory segments",
			)
		}
	case "file":
		value := strings.TrimSpace(background.Value)
		if value == "" {
			*problems = append(*problems, "display.background.value is required when kind is file")
		}
		if containsRemoteReference(value) || filepath.IsAbs(value) || strings.Contains(value, "..") {
			*problems = append(*problems, "display.background.value must be a relative safe file reference")
		}
	case "slideshow":
		if len(background.Images) == 0 {
			*problems = append(
				*problems,
				"display.background.images must contain at least one image when kind is slideshow",
			)
		}
		for _, image := range background.Images {
			value := strings.TrimSpace(image)
			if value == "" {
				*problems = append(*problems, "display.background.images must not contain empty entries")
				continue
			}
			if containsRemoteReference(value) || filepath.IsAbs(value) || strings.Contains(value, "..") {
				*problems = append(*problems, "display.background.images must be relative safe file references")
			}
		}
		if background.IntervalSeconds < 0 {
			*problems = append(*problems, "display.background.intervalSeconds must not be negative")
		}
		if t := strings.TrimSpace(background.Transition); t != "" && t != "none" && t != "crossfade" {
			*problems = append(*problems, "display.background.transition must be none or crossfade")
		}
	case "dynamic":
		value := strings.TrimSpace(background.Value)
		if value == "" {
			*problems = append(*problems, "display.background.value is required when kind is dynamic")
		}
		if !IsSupportedBackgroundID(value) {
			*problems = append(
				*problems,
				"display.background.value must be a supported background ID when kind is dynamic",
			)
		}
	default:
		*problems = append(
			*problems,
			"display.background.kind must be theme, color, asset, file, slideshow, or dynamic",
		)
	}
	switch strings.TrimSpace(background.Fit) {
	case "cover", "contain", "tile":
	default:
		*problems = append(*problems, "display.background.fit must be cover, contain, or tile")
	}
	if strings.TrimSpace(background.Position) == "" {
		*problems = append(*problems, "display.background.position is required")
	}
	switch strings.TrimSpace(background.Overlay) {
	case "none", "dim", "smoked", "frosted":
	default:
		*problems = append(*problems, "display.background.overlay must be none, dim, smoked, or frosted")
	}
}

// ValidateDisplay validates display configuration.
func ValidateDisplay(cfg DisplayConfig) []string {
	var problems []string
	if !isSupportedColorMode(cfg.ColorMode) {
		problems = append(problems, "display.colorMode must be system, light, or dark")
	}
	if cfg.Theme != "" && !isSupportedColorMode(cfg.Theme) {
		problems = append(problems, "display.theme must be system, light, or dark")
	}
	if !IsSupportedThemeID(cfg.ThemeID) {
		problems = append(problems, "display.themeId must be one of: "+strings.Join(SupportedThemeIDs(), ", "))
	}
	if !isSupportedDensity(cfg.Density) {
		problems = append(problems, "display.density must be comfortable, compact, or large-touch")
	}
	if !isSupportedMotion(cfg.Motion) {
		problems = append(problems, "display.motion must be full, reduced, or none")
	}
	validateDisplayBackground(cfg.Background, &problems)
	if !isSupportedWidgetChrome(cfg.WidgetChrome.Default) {
		problems = append(problems, "display.widgetChrome.default must be solid, clear, smoked, frosted, or auto")
	}
	if cfg.WidgetChrome.SmokedOpacity != nil {
		val := *cfg.WidgetChrome.SmokedOpacity
		if val < 0.0 || val > 1.0 {
			problems = append(problems, "display.widgetChrome.smokedOpacity must be between 0.0 and 1.0")
		}
	}
	if cfg.WidgetChrome.FrostedOpacity != nil {
		val := *cfg.WidgetChrome.FrostedOpacity
		if val < 0.0 || val > 1.0 {
			problems = append(problems, "display.widgetChrome.frostedOpacity must be between 0.0 and 1.0")
		}
	}
	return problems
}

// DefaultDisplayConfig returns default settings.
func DefaultDisplayConfig() DisplayConfig {
	return DisplayConfig{
		Theme:     "system",
		ColorMode: "system",
		ThemeID:   "jute-mono",
		Density:   "comfortable",
		Motion:    "full",
		Background: DisplayBackground{
			Kind:     "theme",
			Fit:      "cover",
			Position: "center",
			Overlay:  "none",
		},
		WidgetChrome: DisplayWidgetChrome{
			Default: "solid",
		},
		AccentColor: "neutral",
		IdleMode:    "ambient",
	}
}

// ApplyDisplayDefaults sets default values if fields are empty.
func ApplyDisplayDefaults(cfg *DisplayConfig) {
	defaults := DefaultDisplayConfig()
	if strings.TrimSpace(cfg.ColorMode) == "" {
		cfg.ColorMode = strings.TrimSpace(cfg.Theme)
	}
	if strings.TrimSpace(cfg.Theme) != "" && cfg.Theme != defaults.Theme &&
		cfg.ColorMode == defaults.ColorMode {
		cfg.ColorMode = cfg.Theme
	}
	if strings.TrimSpace(cfg.ColorMode) == "" {
		cfg.ColorMode = defaults.ColorMode
	}
	cfg.Theme = cfg.ColorMode
	if strings.TrimSpace(cfg.ThemeID) == "" {
		cfg.ThemeID = defaults.ThemeID
	}
	if strings.TrimSpace(cfg.Density) == "" {
		cfg.Density = defaults.Density
	}
	if strings.TrimSpace(cfg.Motion) == "" {
		cfg.Motion = defaults.Motion
	}
	if strings.TrimSpace(cfg.Background.Kind) == "" {
		cfg.Background.Kind = defaults.Background.Kind
	}
	if strings.TrimSpace(cfg.Background.Fit) == "" {
		cfg.Background.Fit = defaults.Background.Fit
	}
	if strings.TrimSpace(cfg.Background.Position) == "" {
		cfg.Background.Position = defaults.Background.Position
	}
	if strings.TrimSpace(cfg.Background.Overlay) == "" {
		cfg.Background.Overlay = defaults.Background.Overlay
	}
	if strings.TrimSpace(cfg.Background.Kind) == "slideshow" {
		if cfg.Background.IntervalSeconds <= 0 {
			cfg.Background.IntervalSeconds = 30
		}
		if strings.TrimSpace(cfg.Background.Transition) == "" {
			cfg.Background.Transition = "crossfade"
		}
	}
	if strings.TrimSpace(cfg.WidgetChrome.Default) == "" {
		cfg.WidgetChrome.Default = defaults.WidgetChrome.Default
	}
	if strings.TrimSpace(cfg.AccentColor) == "" {
		cfg.AccentColor = defaults.AccentColor
	}
	if strings.TrimSpace(cfg.IdleMode) == "" {
		cfg.IdleMode = defaults.IdleMode
	}
}
