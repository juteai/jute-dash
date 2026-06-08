package weather

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"jute-dash/apps/hub/pkg/widgetskills"
)

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (f mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWeatherWidget_FetchData(t *testing.T) {
	sampleJSON := `{
  "current_units": {
    "temperature_2m": "°C",
    "wind_speed_10m": "km/h"
  },
  "current": {
    "time": "2026-06-08T20:00:00Z",
    "temperature_2m": 22.5,
    "apparent_temperature": 23.0,
    "relative_humidity_2m": 55,
    "weather_code": 0,
    "wind_speed_10m": 12.0,
    "is_day": 1
  },
  "daily": {
    "sunrise": ["2026-06-08T05:00:00Z"],
    "sunset": ["2026-06-08T21:00:00Z"]
  }
}`

	httpClient := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(sampleJSON)),
			}, nil
		}),
	}

	w := &WeatherWidget{
		client: NewClient(WithHTTPClient(httpClient)),
	}

	settings := map[string]any{
		"locationName":     "London",
		"latitude":         51.5072,
		"longitude":        -0.1276,
		"temperature-unit": "celsius",
		"wind-speed-unit":  "kmh",
		"enabled":          true,
		"provider":         "open-meteo",
	}

	data, err := w.FetchData(context.Background(), settings)
	if err != nil {
		t.Fatalf("FetchData error: %v", err)
	}

	state, ok := data.(State)
	if !ok {
		t.Fatalf("expected State, got %T", data)
	}

	if state.LocationName != "London" ||
		*state.Temperature != 22.5 ||
		state.Condition != "Clear sky" ||
		state.Status != StatusAvailable {
		t.Errorf("unexpected state values: %+v", state)
	}
}

func TestWeatherWidget_ParseSettings(t *testing.T) {
	raw := map[string]any{
		"locationName":    "Paris",
		"latitude":        48.8566,
		"longitude":       2.3522,
		"temperatureUnit": "fahrenheit",
		"windSpeedUnit":   "mph",
		"provider":        "open-meteo",
	}

	s := parseSettings(raw)
	if s.LocationName != "Paris" ||
		s.Latitude != 48.8566 ||
		s.Longitude != 2.3522 ||
		s.TemperatureUnit != "fahrenheit" ||
		s.WindSpeedUnit != "mph" {
		t.Errorf("unexpected parsed settings: %+v", s)
	}
}

func TestWeatherWidget_WeatherContext(t *testing.T) {
	temp := 22.5
	state := State{
		LocationName:    "London",
		Temperature:     &temp,
		TemperatureUnit: "°C",
		Condition:       "Clear sky",
		Status:          StatusAvailable,
		Source:          "open-meteo",
	}

	snapshot := widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:   "inst1",
					Kind: "weather",
					Data: state,
				},
			},
		},
	}

	ctx := weatherContext(snapshot, "inst1")
	if ctx["locationName"] != "London" || ctx["temperature"] != 22.5 || ctx["condition"] != "Clear sky" {
		t.Errorf("unexpected weather context: %+v", ctx)
	}
}
