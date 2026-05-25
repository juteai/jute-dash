package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"jute-dash/internal/a2a"

	"go.yaml.in/yaml/v4"
)

type Config struct {
	Home      HomeConfig      `json:"home" yaml:"home"`
	Server    ServerConfig    `json:"server" yaml:"server"`
	MCP       MCPConfig       `json:"mcp" yaml:"mcp"`
	Display   DisplayConfig   `json:"display" yaml:"display"`
	Weather   WeatherConfig   `json:"weather" yaml:"weather"`
	Voice     VoiceConfig     `json:"voice" yaml:"voice"`
	Dashboard DashboardConfig `json:"dashboard" yaml:"dashboard"`
	Agents    []AgentConfig   `json:"agents" yaml:"agents"`
	Rooms     []RoomConfig    `json:"rooms" yaml:"rooms"`
	Tiles     []TileConfig    `json:"tiles" yaml:"tiles"`
}

type DashboardConfig struct {
	Widgets []DashboardWidgetConfig `json:"widgets" yaml:"widgets"`
}

type DashboardWidgetConfig struct {
	ID       string         `json:"id" yaml:"id"`
	Type     string         `json:"type" yaml:"type"`
	Title    string         `json:"title" yaml:"title"`
	X        int            `json:"x" yaml:"x"`
	Y        int            `json:"y" yaml:"y"`
	W        int            `json:"w" yaml:"w"`
	H        int            `json:"h" yaml:"h"`
	Visible  bool           `json:"visible" yaml:"visible"`
	Settings map[string]any `json:"settings,omitempty" yaml:"settings,omitempty"`
}

type HomeConfig struct {
	Name     string `json:"name" yaml:"name"`
	Timezone string `json:"timezone" yaml:"timezone"`
	Locale   string `json:"locale" yaml:"locale"`
}

type ServerConfig struct {
	ListenAddress string `json:"listenAddress" yaml:"listen-address"`
}

type MCPConfig struct {
	Enabled       bool          `json:"enabled" yaml:"enabled"`
	Transport     string        `json:"transport" yaml:"transport"`
	ListenAddress string        `json:"listenAddress" yaml:"listen-address"`
	Path          string        `json:"path" yaml:"path"`
	AllowLAN      bool          `json:"allowLan" yaml:"allow-lan"`
	Auth          MCPAuthConfig `json:"auth" yaml:"auth"`
}

type MCPAuthConfig struct {
	Mode     string `json:"mode" yaml:"mode"`
	EnvToken string `json:"envToken" yaml:"env-token"`
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

type VoiceConfig struct {
	Enabled                 bool   `json:"enabled" yaml:"enabled"`
	MutedByDefault          bool   `json:"mutedByDefault" yaml:"muted-by-default"`
	WakeWordModelID         string `json:"wakeWordModelId" yaml:"wake-word-model-id"`
	STTProviderID           string `json:"sttProviderId" yaml:"stt-provider-id"`
	TTSProviderID           string `json:"ttsProviderId" yaml:"tts-provider-id"`
	STTModelID              string `json:"sttModelId" yaml:"stt-model-id"`
	TTSModelID              string `json:"ttsModelId" yaml:"tts-model-id"`
	TTSVoiceID              string `json:"ttsVoiceId" yaml:"tts-voice-id"`
	PreferredAgentID        string `json:"preferredAgentId" yaml:"preferred-agent-id"`
	CloudOptIn              bool   `json:"cloudOptIn" yaml:"cloud-opt-in"`
	CommandProvidersEnabled bool   `json:"commandProvidersEnabled" yaml:"command-providers-enabled"`
	SensitiveOutputPolicy   string `json:"sensitiveOutputPolicy" yaml:"sensitive-output-policy"`
	FollowupWindowSeconds   int    `json:"followupWindowSeconds" yaml:"followup-window-seconds"`
	MicrophoneProfile       string `json:"microphoneProfile" yaml:"microphone-profile"`
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
	MCPScopes       []string    `json:"mcpScopes" yaml:"mcp-scopes"`
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
	Home      HomeConfig          `json:"home" yaml:"home"`
	Display   DisplayConfig       `json:"display" yaml:"display"`
	Dashboard DashboardConfig     `json:"dashboard" yaml:"dashboard"`
	Agents    []PublicAgentConfig `json:"agents" yaml:"agents"`
	Rooms     []RoomConfig        `json:"rooms" yaml:"rooms"`
	Tiles     []TileConfig        `json:"tiles" yaml:"tiles"`
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
	MCPScopes       []string `json:"mcpScopes" yaml:"mcp-scopes"`
	AuthConfigured  bool     `json:"authConfigured" yaml:"auth-configured"`
}

const (
	MCPScopeDashboardRead      = "dashboard:read"
	MCPScopeWidgetsRead        = "widgets:read"
	MCPScopeSkillsRead         = "skills:read"
	MCPScopeSkillsContextRead  = "skills:context_read"
	MCPScopeSkillsPromptRead   = "skills:prompt_read"
	MCPScopeSkillsActionInvoke = "skills:action_invoke"
	MCPScopeDisplayWrite       = "display:write_ephemeral"
	MCPScopeDisplayFocusWidget = "display:focus_widget"
)

func DefaultMCPReadScopes() []string {
	return []string{
		MCPScopeDashboardRead,
		MCPScopeWidgetsRead,
		MCPScopeSkillsRead,
		MCPScopeSkillsContextRead,
	}
}

func IsKnownMCPScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case MCPScopeDashboardRead,
		MCPScopeWidgetsRead,
		MCPScopeSkillsRead,
		MCPScopeSkillsContextRead,
		MCPScopeSkillsPromptRead,
		MCPScopeSkillsActionInvoke,
		MCPScopeDisplayWrite,
		MCPScopeDisplayFocusWidget:
		return true
	default:
		return false
	}
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
		MCP: MCPConfig{
			Enabled:       false,
			Transport:     "streamable-http",
			ListenAddress: "127.0.0.1:8790",
			Path:          "/mcp",
			AllowLAN:      false,
			Auth: MCPAuthConfig{
				Mode:     "local-token",
				EnvToken: "JUTE_MCP_TOKEN",
			},
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
		Voice: VoiceConfig{
			Enabled:               false,
			MutedByDefault:        true,
			SensitiveOutputPolicy: "visual_only_sensitive",
			FollowupWindowSeconds: 8,
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
	if strings.TrimSpace(cfg.MCP.Transport) != "streamable-http" {
		problems = append(problems, "mcp.transport must be streamable-http")
	}
	if strings.TrimSpace(cfg.MCP.ListenAddress) == "" {
		problems = append(problems, "mcp.listenAddress is required")
	}
	if strings.TrimSpace(cfg.MCP.Path) == "" || !strings.HasPrefix(cfg.MCP.Path, "/") {
		problems = append(problems, "mcp.path must start with /")
	}
	if cfg.MCP.Auth.Mode != "none" && cfg.MCP.Auth.Mode != "local-token" {
		problems = append(problems, "mcp.auth.mode must be none or local-token")
	}
	if cfg.MCP.Enabled {
		if !cfg.MCP.AllowLAN && !isLoopbackListenAddress(cfg.MCP.ListenAddress) {
			problems = append(problems, "mcp.listenAddress must be loopback unless mcp.allowLan is true")
		}
		if cfg.MCP.Auth.Mode == "local-token" && strings.TrimSpace(cfg.MCP.Auth.EnvToken) == "" {
			problems = append(problems, "mcp.auth.envToken is required when local-token auth is enabled")
		}
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
	if cfg.Voice.FollowupWindowSeconds < 1 || cfg.Voice.FollowupWindowSeconds > 30 {
		problems = append(problems, "voice.followupWindowSeconds must be between 1 and 30")
	}
	if strings.TrimSpace(cfg.Voice.SensitiveOutputPolicy) == "" {
		problems = append(problems, "voice.sensitiveOutputPolicy is required")
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
		seenScopes := map[string]struct{}{}
		for j, scope := range agent.MCPScopes {
			scopeLocation := fmt.Sprintf("%s.mcpScopes[%d]", location, j)
			if strings.TrimSpace(scope) == "" {
				problems = append(problems, scopeLocation+" is required")
				continue
			}
			if !IsKnownMCPScope(scope) {
				problems = append(problems, scopeLocation+" is not supported")
			}
			if _, exists := seenScopes[scope]; exists {
				problems = append(problems, scopeLocation+" duplicates another MCP scope")
			}
			seenScopes[scope] = struct{}{}
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
			MCPScopes:       append([]string(nil), agent.MCPScopes...),
			AuthConfigured:  agent.Auth != nil,
		})
	}

	return PublicConfig{
		Home:      cfg.Home,
		Display:   cfg.Display,
		Dashboard: cfg.Dashboard,
		Agents:    publicAgents,
		Rooms:     append([]RoomConfig(nil), cfg.Rooms...),
		Tiles:     append([]TileConfig(nil), cfg.Tiles...),
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
	if strings.TrimSpace(cfg.MCP.Transport) == "" {
		cfg.MCP.Transport = defaults.MCP.Transport
	}
	if strings.TrimSpace(cfg.MCP.ListenAddress) == "" {
		cfg.MCP.ListenAddress = defaults.MCP.ListenAddress
	}
	if strings.TrimSpace(cfg.MCP.Path) == "" {
		cfg.MCP.Path = defaults.MCP.Path
	}
	if strings.TrimSpace(cfg.MCP.Auth.Mode) == "" {
		cfg.MCP.Auth.Mode = defaults.MCP.Auth.Mode
	}
	if strings.TrimSpace(cfg.MCP.Auth.EnvToken) == "" {
		cfg.MCP.Auth.EnvToken = defaults.MCP.Auth.EnvToken
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
	if strings.TrimSpace(cfg.Voice.SensitiveOutputPolicy) == "" {
		cfg.Voice.SensitiveOutputPolicy = defaults.Voice.SensitiveOutputPolicy
	}
	if cfg.Voice.FollowupWindowSeconds == 0 {
		cfg.Voice.FollowupWindowSeconds = defaults.Voice.FollowupWindowSeconds
	}
	if len(cfg.Dashboard.Widgets) == 0 {
		cfg.Dashboard.Widgets = []DashboardWidgetConfig{
			{ID: "date-time", Type: "date-time", Title: "Date & Time", X: 0, Y: 0, W: 2, H: 1, Visible: true},
			{ID: "weather", Type: "weather", Title: "Weather", X: 2, Y: 0, W: 2, H: 1, Visible: true},
			{ID: "chat-history", Type: "chat-history", Title: "Chat History", X: 0, Y: 1, W: 2, H: 2, Visible: true},
		}
	}
	for i := range cfg.Agents {
		if strings.TrimSpace(cfg.Agents[i].ProtocolBinding) == "" {
			cfg.Agents[i].ProtocolBinding = a2a.ProtocolJSONRPC
		}
		if len(cfg.Agents[i].MCPScopes) == 0 {
			cfg.Agents[i].MCPScopes = DefaultMCPReadScopes()
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

func isLoopbackListenAddress(address string) bool {
	host := address
	if parsedHost, _, err := net.SplitHostPort(address); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
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
