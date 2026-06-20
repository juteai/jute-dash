package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/app/model"
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

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Server.ListenAddress != "127.0.0.1:8787" {
		t.Fatalf("unexpected listen address: %s", cfg.Server.ListenAddress)
	}
	if cfg.Voice.Enabled || !cfg.Voice.MutedByDefault || cfg.Voice.FollowupWindowSeconds != 8 {
		t.Fatalf("unexpected voice defaults: %+v", cfg.Voice)
	}
	if cfg.Display.ColorMode != "system" || cfg.Display.Theme != "system" || cfg.Display.ThemeID != "jute-mono" {
		t.Fatalf("unexpected display theme defaults: %+v", cfg.Display)
	}
	if cfg.Display.Background.Kind != "theme" || cfg.Display.Background.Fit != "cover" ||
		cfg.Display.Background.Overlay != "none" {
		t.Fatalf("unexpected display background defaults: %+v", cfg.Display.Background)
	}
	if cfg.Display.WidgetChrome.Default != "solid" {
		t.Fatalf("unexpected widget chrome default: %+v", cfg.Display.WidgetChrome)
	}
	if cfg.A2A.Loopback == nil || !*cfg.A2A.Loopback || len(cfg.A2A.URLs) != 0 {
		t.Fatalf("unexpected A2A defaults: %+v", cfg.A2A)
	}
}

func TestYAMLConfigLoadsKebabCaseFields(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
server:
  listen-address: 127.0.0.1:9999
mcp:
  enabled: true
  transport: streamable-http
  listen-address: 127.0.0.1:8790
  path: /mcp
  auth:
    mode: none
a2a:
  allow-loopback: true
  allowed-agent-card-urls:
    - https://agent.example.com/.well-known/agent-card.json
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
voice:
  enabled: true
  muted-by-default: false
  wake-word-model-id: openwakeword-hey-jute
  wake-word-phrase: Hey Jute
  wake-sensitivity: 0.7
  stt-provider-id: local-stt
  tts-provider-id: ""
  tts-enabled: true
  tts-locale: en-GB
  tts-speed: 1.1
  tts-volume: 0.8
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

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Server.ListenAddress != "127.0.0.1:9999" {
		t.Fatalf("unexpected listen address: %s", cfg.Server.ListenAddress)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Auth.Mode != "none" || cfg.MCP.Path != "/mcp" {
		t.Fatalf("unexpected MCP config: %+v", cfg.MCP)
	}
	if len(cfg.A2A.URLs) != 1 ||
		cfg.A2A.URLs[0] != "https://agent.example.com/.well-known/agent-card.json" {
		t.Fatalf("unexpected A2A policy: %+v", cfg.A2A)
	}
	if cfg.Display.AccentColor != "neutral" {
		t.Fatalf("kebab-case YAML fields were not decoded: %+v", cfg)
	}
	if cfg.Display.ColorMode != "dark" || cfg.Display.Theme != "dark" || cfg.Display.ThemeID != "jute-mono" ||
		cfg.Display.Density != "compact" ||
		cfg.Display.Motion != "reduced" {
		t.Fatalf("unexpected YAML display config: %+v", cfg.Display)
	}
	if cfg.Display.Background.Kind != "asset" || cfg.Display.Background.Value != "/backgrounds/kitchen.jpg" ||
		cfg.Display.Background.Overlay != "smoked" {
		t.Fatalf("unexpected YAML display background: %+v", cfg.Display.Background)
	}
	if cfg.Display.WidgetChrome.Default != "frosted" {
		t.Fatalf("unexpected YAML widget chrome: %+v", cfg.Display.WidgetChrome)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].CardURL == "" || cfg.Agents[0].Auth.EnvToken != "HOUSE_AGENT_TOKEN" {
		t.Fatalf("unexpected YAML agent: %+v", cfg.Agents)
	}
	if got := strings.Join(
		cfg.Agents[0].MCPScopes,
		",",
	); got != "dashboard:read,widgets:read,skills:read,skills:context_read,skills:prompt_read" {
		t.Fatalf("unexpected YAML MCP scopes: %s", got)
	}
	if !cfg.Voice.Enabled || cfg.Voice.MutedByDefault || cfg.Voice.STTProviderID != "local-stt" ||
		cfg.Voice.WakeWordModelID != "openwakeword-hey-jute" ||
		cfg.Voice.WakeWordPhrase != "Hey Jute" ||
		cfg.Voice.WakeSensitivity != 0.7 ||
		!cfg.Voice.TTSEnabled ||
		cfg.Voice.TTSLocale != "en-GB" ||
		cfg.Voice.TTSSpeed != 1.1 ||
		cfg.Voice.TTSVolume != 0.8 ||
		cfg.Voice.FollowupWindowSeconds != 9 {
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

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Display.ColorMode != "dark" || cfg.Display.Theme != "dark" {
		t.Fatalf("legacy display.theme did not map to colorMode: %+v", cfg.Display)
	}
}

func TestSupportedThemeIDs(t *testing.T) {
	for _, themeID := range model.SupportedThemeIDs() {
		cfg := DefaultConfig()
		cfg.Display.ThemeID = themeID
		if err := ValidateConfig(cfg); err != nil {
			t.Fatalf("ValidateConfig() rejected theme %q: %v", themeID, err)
		}
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
			_, err := LoadConfig(path)
			if err == nil {
				t.Fatal("LoadConfig() expected display validation error")
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
			"sttProviderId": "local-stt",
			"ttsProviderId": "tts-local",
			"ttsEnabled": true,
			"ttsLocale": "en-US",
			"ttsSpeed": 1.2,
			"ttsVolume": 0.75,
			"preferredAgentId": "house",
			"sensitiveOutputPolicy": "visual_only_sensitive",
			"followupWindowSeconds": 7
		},
		"agents": [],
		"rooms": [],
		"tiles": []
	}`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.Voice.Enabled || cfg.Voice.MutedByDefault || cfg.Voice.STTProviderID != "local-stt" ||
		cfg.Voice.TTSProviderID != "tts-local" ||
		!cfg.Voice.TTSEnabled ||
		cfg.Voice.TTSLocale != "en-US" ||
		cfg.Voice.TTSSpeed != 1.2 ||
		cfg.Voice.TTSVolume != 0.75 ||
		cfg.Voice.FollowupWindowSeconds != 7 {
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

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected invalid voice follow-up window error")
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
			_, err := LoadConfig(path)
			if err == nil {
				t.Fatal("LoadConfig() expected MCP validation error")
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

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected unknown YAML field error")
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

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected duplicate ID error")
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

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected duplicate ID error")
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

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected invalid MCP scope error")
	}
	if !strings.Contains(err.Error(), "mcpScopes") || !strings.Contains(err.Error(), "not supported") ||
		!strings.Contains(err.Error(), "duplicates another MCP scope") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsInvalidA2AAgentCardAllowList(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
server: {}
a2a:
  allowed-agent-card-urls:
    - https://api.*.example.com/.well-known/agent-card.json
display: {}
agents: []
rooms: []
tiles: []
`)

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("LoadConfig() expected invalid A2A allow-list error")
	}
	if !strings.Contains(err.Error(), "a2a.allowedAgentCardURLs[0]") {
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

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got, want := strings.Join(
		cfg.Agents[0].MCPScopes,
		",",
	), strings.Join(
		model.DefaultMCPReadScopes(),
		",",
	); got != want {
		t.Fatalf("unexpected default scopes: got %s want %s", got, want)
	}
}

func TestPublicConfigOmitsAuthDetails(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Agents = []model.AgentConfig{
		{
			ID:              "house",
			Name:            "House",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://agent.example.com/a2a/v1",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
			MCPScopes:       []string{model.MCPScopeDashboardRead},
			Auth:            &model.AuthConfig{Type: "bearer", EnvToken: "SECRET_TOKEN"},
		},
	}

	public := cfg.Public()
	if len(public.Agents) != 1 {
		t.Fatalf("expected one public agent, got %d", len(public.Agents))
	}
	if !public.Agents[0].AuthConfigured {
		t.Fatal("expected authConfigured to be true")
	}
	if strings.Join(public.Agents[0].MCPScopes, ",") != model.MCPScopeDashboardRead {
		t.Fatalf("unexpected public MCP scopes: %+v", public.Agents[0].MCPScopes)
	}
}

func TestDevConfigLoads(t *testing.T) {
	cfg, err := LoadConfig(
		filepath.Join("..", "..", "..", "..", "..", "examples", "config", "local", "config.yaml"),
	)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	assertDevAgents(t, cfg)
	assertDevHarnessWidgets(t, cfg)
	assertDevVoice(t, cfg)
}

func TestDevMCPConfigLoads(t *testing.T) {
	cfg, err := LoadConfig(
		filepath.Join("..", "..", "..", "..", "..", "examples", "config", "local", "config.yaml"),
	)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Auth.Mode != "none" || cfg.MCP.ListenAddress != "127.0.0.1:8790" {
		t.Fatalf("unexpected dev MCP config: %+v", cfg.MCP)
	}
	assertDevAgents(t, cfg)
	assertDevVoice(t, cfg)
}

func assertDevVoice(t *testing.T, cfg Config) {
	t.Helper()
	if !cfg.Voice.Enabled ||
		cfg.Voice.MutedByDefault ||
		cfg.Voice.WakeWordModelID != "hey-jute" ||
		cfg.Voice.STTProviderID != "local-dev-stt" ||
		cfg.Voice.TTSProviderID != "local-dev-tts" ||
		cfg.Voice.TTSVoiceID != "amy" ||
		!cfg.Voice.CommandProvidersEnabled {
		t.Fatalf("unexpected dev voice config: %+v", cfg.Voice)
	}
	if len(cfg.ProviderPacks) != 3 {
		t.Fatalf("expected 3 dev voice provider packs, got %d", len(cfg.ProviderPacks))
	}
}

func assertDevAgents(t *testing.T, cfg Config) {
	t.Helper()
	if len(cfg.Agents) != 4 {
		t.Fatalf("expected 4 dev A2A agents, got %d", len(cfg.Agents))
	}
	expected := map[string]struct {
		cardURL string
	}{
		"mock-agent":   {cardURL: "http://127.0.0.1:9696/.well-known/agent-card.json"},
		"kronk-agent":  {cardURL: "http://127.0.0.1:9797/.well-known/agent-card.json"},
		"ollama-agent": {cardURL: "http://127.0.0.1:9999/.well-known/agent-card.json"},
		"gemini-agent": {cardURL: "http://127.0.0.1:9898/.well-known/agent-card.json"},
	}
	for _, agent := range cfg.Agents {
		exp, ok := expected[agent.ID]
		if !ok {
			t.Fatalf("unexpected agent ID: %s", agent.ID)
		}
		if !agent.Enabled || agent.ProtocolBinding != "JSONRPC" {
			t.Fatalf("unexpected state for agent %s: %+v", agent.ID, agent)
		}
		if agent.CardURL != exp.cardURL {
			t.Fatalf("unexpected card URL for agent %s: got %s, want %s", agent.ID, agent.CardURL, exp.cardURL)
		}
	}
}

func assertDevHarnessWidgets(t *testing.T, cfg Config) {
	t.Helper()
	type widgetExpectation struct {
		id, kind, size string
		x, y, w, h     int
		minW, minH     int
		mode           string
		visible        bool
	}
	widget := func(id, kind, size string, x, y, w, h, minW, minH int, mode string) widgetExpectation {
		return widgetExpectation{
			id:      id,
			kind:    kind,
			size:    size,
			x:       x,
			y:       y,
			w:       w,
			h:       h,
			minW:    minW,
			minH:    minH,
			mode:    mode,
			visible: true,
		}
	}
	hiddenWidget := func(id, kind, size string, x, y, w, h, minW, minH int, mode string) widgetExpectation {
		expectation := widget(id, kind, size, x, y, w, h, minW, minH, mode)
		expectation.visible = false
		return expectation
	}
	var want []widgetExpectation
	if cfg.Home.Name == "Jute Local Dev" || cfg.Home.Name == "Jute Kronk A2A Dev" {
		want = []widgetExpectation{
			widget("date-time-widget", "date-time", "small", 0, 0, 4, 2, 3, 1, model.WidgetModeUI),
			widget("weather-widget", "weather", "small", 4, 0, 4, 2, 3, 1, model.WidgetModeUI),
			widget("assistant-chat", "chat-history", "large", 0, 4, 4, 3, 3, 1, model.WidgetModeUI),
			widget("hacker-news", "rss", "large", 6, 3, 4, 3, 3, 1, model.WidgetModeUI),
			widget("stocks-watchlist", "markets", "large", 8, 0, 4, 3, 3, 1, model.WidgetModeUI),
			widget("spotify", "spotify", "medium", 0, 2, 6, 2, 4, 2, model.WidgetModeUI),
			hiddenWidget("apple-music", "apple-music", "medium", 0, 0, 6, 2, 4, 2, model.WidgetModeUI),
			widget("zigbee2mqtt", "zigbee2mqtt", "medium", 0, 0, 6, 2, 4, 2, model.WidgetModeUI),
			widget("philips-hue", "philips-hue", "medium", 0, 2, 6, 2, 4, 2, model.WidgetModeUI),
			widget("timers-alarms", "timers-alarms", "medium", 4, 6, 6, 2, 3, 2, model.WidgetModeHeadless),
			widget("calendar", "calendar", "medium", 4, 6, 6, 2, 3, 2, model.WidgetModeHeadless),
		}
	} else {
		want = []widgetExpectation{
			widget("date-time-widget", "date-time", "wide", 0, 0, 6, 1, 3, 1, model.WidgetModeUI),
			widget("weather-widget", "weather", "wide", 6, 0, 6, 1, 3, 1, model.WidgetModeUI),
			widget("assistant-chat", "chat-history", "medium", 0, 1, 6, 2, 3, 1, model.WidgetModeUI),
			widget("hacker-news", "rss", "medium", 6, 1, 6, 2, 3, 1, model.WidgetModeUI),
			widget("stocks-watchlist", "markets", "medium", 0, 3, 6, 2, 3, 1, model.WidgetModeUI),
		}
	}
	if len(cfg.Dashboard.Widgets) != len(want) {
		t.Fatalf("expected %d harness widgets, got %+v", len(want), cfg.Dashboard.Widgets)
	}
	for i, wantWidget := range want {
		got := cfg.Dashboard.Widgets[i]
		if got.ID != wantWidget.id || got.Type != wantWidget.kind || got.Size != wantWidget.size ||
			got.X != wantWidget.x || got.Y != wantWidget.y || got.W != wantWidget.w || got.H != wantWidget.h ||
			got.MinW != wantWidget.minW || got.MinH != wantWidget.minH ||
			got.Visible != wantWidget.visible || got.Mode != wantWidget.mode {
			t.Fatalf("unexpected harness widget %d: %+v", i, got)
		}
		if (got.Type == "weather" || got.Type == "rss" || got.Type == "markets") && len(got.Settings) == 0 {
			t.Fatalf("expected settings for %s widget", got.Type)
		}
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
