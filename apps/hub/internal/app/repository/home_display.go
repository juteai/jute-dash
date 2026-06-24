package repository

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func displayConfigFromHouseholdDB(hs HouseholdSettingsDB) (DisplaySettings, error) {
	display := defaultDisplaySettings()
	display.Theme = hs.DisplayTheme
	display.ColorMode = hs.DisplayColorMode
	display.ThemeID = hs.DisplayThemeID
	display.Density = hs.DisplayDensity
	display.Motion = hs.DisplayMotion
	display.AccentColor = hs.DisplayAccentColor
	display.IdleMode = hs.DisplayIdleMode

	if err := decodeJSONSetting(hs.DisplayBackgroundJSON, &display.Background); err != nil {
		return DisplaySettings{}, fmt.Errorf("decode display background: %w", err)
	}
	if err := decodeJSONSetting(hs.DisplayWidgetChromeJSON, &display.WidgetChrome); err != nil {
		return DisplaySettings{}, fmt.Errorf("decode display widget chrome: %w", err)
	}

	applyDisplayDefaults(&display)
	return display, nil
}

func normalizeDisplayForSave(display any) (DisplaySettings, error) {
	normalized, err := displayConfigFromAny(display)
	if err != nil {
		return DisplaySettings{}, err
	}
	applyDisplayDefaults(&normalized)
	if problems := validateDisplaySettings(normalized); len(problems) > 0 {
		return DisplaySettings{}, fmt.Errorf(
			"%w: %s",
			ErrInvalidSettings,
			strings.Join(problems, "; "),
		)
	}
	return normalized, nil
}

func displayConfigFromAny(display any) (DisplaySettings, error) {
	if display == nil {
		return defaultDisplaySettings(), nil
	}
	bytes, err := json.Marshal(display)
	if err != nil {
		return DisplaySettings{}, fmt.Errorf("encode display settings: %w", err)
	}
	var cfg DisplaySettings
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return DisplaySettings{}, fmt.Errorf("decode display settings: %w", err)
	}
	return cfg, nil
}

func defaultDisplaySettings() DisplaySettings {
	return DisplaySettings{
		Theme:     "system",
		ColorMode: "system",
		ThemeID:   "jute-mono",
		Density:   "comfortable",
		Motion:    "full",
		Background: map[string]any{
			"kind":     "theme",
			"value":    "",
			"fit":      "cover",
			"position": "center",
			"overlay":  "none",
		},
		WidgetChrome: map[string]any{
			"default": "solid",
		},
		AccentColor: "neutral",
		IdleMode:    "ambient",
	}
}

func applyDisplayDefaults(cfg *DisplaySettings) {
	defaults := defaultDisplaySettings()
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
	if cfg.Background == nil {
		cfg.Background = map[string]any{}
	}
	if strings.TrimSpace(stringSetting(cfg.Background, "kind")) == "" {
		cfg.Background["kind"] = "theme"
	}
	if stringSetting(cfg.Background, "kind") == "theme" {
		cfg.Background["value"] = ""
	}
	if strings.TrimSpace(stringSetting(cfg.Background, "fit")) == "" {
		cfg.Background["fit"] = "cover"
	}
	if strings.TrimSpace(stringSetting(cfg.Background, "position")) == "" {
		cfg.Background["position"] = "center"
	}
	if strings.TrimSpace(stringSetting(cfg.Background, "overlay")) == "" {
		cfg.Background["overlay"] = "none"
	}
	if strings.TrimSpace(stringSetting(cfg.Background, "kind")) == "slideshow" {
		if numberSetting(cfg.Background, "intervalSeconds") <= 0 {
			cfg.Background["intervalSeconds"] = 30
		}
		if strings.TrimSpace(stringSetting(cfg.Background, "transition")) == "" {
			cfg.Background["transition"] = "crossfade"
		}
	}
	if cfg.WidgetChrome == nil {
		cfg.WidgetChrome = map[string]any{}
	}
	if strings.TrimSpace(stringSetting(cfg.WidgetChrome, "default")) == "" {
		cfg.WidgetChrome["default"] = "solid"
	}
	if strings.TrimSpace(cfg.AccentColor) == "" {
		cfg.AccentColor = defaults.AccentColor
	}
	if strings.TrimSpace(cfg.IdleMode) == "" {
		cfg.IdleMode = defaults.IdleMode
	}
}

func validateDisplaySettings(cfg DisplaySettings) []string {
	var problems []string
	if !supportedColorMode(cfg.ColorMode) {
		problems = append(problems, "display.colorMode must be system, light, or dark")
	}
	if cfg.Theme != "" && !supportedColorMode(cfg.Theme) {
		problems = append(problems, "display.theme must be system, light, or dark")
	}
	if !supportedThemeID(cfg.ThemeID) {
		problems = append(problems, "display.themeId must be one of: "+strings.Join(supportedThemeIDs(), ", "))
	}
	if !supportedDensity(cfg.Density) {
		problems = append(problems, "display.density must be comfortable, compact, or large-touch")
	}
	if !supportedMotion(cfg.Motion) {
		problems = append(problems, "display.motion must be full, reduced, or none")
	}
	validateDisplayBackground(cfg.Background, &problems)
	if !supportedWidgetChrome(stringSetting(cfg.WidgetChrome, "default")) {
		problems = append(problems, "display.widgetChrome.default must be solid, clear, smoked, frosted, or auto")
	}
	if value, ok := optionalNumberSetting(cfg.WidgetChrome, "smokedOpacity"); ok && (value < 0.0 || value > 1.0) {
		problems = append(problems, "display.widgetChrome.smokedOpacity must be between 0.0 and 1.0")
	}
	if value, ok := optionalNumberSetting(cfg.WidgetChrome, "frostedOpacity"); ok && (value < 0.0 || value > 1.0) {
		problems = append(problems, "display.widgetChrome.frostedOpacity must be between 0.0 and 1.0")
	}
	return problems
}

func validateDisplayBackground(background map[string]any, problems *[]string) {
	kind := strings.TrimSpace(stringSetting(background, "kind"))
	value := strings.TrimSpace(stringSetting(background, "value"))
	switch kind {
	case "theme":
		if value != "" {
			*problems = append(*problems, "display.background.value must be empty when kind is theme")
		}
	case "color":
		if value == "" {
			*problems = append(*problems, "display.background.value is required when kind is color")
		}
		if containsRemoteReference(value) || strings.Contains(strings.ToLower(value), "url(") {
			*problems = append(*problems, "display.background.value must not contain remote URLs")
		}
	case "asset":
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
		if value == "" {
			*problems = append(*problems, "display.background.value is required when kind is file")
		}
		if containsRemoteReference(value) || filepath.IsAbs(value) || strings.Contains(value, "..") {
			*problems = append(*problems, "display.background.value must be a relative safe file reference")
		}
	case "slideshow":
		images := stringSliceSetting(background, "images")
		if len(images) == 0 {
			*problems = append(
				*problems,
				"display.background.images must contain at least one image when kind is slideshow",
			)
		}
		for _, image := range images {
			value := strings.TrimSpace(image)
			if value == "" {
				*problems = append(*problems, "display.background.images must not contain empty entries")
				continue
			}
			if containsRemoteReference(value) || filepath.IsAbs(value) || strings.Contains(value, "..") {
				*problems = append(*problems, "display.background.images must be relative safe file references")
			}
		}
		if numberSetting(background, "intervalSeconds") < 0 {
			*problems = append(*problems, "display.background.intervalSeconds must not be negative")
		}
		transition := strings.TrimSpace(stringSetting(background, "transition"))
		if transition != "" && transition != "none" && transition != "crossfade" {
			*problems = append(*problems, "display.background.transition must be none or crossfade")
		}
	case "dynamic":
		if value == "" {
			*problems = append(*problems, "display.background.value is required when kind is dynamic")
		}
		if !supportedBackgroundID(value) {
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
	switch strings.TrimSpace(stringSetting(background, "fit")) {
	case "cover", "contain", "tile":
	default:
		*problems = append(*problems, "display.background.fit must be cover, contain, or tile")
	}
	if strings.TrimSpace(stringSetting(background, "position")) == "" {
		*problems = append(*problems, "display.background.position is required")
	}
	switch strings.TrimSpace(stringSetting(background, "overlay")) {
	case "none", "dim", "smoked", "frosted":
	default:
		*problems = append(*problems, "display.background.overlay must be none, dim, smoked, or frosted")
	}
}

func supportedColorMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "system", "light", "dark":
		return true
	default:
		return false
	}
}

func supportedDensity(value string) bool {
	switch strings.TrimSpace(value) {
	case "comfortable", "compact", "large-touch":
		return true
	default:
		return false
	}
}

func supportedMotion(value string) bool {
	switch strings.TrimSpace(value) {
	case "full", "reduced", "none":
		return true
	default:
		return false
	}
}

func supportedWidgetChrome(value string) bool {
	switch strings.TrimSpace(value) {
	case "solid", "clear", "smoked", "frosted", "auto":
		return true
	default:
		return false
	}
}

func supportedThemeID(id string) bool {
	id = strings.TrimSpace(id)
	for _, supported := range supportedThemeIDs() {
		if id == supported {
			return true
		}
	}
	return false
}

func supportedThemeIDs() []string {
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

func supportedBackgroundID(id string) bool {
	id = strings.TrimSpace(id)
	for _, supported := range []string{"stardust", "weather-ambient"} {
		if id == supported {
			return true
		}
	}
	return false
}

func containsRemoteReference(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "//")
}

func stringSetting(settings map[string]any, key string) string {
	if settings == nil {
		return ""
	}
	value, ok := settings[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func numberSetting(settings map[string]any, key string) float64 {
	value, _ := optionalNumberSetting(settings, key)
	return value
}

func optionalNumberSetting(settings map[string]any, key string) (float64, bool) {
	if settings == nil {
		return 0, false
	}
	value, ok := settings[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		parsed, err := v.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func stringSliceSetting(settings map[string]any, key string) []string {
	if settings == nil {
		return nil
	}
	switch v := settings[key].(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			values = append(values, fmt.Sprint(item))
		}
		return values
	default:
		return nil
	}
}
