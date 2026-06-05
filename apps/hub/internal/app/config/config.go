package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/dashboard"
	"jute-dash/apps/hub/internal/app/homestate"
	"jute-dash/apps/hub/internal/app/mcp"
	"jute-dash/apps/hub/internal/app/voice"
	"jute-dash/apps/hub/internal/pkg/a2a"

	"go.yaml.in/yaml/v4"
)

type Config struct {
	Home      homestate.HomeConfig      `json:"home"      yaml:"home"`
	Server    ServerConfig              `json:"server"    yaml:"server"`
	MCP       mcp.Config                `json:"mcp"       yaml:"mcp"`
	A2A       a2a.AgentCardURLPolicy    `json:"a2a"       yaml:"a2a"`
	Display   dashboard.DisplayConfig   `json:"display"   yaml:"display"`
	Weather   homestate.WeatherConfig   `json:"weather"   yaml:"weather"`
	Voice     voice.Config              `json:"voice"     yaml:"voice"`
	Dashboard dashboard.DashboardConfig `json:"dashboard" yaml:"dashboard"`
	Agents    []agents.AgentConfig      `json:"agents"    yaml:"agents"`
	Rooms     []homestate.RoomConfig    `json:"rooms"     yaml:"rooms"`
	Tiles     []homestate.TileConfig    `json:"tiles"     yaml:"tiles"`
}

type ServerConfig struct {
	ListenAddress string `json:"listenAddress" yaml:"listen-address"`
}

type PublicConfig struct {
	Home      homestate.HomeConfig       `json:"home"      yaml:"home"`
	Display   dashboard.DisplayConfig    `json:"display"   yaml:"display"`
	Dashboard dashboard.DashboardConfig  `json:"dashboard" yaml:"dashboard"`
	Agents    []agents.PublicAgentConfig `json:"agents"    yaml:"agents"`
	Rooms     []homestate.RoomConfig     `json:"rooms"     yaml:"rooms"`
	Tiles     []homestate.TileConfig     `json:"tiles"     yaml:"tiles"`
}

func DefaultConfig() Config {
	return Config{
		Home: homestate.DefaultHomeConfig(),
		Server: ServerConfig{
			ListenAddress: "127.0.0.1:8787",
		},
		MCP:     mcp.DefaultConfig(),
		A2A:     a2a.DefaultAgentCardURLPolicy(),
		Display: dashboard.DefaultDisplayConfig(),
		Weather: homestate.DefaultWeatherConfig(),
		Voice:   voice.DefaultConfig(),
	}
}

func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	if err := decodeConfig(file, path, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if err := EnsureValidConfig(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func decodeConfig(file *os.File, path string, cfg *Config) error {
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

func ValidateConfig(cfg Config) error {
	var problems []string

	problems = append(problems, homestate.ValidateHome(cfg.Home)...)

	if strings.TrimSpace(cfg.Server.ListenAddress) == "" {
		problems = append(problems, "server.listenAddress is required")
	}

	problems = append(problems, mcp.Validate(cfg.MCP)...)
	problems = append(problems, a2a.ValidateAgentCardURLPolicy(cfg.A2A)...)
	problems = append(problems, dashboard.ValidateDisplay(cfg.Display)...)
	problems = append(problems, homestate.ValidateWeather(cfg.Weather)...)
	problems = append(problems, voice.Validate(cfg.Voice)...)

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
		seenScopes := map[string]struct{}{}
		for j, scope := range agent.MCPScopes {
			scopeLocation := fmt.Sprintf("%s.mcpScopes[%d]", location, j)
			if strings.TrimSpace(scope) == "" {
				problems = append(problems, scopeLocation+" is required")
				continue
			}
			if !agents.IsKnownMCPScope(scope) {
				problems = append(problems, scopeLocation+" is not supported")
			}
			if _, exists := seenScopes[scope]; exists {
				problems = append(problems, scopeLocation+" duplicates another MCP scope")
			}
			seenScopes[scope] = struct{}{}
		}
	}

	validateUniqueIDs("rooms", cfg.Rooms, func(room homestate.RoomConfig) string { return room.ID }, &problems)
	validateUniqueIDs("tiles", cfg.Tiles, func(tile homestate.TileConfig) string { return tile.ID }, &problems)

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (cfg Config) Public() PublicConfig {
	publicAgents := make([]agents.PublicAgentConfig, 0, len(cfg.Agents))
	for _, agent := range cfg.Agents {
		publicAgents = append(publicAgents, agents.PublicAgentConfig{
			ID:              agent.ID,
			Name:            agent.Name,
			Description:     agent.Description,
			CardURL:         agent.CardURL,
			EndpointURL:     agent.EndpointURL,
			ProtocolBinding: agent.ProtocolBinding,
			Enabled:         agent.Enabled,
			Capabilities:    append([]string(nil), agent.Capabilities...),
			MCPScopes:       append([]string(nil), agent.MCPScopes...),
			AuthConfigured:  agent.Auth != nil,
		})
	}

	return PublicConfig{
		Home:      cfg.Home,
		Display:   cfg.Display,
		Dashboard: cfg.Dashboard,
		Agents:    publicAgents,
		Rooms:     append([]homestate.RoomConfig(nil), cfg.Rooms...),
		Tiles:     append([]homestate.TileConfig(nil), cfg.Tiles...),
	}
}

func ApplyDefaults(cfg *Config) {
	homestate.ApplyHomeDefaults(&cfg.Home)
	if strings.TrimSpace(cfg.Server.ListenAddress) == "" {
		cfg.Server.ListenAddress = "127.0.0.1:8787"
	}
	mcp.ApplyDefaults(&cfg.MCP)
	if cfg.A2A.Loopback == nil {
		allowLoopback := true
		cfg.A2A.Loopback = &allowLoopback
	}
	dashboard.ApplyDisplayDefaults(&cfg.Display)
	homestate.ApplyWeatherDefaults(&cfg.Weather)
	voice.ApplyDefaults(&cfg.Voice)

	if len(cfg.Dashboard.Widgets) == 0 {
		cfg.Dashboard.Widgets = []dashboard.DashboardWidgetConfig{
			{ID: "date-time", Type: "date-time", Title: "Date & Time", X: 0, Y: 0, W: 6, H: 1, MinW: 3, MinH: 1, Size: "wide", Visible: true},
			{ID: "weather", Type: "weather", Title: "Weather", X: 6, Y: 0, W: 6, H: 1, MinW: 3, MinH: 1, Size: "wide", Visible: true},
			{ID: "chat-history", Type: "chat-history", Title: "Chat History", X: 0, Y: 1, W: 6, H: 2, MinW: 3, MinH: 1, Size: "medium", Visible: true},
		}
	}
	for i := range cfg.Agents {
		if strings.TrimSpace(cfg.Agents[i].ProtocolBinding) == "" {
			cfg.Agents[i].ProtocolBinding = a2a.ProtocolJSONRPC
		}
		if len(cfg.Agents[i].MCPScopes) == 0 {
			cfg.Agents[i].MCPScopes = agents.DefaultMCPReadScopes()
		}
	}
}

func EnsureValidConfig(cfg *Config) error {
	ApplyDefaults(cfg)
	return ValidateConfig(*cfg)
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

func SaveYAML(path string, cfg Config) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("config path is required")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yaml" && ext != ".yml" {
		return errors.New("YAML config file is required")
	}
	ApplyDefaults(&cfg)
	if err := ValidateConfig(cfg); err != nil {
		return err
	}
	body, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode YAML config: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".jute-config-*.yaml")
	if err != nil {
		return fmt.Errorf("create config temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod config temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}
