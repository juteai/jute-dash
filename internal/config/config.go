package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"jute-dash/internal/a2a"
)

type Config struct {
	Home    HomeConfig    `json:"home"`
	Server  ServerConfig  `json:"server"`
	Display DisplayConfig `json:"display"`
	Weather WeatherConfig `json:"weather"`
	Agents  []AgentConfig `json:"agents"`
	Rooms   []RoomConfig  `json:"rooms"`
	Tiles   []TileConfig  `json:"tiles"`
}

type HomeConfig struct {
	Name     string `json:"name"`
	Timezone string `json:"timezone"`
	Locale   string `json:"locale"`
}

type ServerConfig struct {
	ListenAddress string `json:"listenAddress"`
}

type DisplayConfig struct {
	Theme       string `json:"theme"`
	AccentColor string `json:"accentColor"`
	IdleMode    string `json:"idleMode"`
}

type WeatherConfig struct {
	Enabled         bool    `json:"enabled"`
	Provider        string  `json:"provider"`
	LocationName    string  `json:"locationName"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	TemperatureUnit string  `json:"temperatureUnit"`
	WindSpeedUnit   string  `json:"windSpeedUnit"`
}

type AgentConfig struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	CardURL         string      `json:"cardUrl"`
	EndpointURL     string      `json:"endpointUrl"`
	ProtocolBinding string      `json:"protocolBinding"`
	Enabled         bool        `json:"enabled"`
	Capabilities    []string    `json:"capabilities"`
	Auth            *AuthConfig `json:"auth,omitempty"`
}

type AuthConfig struct {
	Type     string `json:"type"`
	EnvToken string `json:"envToken"`
}

type RoomConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type TileConfig struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

type PublicConfig struct {
	Home    HomeConfig          `json:"home"`
	Display DisplayConfig       `json:"display"`
	Agents  []PublicAgentConfig `json:"agents"`
	Rooms   []RoomConfig        `json:"rooms"`
	Tiles   []TileConfig        `json:"tiles"`
}

type PublicAgentConfig struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	CardURL         string   `json:"cardUrl"`
	EndpointURL     string   `json:"endpointUrl"`
	ProtocolBinding string   `json:"protocolBinding"`
	Enabled         bool     `json:"enabled"`
	Capabilities    []string `json:"capabilities"`
	AuthConfigured  bool     `json:"authConfigured"`
}

func Default() Config {
	return Config{
		Home: HomeConfig{
			Name:     "Jute Home",
			Timezone: "UTC",
			Locale:   "en",
		},
		Server: ServerConfig{
			ListenAddress: "127.0.0.1:8787",
		},
		Display: DisplayConfig{
			Theme:       "system",
			AccentColor: "teal",
			IdleMode:    "ambient",
		},
		Weather: WeatherConfig{
			Enabled:         true,
			Provider:        "open-meteo",
			LocationName:    "London",
			Latitude:        51.5072,
			Longitude:       -0.1276,
			TemperatureUnit: "celsius",
			WindSpeedUnit:   "kmh",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func Validate(cfg Config) error {
	var problems []string

	if strings.TrimSpace(cfg.Home.Name) == "" {
		problems = append(problems, "home.name is required")
	}
	if strings.TrimSpace(cfg.Server.ListenAddress) == "" {
		problems = append(problems, "server.listenAddress is required")
	}
	if cfg.Weather.Enabled {
		if cfg.Weather.Provider != "open-meteo" {
			problems = append(problems, "weather.provider must be open-meteo")
		}
		if strings.TrimSpace(cfg.Weather.LocationName) == "" {
			problems = append(problems, "weather.locationName is required")
		}
		if cfg.Weather.Latitude < -90 || cfg.Weather.Latitude > 90 {
			problems = append(problems, "weather.latitude must be between -90 and 90")
		}
		if cfg.Weather.Longitude < -180 || cfg.Weather.Longitude > 180 {
			problems = append(problems, "weather.longitude must be between -180 and 180")
		}
		if !isSupportedTemperatureUnit(cfg.Weather.TemperatureUnit) {
			problems = append(problems, "weather.temperatureUnit must be celsius or fahrenheit")
		}
		if !isSupportedWindSpeedUnit(cfg.Weather.WindSpeedUnit) {
			problems = append(problems, "weather.windSpeedUnit must be kmh, mph, ms, or kn")
		}
	}

	seenAgents := map[string]struct{}{}
	for i, agent := range cfg.Agents {
		location := fmt.Sprintf("agents[%d]", i)
		if strings.TrimSpace(agent.ID) == "" {
			problems = append(problems, location+".id is required")
		}
		if _, exists := seenAgents[agent.ID]; agent.ID != "" && exists {
			problems = append(problems, location+".id duplicates another agent")
		}
		seenAgents[agent.ID] = struct{}{}
		if strings.TrimSpace(agent.Name) == "" {
			problems = append(problems, location+".name is required")
		}
		if err := validateHTTPURL(agent.CardURL); err != nil {
			problems = append(problems, location+".cardUrl "+err.Error())
		}
		if err := validateHTTPURL(agent.EndpointURL); err != nil {
			problems = append(problems, location+".endpointUrl "+err.Error())
		}
		if !a2a.IsSupportedProtocolBinding(agent.ProtocolBinding) {
			problems = append(problems, location+".protocolBinding must be JSONRPC, HTTP+JSON, or GRPC")
		}
		if agent.Auth != nil && strings.TrimSpace(agent.Auth.EnvToken) == "" {
			problems = append(problems, location+".auth.envToken is required when auth is configured")
		}
	}

	validateUniqueIDs("rooms", cfg.Rooms, func(room RoomConfig) string { return room.ID }, &problems)
	validateUniqueIDs("tiles", cfg.Tiles, func(tile TileConfig) string { return tile.ID }, &problems)

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (cfg Config) Public() PublicConfig {
	publicAgents := make([]PublicAgentConfig, 0, len(cfg.Agents))
	for _, agent := range cfg.Agents {
		publicAgents = append(publicAgents, PublicAgentConfig{
			ID:              agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			CardURL:         agent.CardURL,
			EndpointURL:     agent.EndpointURL,
			ProtocolBinding: agent.ProtocolBinding,
			Enabled:         agent.Enabled,
			Capabilities:    append([]string(nil), agent.Capabilities...),
			AuthConfigured:  agent.Auth != nil,
		})
	}

	return PublicConfig{
		Home:    cfg.Home,
		Display: cfg.Display,
		Agents:  publicAgents,
		Rooms:   append([]RoomConfig(nil), cfg.Rooms...),
		Tiles:   append([]TileConfig(nil), cfg.Tiles...),
	}
}

func applyDefaults(cfg *Config) {
	defaults := Default()
	if strings.TrimSpace(cfg.Home.Name) == "" {
		cfg.Home.Name = defaults.Home.Name
	}
	if strings.TrimSpace(cfg.Home.Timezone) == "" {
		cfg.Home.Timezone = defaults.Home.Timezone
	}
	if strings.TrimSpace(cfg.Home.Locale) == "" {
		cfg.Home.Locale = defaults.Home.Locale
	}
	if strings.TrimSpace(cfg.Server.ListenAddress) == "" {
		cfg.Server.ListenAddress = defaults.Server.ListenAddress
	}
	if strings.TrimSpace(cfg.Display.Theme) == "" {
		cfg.Display.Theme = defaults.Display.Theme
	}
	if strings.TrimSpace(cfg.Display.AccentColor) == "" {
		cfg.Display.AccentColor = defaults.Display.AccentColor
	}
	if strings.TrimSpace(cfg.Display.IdleMode) == "" {
		cfg.Display.IdleMode = defaults.Display.IdleMode
	}
	if strings.TrimSpace(cfg.Weather.Provider) == "" {
		cfg.Weather.Provider = defaults.Weather.Provider
	}
	if strings.TrimSpace(cfg.Weather.LocationName) == "" {
		cfg.Weather.LocationName = defaults.Weather.LocationName
	}
	if strings.TrimSpace(cfg.Weather.TemperatureUnit) == "" {
		cfg.Weather.TemperatureUnit = defaults.Weather.TemperatureUnit
	}
	if strings.TrimSpace(cfg.Weather.WindSpeedUnit) == "" {
		cfg.Weather.WindSpeedUnit = defaults.Weather.WindSpeedUnit
	}
	for i := range cfg.Agents {
		if strings.TrimSpace(cfg.Agents[i].ProtocolBinding) == "" {
			cfg.Agents[i].ProtocolBinding = a2a.ProtocolJSONRPC
		}
	}
}

func validateHTTPURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("is invalid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("must include a host")
	}
	return nil
}

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

func validateUniqueIDs[T any](name string, values []T, getID func(T) string, problems *[]string) {
	seen := map[string]struct{}{}
	for i, value := range values {
		id := getID(value)
		location := fmt.Sprintf("%s[%d]", name, i)
		if strings.TrimSpace(id) == "" {
			*problems = append(*problems, location+".id is required")
			continue
		}
		if _, exists := seen[id]; exists {
			*problems = append(*problems, location+".id duplicates another "+strings.TrimSuffix(name, "s"))
			continue
		}
		seen[id] = struct{}{}
	}
}
