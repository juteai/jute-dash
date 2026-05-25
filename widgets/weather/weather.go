package weather

import (
	"context"
	"jute-dash/internal/config"
	"jute-dash/internal/weather"
	"jute-dash/internal/widgetskills"
	"jute-dash/widgets"
)

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

	if val, ok := settings["location"].(string); ok && val != "" {
		cfg.LocationName = val
	}
	if val, ok := settings["latitude"].(float64); ok {
		cfg.Latitude = val
	}
	if val, ok := settings["longitude"].(float64); ok {
		cfg.Longitude = val
	}
	if val, ok := settings["temperature-unit"].(string); ok && val != "" {
		cfg.TemperatureUnit = val
	}
	if val, ok := settings["wind-speed-unit"].(string); ok && val != "" {
		cfg.WindSpeedUnit = val
	}

	state := w.client.Current(ctx, cfg)
	return state, nil
}

func (w *WeatherWidget) Skill() *widgetskills.Definition {
	return nil
}

func init() {
	widgets.Register(&WeatherWidget{})
}
