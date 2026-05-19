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
}

func TestYAMLConfigLoadsKebabCaseFields(t *testing.T) {
	path := writeYAMLConfig(t, `
home:
  name: Workshop
  timezone: Europe/London
  locale: en-GB
server:
  listen-address: 127.0.0.1:9999
display:
  theme: dark
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
agents:
  - id: house
    name: House
    card-url: https://agent.example.com/.well-known/agent-card.json
    endpoint-url: https://agent.example.com/a2a/v1
    protocol-binding: JSONRPC
    enabled: true
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
	if cfg.Display.AccentColor != "neutral" || cfg.Weather.LocationName != "York" {
		t.Fatalf("kebab-case YAML fields were not decoded: %+v", cfg)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].CardURL == "" || cfg.Agents[0].Auth.EnvToken != "HOUSE_AGENT_TOKEN" {
		t.Fatalf("unexpected YAML agent: %+v", cfg.Agents)
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
}

func TestDevA2AConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config", "jute.dev-a2a.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Agents) != 1 {
		t.Fatalf("expected one dev A2A agent, got %d", len(cfg.Agents))
	}
	agent := cfg.Agents[0]
	if agent.ID != "kronk-local" || !agent.Enabled || agent.ProtocolBinding != "JSONRPC" {
		t.Fatalf("unexpected dev A2A agent: %+v", agent)
	}
	if agent.CardURL != "http://127.0.0.1:9797/.well-known/agent-card.json" {
		t.Fatalf("unexpected card URL: %s", agent.CardURL)
	}
}

func TestJSONDevA2AConfigStillLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config", "jute.dev-a2a.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Agents) != 1 || cfg.Agents[0].ID != "kronk-local" {
		t.Fatalf("unexpected JSON dev A2A config: %+v", cfg.Agents)
	}
}

func TestExampleYAMLConfigLoads(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "config", "jute.example.yaml"))
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
