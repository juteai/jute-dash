package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"jute-dash/internal/a2a"

	"go.yaml.in/yaml/v4"
)

type Config struct {
	Home    HomeConfig    `json:"home" yaml:"home"`
	Server  ServerConfig  `json:"server" yaml:"server"`
	Display DisplayConfig `json:"display" yaml:"display"`
	Weather WeatherConfig `json:"weather" yaml:"weather"`
	Agents  []AgentConfig `json:"agents" yaml:"agents"`
	Rooms   []RoomConfig  `json:"rooms" yaml:"rooms"`
	Tiles   []TileConfig  `json:"tiles" yaml:"tiles"`
}

type HomeConfig struct {
	Name     string `json:"name" yaml:"name"`
	Timezone string `json:"timezone" yaml:"timezone"`
	Locale   string `json:"locale" yaml:"locale"`
}

type ServerConfig struct {
	ListenAddress string `json:"listenAddress" yaml:"listen-address"`
}

type DisplayConfig struct {
	Theme       string `json:"theme" yaml:"theme"`
	AccentColor string `json:"accentColor" yaml:"accent-color"`
	IdleMode    string `json:"idleMode" yaml:"idle-mode"`
}

type WeatherConfig struct {
	Enabled         bool    `json:"enabled" yaml:"enabled"`
	Provider        string  `json:"provider" yaml:"provider"`
	LocationName    string  `json:"locationName" yaml:"location-name"`
	Latitude        float64 `json:"latitude" yaml:"latitude"`
	Longitude       float64 `json:"longitude" yaml:"longitude"`
	TemperatureUnit string  `json:"temperatureUnit" yaml:"temperature-unit"`
	WindSpeedUnit   string  `json:"windSpeedUnit" yaml:"wind-speed-unit"`
}

type AgentConfig struct {
	ID              string      `json:"id" yaml:"id"`
	Name            string      `json:"name" yaml:"name"`
	Description     string      `json:"description" yaml:"description"`
	CardURL         string      `json:"cardUrl" yaml:"card-url"`
	EndpointURL     string      `json:"endpointUrl" yaml:"endpoint-url"`
	ProtocolBinding string      `json:"protocolBinding" yaml:"protocol-binding"`
	Enabled         bool        `json:"enabled" yaml:"enabled"`
	Capabilities    []string    `json:"capabilities" yaml:"capabilities"`
	Auth            *AuthConfig `json:"auth,omitempty" yaml:"auth,omitempty"`
}

type AuthConfig struct {
	Type     string `json:"type" yaml:"type"`
	EnvToken string `json:"envToken" yaml:"env-token"`
}

type RoomConfig struct {
	ID      string `json:"id" yaml:"id"`
	Name    string `json:"name" yaml:"name"`
	Summary string `json:"summary" yaml:"summary"`
	Status  string `json:"status" yaml:"status"`
}

type TileConfig struct {
	ID     string `json:"id" yaml:"id"`
	Kind   string `json:"kind" yaml:"kind"`
	Label  string `json:"label" yaml:"label"`
	Value  string `json:"value" yaml:"value"`
	Detail string `json:"detail" yaml:"detail"`
}

type PublicConfig struct {
	Home    HomeConfig          `json:"home" yaml:"home"`
	Display DisplayConfig       `json:"display" yaml:"display"`
	Agents  []PublicAgentConfig `json:"agents" yaml:"agents"`
	Rooms   []RoomConfig        `json:"rooms" yaml:"rooms"`
	Tiles   []TileConfig        `json:"tiles" yaml:"tiles"`
}

type PublicAgentConfig struct {
	ID              string   `json:"id" yaml:"id"`
	Name            string   `json:"name" yaml:"name"`
	Description     string   `json:"description" yaml:"description"`
	CardURL         string   `json:"cardUrl" yaml:"card-url"`
	EndpointURL     string   `json:"endpointUrl" yaml:"endpoint-url"`
	ProtocolBinding string   `json:"protocolBinding" yaml:"protocol-binding"`
	Enabled         bool     `json:"enabled" yaml:"enabled"`
	Capabilities    []string `json:"capabilities" yaml:"capabilities"`
	AuthConfigured  bool     `json:"authConfigured" yaml:"auth-configured"`
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

	if err := decode(file, path, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	applyDefaults(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func decode(file *os.File, path string, cfg *Config) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		decoder := yaml.NewDecoder(file)
		decoder.KnownFields(true)
		return decoder.Decode(cfg)
	case ".json", "":
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		return decoder.Decode(cfg)
	default:
		return fmt.Errorf("unsupported config file extension %q", filepath.Ext(path))
	}
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
