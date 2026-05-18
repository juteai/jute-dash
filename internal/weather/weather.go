package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"jute-dash/internal/config"
)

const (
	ProviderOpenMeteo = "open-meteo"

	StatusAvailable   = "available"
	StatusUnavailable = "unavailable"
	StatusDisabled    = "disabled"
)

type State struct {
	LocationName        string   `json:"locationName"`
	Temperature         *float64 `json:"temperature"`
	TemperatureUnit     string   `json:"temperatureUnit"`
	ApparentTemperature *float64 `json:"apparentTemperature"`
	Condition           string   `json:"condition"`
	Icon                string   `json:"icon"`
	WeatherCode         *int     `json:"weatherCode"`
	Humidity            *int     `json:"humidity"`
	WindSpeed           *float64 `json:"windSpeed"`
	WindSpeedUnit       string   `json:"windSpeedUnit"`
	Sunrise             string   `json:"sunrise"`
	Sunset              string   `json:"sunset"`
	IsDay               *bool    `json:"isDay"`
	UpdatedAt           string   `json:"updatedAt"`
	Source              string   `json:"source"`
	Status              string   `json:"status"`
}

type Provider interface {
	Current(context.Context, config.WeatherConfig) State
}

type Client struct {
	httpClient *http.Client
	endpoint   string
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

func WithEndpoint(endpoint string) Option {
	return func(client *Client) {
		client.endpoint = endpoint
	}
}

func NewClient(options ...Option) *Client {
	client := &Client{
		httpClient: &http.Client{Timeout: 4 * time.Second},
		endpoint:   "https://api.open-meteo.com/v1/forecast",
	}
	for _, option := range options {
		option(client)
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: 4 * time.Second}
	}
	return client
}

func (client *Client) Current(ctx context.Context, cfg config.WeatherConfig) State {
	if !cfg.Enabled {
		return disabledState(cfg)
	}
	if cfg.Provider != ProviderOpenMeteo {
		return unavailableState(cfg)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.forecastURL(cfg), nil)
	if err != nil {
		return unavailableState(cfg)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return unavailableState(cfg)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return unavailableState(cfg)
	}

	var payload openMeteoResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return unavailableState(cfg)
	}

	return mapOpenMeteo(cfg, payload)
}

func (client *Client) forecastURL(cfg config.WeatherConfig) string {
	values := url.Values{}
	values.Set("latitude", strconv.FormatFloat(cfg.Latitude, 'f', -1, 64))
	values.Set("longitude", strconv.FormatFloat(cfg.Longitude, 'f', -1, 64))
	values.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,weather_code,wind_speed_10m,is_day")
	values.Set("daily", "sunrise,sunset")
	values.Set("forecast_days", "1")
	values.Set("timezone", "auto")
	values.Set("temperature_unit", cfg.TemperatureUnit)
	values.Set("wind_speed_unit", cfg.WindSpeedUnit)

	return fmt.Sprintf("%s?%s", client.endpoint, values.Encode())
}

func mapOpenMeteo(cfg config.WeatherConfig, payload openMeteoResponse) State {
	code := payload.Current.WeatherCode
	isDay := payload.Current.IsDay == 1
	condition, icon := conditionForCode(code, isDay)

	return State{
		LocationName:        cfg.LocationName,
		Temperature:         &payload.Current.Temperature,
		TemperatureUnit:     firstNonEmpty(payload.CurrentUnits.Temperature, unitLabel(cfg.TemperatureUnit)),
		ApparentTemperature: &payload.Current.ApparentTemperature,
		Condition:           condition,
		Icon:                icon,
		WeatherCode:         &code,
		Humidity:            &payload.Current.Humidity,
		WindSpeed:           &payload.Current.WindSpeed,
		WindSpeedUnit:       firstNonEmpty(payload.CurrentUnits.WindSpeed, windUnitLabel(cfg.WindSpeedUnit)),
		Sunrise:             first(payload.Daily.Sunrise),
		Sunset:              first(payload.Daily.Sunset),
		IsDay:               &isDay,
		UpdatedAt:           payload.Current.Time,
		Source:              ProviderOpenMeteo,
		Status:              StatusAvailable,
	}
}

func disabledState(cfg config.WeatherConfig) State {
	return State{
		LocationName:    cfg.LocationName,
		TemperatureUnit: unitLabel(cfg.TemperatureUnit),
		WindSpeedUnit:   windUnitLabel(cfg.WindSpeedUnit),
		Condition:       "Weather disabled",
		Icon:            "cloud",
		Source:          ProviderOpenMeteo,
		Status:          StatusDisabled,
	}
}

func unavailableState(cfg config.WeatherConfig) State {
	return State{
		LocationName:    cfg.LocationName,
		TemperatureUnit: unitLabel(cfg.TemperatureUnit),
		WindSpeedUnit:   windUnitLabel(cfg.WindSpeedUnit),
		Condition:       "Weather unavailable",
		Icon:            "cloud",
		Source:          ProviderOpenMeteo,
		Status:          StatusUnavailable,
	}
}

func conditionForCode(code int, isDay bool) (string, string) {
	switch code {
	case 0:
		if isDay {
			return "Clear sky", "sun"
		}
		return "Clear sky", "moon"
	case 1:
		return "Mainly clear", "cloud-sun"
	case 2:
		return "Partly cloudy", "cloud-sun"
	case 3:
		return "Overcast", "cloud"
	case 45, 48:
		return "Fog", "fog"
	case 51, 53, 55, 56, 57:
		return "Drizzle", "cloud-drizzle"
	case 61, 63, 65, 66, 67:
		return "Rain", "cloud-rain"
	case 71, 73, 75, 77:
		return "Snow", "snowflake"
	case 80, 81, 82:
		return "Rain showers", "cloud-rain"
	case 85, 86:
		return "Snow showers", "snowflake"
	case 95, 96, 99:
		return "Thunderstorm", "cloud-lightning"
	default:
		return "Weather update", "cloud"
	}
}

func unitLabel(unit string) string {
	if unit == "fahrenheit" {
		return "°F"
	}
	return "°C"
}

func windUnitLabel(unit string) string {
	switch unit {
	case "mph":
		return "mph"
	case "ms":
		return "m/s"
	case "kn":
		return "kn"
	default:
		return "km/h"
	}
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type openMeteoResponse struct {
	CurrentUnits openMeteoCurrentUnits `json:"current_units"`
	Current      openMeteoCurrent      `json:"current"`
	Daily        openMeteoDaily        `json:"daily"`
}

type openMeteoCurrentUnits struct {
	Temperature string `json:"temperature_2m"`
	WindSpeed   string `json:"wind_speed_10m"`
}

type openMeteoCurrent struct {
	Time                string  `json:"time"`
	Temperature         float64 `json:"temperature_2m"`
	ApparentTemperature float64 `json:"apparent_temperature"`
	Humidity            int     `json:"relative_humidity_2m"`
	WeatherCode         int     `json:"weather_code"`
	WindSpeed           float64 `json:"wind_speed_10m"`
	IsDay               int     `json:"is_day"`
}

type openMeteoDaily struct {
	Sunrise []string `json:"sunrise"`
	Sunset  []string `json:"sunset"`
}
