package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"jute-dash/apps/hub/mocks"

	"github.com/stretchr/testify/mock"
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
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore,
	)

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
	if body.Home.Name != "Updated Home" || body.Weather.LocationName != "Manchester" ||
		!body.Setup.Complete {
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
	if reloaded.Home.Name != "YAML Home" || reloaded.Weather.LocationName != "Bristol" ||
		len(reloaded.Agents) != 2 {
		t.Fatalf("unexpected saved config: %+v", reloaded)
	}
}

func TestRoomSettingsEndpointUpdatesStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore,
	)

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
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore,
	)

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
	if len(body.Tiles) != 1 || body.Tiles[0].ID != "front-door" ||
		body.Tiles[0].Kind != "security" {
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
		bytes.NewBufferString(
			`{"rooms":[{"id":"Office","name":"Office","summary":"Work room","status":"Quiet"}]}`,
		),
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
	if body.Version != "test-version" || body.MCP.ServiceStatus != "enabled" ||
		body.MCP.AuthMode != "local-token" {
		t.Fatalf("unexpected status response: %+v", body)
	}
	if body.Agents.Total != 2 || body.Agents.Enabled != 1 || body.EventStream.Available != true {
		t.Fatalf("unexpected status summary: %+v", body)
	}
	raw := rec.Body.String()
	if strings.Contains(raw, "VERY_SECRET_ENV_NAME") ||
		strings.Contains(raw, "AGENT_SECRET_TOKEN") ||
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
	if body.Enabled || !body.Muted || body.State != "muted" ||
		body.ServiceStatus != "not_configured" {
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
	handler := NewWithSetupStatusAndLayout(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		layout,
	)
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
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore,
	)

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
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore,
	)
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
	layoutStore := mocks.NewLayoutStore(t)
	layoutStore.EXPECT().WidgetLayout(mock.Anything, "").Return(layout, nil)
	layoutStore.EXPECT().
		SaveWidgetLayout(mock.Anything, mock.Anything).
		Return(WidgetLayout{}, errors.New("sqlite path /private/raw/details failed"))
	handler := NewWithSetupStatusAndLayoutStore(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		layoutStore,
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
	agentCardServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"name":        "Discovered Agent",
				"description": "Test card",
				"version":     "1.0.0",
				"supportedInterfaces": []map[string]string{
					{
						"url":             "http://agent.local/invoke",
						"protocolBinding": "JSONRPC",
						"protocolVersion": "1.0",
					},
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
		}),
	)
	defer agentCardServer.Close()

	cfg := testConfig()
	cfg.Agents = cfg.Agents[:1]
	cfg.Agents[0].CardURL = agentCardServer.URL
	allowAgentCardURL(&cfg, agentCardServer.URL)
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
	if !body.Agents[0].DashboardContextSupported || !body.Agents[0].Streaming ||
		len(body.Agents[0].Skills) != 1 {
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

func TestAgentProxyEndpoint(t *testing.T) {
	var receivedAuthHeader string
	var receivedPath string
	var receivedQuery string
	agentServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuthHeader = r.Header.Get("Authorization")
			receivedPath = r.URL.Path
			receivedQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}),
	)
	defer agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []agents.AgentConfig{
		{
			ID:              "house-proxy-test",
			Name:            "Concierge Proxy Test",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     agentServer.URL + "/api/v1/rpc",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
			Auth: &agents.AuthConfig{
				Type:     "bearer",
				EnvToken: "HOUSE_PROXY_TEST_TOKEN",
			},
		},
	}

	t.Setenv("HOUSE_PROXY_TEST_TOKEN", "super-secret-token")

	handler := New(cfg, "test")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/house-proxy-test/foo/bar?baz=qux",
		strings.NewReader(`{"hello":"world"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]bool
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode proxy response: %v", err)
	}
	if !resp["success"] {
		t.Fatalf("unexpected proxy response content: %+v", resp)
	}

	if receivedAuthHeader != "Bearer super-secret-token" {
		t.Errorf(
			"expected Authorization header 'Bearer super-secret-token', got %q",
			receivedAuthHeader,
		)
	}
	if receivedPath != "/api/v1/rpc/foo/bar" {
		t.Errorf("expected proxied path '/api/v1/rpc/foo/bar', got %q", receivedPath)
	}
	if receivedQuery != "baz=qux" {
		t.Errorf("expected proxied query 'baz=qux', got %q", receivedQuery)
	}
}

func TestAgentProxyEndpoint_EmptySubpath(t *testing.T) {
	var receivedPath string
	agentServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}),
	)
	defer agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []agents.AgentConfig{
		{
			ID:              "house-proxy-empty",
			Name:            "Empty Subpath Proxy Test",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     agentServer.URL + "/invoke",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
		},
	}

	handler := New(cfg, "test")

	// Call without trailing slash on agent ID
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/house-proxy-empty",
		strings.NewReader(`{"hello":"world"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if receivedPath != "/invoke" {
		t.Errorf("expected proxied path '/invoke', got %q", receivedPath)
	}

	// Call with trailing slash on agent ID
	req2 := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/house-proxy-empty/",
		strings.NewReader(`{"hello":"world"}`),
	)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()

	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if receivedPath != "/invoke" {
		t.Errorf("expected proxied path '/invoke' with trailing slash, got %q", receivedPath)
	}
}

func TestAgentsEndpointAddsAgentToYAMLConfig(t *testing.T) {
	agentCardServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"name":        "Kitchen Helper",
				"description": "Local kitchen assistant",
				"version":     "1.0.0",
				"supportedInterfaces": []map[string]string{
					{
						"url":             "http://127.0.0.1:9797/invoke",
						"protocolBinding": "JSONRPC",
						"protocolVersion": "1.0",
					},
				},
				"defaultInputModes":  []string{"text/plain"},
				"defaultOutputModes": []string{"text/plain"},
			})
		}),
	)
	defer agentCardServer.Close()

	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	cfg.Agents = nil
	allowAgentCardURL(&cfg, agentCardServer.URL)
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
	if reloaded.Agents[0].ID != "kitchen-helper" ||
		reloaded.Agents[0].EndpointURL != "http://127.0.0.1:9797/invoke" ||
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

func allowAgentCardURL(cfg *config.Config, cardURL string) {
	cfg.A2A.URLs = append(cfg.A2A.URLs, cardURL)
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

// Removed fakeMessageSender, fakeTaskHistorySender, fakeStreamingSender structs in favor of a2a.InMemoryClient

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
