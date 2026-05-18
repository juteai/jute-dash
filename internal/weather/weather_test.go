package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"jute-dash/internal/config"
)

func TestCurrentReturnsDisabledWithoutFetching(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	defer server.Close()

	client := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	cfg := config.Default().Weather
	cfg.Enabled = false

	state := client.Current(context.Background(), cfg)

	if state.Status != StatusDisabled {
		t.Fatalf("expected disabled status, got %s", state.Status)
	}
	if atomic.LoadInt32(&requests) != 0 {
		t.Fatal("disabled weather should not call provider")
	}
}

func TestCurrentMapsOpenMeteoResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !strings.Contains(query.Get("current"), "temperature_2m") {
			t.Fatalf("missing current temperature request: %s", query.Get("current"))
		}
		if query.Get("daily") != "sunrise,sunset" {
			t.Fatalf("unexpected daily query: %s", query.Get("daily"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"current_units": {
				"temperature_2m": "°C",
				"wind_speed_10m": "km/h"
			},
			"current": {
				"time": "2026-05-16T18:00",
				"temperature_2m": 17.8,
				"apparent_temperature": 16.9,
				"relative_humidity_2m": 68,
				"weather_code": 2,
				"wind_speed_10m": 11.4,
				"is_day": 1
			},
			"daily": {
				"sunrise": ["2026-05-16T05:04"],
				"sunset": ["2026-05-16T20:47"]
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	cfg := config.Default().Weather

	state := client.Current(context.Background(), cfg)

	if state.Status != StatusAvailable {
		t.Fatalf("expected available status, got %s", state.Status)
	}
	if state.Condition != "Partly cloudy" {
		t.Fatalf("unexpected condition: %s", state.Condition)
	}
	if state.Icon != "cloud-sun" {
		t.Fatalf("unexpected icon: %s", state.Icon)
	}
	if state.Temperature == nil || *state.Temperature != 17.8 {
		t.Fatalf("unexpected temperature: %v", state.Temperature)
	}
	if state.Humidity == nil || *state.Humidity != 68 {
		t.Fatalf("unexpected humidity: %v", state.Humidity)
	}
	if state.Sunset != "2026-05-16T20:47" {
		t.Fatalf("unexpected sunset: %s", state.Sunset)
	}
}

func TestCurrentReturnsUnavailableOnProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewClient(WithEndpoint(server.URL), WithHTTPClient(server.Client()))
	state := client.Current(context.Background(), config.Default().Weather)

	if state.Status != StatusUnavailable {
		t.Fatalf("expected unavailable status, got %s", state.Status)
	}
	if state.Condition != "Weather unavailable" {
		t.Fatalf("unexpected condition: %s", state.Condition)
	}
}
