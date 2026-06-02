package weather

import (
	"context"
	"strings"

	"jute-dash/internal/config"
	"jute-dash/internal/weather"
	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

const SkillID = "jute.weather.current"

type WeatherWidget struct {
	client *weather.Client
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

func (w *WeatherWidget) FetchData(ctx context.Context, settings map[string]any) (any, error) {
	if w.client == nil {
		w.client = weather.NewClient()
	}

	cfg := config.WeatherConfig{
		Enabled:         true,
		Provider:        "open-meteo",
		LocationName:    "London",
		Latitude:        51.5072,
		Longitude:       -0.1276,
		TemperatureUnit: "celsius",
		WindSpeedUnit:   "kmh",
	}

	if val, ok := settings["locationName"].(string); ok && val != "" {
		cfg.LocationName = val
	}
	if val, ok := settings["location"].(string); ok && val != "" {
		cfg.LocationName = val
	}
	if val, ok := settings["latitude"].(float64); ok {
		cfg.Latitude = val
	}
	if val, ok := settings["longitude"].(float64); ok {
		cfg.Longitude = val
	}
	if val, ok := settings["temperatureUnit"].(string); ok && val != "" {
		cfg.TemperatureUnit = val
	}
	if val, ok := settings["temperature-unit"].(string); ok && val != "" {
		cfg.TemperatureUnit = val
	}
	if val, ok := settings["windSpeedUnit"].(string); ok && val != "" {
		cfg.WindSpeedUnit = val
	}
	if val, ok := settings["wind-speed-unit"].(string); ok && val != "" {
		cfg.WindSpeedUnit = val
	}
	if val, ok := settings["enabled"].(bool); ok {
		cfg.Enabled = val
	}
	if val, ok := settings["provider"].(string); ok && val != "" {
		cfg.Provider = val
	}

	state := w.client.Current(ctx, cfg)
	return state, nil
}

func (w *WeatherWidget) Skill() *widgetskills.Definition {
	return weatherSkill()
}

func init() {
	widget := &WeatherWidget{}
	widgets.Register(widget)
	widgetskills.Register(*widget.Skill(), weatherContext)
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
			{Name: "temperature", Type: "number", Description: "Current temperature.", Nullable: true, Sensitivity: "public"},
			{Name: "temperatureUnit", Type: "string", Description: "Temperature unit.", Sensitivity: "public"},
			{Name: "humidity", Type: "integer", Description: "Current relative humidity percentage.", Nullable: true, Sensitivity: "public"},
			{Name: "windSpeed", Type: "number", Description: "Current wind speed.", Nullable: true, Sensitivity: "public"},
			{Name: "windSpeedUnit", Type: "string", Description: "Wind speed unit.", Sensitivity: "public"},
			{Name: "sunrise", Type: "datetime", Description: "Today's sunrise time when available.", Nullable: true, Sensitivity: "public"},
			{Name: "sunset", Type: "datetime", Description: "Today's sunset time when available.", Nullable: true, Sensitivity: "public"},
			{Name: "status", Type: "enum", Description: "Weather provider status.", EnumValues: []string{"available", "unavailable", "disabled"}, Sensitivity: "public"},
			{Name: "updatedAt", Type: "datetime", Description: "Provider update time when available.", Nullable: true, Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("refresh", "Refresh weather context", "Return the latest public weather context available to the hub."),
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
		case weather.State:
			return weatherStateContext(state)
		case map[string]any:
			return cleanWeatherMap(state)
		}
		return map[string]any{}
	}
	return map[string]any{}
}

func weatherStateContext(state weather.State) map[string]any {
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
