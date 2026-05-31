package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	path := writeJSONConfig(t, `{
		"home": {"name": "Workshop"},
		"server": {},
		"display": {},
		"agents": [],
		"rooms": [],
		"tiles": []
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.ListenAddress != "127.0.0.1:8787" {
		t.Fatalf("unexpected listen address: %s", cfg.Server.ListenAddress)
	}
	if cfg.Home.Timezone != "UTC" {
		t.Fatalf("unexpected timezone: %s", cfg.Home.Timezone)
	}
	if !cfg.Weather.Enabled {
		t.Fatal("expected weather to be enabled by default")
	}
	if cfg.Weather.Provider != "open-meteo" {
		t.Fatalf("unexpected weather provider: %s", cfg.Weather.Provider)
	}
	if cfg.Weather.LocationName != "London" {
		t.Fatalf("unexpected weather location: %s", cfg.Weather.LocationName)
	}
	if cfg.Voice.Enabled || !cfg.Voice.MutedByDefault || cfg.Voice.FollowupWindowSeconds != 8 {
		t.Fatalf("unexpected voice defaults: %+v", cfg.Voice)
	}
	if cfg.Display.ColorMode != "system" || cfg.Display.Theme != "system" || cfg.Display.ThemeID != "jute-mono" {
		t.Fatalf("unexpected display theme defaults: %+v", cfg.Display)
	}
	if cfg.Display.Background.Kind != "theme" || cfg.Display.Background.Fit != "cover" || cfg.Display.Background.Overlay != "none" {
		t.Fatalf("unexpected display background defaults: %+v", cfg.Display.Background)
	}
	if cfg.Display.WidgetChrome.Default != "solid" {
		t.Fatalf("unexpected widget chrome default: %+v", cfg.Display.WidgetChrome)
	}
}

func TestYAMLConfigLoadsKebabCaseFields(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
  timezone: Europe/London
  locale: en-GB
server:
  listen-address: 127.0.0.1:9999
mcp:
  enabled: true
  transport: streamable-http
  listen-address: 127.0.0.1:8790
  path: /mcp
  auth:
    mode: none
display:
  color-mode: dark
  theme-id: jute-mono
  density: compact
  motion: reduced
  background:
    kind: asset
    value: /backgrounds/kitchen.jpg
    fit: cover
    position: center
    overlay: smoked
  widget-chrome:
    default: frosted
  accent-color: neutral
  idle-mode: ambient
weather:
  enabled: true
  provider: open-meteo
  location-name: York
  latitude: 53.959
  longitude: -1.0815
  temperature-unit: fahrenheit
  wind-speed-unit: mph
voice:
  enabled: true
  muted-by-default: false
  wake-word-model-id: openwakeword-hey-jute
  stt-provider-id: wyoming-local
  tts-provider-id: ""
  preferred-agent-id: house
  cloud-opt-in: false
  command-providers-enabled: false
  sensitive-output-policy: visual_only_sensitive
  followup-window-seconds: 9
  microphone-profile: kiosk-array
agents:
  - id: house
    name: House
    card-url: https://agent.example.com/.well-known/agent-card.json
    endpoint-url: https://agent.example.com/a2a/v1
    protocol-binding: JSONRPC
    enabled: true
    mcp-scopes:
      - dashboard:read
      - widgets:read
      - skills:read
      - skills:context_read
      - skills:prompt_read
    auth:
      type: bearer
      env-token: HOUSE_AGENT_TOKEN
rooms: []
tiles: []
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.ListenAddress != "127.0.0.1:9999" {
		t.Fatalf("unexpected listen address: %s", cfg.Server.ListenAddress)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Auth.Mode != "none" || cfg.MCP.Path != "/mcp" {
		t.Fatalf("unexpected MCP config: %+v", cfg.MCP)
	}
	if cfg.Display.AccentColor != "neutral" || cfg.Weather.LocationName != "York" {
		t.Fatalf("kebab-case YAML fields were not decoded: %+v", cfg)
	}
	if cfg.Display.ColorMode != "dark" || cfg.Display.Theme != "dark" || cfg.Display.ThemeID != "jute-mono" || cfg.Display.Density != "compact" || cfg.Display.Motion != "reduced" {
		t.Fatalf("unexpected YAML display config: %+v", cfg.Display)
	}
	if cfg.Display.Background.Kind != "asset" || cfg.Display.Background.Value != "/backgrounds/kitchen.jpg" || cfg.Display.Background.Overlay != "smoked" {
		t.Fatalf("unexpected YAML display background: %+v", cfg.Display.Background)
	}
	if cfg.Display.WidgetChrome.Default != "frosted" {
		t.Fatalf("unexpected YAML widget chrome: %+v", cfg.Display.WidgetChrome)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].CardURL == "" || cfg.Agents[0].Auth.EnvToken != "HOUSE_AGENT_TOKEN" {
		t.Fatalf("unexpected YAML agent: %+v", cfg.Agents)
	}
	if got := strings.Join(cfg.Agents[0].MCPScopes, ","); got != "dashboard:read,widgets:read,skills:read,skills:context_read,skills:prompt_read" {
		t.Fatalf("unexpected YAML MCP scopes: %s", got)
	}
	if !cfg.Voice.Enabled || cfg.Voice.MutedByDefault || cfg.Voice.STTProviderID != "wyoming-local" || cfg.Voice.FollowupWindowSeconds != 9 {
		t.Fatalf("unexpected YAML voice config: %+v", cfg.Voice)
	}
}

func TestLegacyDisplayThemeMapsToColorMode(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
display:
  theme: dark
agents: []
rooms: []
tiles: []
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Display.ColorMode != "dark" || cfg.Display.Theme != "dark" {
		t.Fatalf("legacy display.theme did not map to colorMode: %+v", cfg.Display)
	}
}

func TestLoadRejectsInvalidDisplayCustomization(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "bad color mode",
			body: `
home:
  name: Workshop
display:
  color-mode: sepia
agents: []
rooms: []
tiles: []
`,
			want: "display.colorMode",
		},
		{
			name: "bad theme id",
			body: `
home:
  name: Workshop
display:
  theme-id: neon
agents: []
rooms: []
tiles: []
`,
			want: "display.themeId",
		},
		{
			name: "remote asset",
			body: `
home:
  name: Workshop
display:
  background:
    kind: asset
    value: https://example.com/wallpaper.jpg
agents: []
rooms: []
tiles: []
`,
			want: "display.background.value",
		},
		{
			name: "unsafe file",
			body: `
home:
  name: Workshop
display:
  background:
    kind: file
    value: ../secret.jpg
agents: []
rooms: []
tiles: []
`,
			want: "display.background.value",
		},
		{
			name: "bad widget chrome",
			body: `
home:
  name: Workshop
display:
  widget-chrome:
    default: glassy
agents: []
rooms: []
tiles: []
`,
			want: "display.widgetChrome.default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAMLConfig(t, tt.body)
			_, err := Load(path)
			if err == nil {
				t.Fatal("Load() expected display validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q in error, got %v", tt.want, err)
			}
		})
	}
}

func TestJSONConfigLoadsVoiceFields(t *testing.T) {
	path := writeJSONConfig(t, `{
		"home": {"name": "Workshop"},
		"voice": {
			"enabled": true,
			"mutedByDefault": false,
			"sttProviderId": "wyoming-local",
			"ttsProviderId": "tts-local",
			"preferredAgentId": "house",
			"sensitiveOutputPolicy": "visual_only_sensitive",
			"followupWindowSeconds": 7
		},
		"agents": [],
		"rooms": [],
		"tiles": []
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Voice.Enabled || cfg.Voice.MutedByDefault || cfg.Voice.STTProviderID != "wyoming-local" || cfg.Voice.TTSProviderID != "tts-local" || cfg.Voice.FollowupWindowSeconds != 7 {
		t.Fatalf("unexpected JSON voice config: %+v", cfg.Voice)
	}
}

func TestLoadRejectsInvalidVoiceFollowupWindow(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
voice:
  followup-window-seconds: 31
agents: []
rooms: []
tiles: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected invalid voice follow-up window error")
	}
	if !strings.Contains(err.Error(), "voice.followupWindowSeconds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidMCPConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "bad transport",
			body: `
home:
  name: Workshop
mcp:
  transport: stdio
agents: []
rooms: []
tiles: []
`,
			want: "mcp.transport",
		},
		{
			name: "bad path",
			body: `
home:
  name: Workshop
mcp:
  path: mcp
agents: []
rooms: []
tiles: []
`,
			want: "mcp.path",
		},
		{
			name: "bad auth",
			body: `
home:
  name: Workshop
mcp:
  auth:
    mode: password
agents: []
rooms: []
tiles: []
`,
			want: "mcp.auth.mode",
		},
		{
			name: "lan without allow",
			body: `
home:
  name: Workshop
mcp:
  enabled: true
  listen-address: 0.0.0.0:8790
  auth:
    mode: none
agents: []
rooms: []
tiles: []
`,
			want: "mcp.listenAddress",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeYAMLConfig(t, tt.body)
			_, err := Load(path)
			if err == nil {
				t.Fatal("Load() expected MCP validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q in error, got %v", tt.want, err)
			}
		})
	}
}

func TestLoadRejectsUnknownYAMLFields(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
  surprise: nope
agents: []
rooms: []
tiles: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected unknown YAML field error")
	}
	if !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsDuplicateAgentIDs(t *testing.T) {
	path := writeJSONConfig(t, `{
		"home": {"name": "Workshop"},
		"server": {},
		"display": {},
		"agents": [
			{
				"id": "agent",
				"name": "One",
				"cardUrl": "https://agent.example.com/.well-known/agent-card.json",
				"endpointUrl": "https://agent.example.com/a2a/v1",
				"protocolBinding": "JSONRPC",
				"enabled": true
			},
			{
				"id": "agent",
				"name": "Two",
				"cardUrl": "https://agent2.example.com/.well-known/agent-card.json",
				"endpointUrl": "https://agent2.example.com/a2a/v1",
				"protocolBinding": "JSONRPC",
				"enabled": true
			}
		],
		"rooms": [],
		"tiles": []
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected duplicate ID error")
	}
	if !strings.Contains(err.Error(), "duplicates another agent") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsDuplicateAgentIDsFromYAML(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
agents:
  - id: agent
    name: One
    card-url: https://agent.example.com/.well-known/agent-card.json
    endpoint-url: https://agent.example.com/a2a/v1
    protocol-binding: JSONRPC
    enabled: true
  - id: agent
    name: Two
    card-url: https://agent2.example.com/.well-known/agent-card.json
    endpoint-url: https://agent2.example.com/a2a/v1
    protocol-binding: JSONRPC
    enabled: true
rooms: []
tiles: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected duplicate ID error")
	}
	if !strings.Contains(err.Error(), "duplicates another agent") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidAgentMCPScopes(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
agents:
  - id: agent
    name: Agent
    card-url: https://agent.example.com/.well-known/agent-card.json
    endpoint-url: https://agent.example.com/a2a/v1
    protocol-binding: JSONRPC
    enabled: true
    mcp-scopes:
      - dashboard:read
      - dashboard:read
      - home:destroy
rooms: []
tiles: []
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected invalid MCP scope error")
	}
	if !strings.Contains(err.Error(), "mcpScopes") || !strings.Contains(err.Error(), "not supported") || !strings.Contains(err.Error(), "duplicates another MCP scope") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAgentMCPScopesDefaultToReadOnly(t *testing.T) {
	path := writeJSONConfig(t, `{
		"home": {"name": "Workshop"},
		"agents": [
			{
				"id": "agent",
				"name": "Agent",
				"cardUrl": "https://agent.example.com/.well-known/agent-card.json",
				"endpointUrl": "https://agent.example.com/a2a/v1",
				"protocolBinding": "JSONRPC",
				"enabled": true
			}
		],
		"rooms": [],
		"tiles": []
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := strings.Join(cfg.Agents[0].MCPScopes, ","), strings.Join(DefaultMCPReadScopes(), ","); got != want {
		t.Fatalf("unexpected default scopes: got %s want %s", got, want)
	}
}

func TestPublicConfigOmitsAuthDetails(t *testing.T) {
	cfg := Default()
	cfg.Agents = []AgentConfig{
		{
			ID:              "house",
			Name:            "House",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://agent.example.com/a2a/v1",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
			MCPScopes:       []string{MCPScopeDashboardRead},
			Auth:            &AuthConfig{Type: "bearer", EnvToken: "SECRET_TOKEN"},
		},
	}

	public := cfg.Public()
	if len(public.Agents) != 1 {
		t.Fatalf("expected one public agent, got %d", len(public.Agents))
	}
	if !public.Agents[0].AuthConfigured {
		t.Fatal("expected authConfigured to be true")
	}
	if strings.Join(public.Agents[0].MCPScopes, ",") != MCPScopeDashboardRead {
		t.Fatalf("unexpected public MCP scopes: %+v", public.Agents[0].MCPScopes)
	}
}

func TestDevMockA2AConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "examples", "harnesses", "mock-a2a", "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertSingleDevAgent(t, cfg, "mock-a2a-agent")
}

func TestDevMockA2AMCPConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "examples", "harnesses", "mock-a2a-mcp", "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Auth.Mode != "none" || cfg.MCP.ListenAddress != "127.0.0.1:8790" {
		t.Fatalf("unexpected dev MCP config: %+v", cfg.MCP)
	}
	assertSingleDevAgent(t, cfg, "mock-a2a-agent")
}

func TestDevKronkA2AConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "examples", "harnesses", "kronk-a2a", "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertSingleDevAgent(t, cfg, "kronk-local")
}

func TestDevKronkA2AMCPConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "examples", "harnesses", "kronk-a2a", "config.mcp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Auth.Mode != "none" || cfg.MCP.ListenAddress != "127.0.0.1:8790" {
		t.Fatalf("unexpected dev MCP config: %+v", cfg.MCP)
	}
	assertSingleDevAgent(t, cfg, "kronk-local")
}

func assertSingleDevAgent(t *testing.T, cfg Config, wantID string) {
	t.Helper()
	if len(cfg.Agents) != 1 {
		t.Fatalf("expected one dev A2A agent, got %d", len(cfg.Agents))
	}
	agent := cfg.Agents[0]
	if agent.ID != wantID || !agent.Enabled || agent.ProtocolBinding != "JSONRPC" {
		t.Fatalf("unexpected dev A2A agent: %+v", agent)
	}
	if agent.CardURL != "http://127.0.0.1:9797/.well-known/agent-card.json" {
		t.Fatalf("unexpected card URL: %s", agent.CardURL)
	}
}

func TestExampleYAMLConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "examples", "config", "jute.example.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Home.Name != "Jute House" || len(cfg.Agents) != 2 {
		t.Fatalf("unexpected example YAML config: %+v", cfg)
	}
}

func TestLoadRejectsInvalidWeatherCoordinates(t *testing.T) {
	path := writeJSONConfig(t, `{
		"home": {"name": "Workshop"},
		"server": {},
		"display": {},
		"weather": {
			"enabled": true,
			"provider": "open-meteo",
			"locationName": "Nowhere",
			"latitude": 120,
			"longitude": 0,
			"temperatureUnit": "celsius",
			"windSpeedUnit": "kmh"
		},
		"agents": [],
		"rooms": [],
		"tiles": []
	}`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() expected invalid latitude error")
	}
	if !strings.Contains(err.Error(), "weather.latitude") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAllowsDisabledWeatherWithoutProviderDetails(t *testing.T) {
	path := writeJSONConfig(t, `{
		"home": {"name": "Workshop"},
		"server": {},
		"display": {},
		"weather": {
			"enabled": false,
			"provider": "offline",
			"locationName": "",
			"latitude": 120,
			"longitude": 240,
			"temperatureUnit": "kelvin",
			"windSpeedUnit": "warp"
		},
		"agents": [],
		"rooms": [],
		"tiles": []
	}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Weather.Enabled {
		t.Fatal("expected weather to remain disabled")
	}
}

func writeJSONConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jute.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func writeYAMLConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jute.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
