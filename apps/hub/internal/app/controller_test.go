package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"jute-dash/apps/hub/internal/app/agents"
	"jute-dash/apps/hub/internal/app/config"
	a2a "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/displayactions"
)

func TestHealthEndpoint(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" || body.Version != "test" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestEventsStreamDisplayActions(t *testing.T) {
	dispatcher := displayactions.NewDispatcher()
	handler := newServer(
		testConfig(),
		"test",
		nil,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		dispatcher,
	)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx := t.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for range 8 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read initial event stream: %v", err)
		}
		if strings.Contains(line, "event: hub.connected") {
			break
		}
	}
	if _, err := dispatcher.Notify("Hello dashboard", "info"); err != nil {
		t.Fatalf("notify: %v", err)
	}
	var sawNotification bool
	for range 20 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read event stream: %v", err)
		}
		if strings.Contains(line, "event: display.notification") {
			sawNotification = true
			break
		}
	}
	if !sawNotification {
		t.Fatal("event stream did not include display.notification")
	}
}

func TestMessageEndpointRejectsDisabledAgent(t *testing.T) {
	handler := New(testConfig(), "test")
	payload := bytes.NewBufferString(`{"agentId":"energy","text":"How much power are we using?"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", rec.Code)
	}
}

func TestMessageEndpointRejectsUnknownAgentBeforeTransport(t *testing.T) {
	sender := &fakeMessageSender{}
	handler := NewWithMessageSender(testConfig(), "test", sender)
	payload := bytes.NewBufferString(`{"agentId":"missing","text":"Hello?"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if sender.called {
		t.Fatal("sender should not be called for unknown agent")
	}
}

func TestMessageEndpointAcceptsEnabledAgent(t *testing.T) {
	sender := &fakeMessageSender{
		result: a2a.SendMessageResult{
			ConversationID: "ctx-1",
			Status:         "completed",
			Text:           "The house looks calm.",
		},
	}
	handler := NewWithMessageSender(testConfig(), "test", sender)
	payload := bytes.NewBufferString(`{"agentId":"house","text":"What needs attention?"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body MessageResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.AgentID != "house" || body.Status != "completed" || body.Message != "The house looks calm." {
		t.Fatalf("unexpected response: %+v", body)
	}
	if sender.last.EndpointURL != "https://agent.example.com/a2a/v1" || sender.last.Text != "What needs attention?" {
		t.Fatalf("unexpected sender request: %+v", sender.last)
	}
}

func TestMessageEndpointUsesDiscoveredA2A10InterfaceAndDashboardContext(t *testing.T) {
	agentCardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        "Discovered Agent",
			"description": "Test card",
			"version":     "1.0.0",
			"supportedInterfaces": []map[string]string{
				{"url": "http://agent.local/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
			},
			"capabilities": map[string]any{
				"streaming": false,
				"extensions": []map[string]any{
					{"uri": a2a.DashboardContextExtensionURI},
				},
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
			"skills": []map[string]any{
				{"id": "chat", "name": "Chat", "description": "Talk", "tags": []string{"chat"}},
			},
		})
	}))
	defer agentCardServer.Close()

	cfg := testConfig()
	cfg.Agents[0].CardURL = agentCardServer.URL
	cfg.Agents[0].EndpointURL = "http://configured.local/legacy"
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer runtimeStore.Close()
	result, err := runtimeStore.Initialize(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	sender := &fakeMessageSender{result: a2a.SendMessageResult{
		ConversationID: "ctx-discovered",
		Status:         "completed",
		Text:           "Context received.",
	}}
	handler := newServer(cfg, "test", sender, result.Setup, DefaultWidgetLayout(), runtimeStore, "", nil)

	payload := bytes.NewBufferString(`{"agentId":"house","text":"What can you see?","conversationId":"ctx-existing"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !sender.called {
		t.Fatal("sender was not called")
	}
	if sender.last.EndpointURL != "http://agent.local/invoke" || sender.last.ProtocolVersion != a2a.ProtocolVersion10 {
		t.Fatalf("unexpected selected interface: %+v", sender.last)
	}
	if sender.last.ConversationID != "ctx-existing" {
		t.Fatalf("unexpected conversation ID: %+v", sender.last)
	}
	if len(sender.last.Extensions) != 1 || sender.last.Extensions[0] != a2a.DashboardContextExtensionURI {
		t.Fatalf("dashboard extension not activated: %+v", sender.last.Extensions)
	}
	if sender.last.Metadata[a2a.DashboardContextExtensionURI] == nil {
		t.Fatalf("dashboard metadata missing: %+v", sender.last.Metadata)
	}
}

func TestMessageEndpointReturnsSafeAgentFailure(t *testing.T) {
	handler := NewWithMessageSender(
		testConfig(),
		"test",
		&fakeMessageSender{err: errors.New("raw remote failure with internal details")},
	)
	payload := bytes.NewBufferString(`{"agentId":"house","text":"What needs attention?"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "agent request failed" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func TestMessageEndpointRejectsUnsupportedBindingBeforeTransport(t *testing.T) {
	cfg := testConfig()
	cfg.Agents[0].ProtocolBinding = a2a.ProtocolHTTPJSON
	sender := &fakeMessageSender{}
	handler := NewWithMessageSender(cfg, "test", sender)
	payload := bytes.NewBufferString(`{"agentId":"house","text":"What needs attention?"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", rec.Code)
	}
	if sender.called {
		t.Fatal("sender should not be called for unsupported binding")
	}
}

func TestHomeEndpointExcludesWeather(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, exists := body["weather"]; exists {
		t.Fatalf("home response should not include global weather: %+v", body)
	}
}

func TestSetupStatusEndpoint(t *testing.T) {
	handler := NewWithSetupStatus(testConfig(), "test", SetupStatus{
		Complete: false,
		Missing:  []string{"home.name"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body SetupStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Complete || len(body.Missing) != 1 || body.Missing[0] != "home.name" {
		t.Fatalf("unexpected setup status: %+v", body)
	}
}

func TestHouseholdSettingsEndpointUpdatesStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", SetupStatus{Complete: true}, runtimeStore)

	payload := bytes.NewBufferString(`{
		"home":{"name":"Updated Home","timezone":"Europe/London","locale":"en-GB"},
		"display":{"theme":"dark","accentColor":"teal","idleMode":"ambient"},
		"weather":{"enabled":true,"provider":"open-meteo","locationName":"Manchester","latitude":53.4808,"longitude":-2.2426,"temperatureUnit":"celsius","windSpeedUnit":"kmh"}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body HouseholdSettings
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Home.Name != "Updated Home" || body.Weather.LocationName != "Manchester" || !body.Setup.Complete {
		t.Fatalf("unexpected household settings: %+v", body)
	}
	reloaded, err := runtimeStore.HouseholdSettings(context.Background())
	if err != nil {
		t.Fatalf("reload household settings: %v", err)
	}
	if reloaded.Home.Name != "Updated Home" || reloaded.Weather.Latitude != 53.4808 {
		t.Fatalf("settings did not persist: %+v", reloaded)
	}
}

func TestHouseholdSettingsEndpointRejectsInvalidTimezone(t *testing.T) {
	handler := NewWithSetupStatus(testConfig(), "test", SetupStatus{Complete: true})
	payload := bytes.NewBufferString(`{
		"home":{"name":"Updated Home","timezone":"Nope/Nowhere","locale":"en-GB"},
		"display":{"theme":"dark","accentColor":"teal","idleMode":"ambient"},
		"weather":{"enabled":true,"provider":"open-meteo","locationName":"Manchester","latitude":53.4808,"longitude":-2.2426,"temperatureUnit":"celsius","windSpeedUnit":"kmh"}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "Nope/Nowhere") {
		t.Fatalf("error response leaked raw invalid value: %s", rec.Body.String())
	}
}

func TestHouseholdSettingsEndpointUpdatesYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStoreAndConfigPath(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		configPath,
	)

	payload := bytes.NewBufferString(`{
		"home":{"name":"YAML Home","timezone":"Europe/London","locale":"en-GB"},
		"display":{"theme":"light","accentColor":"teal","idleMode":"ambient"},
		"weather":{"enabled":true,"provider":"open-meteo","locationName":"Bristol","latitude":51.4545,"longitude":-2.5879,"temperatureUnit":"celsius","windSpeedUnit":"kmh"}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	reloaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Home.Name != "YAML Home" || reloaded.Weather.LocationName != "Bristol" || len(reloaded.Agents) != 2 {
		t.Fatalf("unexpected saved config: %+v", reloaded)
	}
}

func TestRoomSettingsEndpointUpdatesStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", SetupStatus{Complete: true}, runtimeStore)

	payload := bytes.NewBufferString(
		`{"rooms":[{"id":"Living Room","name":"Living Room","summary":"Downstairs","status":"Comfortable"}]}`,
	)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/rooms", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Rooms []RoomConfig `json:"rooms"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Rooms) != 1 || body.Rooms[0].ID != "living-room" {
		t.Fatalf("unexpected room response: %+v", body.Rooms)
	}
	reloaded, err := runtimeStore.Rooms(context.Background())
	if err != nil {
		t.Fatalf("reload rooms: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Name != "Living Room" {
		t.Fatalf("rooms did not persist: %+v", reloaded)
	}
}

func TestRoomSettingsEndpointRejectsInvalidRooms(t *testing.T) {
	handler := NewWithSetupStatus(testConfig(), "test", SetupStatus{Complete: true})
	payload := bytes.NewBufferString(
		`{"rooms":[{"id":"kitchen","name":"Kitchen"},{"id":"kitchen","name":"Duplicate"}]}`,
	)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/rooms", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "duplicate room id") {
		t.Fatalf("unexpected error response: %s", rec.Body.String())
	}
}

func TestTileSettingsEndpointUpdatesStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", SetupStatus{Complete: true}, runtimeStore)

	payload := bytes.NewBufferString(
		`{"tiles":[{"id":"Front Door","kind":"security","label":"Front door","value":"Locked","detail":"Last checked now"}]}`,
	)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/tiles", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Tiles []TileConfig `json:"tiles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Tiles) != 1 || body.Tiles[0].ID != "front-door" || body.Tiles[0].Kind != "security" {
		t.Fatalf("unexpected tile response: %+v", body.Tiles)
	}
	reloaded, err := runtimeStore.Tiles(context.Background())
	if err != nil {
		t.Fatalf("reload tiles: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Value != "Locked" {
		t.Fatalf("tiles did not persist: %+v", reloaded)
	}
}

func TestTileSettingsEndpointRejectsInvalidTiles(t *testing.T) {
	handler := NewWithSetupStatus(testConfig(), "test", SetupStatus{Complete: true})
	payload := bytes.NewBufferString(
		`{"tiles":[{"id":"temperature","kind":"status","label":"Temperature","value":""}]}`,
	)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/tiles", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tile value is required") {
		t.Fatalf("unexpected error response: %s", rec.Body.String())
	}
}

func TestRoomAndTileSettingsEndpointUpdatesYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStoreAndConfigPath(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		configPath,
	)

	roomReq := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings/rooms",
		bytes.NewBufferString(`{"rooms":[{"id":"Office","name":"Office","summary":"Work room","status":"Quiet"}]}`),
	)
	roomRec := httptest.NewRecorder()
	handler.ServeHTTP(roomRec, roomReq)
	if roomRec.Code != http.StatusOK {
		t.Fatalf("expected room status 200, got %d: %s", roomRec.Code, roomRec.Body.String())
	}

	tileReq := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/settings/tiles",
		bytes.NewBufferString(
			`{"tiles":[{"id":"office-temp","kind":"climate","label":"Office","value":"20 C","detail":"Comfortable"}]}`,
		),
	)
	tileRec := httptest.NewRecorder()
	handler.ServeHTTP(tileRec, tileReq)
	if tileRec.Code != http.StatusOK {
		t.Fatalf("expected tile status 200, got %d: %s", tileRec.Code, tileRec.Body.String())
	}

	reloaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Rooms) != 1 || reloaded.Rooms[0].ID != "office" {
		t.Fatalf("unexpected saved rooms: %+v", reloaded.Rooms)
	}
	if len(reloaded.Tiles) != 1 || reloaded.Tiles[0].ID != "office-temp" {
		t.Fatalf("unexpected saved tiles: %+v", reloaded.Tiles)
	}
	if len(reloaded.Agents) != 2 {
		t.Fatalf("agents were not preserved: %+v", reloaded.Agents)
	}
}

func TestStatusEndpointReturnsSafeSummary(t *testing.T) {
	cfg := testConfig()
	cfg.MCP.Enabled = true
	cfg.MCP.Transport = "streamable-http"
	cfg.MCP.ListenAddress = "127.0.0.1:8790"
	cfg.MCP.Path = "/mcp"
	cfg.MCP.Auth.Mode = "local-token"
	cfg.MCP.Auth.EnvToken = "VERY_SECRET_ENV_NAME"
	cfg.Agents[0].Auth = &AuthConfig{Type: "bearer", EnvToken: "AGENT_SECRET_TOKEN"}
	t.Setenv("AGENT_SECRET_TOKEN", "secret-value")

	handler := NewWithSetupStatus(cfg, "test-version", SetupStatus{Complete: true})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body StatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Version != "test-version" || body.MCP.ServiceStatus != "enabled" || body.MCP.AuthMode != "local-token" {
		t.Fatalf("unexpected status response: %+v", body)
	}
	if body.Agents.Total != 2 || body.Agents.Enabled != 1 || body.EventStream.Available != true {
		t.Fatalf("unexpected status summary: %+v", body)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "VERY_SECRET_ENV_NAME") || strings.Contains(raw, "AGENT_SECRET_TOKEN") ||
		strings.Contains(raw, "secret-value") {
		t.Fatalf("status response leaked secret material: %s", raw)
	}
}

func TestVoiceStatusEndpointReturnsSafeDefaults(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/voice/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body VoiceStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Enabled || !body.Muted || body.State != "muted" || body.ServiceStatus != "not_configured" {
		t.Fatalf("unexpected voice status: %+v", body)
	}
}

func TestVoiceMuteEndpointsUpdateState(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = true
	cfg.Voice.STTProviderID = "wyoming-local"
	handler := New(cfg, "test")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/voice/unmute", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body VoiceStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode unmute response: %v", err)
	}
	if body.Muted || body.State != "wake_listening" || body.ServiceStatus != "ready" {
		t.Fatalf("unexpected unmuted status: %+v", body)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/voice/mute", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode mute response: %v", err)
	}
	if !body.Muted || body.State != "muted" {
		t.Fatalf("unexpected muted status: %+v", body)
	}
}

func TestVoiceProvidersEndpointReturnsStableEmptyList(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/voice/providers", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body struct {
		Providers []VoiceProviderPack `json:"providers"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Providers == nil || len(body.Providers) != 0 {
		t.Fatalf("unexpected providers response: %+v", body.Providers)
	}
}

func TestVoiceCancelPreservesSafeStatus(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/voice/cancel", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body VoiceStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.State != "muted" || body.ServiceStatus != "not_configured" {
		t.Fatalf("unexpected cancel status: %+v", body)
	}
}

func TestWidgetLayoutEndpoint(t *testing.T) {
	layout := WidgetLayout{
		ProfileID: "default-dashboard",
		Widgets: []WidgetInstance{
			{
				ID:      "date-time",
				Kind:    "date-time",
				Title:   "Date & Time",
				W:       2,
				H:       1,
				MinW:    1,
				MinH:    1,
				Size:    "wide",
				Visible: true,
			},
			{
				ID:      "weather",
				Kind:    "weather",
				Title:   "Weather",
				X:       2,
				W:       2,
				H:       1,
				MinW:    1,
				MinH:    1,
				Size:    "wide",
				Visible: true,
			},
			{
				ID:      "chat-history",
				Kind:    "chat-history",
				Title:   "Chat History",
				Y:       1,
				W:       2,
				H:       2,
				MinW:    1,
				MinH:    1,
				Size:    "medium",
				Visible: true,
			},
		},
	}
	handler := NewWithSetupStatusAndLayout(testConfig(), "test", SetupStatus{Complete: true}, layout)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/widgets/layout", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body WidgetLayout
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ProfileID != "default-dashboard" || len(body.Widgets) != 3 {
		t.Fatalf("unexpected layout response: %+v", body)
	}
	if body.Widgets[0].Kind != "date-time" || body.Widgets[1].Kind != "weather" ||
		body.Widgets[2].Kind != "chat-history" {
		t.Fatalf("unexpected widget order: %+v", body.Widgets)
	}
}

func TestWidgetCatalogEndpoint(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/widgets/catalog", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body struct {
		Widgets []WidgetCatalogItem `json:"widgets"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Widgets) < 3 {
		t.Fatalf("unexpected catalog response length: %+v", body.Widgets)
	}
	foundDateTime := false
	for _, it := range body.Widgets {
		if it.Kind == "date-time" {
			foundDateTime = true
			break
		}
	}
	if !foundDateTime {
		t.Fatalf("catalog response missing date-time widget: %+v", body.Widgets)
	}
}

func TestWidgetLayoutPutPersistsWithStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", SetupStatus{Complete: true}, runtimeStore)

	layout := DefaultWidgetLayout()
	layout.Widgets[0].X = 1
	layout.Widgets[1].Visible = false
	payload, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("marshal layout: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/widgets/layout", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	reloaded, err := runtimeStore.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if reloaded.Widgets[0].X != 1 || reloaded.Widgets[1].Visible {
		t.Fatalf("layout did not persist: %+v", reloaded.Widgets)
	}
}

func TestWidgetLayoutPutRejectsInvalidLayout(t *testing.T) {
	handler := New(testConfig(), "test")
	payload := bytes.NewBufferString(
		`{"profileId":"default-dashboard","widgets":[{"id":"bad","kind":"missing","title":"Bad","x":0,"y":0,"w":1,"h":1,"minW":1,"minH":1,"size":"small","settings":{},"visible":true}]}`,
	)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/widgets/layout", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "invalid widget layout" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func TestWidgetLayoutResetEndpoint(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	layout := DefaultWidgetLayout()
	layout.Widgets[0].Visible = false
	if _, err := runtimeStore.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", SetupStatus{Complete: true}, runtimeStore)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/widgets/layout/reset", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body WidgetLayout
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Widgets) != 3 || !body.Widgets[0].Visible {
		t.Fatalf("unexpected reset layout: %+v", body)
	}
}

func TestWidgetLayoutPutReturnsSafeStoreFailure(t *testing.T) {
	layout := DefaultWidgetLayout()
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		&failingLayoutStore{
			layout: layout,
			err:    errors.New("sqlite path /private/raw/details failed"),
		},
	)
	payload, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("marshal layout: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/widgets/layout", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "widget layout could not be saved" {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func TestStoreBackedConfigWorksWithExistingEndpoints(t *testing.T) {
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer runtimeStore.Close()

	if _, err := runtimeStore.Initialize(context.Background(), testConfig(), true); err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	cfg := testConfig()
	handler := New(cfg, "test")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body struct {
		Agents []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode agents response: %v", err)
	}
	if len(body.Agents) != 2 || body.Agents[0].ID != "house" || body.Agents[1].ID != "energy" {
		t.Fatalf("unexpected store-backed agents response: %+v", body.Agents)
	}
}

func TestAgentsEndpointIncludesDiscoveredCardMetadata(t *testing.T) {
	agentCardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        "Discovered Agent",
			"description": "Test card",
			"version":     "1.0.0",
			"supportedInterfaces": []map[string]string{
				{"url": "http://agent.local/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
			},
			"capabilities": map[string]any{
				"streaming": true,
				"extensions": []map[string]any{
					{"uri": a2a.DashboardContextExtensionURI},
				},
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
			"skills": []map[string]any{
				{"id": "chat", "name": "Chat", "description": "Talk", "tags": []string{"chat"}},
			},
		})
	}))
	defer agentCardServer.Close()

	cfg := testConfig()
	cfg.Agents = cfg.Agents[:1]
	cfg.Agents[0].CardURL = agentCardServer.URL
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer runtimeStore.Close()
	result, err := runtimeStore.Initialize(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStore(cfg, "test", result.Setup, runtimeStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Agents []struct {
			ID                        string           `json:"id"`
			CardStatus                string           `json:"cardStatus"`
			SelectedEndpointURL       string           `json:"selectedEndpointUrl"`
			SelectedProtocolVersion   string           `json:"selectedProtocolVersion"`
			DashboardContextSupported bool             `json:"dashboardContextSupported"`
			Streaming                 bool             `json:"streaming"`
			Skills                    []a2a.AgentSkill `json:"skills"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Agents) != 1 || body.Agents[0].CardStatus != "available" ||
		body.Agents[0].SelectedEndpointURL != "http://agent.local/invoke" {
		t.Fatalf("unexpected agent discovery response: %+v", body.Agents)
	}
	if !body.Agents[0].DashboardContextSupported || !body.Agents[0].Streaming || len(body.Agents[0].Skills) != 1 {
		t.Fatalf("missing discovered metadata: %+v", body.Agents[0])
	}
}

func TestAgentsEndpointIncludesSafeAuthAvailability(t *testing.T) {
	cfg := testConfig()
	cfg.Agents = cfg.Agents[:1]
	cfg.Agents[0].Auth = &AuthConfig{Type: "bearer", EnvToken: "HOUSE_AGENT_TOKEN"}
	handler := New(cfg, "test")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Agents []struct {
			AuthConfigured bool `json:"authConfigured"`
			AuthAvailable  bool `json:"authAvailable"`
		} `json:"agents"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Agents) != 1 || !body.Agents[0].AuthConfigured || body.Agents[0].AuthAvailable {
		t.Fatalf("unexpected auth availability: %+v", body.Agents)
	}
	if strings.Contains(rec.Body.String(), "HOUSE_AGENT_TOKEN") {
		t.Fatalf("agents response leaked auth env reference: %s", rec.Body.String())
	}
}

func TestAgentsEndpointAddsAgentToYAMLConfig(t *testing.T) {
	agentCardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        "Kitchen Helper",
			"description": "Local kitchen assistant",
			"version":     "1.0.0",
			"supportedInterfaces": []map[string]string{
				{"url": "http://127.0.0.1:9797/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
		})
	}))
	defer agentCardServer.Close()

	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	cfg.Agents = nil
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStoreAndConfigPath(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		configPath,
	)

	payload := bytes.NewBufferString(`{"cardUrl":` + strconv.Quote(agentCardServer.URL) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents", payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	reloaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Agents) != 1 {
		t.Fatalf("expected one saved agent, got %+v", reloaded.Agents)
	}
	if reloaded.Agents[0].ID != "kitchen-helper" || reloaded.Agents[0].EndpointURL != "http://127.0.0.1:9797/invoke" ||
		!reloaded.Agents[0].Enabled {
		t.Fatalf("unexpected saved agent: %+v", reloaded.Agents[0])
	}
	savedBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(savedBytes), "card-url: "+agentCardServer.URL) {
		t.Fatalf("saved config does not include card URL:\n%s", string(savedBytes))
	}
}

func TestConversationListUsesAgentTaskHistory(t *testing.T) {
	history := &fakeTaskHistorySender{
		tasks: []a2a.TaskRecord{
			{
				ID:        "task-1",
				ContextID: "ctx-1",
				Status:    "completed",
				Messages: []a2a.TaskMessage{
					{ID: "user-1", Role: "user", Text: "What is happening?"},
					{ID: "agent-1", Role: "assistant", Text: "All calm."},
				},
				UpdatedAt: "2026-05-19T10:00:00Z",
			},
		},
	}
	handler := newServer(
		testConfig(),
		"test",
		history,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?agentId=house", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Conversations []Conversation `json:"conversations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Conversations) != 1 || body.Conversations[0].ID != "ctx-1" ||
		body.Conversations[0].Title != "What is happening?" {
		t.Fatalf("unexpected conversations: %+v", body.Conversations)
	}
	if history.listReq.EndpointURL != "https://agent.example.com/a2a/v1" || history.listReq.PageSize != 50 {
		t.Fatalf("unexpected list request: %+v", history.listReq)
	}
}

func TestConversationDetailUsesGetTaskHistory(t *testing.T) {
	history := &fakeTaskHistorySender{
		tasks: []a2a.TaskRecord{
			{ID: "task-1", ContextID: "ctx-1", Status: "completed", UpdatedAt: "2026-05-19T10:00:00Z"},
		},
		records: map[string]a2a.TaskRecord{
			"task-1": {
				ID:        "task-1",
				ContextID: "ctx-1",
				Status:    "completed",
				Messages: []a2a.TaskMessage{
					{ID: "user-1", Role: "user", Text: "Hello"},
					{ID: "agent-1", Role: "assistant", Text: "Hello back"},
				},
				UpdatedAt: "2026-05-19T10:01:00Z",
			},
		},
	}
	handler := newServer(
		testConfig(),
		"test",
		history,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/ctx-1?agentId=house", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var detail ConversationDetail
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.Conversation.ID != "ctx-1" || detail.Conversation.LatestTaskID != "task-1" {
		t.Fatalf("unexpected conversation: %+v", detail.Conversation)
	}
	if len(detail.Messages) != 2 || detail.Messages[1].Content != "Hello back" {
		t.Fatalf("unexpected messages: %+v", detail.Messages)
	}
	if history.getReq.TaskID != "task-1" || history.getReq.HistoryLength != 50 {
		t.Fatalf("unexpected get request: %+v", history.getReq)
	}
}

func TestConversationCreateWithInitialTextSendsTurnToAgent(t *testing.T) {
	sender := &fakeTaskHistorySender{
		fakeMessageSender: fakeMessageSender{result: a2a.SendMessageResult{
			ConversationID: "ctx-created",
			TaskID:         "task-created",
			Status:         "completed",
			Text:           "Welcome home.",
		}},
		records: map[string]a2a.TaskRecord{
			"task-created": {
				ID:        "task-created",
				ContextID: "ctx-created",
				Status:    "completed",
				Messages: []a2a.TaskMessage{
					{ID: "user-created", Role: "user", Text: "Hello"},
					{ID: "agent-created", Role: "assistant", Text: "Welcome home."},
				},
				UpdatedAt: "2026-06-02T10:01:00Z",
			},
		},
	}
	handler := newServer(
		testConfig(),
		"test",
		sender,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)

	payload := bytes.NewBufferString(`{"agentId":"house","title":"Kitchen","initialText":"Hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var detail ConversationDetail
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !sender.called || sender.last.Text != "Hello" || sender.last.EndpointURL != "https://agent.example.com/a2a/v1" {
		t.Fatalf("unexpected send request: %+v", sender.last)
	}
	if detail.Conversation.ID != "ctx-created" || detail.Conversation.LatestTaskID != "task-created" {
		t.Fatalf("unexpected conversation: %+v", detail.Conversation)
	}
	if len(detail.Messages) != 2 || detail.Messages[1].Content != "Welcome home." {
		t.Fatalf("unexpected messages: %+v", detail.Messages)
	}
}

func TestConversationListShowsUnsupportedStateWhenAgentDoesNotExposeHistory(t *testing.T) {
	handler := NewWithMessageSender(testConfig(), "test", &fakeMessageSender{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?agentId=house", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Conversations []Conversation `json:"conversations"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Conversations) != 1 || !body.Conversations[0].HistoryUnsupported {
		t.Fatalf("expected unsupported history state, got %+v", body.Conversations)
	}
}

func TestConversationTurnStreamEmitsDeltasAndCompletion(t *testing.T) {
	agentCardServer := streamingAgentCardServer(t, true)
	defer agentCardServer.Close()
	cfg := testConfig()
	cfg.Agents[0].CardURL = agentCardServer.URL
	streamer := &fakeStreamingSender{
		streamEvents: []a2a.StreamEvent{
			{Kind: "task", ConversationID: "ctx-1", TaskID: "task-1", Status: "working"},
			{Kind: "artifact", ConversationID: "ctx-1", TaskID: "task-1", Text: "Hel", Append: true},
			{Kind: "artifact", ConversationID: "ctx-1", TaskID: "task-1", Text: "lo", Append: true},
			{Kind: "status", ConversationID: "ctx-1", TaskID: "task-1", Status: "completed", Terminal: true},
		},
		fakeTaskHistorySender: fakeTaskHistorySender{
			records: map[string]a2a.TaskRecord{
				"task-1": {
					ID:        "task-1",
					ContextID: "ctx-1",
					Status:    "completed",
					Messages: []a2a.TaskMessage{
						{ID: "user-1", Role: "user", Text: "Hello"},
						{ID: "agent-1", Role: "assistant", Text: "Hello"},
					},
					UpdatedAt: "2026-05-19T10:01:00Z",
				},
			},
		},
	}
	handler := newServer(
		cfg,
		"test",
		streamer,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)

	payload := bytes.NewBufferString(`{"agentId":"house","text":"Hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/ctx-1/turns/stream", payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	events := parseSSEEvents(t, rec.Body)
	if got := eventNames(
		events,
	); strings.Join(
		got,
		",",
	) != "turn_started,status_changed,assistant_delta,assistant_delta,status_changed,turn_completed" {
		t.Fatalf("unexpected events: %+v", got)
	}
	if events[2].Data["text"] != "Hel" || events[3].Data["text"] != "lo" {
		t.Fatalf("unexpected delta events: %+v", events)
	}
	if !streamer.streamCalled || streamer.lastStream.ConversationID != "ctx-1" || streamer.lastStream.Text != "Hello" {
		t.Fatalf("unexpected stream request: %+v", streamer.lastStream)
	}
}

func TestConversationTurnStreamFallsBackForNonStreamingAgent(t *testing.T) {
	agentCardServer := streamingAgentCardServer(t, false)
	defer agentCardServer.Close()
	cfg := testConfig()
	cfg.Agents[0].CardURL = agentCardServer.URL
	sender := &fakeStreamingSender{
		fakeTaskHistorySender: fakeTaskHistorySender{
			fakeMessageSender: fakeMessageSender{result: a2a.SendMessageResult{
				ConversationID: "ctx-1",
				TaskID:         "task-1",
				Status:         "completed",
				Text:           "Blocking answer",
			}},
		},
	}
	handler := newServer(
		cfg,
		"test",
		sender,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)

	payload := bytes.NewBufferString(`{"agentId":"house","text":"Hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/ctx-1/turns/stream", payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	events := parseSSEEvents(t, rec.Body)
	if got := eventNames(events); strings.Join(got, ",") != "turn_started,turn_completed" {
		t.Fatalf("unexpected events: %+v", got)
	}
	if sender.streamCalled {
		t.Fatal("streamer should not be called for non-streaming agent")
	}
	if !sender.called {
		t.Fatal("blocking sender should be called")
	}
}

func TestConversationTurnStreamEmitsSafeFailureAfterPartialStream(t *testing.T) {
	agentCardServer := streamingAgentCardServer(t, true)
	defer agentCardServer.Close()
	cfg := testConfig()
	cfg.Agents[0].CardURL = agentCardServer.URL
	streamer := &fakeStreamingSender{
		streamEvents: []a2a.StreamEvent{
			{Kind: "artifact", ConversationID: "ctx-1", TaskID: "task-1", Text: "Partial", Append: true},
		},
		streamErr: errors.New("raw remote stream failure with internals"),
	}
	handler := newServer(
		cfg,
		"test",
		streamer,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		"",
		nil,
	)

	payload := bytes.NewBufferString(`{"agentId":"house","text":"Hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/ctx-1/turns/stream", payload)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	events := parseSSEEvents(t, rec.Body)
	if got := eventNames(events); strings.Join(got, ",") != "turn_started,assistant_delta,turn_failed" {
		t.Fatalf("unexpected events: %+v", got)
	}
	if message := events[2].Data["message"]; message != "Agent request failed" {
		t.Fatalf("unexpected failure message: %+v", events[2].Data)
	}
	if strings.Contains(rec.Body.String(), "raw remote") {
		t.Fatalf("stream leaked raw error: %s", rec.Body.String())
	}
}

func testConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Agents = []agents.AgentConfig{
		{
			ID:              "house",
			Name:            "House Concierge",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://agent.example.com/a2a/v1",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
		},
		{
			ID:              "energy",
			Name:            "Energy Watch",
			CardURL:         "https://energy.example.com/.well-known/agent-card.json",
			EndpointURL:     "https://energy.example.com/a2a/v1",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         false,
		},
	}
	return cfg
}

func openInitializedServerStore(t *testing.T) *Store {
	t.Helper()
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := runtimeStore.Initialize(context.Background(), testConfig(), true); err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	return runtimeStore
}

type failingLayoutStore struct {
	layout WidgetLayout
	err    error
}

func (s *failingLayoutStore) WidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	return s.layout, nil
}

func (s *failingLayoutStore) SaveWidgetLayout(
	ctx context.Context,
	layout WidgetLayout,
) (WidgetLayout, error) {
	return WidgetLayout{}, s.err
}

func (s *failingLayoutStore) ResetWidgetLayout(ctx context.Context, profileID string) (WidgetLayout, error) {
	return WidgetLayout{}, s.err
}

type fakeMessageSender struct {
	result a2a.SendMessageResult
	err    error
	last   a2a.SendMessageRequest
	called bool
}

func (s *fakeMessageSender) SendMessage(
	ctx context.Context,
	req a2a.SendMessageRequest,
) (a2a.SendMessageResult, error) {
	s.called = true
	s.last = req
	if s.err != nil {
		return a2a.SendMessageResult{}, s.err
	}
	return s.result, nil
}

type fakeTaskHistorySender struct {
	fakeMessageSender

	tasks   []a2a.TaskRecord
	records map[string]a2a.TaskRecord
	listReq a2a.ListTasksRequest
	getReq  a2a.GetTaskRequest
}

func (s *fakeTaskHistorySender) ListTasks(ctx context.Context, req a2a.ListTasksRequest) (a2a.ListTasksResult, error) {
	s.listReq = req
	tasks := make([]a2a.TaskRecord, 0, len(s.tasks))
	for _, task := range s.tasks {
		if req.ContextID != "" && task.ContextID != req.ContextID {
			continue
		}
		tasks = append(tasks, task)
	}
	return a2a.ListTasksResult{Tasks: tasks}, nil
}

func (s *fakeTaskHistorySender) GetTask(ctx context.Context, req a2a.GetTaskRequest) (a2a.TaskRecord, error) {
	s.getReq = req
	if task, ok := s.records[req.TaskID]; ok {
		return task, nil
	}
	return a2a.TaskRecord{}, errors.New("task not found")
}

type fakeStreamingSender struct {
	fakeTaskHistorySender

	streamEvents []a2a.StreamEvent
	streamErr    error
	lastStream   a2a.SendMessageRequest
	streamCalled bool
}

func (s *fakeStreamingSender) StreamMessage(
	ctx context.Context,
	req a2a.SendMessageRequest,
	handler a2a.StreamHandler,
) error {
	s.streamCalled = true
	s.lastStream = req
	for _, event := range s.streamEvents {
		if err := handler(event); err != nil {
			return err
		}
	}
	return s.streamErr
}

type sseEvent struct {
	Name string
	Data map[string]any
}

func parseSSEEvents(t *testing.T, reader io.Reader) []sseEvent {
	t.Helper()
	var events []sseEvent
	var name string
	var data strings.Builder
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if name != "" {
				var payload map[string]any
				if err := json.Unmarshal([]byte(data.String()), &payload); err != nil {
					t.Fatalf("decode SSE data for %s: %v\n%s", name, err, data.String())
				}
				events = append(events, sseEvent{Name: name, Data: payload})
			}
			name = ""
			data.Reset()
			continue
		}
		if after, ok := strings.CutPrefix(line, "event:"); ok {
			name = strings.TrimSpace(after)
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan SSE events: %v", err)
	}
	return events
}

func eventNames(events []sseEvent) []string {
	names := make([]string, 0, len(events))
	for _, event := range events {
		names = append(names, event.Name)
	}
	return names
}

func streamingAgentCardServer(t *testing.T, streaming bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        "Streaming Agent",
			"description": "Test streaming card",
			"version":     "1.0.0",
			"supportedInterfaces": []map[string]string{
				{"url": "http://agent.local/invoke", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"},
			},
			"capabilities": map[string]any{
				"streaming": streaming,
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
		})
	}))
}
