package weather

import (
	"context"
	"strings"

	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

const SkillID = "jute.weather.current"

// Settings holds the parsed per-instance configuration for the weather widget.
type Settings struct {
	LocationName    string
	Latitude        float64
	Longitude       float64
	TemperatureUnit string
	WindSpeedUnit   string
	Enabled         bool
	Provider        string
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{
		LocationName:    "London",
		Latitude:        51.5072,
		Longitude:       -0.1276,
		TemperatureUnit: "celsius",
		WindSpeedUnit:   "kmh",
		Enabled:         true,
		Provider:        "open-meteo",
	}
	// Support both camelCase and kebab-case key variants from YAML config.
	if v, ok := raw["locationName"].(string); ok && v != "" {
		s.LocationName = v
	} else if v, ok := raw["location"].(string); ok && v != "" {
		s.LocationName = v
	}
	if v, ok := raw["latitude"].(float64); ok {
		s.Latitude = v
	}
	if v, ok := raw["longitude"].(float64); ok {
		s.Longitude = v
	}
	if v, ok := raw["temperatureUnit"].(string); ok && v != "" {
		s.TemperatureUnit = v
	} else if v, ok := raw["temperature-unit"].(string); ok && v != "" {
		s.TemperatureUnit = v
	}
	if v, ok := raw["windSpeedUnit"].(string); ok && v != "" {
		s.WindSpeedUnit = v
	} else if v, ok := raw["wind-speed-unit"].(string); ok && v != "" {
		s.WindSpeedUnit = v
	}
	if v, ok := raw["enabled"].(bool); ok {
		s.Enabled = v
	}
	if v, ok := raw["provider"].(string); ok && v != "" {
		s.Provider = v
	}
	return s
}

type WeatherWidget struct {
	client *Client
}

func (w *WeatherWidget) Kind() string {
	return "weather"
}

func (w *WeatherWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:          "weather",
		Name:          "Weather",
		Description:   "Current temperature, apparent temperature, and condition from Open-Meteo.",
		DefaultTitle:  "Weather",
		DefaultW:      2,
		DefaultH:      1,
		MinW:          1,
		MinH:          1,
		DefaultSize:   "wide",
		Overflow:      "clip",
		AllowMultiple: false,
	}
}

func (w *WeatherWidget) FetchData(ctx context.Context, raw map[string]any) (any, error) {
	if w.client == nil {
		w.client = NewClient()
	}
	s := parseSettings(raw)
	return w.client.Current(ctx, Request{
		Enabled:         s.Enabled,
		Provider:        s.Provider,
		LocationName:    s.LocationName,
		Latitude:        s.Latitude,
		Longitude:       s.Longitude,
		TemperatureUnit: s.TemperatureUnit,
		WindSpeedUnit:   s.WindSpeedUnit,
	}), nil
}

func (w *WeatherWidget) Skill() *widgetskills.Definition {
	return weatherSkill()
}

func init() {
	widgets.RegisterWithSkill(&WeatherWidget{}, weatherContext)
}

func weatherSkill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          "weather",
		DisplayName:         "Weather",
		Summary:             "Read current weather, temperature, humidity, wind, sunrise, sunset, and freshness for the configured location.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "locationName", Type: "string", Description: "Configured weather location.", Sensitivity: "public"},
			{Name: "condition", Type: "string", Description: "Current weather condition.", Sensitivity: "public"},
			{
				Name:        "temperature",
				Type:        "number",
				Description: "Current temperature.",
				Nullable:    true,
				Sensitivity: "public",
			},
			{Name: "temperatureUnit", Type: "string", Description: "Temperature unit.", Sensitivity: "public"},
			{
				Name:        "humidity",
				Type:        "integer",
				Description: "Current relative humidity percentage.",
				Nullable:    true,
				Sensitivity: "public",
			},
			{
				Name:        "windSpeed",
				Type:        "number",
				Description: "Current wind speed.",
				Nullable:    true,
				Sensitivity: "public",
			},
			{Name: "windSpeedUnit", Type: "string", Description: "Wind speed unit.", Sensitivity: "public"},
			{
				Name:        "sunrise",
				Type:        "datetime",
				Description: "Today's sunrise time when available.",
				Nullable:    true,
				Sensitivity: "public",
			},
			{
				Name:        "sunset",
				Type:        "datetime",
				Description: "Today's sunset time when available.",
				Nullable:    true,
				Sensitivity: "public",
			},
			{
				Name:        "status",
				Type:        "enum",
				Description: "Weather provider status.",
				EnumValues:  []string{"available", "unavailable", "disabled"},
				Sensitivity: "public",
			},
			{
				Name:        "updatedAt",
				Type:        "datetime",
				Description: "Provider update time when available.",
				Nullable:    true,
				Sensitivity: "public",
			},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction(
				"refresh",
				"Refresh weather context",
				"Return the latest public weather context available to the hub.",
			),
		},
		Prompts: []widgetskills.Prompt{{
			ID:      "weather_briefing",
			Title:   "Use weather context",
			Purpose: "Guide an agent when answering weather, clothing, travel, or daylight questions.",
		}},
		SupportedWidgetSizes: []string{"small", "medium", "wide"},
	}
}

func weatherContext(snapshot widgetskills.Snapshot, instanceID string) map[string]any {
	for _, widget := range snapshot.Layout.Widgets {
		if widget.ID != instanceID {
			continue
		}
		switch state := widget.Data.(type) {
		case State:
			return weatherStateContext(state)
		case map[string]any:
			return cleanWeatherMap(state)
		}
		return map[string]any{}
	}
	return map[string]any{}
}

func weatherStateContext(state State) map[string]any {
	return map[string]any{
		"locationName":        state.LocationName,
		"condition":           state.Condition,
		"temperature":         pointerValue(state.Temperature),
		"temperatureUnit":     state.TemperatureUnit,
		"apparentTemperature": pointerValue(state.ApparentTemperature),
		"humidity":            pointerValue(state.Humidity),
		"windSpeed":           pointerValue(state.WindSpeed),
		"windSpeedUnit":       state.WindSpeedUnit,
		"sunrise":             emptyToNil(state.Sunrise),
		"sunset":              emptyToNil(state.Sunset),
		"isDay":               pointerValue(state.IsDay),
		"status":              state.Status,
		"updatedAt":           emptyToNil(state.UpdatedAt),
		"source":              state.Source,
	}
}

func cleanWeatherMap(state map[string]any) map[string]any {
	allowed := map[string]struct{}{
		"locationName": {}, "condition": {}, "temperature": {}, "temperatureUnit": {}, "apparentTemperature": {},
		"humidity": {}, "windSpeed": {}, "windSpeedUnit": {}, "sunrise": {}, "sunset": {}, "isDay": {},
		"status": {}, "updatedAt": {}, "source": {}, "weatherCode": {}, "icon": {},
	}
	cleaned := make(map[string]any, len(allowed))
	for key, value := range state {
		if _, ok := allowed[key]; ok {
			cleaned[key] = value
		}
	}
	return cleaned
}

func emptyToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func pointerValue[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}
