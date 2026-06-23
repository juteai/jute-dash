package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"jute-dash/apps/hub/internal/app/config"
	"jute-dash/apps/hub/internal/app/repository"
	"jute-dash/apps/hub/internal/app/service"
	a2a "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/displayactions"
	"jute-dash/apps/hub/internal/pkg/filesync"
	"jute-dash/apps/hub/internal/pkg/middleware"
	"jute-dash/apps/hub/internal/pkg/registry"
	"jute-dash/apps/hub/tests/mocks"

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

func waitForConfig(
	t *testing.T,
	configPath string,
	matches func(config.Config) bool,
) config.Config {
	t.Helper()
	var reloaded config.Config
	var err error
	for range 100 {
		reloaded, err = LoadConfig(configPath)
		if err == nil && matches(reloaded) {
			return reloaded
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	t.Fatalf("config did not reach expected state: %+v", reloaded)
	return config.Config{}
}

func TestAgentProxyCORSPreflightAllowsA2AVersionHeader(t *testing.T) {
	handler := New(testConfig(), "test")
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/proxy/agents/house", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "a2a-version,content-type")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected local origin to be allowed, got %q", got)
	}
	allowedHeaders := strings.ToLower(rec.Header().Get("Access-Control-Allow-Headers"))
	for _, required := range []string{"content-type", "a2a-version"} {
		if !strings.Contains(allowedHeaders, required) {
			t.Fatalf("expected allowed headers to contain %q, got %q", required, allowedHeaders)
		}
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
		nil,
		nil,
		"",
		dispatcher,
	)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
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

func TestEventsStreamVoiceStateChanges(t *testing.T) {
	handler := New(testConfig(), "test")
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	resp, err := ts.Client().Do(req)
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

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/api/v1/voice/unmute", nil)
	if err != nil {
		t.Fatalf("create unmute request: %v", err)
	}
	postResp, err := ts.Client().Do(postReq)
	if err != nil {
		t.Fatalf("unmute voice: %v", err)
	}
	_ = postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("unmute status = %d", postResp.StatusCode)
	}

	var sawVoiceState bool
	var sawSafeStatus bool
	for range 20 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read event stream: %v", err)
		}
		if strings.Contains(line, "event: voice.state_changed") {
			sawVoiceState = true
		}
		if strings.Contains(line, `"deviceId":"default-display"`) &&
			strings.Contains(line, `"serviceStatus":"not_configured"`) &&
			!strings.Contains(strings.ToLower(line), "secret") {
			sawSafeStatus = true
		}
		if sawVoiceState && sawSafeStatus {
			break
		}
	}
	if !sawVoiceState {
		t.Fatal("event stream did not include voice.state_changed")
	}
	if !sawSafeStatus {
		t.Fatal("event stream did not include safe voice status payload")
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
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
	)

	payload := bytes.NewBufferString(`{
		"home":{"name":"Updated Home"},
		"display":{"theme":"dark","accentColor":"teal","idleMode":"ambient"}
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
	if body.Home.Name != "Updated Home" || !body.Setup.Complete {
		t.Fatalf("unexpected household settings: %+v", body)
	}
	reloaded, err := runtimeStore.HomestateRepo.HouseholdSettings(context.Background())
	if err != nil {
		t.Fatalf("reload household settings: %v", err)
	}
	if reloaded.Home.Name != "Updated Home" {
		t.Fatalf("settings did not persist: %+v", reloaded)
	}
}

func TestHouseholdSettingsEndpointUpdatesYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()

	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		configPath,
		nil,
	)

	payload := bytes.NewBufferString(`{
		"home":{"name":"YAML Home"},
		"display":{"theme":"light","accentColor":"teal","idleMode":"ambient"}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Poll configPath for changes since syncing is asynchronous
	var reloaded config.Config
	var err error
	for range 100 {
		reloaded, err = LoadConfig(configPath)
		if err == nil && reloaded.Home.Name == "YAML Home" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Home.Name != "YAML Home" || len(reloaded.Agents) != 2 {
		t.Fatalf("unexpected saved config: %+v", reloaded)
	}
}

func TestHouseholdSettingsEndpointNormalizesAppearanceForYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()

	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		configPath,
		nil,
	)

	payload := bytes.NewBufferString(`{
		"display":{
			"background":{"kind":"dynamic","value":"stardust"},
			"widgetChrome":{"default":"auto"}
		}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var reloaded config.Config
	var err error
	for range 100 {
		reloaded, err = LoadConfig(configPath)
		if err == nil && reloaded.Display.Background.Kind == "dynamic" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if reloaded.Display.Background.Value != "stardust" ||
		reloaded.Display.Background.Fit != "cover" ||
		reloaded.Display.Background.Position != "center" ||
		reloaded.Display.Background.Overlay != "none" {
		t.Fatalf("appearance background was not normalized: %+v", reloaded.Display.Background)
	}
	if reloaded.Display.WidgetChrome.Default != "auto" {
		t.Fatalf("appearance widget chrome was not persisted: %+v", reloaded.Display.WidgetChrome)
	}
}

func TestHouseholdSettingsEndpointPersistsRepeatedAppearanceSavesToYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()

	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		configPath,
		nil,
	)

	firstPayload := bytes.NewBufferString(`{
		"display":{
			"colorMode":"dark",
			"background":{"kind":"dynamic","value":"stardust"},
			"widgetChrome":{"default":"smoked","smokedOpacity":0.4}
		}
	}`)
	firstReq := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", firstPayload)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("expected first status 200, got %d: %s", firstRec.Code, firstRec.Body.String())
	}
	waitForConfig(t, configPath, func(cfg config.Config) bool {
		return cfg.Display.ColorMode == "dark" &&
			cfg.Display.WidgetChrome.Default == "smoked"
	})

	secondPayload := bytes.NewBufferString(`{
		"display":{
			"colorMode":"light",
			"background":{"kind":"dynamic","value":"weather-ambient"},
			"widgetChrome":{"default":"auto"}
		}
	}`)
	secondReq := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", secondPayload)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("expected second status 200, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
	reloaded := waitForConfig(t, configPath, func(cfg config.Config) bool {
		return cfg.Display.ColorMode == "light" &&
			cfg.Display.Background.Value == "weather-ambient" &&
			cfg.Display.WidgetChrome.Default == "auto"
	})
	if reloaded.Display.ColorMode != "light" ||
		reloaded.Display.Background.Value != "weather-ambient" ||
		reloaded.Display.WidgetChrome.Default != "auto" {
		t.Fatalf("second appearance save was not written to YAML: %+v", reloaded.Display)
	}
}

func TestHouseholdSettingsEndpointClearsThemeBackgroundValue(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
	)

	payload := bytes.NewBufferString(`{
		"display":{
			"background":{"kind":"theme","value":"stale-upload.jpg"},
			"widgetChrome":{"default":"solid"}
		}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Display struct {
			Background map[string]any `json:"background"`
		} `json:"display"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body.Display.Background["value"]; got != "" {
		t.Fatalf("theme background value was not cleared: %+v", body.Display.Background)
	}
}

func TestHouseholdSettingsEndpointRejectsInvalidAppearance(t *testing.T) {
	handler := NewWithSetupStatus(testConfig(), "test", SetupStatus{Complete: true})
	payload := bytes.NewBufferString(`{
		"display":{
			"background":{"kind":"color","value":""},
			"widgetChrome":{"default":"auto"}
		}
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/household", payload)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "display.background.value is required") {
		t.Fatalf("unexpected error response: %s", rec.Body.String())
	}
}

func TestServerStartupPersistsNormalizedLayoutVariantsToYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	cfg.Dashboard.SchemaVersion = 0
	cfg.Dashboard.DefaultVariant = ""
	cfg.Dashboard.Variants = nil
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()

	_ = NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		configPath,
		nil,
	)

	reloaded := waitForConfig(t, configPath, func(cfg config.Config) bool {
		return cfg.Dashboard.SchemaVersion == repository.LayoutSchemaVersion &&
			cfg.Dashboard.DefaultScreen != "" &&
			len(cfg.Dashboard.Screens) > 0 &&
			cfg.Dashboard.Screens[0].DefaultVariant != "" &&
			len(cfg.Dashboard.Screens[0].Variants) > 0
	})
	if reloaded.Dashboard.SchemaVersion != repository.LayoutSchemaVersion {
		t.Fatalf("schema version was not persisted: %+v", reloaded.Dashboard)
	}
	if reloaded.Dashboard.DefaultScreen == "" || len(reloaded.Dashboard.Screens) == 0 {
		t.Fatalf("layout screens were not persisted: %+v", reloaded.Dashboard)
	}
	screen := reloaded.Dashboard.Screens[0]
	if screen.DefaultVariant == "" || len(screen.Variants) == 0 {
		t.Fatalf("layout variants were not persisted: %+v", screen)
	}
	firstWidgetID := screen.Widgets[0].ID
	if _, ok := screen.Variants[0].Placements[firstWidgetID]; !ok {
		t.Fatalf(
			"layout variant placement for %q was not persisted: %+v",
			firstWidgetID,
			screen.Variants[0],
		)
	}
}

func TestRoomSettingsEndpointUpdatesStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
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
	reloaded, err := runtimeStore.HomestateRepo.Rooms(context.Background())
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
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
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
	reloaded, err := runtimeStore.HomestateRepo.Tiles(context.Background())
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
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()

	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		configPath,
		nil,
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

	reloaded := waitForConfig(t, configPath, func(cfg config.Config) bool {
		return len(cfg.Rooms) == 1 && len(cfg.Tiles) == 1
	})
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
	cfg.Voice.STTProviderID = "local-stt"
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

func TestVoiceSettingsPatchPersistsSafeSettings(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
	)
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/voice/settings",
		bytes.NewBufferString(`{
			"enabled": true,
			"wakeWordModelId": "hey-jute",
			"wakeWordPhrase": "Hey Jute",
			"wakeSensitivity": 0.4,
			"sttProviderId": "local-stt",
			"ttsProviderId": "local-tts",
			"ttsVoiceId": "amy",
			"ttsEnabled": true,
			"ttsLocale": "en-GB",
			"ttsSpeed": 1.15,
			"ttsVolume": 0,
			"preferredAgentId": "house",
			"cloudOptIn": true,
			"commandProvidersEnabled": true,
			"followupWindowSeconds": 9,
			"microphoneProfile": "kitchen-array"
		}`),
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body VoiceStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Enabled ||
		body.WakeWordModelID != "hey-jute" ||
		body.WakeWordPhrase != "Hey Jute" ||
		body.WakeSensitivity != 0.4 ||
		body.STTProviderID != "local-stt" ||
		body.TTSProviderID != "local-tts" ||
		body.TTSVoiceID != "amy" ||
		!body.TTSEnabled ||
		body.TTSLocale != "en-GB" ||
		body.TTSSpeed != 1.15 ||
		body.TTSVolume != 0 ||
		body.PreferredAgentID != "house" ||
		!body.CloudOptIn ||
		!body.CommandProvidersEnabled ||
		body.FollowupWindowSeconds != 9 ||
		body.MicrophoneProfile != "kitchen-array" {
		t.Fatalf("unexpected voice settings response: %+v", body)
	}
}

func TestVoiceSettingsPatchRejectsUnsafeOrInvalidPayloads(t *testing.T) {
	handler := New(testConfig(), "test")
	tests := []struct {
		name string
		body string
	}{
		{
			name: "secret reference field",
			body: `{"enabled":true,"credentialEnv":"OPENAI_API_KEY"}`,
		},
		{
			name: "raw credential field",
			body: `{"enabled":true,"apiKey":"secret-value"}`,
		},
		{
			name: "invalid followup",
			body: `{"followupWindowSeconds":45}`,
		},
		{
			name: "trailing credential payload",
			body: `{"enabled":true,"cloudOptIn":false,"commandProvidersEnabled":false}{"apiKey":"secret-value","credentialEnv":"OPENAI_API_KEY"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPatch,
				"/api/v1/voice/settings",
				bytes.NewBufferString(tt.body),
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "OPENAI_API_KEY") ||
				strings.Contains(rec.Body.String(), "secret-value") {
				t.Fatalf("response leaked unsafe payload: %s", rec.Body.String())
			}
		})
	}
}

func TestVoiceSettingsPatchRejectsTrailingCredentialPayloadWithoutLoggingIt(t *testing.T) {
	handler := New(testConfig(), "test")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	loggedHandler := middleware.RequestLogger(logger)(handler)
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/voice/settings",
		bytes.NewBufferString(
			`{"enabled":true,"cloudOptIn":false,"commandProvidersEnabled":false}{"apiKey":"secret-value","credentialEnv":"OPENAI_API_KEY"}`,
		),
	)
	rec := httptest.NewRecorder()

	loggedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, leaked := range []string{
		"secret-value",
		"OPENAI_API_KEY",
		"credentialEnv",
		"apiKey",
	} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Fatalf("response leaked %q: %s", leaked, rec.Body.String())
		}
		if strings.Contains(logs.String(), leaked) {
			t.Fatalf("request log leaked %q: %s", leaked, logs.String())
		}
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

func TestTTSVoicesEndpointReturnsSelectedProviderVoices(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	cfg := testConfig()
	cfg.Voice.TTSProviderID = "local-tts"
	cfg.Voice.TTSVoiceID = "amy"
	cfg.Voice.TTSLocale = "en-GB"
	cfg.Voice.TTSSpeed = 1.1
	cfg.Voice.TTSVolume = 0.8
	insertTTSProvider(t, runtimeStore, "local-tts", "available", true, nil)
	deviceProfileID := "bedroom-display"
	selectedVoice := "dan"
	selectedLocale := "cy-GB"
	selectedSpeed := 0.9
	selectedVolume := 0.4
	if err := runtimeStore.DB().Create(&repository.SettingsDB{
		DeviceProfileID: deviceProfileID,
		TTSProviderID:   cfg.Voice.TTSProviderID,
		TTSVoiceID:      selectedVoice,
		TTSLocale:       selectedLocale,
		TTSSpeed:        selectedSpeed,
		TTSVolume:       selectedVolume,
	}).Error; err != nil {
		t.Fatalf("seed device profile voice settings: %v", err)
	}
	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
	)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/tts/voices?providerId=local-tts&deviceProfileId=bedroom-display",
		nil,
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ProviderID   string     `json:"providerId"`
		SetupStatus  string     `json:"setupStatus"`
		HealthStatus string     `json:"healthStatus"`
		VoiceID      string     `json:"selectedVoiceId"`
		Locale       string     `json:"locale"`
		Speed        float64    `json:"speed"`
		Volume       float64    `json:"volume"`
		Voices       []TTSVoice `json:"voices"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ProviderID != "local-tts" ||
		body.SetupStatus != "available" ||
		body.HealthStatus != "available" ||
		body.VoiceID != "dan" ||
		body.Locale != "cy-GB" ||
		body.Speed != 0.9 ||
		body.Volume != 0.4 ||
		len(body.Voices) != 2 {
		t.Fatalf("unexpected TTS voices response: %+v", body)
	}
}

func TestTTSSpeakSensitiveOutputDefaultsToVisualOnly(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.TTSProviderID = "local-tts"
	cfg.Voice.TTSVoiceID = "amy"
	handler := New(cfg, "test")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		bytes.NewBufferString(`{"text":"the door code is 1234","conversationId":"conversation-1"}`),
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body TTSActionResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.VisualOnly ||
		body.State != "visual_only" ||
		body.Reason != "sensitive_output_visual_only" {
		t.Fatalf("unexpected sensitive TTS response: %+v", body)
	}
}

func TestTTSActionRejectsTrailingSensitivePayloadWithoutLoggingIt(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.TTSProviderID = "local-tts"
	cfg.Voice.TTSVoiceID = "amy"
	handler := New(cfg, "test")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	loggedHandler := middleware.RequestLogger(logger)(handler)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/speak",
		bytes.NewBufferString(`{"text":"hello kitchen"}{"text":"the door code is 9876","apiKey":"secret-value"}`),
	)
	rec := httptest.NewRecorder()

	loggedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, leaked := range []string{
		"door code",
		"9876",
		"secret-value",
		"apiKey",
	} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Fatalf("response leaked %q: %s", leaked, rec.Body.String())
		}
		if strings.Contains(logs.String(), leaked) {
			t.Fatalf("request log leaked %q: %s", leaked, logs.String())
		}
	}
}

func TestTTSStopRejectsTrailingCredentialPayloadWithoutLoggingIt(t *testing.T) {
	handler := New(testConfig(), "test")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	loggedHandler := middleware.RequestLogger(logger)(handler)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tts/stop",
		bytes.NewBufferString(`{"reason":"barge_in"}{"credentialEnv":"OPENAI_API_KEY","token":"secret-value"}`),
	)
	rec := httptest.NewRecorder()

	loggedHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	for _, leaked := range []string{
		"OPENAI_API_KEY",
		"secret-value",
		"credentialEnv",
	} {
		if strings.Contains(rec.Body.String(), leaked) {
			t.Fatalf("response leaked %q: %s", leaked, rec.Body.String())
		}
		if strings.Contains(logs.String(), leaked) {
			t.Fatalf("request log leaked %q: %s", leaked, logs.String())
		}
	}
}

func TestTTSSpeakAndStopEndpointsEmitSafeEvents(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.TTSProviderID = "local-tts"
	cfg.Voice.TTSVoiceID = "amy"
	dispatcher := displayactions.NewDispatcher()
	handler := newServer(
		cfg,
		"test",
		nil,
		SetupStatus{Complete: true},
		DefaultWidgetLayout(),
		nil,
		nil,
		nil,
		"",
		dispatcher,
	)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("create stream request: %v", err)
	}
	streamResp, err := ts.Client().Do(streamReq)
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	defer streamResp.Body.Close()
	reader := bufio.NewReader(streamResp.Body)
	for range 8 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read initial stream: %v", err)
		}
		if strings.Contains(line, "event: hub.connected") {
			break
		}
	}

	speakReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		ts.URL+"/api/v1/tts/speak",
		bytes.NewBufferString(
			`{"text":"hello kitchen","conversationId":"conversation-1","turnId":"turn-1"}`,
		),
	)
	if err != nil {
		t.Fatalf("create speak request: %v", err)
	}
	speakResp, err := ts.Client().Do(speakReq)
	if err != nil {
		t.Fatalf("speak request: %v", err)
	}
	_ = speakResp.Body.Close()
	if speakResp.StatusCode != http.StatusOK {
		t.Fatalf("speak status = %d", speakResp.StatusCode)
	}

	stopReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		ts.URL+"/api/v1/tts/stop",
		bytes.NewBufferString(`{"reason":"barge_in"}`),
	)
	if err != nil {
		t.Fatalf("create stop request: %v", err)
	}
	stopResp, err := ts.Client().Do(stopReq)
	if err != nil {
		t.Fatalf("stop request: %v", err)
	}
	var stopBody TTSActionResponse
	if err := json.NewDecoder(stopResp.Body).Decode(&stopBody); err != nil {
		t.Fatalf("decode stop response: %v", err)
	}
	_ = stopResp.Body.Close()
	if stopBody.State != "stopped" || stopBody.Reason != "barge_in" {
		t.Fatalf("unexpected stop response: %+v", stopBody)
	}

	sawStarted := false
	sawCompleted := false
	sawStopped := false
	for range 40 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read stream: %v", err)
		}
		if strings.Contains(line, "event: tts.started") {
			sawStarted = true
		}
		if strings.Contains(line, "event: tts.completed") {
			sawCompleted = true
		}
		if strings.Contains(line, "event: tts.stopped") {
			sawStopped = true
		}
		if strings.Contains(line, "token") || strings.Contains(line, "secret") {
			t.Fatalf("TTS event leaked sensitive text: %s", line)
		}
		if sawStarted && sawCompleted && sawStopped {
			break
		}
	}
	if !sawStarted || !sawCompleted || !sawStopped {
		t.Fatalf("missing TTS events: started=%v completed=%v stopped=%v", sawStarted, sawCompleted, sawStopped)
	}
}

func TestVoiceFinalTranscriptStartsHubOwnedAgentTurn(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-voice-1",
			Status:         "completed",
			Text:           "The kitchen lights are on.",
		}, nil
	}
	handler := NewWithMessageSender(cfg, "test", client)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"turn on the kitchen lights","deviceId":"kitchen-display"}`),
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body VoiceFinalTranscriptResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Followup.Active || body.Followup.Turns != 1 || body.Followup.MaxTurns != service.MaxConversationTurns {
		t.Fatalf("unexpected follow-up response: %+v", body.Followup)
	}
	if len(client.SentMessages) != 1 {
		t.Fatalf("expected one A2A send, got %d", len(client.SentMessages))
	}
	sent := client.SentMessages[0]
	if sent.Text != "turn on the kitchen lights" ||
		sent.ConversationID == "" ||
		sent.BearerToken != "" {
		t.Fatalf("unexpected A2A request: %+v", sent)
	}
}

func TestVoiceFinalTranscriptEmitsDisplayEventsInOrder(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-voice-order",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	dispatcher := service.NewVoiceDispatcher()
	server := &Server{
		cfg:             cfg,
		agentsManager:   manager,
		messages:        client,
		voiceStore:      repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
		voiceDispatcher: dispatcher,
		voiceRuntime:    service.NewConversationRuntime(),
	}
	server.turnRunner = service.NewRunner(service.RunnerOptions{
		GetRegistry:    manager.ActiveRegistry,
		GetAgentConfig: manager.ConfiguredAgent,
		GetAgentCardCache: func(context.Context, registry.Agent) (service.AgentCardCache, bool) {
			return service.AgentCardCache{
				SelectedEndpointURL:     "https://agent.example.com/a2a/v1",
				SelectedProtocolBinding: a2a.ProtocolJSONRPC,
			}, true
		},
		GetDashboardContext: func(context.Context) map[string]any {
			return map[string]any{}
		},
		Messages: client,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	events := dispatcher.Subscribe(ctx)

	_, err := server.submitFinalTranscript(ctx, VoiceFinalTranscriptRequest{
		Text:     "turn on the kitchen lights",
		DeviceID: "kitchen-display",
	})
	if err != nil {
		t.Fatalf("submitFinalTranscript() error = %v", err)
	}
	expected := []string{
		service.EventConversationStarted,
		service.EventVoiceTranscriptFinal,
		service.EventConversationTurnStarted,
		service.EventConversationTurnCompleted,
		service.EventConversationFollowupStarted,
	}
	got := make([]string, 0, len(expected))
	conversationID := client.SentMessages[0].ConversationID
	for len(got) < len(expected) {
		select {
		case event := <-events:
			voiceEvent, ok := event.Data.(service.VoiceEvent)
			if !ok {
				t.Fatalf("unexpected event data: %+v", event.Data)
			}
			got = append(got, event.Type)
			if voiceEvent.DeviceID != "kitchen-display" || voiceEvent.ConversationID != conversationID {
				t.Fatalf("event lost display routing context: %+v", voiceEvent)
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for ordered voice events, got %v", got)
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(expected) {
		t.Fatalf("voice events out of order: got %v want %v", got, expected)
	}
}

func TestVoiceFinalTranscriptTriggersTTSEvents(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	cfg.Voice.TTSEnabled = true
	cfg.Voice.TTSProviderID = "local-tts"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			Status:         "completed",
			Text:           "The kitchen lights are on.",
		}, nil
	}
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	dispatcher := service.NewVoiceDispatcher()
	store := repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice)
	server := &Server{
		cfg:             cfg,
		agentsManager:   manager,
		messages:        client,
		voiceStore:      store,
		voiceDispatcher: dispatcher,
		voiceRuntime:    service.NewConversationRuntime(),
	}
	server.voiceSpeaker = service.NewSpeaker(store, dispatcher, nil)
	server.turnRunner = service.NewRunner(service.RunnerOptions{
		GetRegistry:    manager.ActiveRegistry,
		GetAgentConfig: manager.ConfiguredAgent,
		GetAgentCardCache: func(context.Context, registry.Agent) (service.AgentCardCache, bool) {
			return service.AgentCardCache{
				SelectedEndpointURL:     "https://agent.example.com/a2a/v1",
				SelectedProtocolBinding: a2a.ProtocolJSONRPC,
			}, true
		},
		GetDashboardContext: func(context.Context) map[string]any {
			return map[string]any{}
		},
		Messages: client,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	events := dispatcher.Subscribe(ctx)

	_, err := server.submitFinalTranscript(ctx, VoiceFinalTranscriptRequest{
		Text:     "turn on the kitchen lights",
		DeviceID: "kitchen-display",
	})
	if err != nil {
		t.Fatalf("submitFinalTranscript() error = %v", err)
	}

	var sawStarted, sawCompleted bool
	for !sawStarted || !sawCompleted {
		select {
		case event := <-events:
			voiceEvent, ok := event.Data.(service.VoiceEvent)
			if !ok {
				t.Fatalf("unexpected event data: %+v", event.Data)
			}
			switch event.Type {
			case service.EventTTSStarted:
				sawStarted = true
				if voiceEvent.DeviceID != "kitchen-display" ||
					voiceEvent.ConversationID == "" {
					t.Fatalf("TTS started lost voice routing context: %+v", voiceEvent)
				}
			case service.EventTTSCompleted:
				sawCompleted = true
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for TTS events started=%v completed=%v", sawStarted, sawCompleted)
		}
	}
}

func TestVoiceFinalTranscriptStreamsAssistantTextSafely(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.StubSendMessage(a2a.SendMessageResult{
		ConversationID: "voice-conversation-1",
		Status:         "completed",
		Text:           "The kitchen token=secret lights are on.",
	}, nil)
	handler := NewWithMessageSender(cfg, "test", client)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("create event stream request: %v", err)
	}
	streamResp, err := ts.Client().Do(streamReq)
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("event stream status = %d", streamResp.StatusCode)
	}
	reader := bufio.NewReader(streamResp.Body)
	for range 8 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read initial event stream: %v", err)
		}
		if strings.Contains(line, "event: hub.connected") {
			break
		}
	}

	postReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		ts.URL+"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"turn on the kitchen lights","deviceId":"kitchen-display"}`),
	)
	if err != nil {
		t.Fatalf("create transcript request: %v", err)
	}
	postResp, err := ts.Client().Do(postReq)
	if err != nil {
		t.Fatalf("post final transcript: %v", err)
	}
	_ = postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("transcript status = %d", postResp.StatusCode)
	}

	var awaitingTurnCompletedData bool
	for range 40 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read voice event stream: %v", err)
		}
		if strings.Contains(line, "event: conversation.turn_completed") {
			awaitingTurnCompletedData = true
			continue
		}
		if awaitingTurnCompletedData && strings.HasPrefix(line, "data: ") {
			if strings.Contains(line, "token=secret") {
				t.Fatalf("turn completed event leaked assistant secret text: %s", line)
			}
			if !strings.Contains(line, `"text":"The kitchen token=[redacted] lights are on."`) {
				t.Fatalf("turn completed event omitted safe assistant text: %s", line)
			}
			return
		}
	}
	t.Fatal("event stream did not include conversation.turn_completed data")
}

func TestLocalVoiceServiceBuilderRoutesCapturedUtteranceThroughSTT(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-local-voice-1",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	store := &fixtureActiveSTTVoiceStore{
		MemoryVoiceRepository: repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
		provider: &fixtureAppSTTProvider{
			result: service.STTResult{
				Text:       "turn on the kitchen lights",
				ProviderID: "local-stt",
				ModelID:    "tiny-en",
				Language:   "en-GB",
				Duration:   40 * time.Millisecond,
			},
		},
	}
	server := &Server{
		cfg:             cfg,
		agentsManager:   manager,
		messages:        client,
		voiceStore:      store,
		voiceDispatcher: service.NewVoiceDispatcher(),
		voiceRuntime:    service.NewConversationRuntime(),
	}
	server.turnRunner = service.NewRunner(service.RunnerOptions{
		GetRegistry:    manager.ActiveRegistry,
		GetAgentConfig: manager.ConfiguredAgent,
		GetAgentCardCache: func(context.Context, registry.Agent) (service.AgentCardCache, bool) {
			return service.AgentCardCache{
				SelectedEndpointURL:     "https://agent.example.com/a2a/v1",
				SelectedProtocolBinding: a2a.ProtocolJSONRPC,
			}, true
		},
		GetDashboardContext: func(context.Context) map[string]any {
			return map[string]any{}
		},
		Messages: client,
	})
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	voiceSvc, err := server.newLocalVoiceService(
		context.Background(),
		"",
		"kitchen-display",
		fixtureAppCapture{frames: []service.AudioFrame{
			fixtureAppFrame(start, 0, 0),
			fixtureAppFrame(start, 100*time.Millisecond, 42),
			fixtureAppFrame(start, 200*time.Millisecond, 0),
			fixtureAppFrame(start, 300*time.Millisecond, 0),
		}},
		fixtureAppVAD{threshold: 10},
	)
	if err != nil {
		t.Fatalf("newLocalVoiceService() error = %v", err)
	}

	if err := voiceSvc.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	waitForSentMessages(t, client, 1)

	if client.SentMessages[0].Text != "turn on the kitchen lights" ||
		client.SentMessages[0].ConversationID == "" {
		t.Fatalf("unexpected A2A request: %+v", client.SentMessages[0])
	}
}

func TestVoiceAudioRoutesBrowserPCMThroughHubSTT(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-browser-voice-1",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	stt := &fixtureAppSTTProvider{
		result: service.STTResult{
			Text:       "open the blinds",
			ProviderID: "local-stt",
			Duration:   40 * time.Millisecond,
		},
	}
	server := &Server{
		cfg:           cfg,
		agentsManager: manager,
		messages:      client,
		voiceStore: &fixtureActiveSTTVoiceStore{
			MemoryVoiceRepository: repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
			provider:              stt,
		},
		voiceDispatcher: service.NewVoiceDispatcher(),
		voiceRuntime:    service.NewConversationRuntime(),
	}
	server.turnRunner = service.NewRunner(service.RunnerOptions{
		GetRegistry:    manager.ActiveRegistry,
		GetAgentConfig: manager.ConfiguredAgent,
		GetAgentCardCache: func(context.Context, registry.Agent) (service.AgentCardCache, bool) {
			return service.AgentCardCache{
				SelectedEndpointURL:     "https://agent.example.com/a2a/v1",
				SelectedProtocolBinding: a2a.ProtocolJSONRPC,
			}, true
		},
		GetDashboardContext: func(context.Context) map[string]any { return map[string]any{} },
		Messages:            client,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/voice/audio", bytes.NewReader([]byte{0xff, 0x7f, 0xff, 0x7f}))
	req.Header.Set("X-Jute-Sample-Rate", "16000")
	req.Header.Set("X-Jute-Channels", "1")
	req.Header.Set("X-Jute-Device-Id", "browser-display")
	rec := httptest.NewRecorder()

	server.handleVoiceAudio(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if client.SentMessages[0].Text != "open the blinds" {
		t.Fatalf("unexpected sent voice message: %+v", client.SentMessages[0])
	}
	if len(stt.seen.Frames) == 0 {
		t.Fatalf("STT did not receive browser utterance frames: %+v", stt.seen)
	}
}

func TestVoiceAudioReportsSTTFailure(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	server := &Server{
		cfg: cfg,
		voiceStore: &fixtureActiveSTTVoiceStore{
			MemoryVoiceRepository: repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
			provider:              &fixtureAppSTTProvider{err: errors.New("stt failed")},
		},
		voiceDispatcher: service.NewVoiceDispatcher(),
		voiceRuntime:    service.NewConversationRuntime(),
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/audio",
		bytes.NewReader([]byte{0xff, 0x7f, 0xff, 0x7f}),
	)
	rec := httptest.NewRecorder()

	server.handleVoiceAudio(rec, req)

	if rec.Code != http.StatusServiceUnavailable ||
		!strings.Contains(rec.Body.String(), "transcription_failed") {
		t.Fatalf("expected transcription failure, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestVoiceAudioWakeRequestRoutesDetectedCommandThroughSTT(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	cfg.Voice.WakeWordPhrase = "hey jarvis"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-wake-command",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	stt := &fixtureAppSTTProvider{
		result: service.STTResult{Text: "hey jarvis turn on the lights", ProviderID: "local-stt"},
	}
	dispatcher := service.NewVoiceDispatcher()
	store := &fixtureActiveWakeSTTVoiceStore{
		fixtureActiveSTTVoiceStore: &fixtureActiveSTTVoiceStore{
			MemoryVoiceRepository: repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
			provider:              stt,
		},
		wake: &fixtureAppWakeProvider{detected: true},
	}
	server := &Server{
		cfg:             cfg,
		agentsManager:   manager,
		messages:        client,
		voiceStore:      store,
		voiceDispatcher: dispatcher,
		voiceRuntime:    service.NewConversationRuntime(),
	}
	server.turnRunner = service.NewRunner(service.RunnerOptions{
		GetRegistry:    manager.ActiveRegistry,
		GetAgentConfig: manager.ConfiguredAgent,
		GetAgentCardCache: func(context.Context, registry.Agent) (service.AgentCardCache, bool) {
			return service.AgentCardCache{
				SelectedEndpointURL:     "https://agent.example.com/a2a/v1",
				SelectedProtocolBinding: a2a.ProtocolJSONRPC,
			}, true
		},
		GetDashboardContext: func(context.Context) map[string]any { return map[string]any{} },
		Messages:            client,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/voice/audio?wake=true", bytes.NewReader([]byte{0xff, 0x7f}))
	rec := httptest.NewRecorder()

	server.handleVoiceAudio(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !store.wake.seen {
		t.Fatal("wake provider was not called")
	}
	if stt.calls != 1 {
		t.Fatalf("wake audio should be transcribed once, STT calls=%d", stt.calls)
	}
	if len(client.SentMessages) != 1 ||
		client.SentMessages[0].Text != "turn on the lights" {
		t.Fatalf("unexpected A2A send: %+v", client.SentMessages)
	}
}

func TestVoiceAudioWakeOnlyStartsConversationWithoutA2A(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	cfg.Voice.WakeWordPhrase = "hey jarvis"
	client := a2a.NewInMemoryClient()
	stt := &fixtureAppSTTProvider{
		result: service.STTResult{Text: "hey jarvis", ProviderID: "local-stt"},
	}
	dispatcher := service.NewVoiceDispatcher()
	events := dispatcher.Subscribe(t.Context())
	store := &fixtureActiveWakeSTTVoiceStore{
		fixtureActiveSTTVoiceStore: &fixtureActiveSTTVoiceStore{
			MemoryVoiceRepository: repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
			provider:              stt,
		},
		wake: &fixtureAppWakeProvider{detected: true},
	}
	server := &Server{
		cfg:             cfg,
		messages:        client,
		voiceStore:      store,
		voiceDispatcher: dispatcher,
		voiceRuntime:    service.NewConversationRuntime(),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/voice/audio?wake=true", bytes.NewReader([]byte{0xff, 0x7f}))
	rec := httptest.NewRecorder()

	server.handleVoiceAudio(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if stt.calls != 1 {
		t.Fatalf("wake-only audio should still be transcribed once, STT calls=%d", stt.calls)
	}
	if len(client.SentMessages) != 0 {
		t.Fatalf("wake-only audio should not send A2A messages: %+v", client.SentMessages)
	}
	select {
	case event := <-events:
		if event.Type != service.EventVoiceWakeDetected {
			t.Fatalf("expected wake event, got %q", event.Type)
		}
		voiceEvent, ok := event.Data.(service.VoiceEvent)
		if !ok || voiceEvent.ConversationID == "" {
			t.Fatalf("wake event did not include conversation id: %+v", event.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for wake event")
	}
}

func TestLocalVoiceServiceBuilderEmitsRecoverableErrorWhenSTTFails(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	dispatcher := service.NewVoiceDispatcher()
	store := &fixtureActiveSTTVoiceStore{
		MemoryVoiceRepository: repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
		provider: &fixtureAppSTTProvider{
			err: errors.New("dial tcp 127.0.0.1:10300: token=secret unavailable"),
		},
	}
	server := &Server{
		cfg:             cfg,
		agentsManager:   manager,
		messages:        client,
		voiceStore:      store,
		voiceDispatcher: dispatcher,
		voiceRuntime:    service.NewConversationRuntime(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := dispatcher.Subscribe(ctx)
	start := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	voiceSvc, err := server.newLocalVoiceService(
		context.Background(),
		"",
		"kitchen-display",
		fixtureAppCapture{frames: []service.AudioFrame{
			fixtureAppFrame(start, 0, 42),
			fixtureAppFrame(start, 100*time.Millisecond, 0),
			fixtureAppFrame(start, 200*time.Millisecond, 0),
		}},
		fixtureAppVAD{threshold: 10},
	)
	if err != nil {
		t.Fatalf("newLocalVoiceService() error = %v", err)
	}

	if err := voiceSvc.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	event := waitForVoiceStateEvent(t, events, service.ServiceStateError)
	recovered := waitForVoiceStateEvent(t, events, "wake_listening")

	if len(client.SentMessages) != 0 {
		t.Fatalf("STT failure should not send A2A messages: %+v", client.SentMessages)
	}
	payload, ok := event.Data.(service.VoiceEvent)
	if !ok {
		t.Fatalf("unexpected event data: %+v", event.Data)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if strings.Contains(string(raw), "127.0.0.1") ||
		strings.Contains(string(raw), "token=secret") ||
		strings.Contains(string(raw), "dial tcp") {
		t.Fatalf("voice error event leaked provider details: %s", raw)
	}
	recoveredPayload, ok := recovered.Data.(service.VoiceEvent)
	if !ok {
		t.Fatalf("unexpected recovery event data: %+v", recovered.Data)
	}
	state, ok := recoveredPayload.Payload.(service.VoiceStatePayload)
	if !ok || state.State != "wake_listening" || state.ServiceStatus != "ready" {
		t.Fatalf("unexpected recovery state after STT failure: %+v", recoveredPayload.Payload)
	}
}

func TestVoiceFinalTranscriptContinuesFollowupConversation(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-followup",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}
	handler := NewWithMessageSender(cfg, "test", client)

	first := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"start a timer"}`),
	)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d: %s", firstRec.Code, firstRec.Body.String())
	}
	firstConversationID := client.SentMessages[0].ConversationID

	second := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"make it ten minutes","conversationId":"`+firstConversationID+`"}`),
	)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second status = %d: %s", secondRec.Code, secondRec.Body.String())
	}
	if len(client.SentMessages) != 2 {
		t.Fatalf("expected two A2A sends, got %d", len(client.SentMessages))
	}
	if client.SentMessages[1].ConversationID != firstConversationID {
		t.Fatalf("follow-up used wrong conversation: %+v", client.SentMessages[1])
	}
}

func TestVoiceFinalTranscriptEndsFollowupAfterMaxTurns(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-followup-limit",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}
	syncer := filesync.NewInMemorySyncer(cfg)
	cards := service.NewCardService()
	manager := service.NewAgentManager(syncer, cards, "")
	dispatcher := service.NewVoiceDispatcher()
	server := &Server{
		cfg:             cfg,
		agentsManager:   manager,
		messages:        client,
		voiceStore:      repository.NewMemoryVoiceRepositoryFromConfig(cfg.Voice),
		voiceDispatcher: dispatcher,
		voiceRuntime:    service.NewConversationRuntime(),
	}
	server.turnRunner = service.NewRunner(service.RunnerOptions{
		GetRegistry:    manager.ActiveRegistry,
		GetAgentConfig: manager.ConfiguredAgent,
		GetAgentCardCache: func(context.Context, registry.Agent) (service.AgentCardCache, bool) {
			return service.AgentCardCache{
				SelectedEndpointURL:     "https://agent.example.com/a2a/v1",
				SelectedProtocolBinding: a2a.ProtocolJSONRPC,
			}, true
		},
		GetDashboardContext: func(context.Context) map[string]any {
			return map[string]any{}
		},
		Messages: client,
	})

	var response VoiceFinalTranscriptResponse
	conversationID := ""
	for i := range service.MaxConversationTurns {
		req := VoiceFinalTranscriptRequest{
			Text:           "turn",
			ConversationID: conversationID,
			DeviceID:       "kitchen-display",
		}
		var err error
		response, err = server.submitFinalTranscript(context.Background(), req)
		if err != nil {
			t.Fatalf("turn %d submitFinalTranscript() error = %v", i, err)
		}
		if conversationID == "" {
			conversationID = client.SentMessages[0].ConversationID
		}
	}

	if response.Followup.Active ||
		response.Followup.Turns != service.MaxConversationTurns ||
		response.Followup.MaxTurns != service.MaxConversationTurns ||
		response.Followup.ExpiresAt != "" {
		t.Fatalf("expected inactive follow-up at max turns, got %+v", response.Followup)
	}
	if len(client.SentMessages) != service.MaxConversationTurns {
		t.Fatalf("expected %d A2A sends, got %d", service.MaxConversationTurns, len(client.SentMessages))
	}

	_, err := server.submitFinalTranscript(context.Background(), VoiceFinalTranscriptRequest{
		Text:           "one too many",
		ConversationID: conversationID,
		DeviceID:       "kitchen-display",
	})
	var transcriptErr voiceTranscriptError
	ok := errors.As(err, &transcriptErr)
	if !ok || transcriptErr.status != http.StatusConflict {
		t.Fatalf("expected follow-up conflict after max turns, got %T %v", err, err)
	}
	if len(client.SentMessages) != service.MaxConversationTurns {
		t.Fatalf("follow-up limit should block extra A2A send, got %d sends", len(client.SentMessages))
	}
}

func TestVoiceFinalTranscriptRejectsRawAudioPayloads(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	handler := New(cfg, "test")

	tests := []struct {
		name string
		body string
	}{
		{
			name: "pre-roll pcm",
			body: `{"text":"hello","preRollPcm":"raw-audio"}`,
		},
		{
			name: "raw audio pcm",
			body: `{"text":"hello","rawAudioPcm":"raw-audio"}`,
		},
		{
			name: "audio frames",
			body: `{"text":"hello","frames":[{"pcm":"raw-audio"}]}`,
		},
		{
			name: "provider internals",
			body: `{"text":"hello","providerPayload":{"audio":"raw-audio","secret":"token=secret"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/voice/transcripts/final",
				bytes.NewBufferString(tt.body),
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "raw-audio") ||
				strings.Contains(rec.Body.String(), "token=secret") {
				t.Fatalf("error response leaked raw payload: %s", rec.Body.String())
			}
		})
	}
}

func TestVoiceTranscriptRequestLoggingOmitsRawTranscriptsAndAudio(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	cfg.Voice.PreferredAgentID = "house"
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{
			ConversationID: req.ConversationID,
			TaskID:         "task-log-safety",
			Status:         "completed",
			Text:           "Done.",
		}, nil
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	handler := middleware.RequestLogger(logger)(NewWithMessageSender(cfg, "test", client))

	transcriptNeedle := "RAW_TRANSCRIPT_SHOULD_NOT_BE_LOGGED"
	audioNeedle := "RAW_AUDIO_SHOULD_NOT_BE_LOGGED"
	validReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"`+transcriptNeedle+`","deviceId":"kitchen-display"}`),
	)
	validRec := httptest.NewRecorder()
	handler.ServeHTTP(validRec, validReq)
	if validRec.Code != http.StatusOK {
		t.Fatalf("valid transcript status = %d: %s", validRec.Code, validRec.Body.String())
	}

	rawAudioReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(
			`{"text":"hello","rawAudioPcm":"`+audioNeedle+`","frames":[{"pcm":"`+audioNeedle+`"}],"providerPayload":{"audio":"`+audioNeedle+`"}}`,
		),
	)
	rawAudioRec := httptest.NewRecorder()
	handler.ServeHTTP(rawAudioRec, rawAudioReq)
	if rawAudioRec.Code != http.StatusBadRequest {
		t.Fatalf("raw audio status = %d: %s", rawAudioRec.Code, rawAudioRec.Body.String())
	}

	logBody := logs.String()
	for _, leaked := range []string{
		transcriptNeedle,
		audioNeedle,
		"rawAudioPcm",
		"frames",
		"providerPayload",
	} {
		if strings.Contains(logBody, leaked) {
			t.Fatalf("request log leaked %q: %s", leaked, logBody)
		}
	}
	if !strings.Contains(logBody, "/api/v1/voice/transcripts/final") {
		t.Fatalf("expected request metadata in logs, got %s", logBody)
	}
}

func TestVoiceFinalTranscriptCancelClearsFollowup(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{ConversationID: req.ConversationID, Status: "completed", Text: "Done."}, nil
	}
	handler := NewWithMessageSender(cfg, "test", client)

	first := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"start listening"}`),
	)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d: %s", firstRec.Code, firstRec.Body.String())
	}
	conversationID := client.SentMessages[0].ConversationID

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/voice/cancel", nil)
	cancelRec := httptest.NewRecorder()
	handler.ServeHTTP(cancelRec, cancelReq)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d: %s", cancelRec.Code, cancelRec.Body.String())
	}

	followup := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"continue","conversationId":"`+conversationID+`"}`),
	)
	followupRec := httptest.NewRecorder()
	handler.ServeHTTP(followupRec, followup)
	if followupRec.Code != http.StatusConflict {
		t.Fatalf("expected expired follow-up after cancel, got %d: %s", followupRec.Code, followupRec.Body.String())
	}
}

func TestVoiceCancelEmitsConversationEndedEvent(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	client := a2a.NewInMemoryClient()
	client.SendMessageFunc = func(_ context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
		return a2a.SendMessageResult{ConversationID: req.ConversationID, Status: "completed", Text: "Done."}, nil
	}
	handler := NewWithMessageSender(cfg, "test", client)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("create event stream request: %v", err)
	}
	streamResp, err := ts.Client().Do(streamReq)
	if err != nil {
		t.Fatalf("open event stream: %v", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("event stream status = %d", streamResp.StatusCode)
	}
	reader := bufio.NewReader(streamResp.Body)
	for range 8 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read initial event stream: %v", err)
		}
		if strings.Contains(line, "event: hub.connected") {
			break
		}
	}

	firstReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		ts.URL+"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"start listening","deviceId":"kitchen-display"}`),
	)
	if err != nil {
		t.Fatalf("create transcript request: %v", err)
	}
	firstResp, err := ts.Client().Do(firstReq)
	if err != nil {
		t.Fatalf("post final transcript: %v", err)
	}
	_ = firstResp.Body.Close()
	if firstResp.StatusCode != http.StatusOK {
		t.Fatalf("transcript status = %d", firstResp.StatusCode)
	}
	if len(client.SentMessages) != 1 {
		t.Fatalf("expected one A2A send, got %d", len(client.SentMessages))
	}
	conversationID := client.SentMessages[0].ConversationID

	cancelReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ts.URL+"/api/v1/voice/cancel", nil)
	if err != nil {
		t.Fatalf("create cancel request: %v", err)
	}
	cancelResp, err := ts.Client().Do(cancelReq)
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	_ = cancelResp.Body.Close()
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("cancel status = %d", cancelResp.StatusCode)
	}

	var awaitingEndedData bool
	for range 60 {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read voice event stream: %v", err)
		}
		if strings.Contains(line, "event: conversation.ended") {
			awaitingEndedData = true
			continue
		}
		if awaitingEndedData && strings.HasPrefix(line, "data: ") {
			if !strings.Contains(line, `"conversationId":"`+conversationID+`"`) ||
				!strings.Contains(line, `"deviceId":"kitchen-display"`) ||
				!strings.Contains(line, `"reason":"canceled"`) {
				t.Fatalf("conversation ended event did not describe cancellation safely: %s", line)
			}
			return
		}
	}
	t.Fatal("event stream did not include conversation.ended after cancel")
}

func TestVoiceFinalTranscriptReturnsSafeAgentFailure(t *testing.T) {
	cfg := testConfig()
	cfg.Voice.Enabled = true
	cfg.Voice.MutedByDefault = false
	client := a2a.NewInMemoryClient()
	client.StubSendMessage(
		a2a.SendMessageResult{},
		errors.New("token SECRET_VALUE failed at https://agent.example.com/private"),
	)
	handler := NewWithMessageSender(cfg, "test", client)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/voice/transcripts/final",
		bytes.NewBufferString(`{"text":"hello"}`),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "SECRET_VALUE") ||
		strings.Contains(rec.Body.String(), "agent.example.com/private") {
		t.Fatalf("agent failure leaked internals: %s", rec.Body.String())
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
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
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
	reloaded, err := runtimeStore.DashboardRepo.WidgetLayout(context.Background(), "")
	if err != nil {
		t.Fatalf("WidgetLayout() error = %v", err)
	}
	if reloaded.Widgets[0].X != 1 || reloaded.Widgets[1].Visible {
		t.Fatalf("layout did not persist: %+v", reloaded.Widgets)
	}
}

func TestWidgetLayoutPutPersistsToYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		configPath,
		nil,
	)

	layout := DefaultWidgetLayout()
	layout.DefaultVariant = "desktop"
	layout.Variants[3].Columns = 14
	layout.Variants[3].Rows = 7
	layout.Variants[3].Placements[layout.Widgets[0].ID] = WidgetPlacement{
		X: 3,
		Y: 1,
		W: 4,
		H: 1,
	}
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

	var reloaded config.Config
	for range 100 {
		reloaded, err = LoadConfig(configPath)
		if err == nil &&
			len(reloaded.Dashboard.Screens) > 0 &&
			reloaded.Dashboard.Screens[0].DefaultVariant == "desktop" &&
			len(reloaded.Dashboard.Screens[0].Variants) > 3 &&
			reloaded.Dashboard.Screens[0].Variants[3].Columns == 14 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Dashboard.Screens) == 0 ||
		reloaded.Dashboard.Screens[0].DefaultVariant != "desktop" ||
		len(reloaded.Dashboard.Screens[0].Variants) <= 3 ||
		reloaded.Dashboard.Screens[0].Variants[3].Columns != 14 {
		t.Fatalf("layout was not written to YAML config: %+v", reloaded.Dashboard)
	}
	if got := reloaded.Dashboard.Screens[0].Variants[3].Placements[layout.Widgets[0].ID]; got.X != 3 || got.W != 4 {
		t.Fatalf("layout placement was not written to YAML config: %+v", got)
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
	if !strings.HasPrefix(body["error"], "invalid widget layout") {
		t.Fatalf("unexpected error response: %+v", body)
	}
}

func TestWidgetLayoutResetEndpoint(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	layout := DefaultWidgetLayout()
	layout.Widgets[0].Visible = false
	if _, err := runtimeStore.DashboardRepo.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
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
	if len(body.Widgets) != 4 || !body.Widgets[0].Visible {
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
	handler := NewServer(
		testConfig(),
		"test",
		SetupStatus{Complete: true},
		layoutStore,
		nil,
		nil,
		"",
		nil,
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
	logger := slog.New(slog.DiscardHandler)
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"), logger)
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
			writeJSON(w, map[string]any{
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
	logger := slog.New(slog.DiscardHandler)
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"), logger)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer runtimeStore.Close()
	result, err := runtimeStore.Initialize(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	handler := NewServer(
		cfg,
		"test",
		result.Setup,
		runtimeStore.DashboardRepo,
		runtimeStore.HomestateRepo,
		runtimeStore.VoiceRepo,
		"",
		nil,
	)

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
	var receivedA2AVersion string
	var receivedAccept string
	var receivedBody string
	var receivedPath string
	var receivedQuery string
	agentServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuthHeader = r.Header.Get("Authorization")
			receivedA2AVersion = r.Header.Get("A2a-Version")
			receivedAccept = r.Header.Get("Accept")
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read proxied body: %v", err)
			}
			receivedBody = string(body)
			receivedPath = r.URL.Path
			receivedQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}),
	)
	defer agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []service.AgentConfig{
		{
			ID:              "house-proxy-test",
			Name:            "Concierge Proxy Test",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     agentServer.URL + "/api/v1/rpc",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
			Auth: &service.AuthConfig{
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
	req.Header.Set("A2a-Version", "1.0")
	req.Header.Set("Accept", "text/event-stream")
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
	if receivedA2AVersion != "1.0" {
		t.Errorf("expected A2A-Version 1.0, got %q", receivedA2AVersion)
	}
	if receivedAccept != "text/event-stream" {
		t.Errorf("expected streaming Accept header, got %q", receivedAccept)
	}
	if receivedBody != `{"hello":"world"}` {
		t.Errorf("expected request body to pass through unchanged, got %q", receivedBody)
	}
	if receivedPath != "/api/v1/rpc/foo/bar" {
		t.Errorf("expected proxied path '/api/v1/rpc/foo/bar', got %q", receivedPath)
	}
	if receivedQuery != "baz=qux" {
		t.Errorf("expected proxied query 'baz=qux', got %q", receivedQuery)
	}
}

func TestAgentProxyReturnsSafeBadGatewayWhenUpstreamIsUnavailable(t *testing.T) {
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpointURL := agentServer.URL + "/invoke"
	agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []service.AgentConfig{
		{
			ID:              "offline-proxy",
			Name:            "Offline Proxy",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     endpointURL,
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
			Auth: &service.AuthConfig{
				Type:     "bearer",
				EnvToken: "OFFLINE_PROXY_TOKEN",
			},
		},
	}
	t.Setenv("OFFLINE_PROXY_TOKEN", "must-not-leak")
	handler := New(cfg, "test")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/offline-proxy",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"SendMessage"}`),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, sensitive := range []string{endpointURL, "OFFLINE_PROXY_TOKEN", "must-not-leak"} {
		if strings.Contains(body, sensitive) {
			t.Fatalf("gateway response leaked %q: %s", sensitive, body)
		}
	}
}

func TestAgentProxyDoesNotAcceptBrowserSuppliedAuthorization(t *testing.T) {
	var receivedAuthHeader string
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		writeJSON(w, map[string]bool{"success": true})
	}))
	defer agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []service.AgentConfig{
		{
			ID:              "no-auth-proxy",
			Name:            "No Auth Proxy",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     agentServer.URL + "/invoke",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
		},
	}
	handler := New(cfg, "test")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/no-auth-proxy",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"SendMessage"}`),
	)
	req.Header.Set("Authorization", "Bearer browser-controlled-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("proxy status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if receivedAuthHeader != "" {
		t.Fatalf("proxy forwarded browser-supplied authorization: %q", receivedAuthHeader)
	}
}

func TestAgentProxyRejectsUnknownAndDisabledAgents(t *testing.T) {
	var upstreamCalls int
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		writeJSON(w, map[string]bool{"success": true})
	}))
	defer agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []service.AgentConfig{
		{
			ID:              "disabled-proxy",
			Name:            "Disabled Proxy",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     agentServer.URL + "/invoke",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         false,
		},
	}
	handler := New(cfg, "test")

	tests := []struct {
		name       string
		agentID    string
		wantStatus int
	}{
		{name: "unknown", agentID: "unknown-proxy", wantStatus: http.StatusNotFound},
		{name: "disabled", agentID: "disabled-proxy", wantStatus: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/proxy/agents/"+tt.agentID,
				strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"SendMessage"}`),
			)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
	if upstreamCalls != 0 {
		t.Fatalf("rejected proxy requests reached upstream %d times", upstreamCalls)
	}
}

func TestAgentProxyDoesNotFallBackToBrowserAuthWhenHubCredentialIsMissing(t *testing.T) {
	var upstreamCalls int
	var receivedAuthHeader string
	agentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		receivedAuthHeader = r.Header.Get("Authorization")
		writeJSON(w, map[string]bool{"success": true})
	}))
	defer agentServer.Close()

	cfg := testConfig()
	cfg.Agents = []service.AgentConfig{
		{
			ID:              "missing-token-proxy",
			Name:            "Missing Token Proxy",
			CardURL:         "https://agent.example.com/.well-known/agent-card.json",
			EndpointURL:     agentServer.URL + "/invoke",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
			Auth: &service.AuthConfig{
				Type:     "bearer",
				EnvToken: "MISSING_PROXY_TOKEN",
			},
		},
	}
	t.Setenv("MISSING_PROXY_TOKEN", "")
	handler := New(cfg, "test")

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/missing-token-proxy",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"SendMessage"}`),
	)
	req.Header.Set("Authorization", "Bearer browser-fallback-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("proxy status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("request without hub credential reached upstream %d times", upstreamCalls)
	}
	if receivedAuthHeader != "" {
		t.Fatalf("proxy used browser auth when hub credential was missing: %q", receivedAuthHeader)
	}
}

func TestAgentProxyUsesDiscoveredEndpoint(t *testing.T) {
	var staleEndpointCalled bool
	staleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staleEndpointCalled = true
		w.WriteHeader(http.StatusTeapot)
	}))
	defer staleServer.Close()

	var selectedEndpointCalled bool
	selectedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		selectedEndpointCalled = true
		writeJSON(w, map[string]bool{"success": true})
	}))
	defer selectedServer.Close()

	cardServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"name":    "Discovered Proxy Agent",
			"version": "1.0.0",
			"supportedInterfaces": []map[string]string{
				{
					"url":             selectedServer.URL + "/invoke",
					"protocolBinding": "JSONRPC",
					"protocolVersion": "1.0",
				},
			},
			"defaultInputModes":  []string{"text/plain"},
			"defaultOutputModes": []string{"text/plain"},
		})
	}))
	defer cardServer.Close()

	cfg := testConfig()
	cfg.Agents = []service.AgentConfig{
		{
			ID:              "discovered-proxy",
			Name:            "Discovered Proxy Agent",
			CardURL:         cardServer.URL,
			EndpointURL:     staleServer.URL + "/stale",
			ProtocolBinding: a2a.ProtocolJSONRPC,
			Enabled:         true,
		},
	}
	allowAgentCardURL(&cfg, cardServer.URL)
	handler := New(cfg, "test")

	discoverReq := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	discoverRec := httptest.NewRecorder()
	handler.ServeHTTP(discoverRec, discoverReq)
	if discoverRec.Code != http.StatusOK {
		t.Fatalf("discover status = %d, want 200: %s", discoverRec.Code, discoverRec.Body.String())
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/proxy/agents/discovered-proxy",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"SendMessage"}`),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("proxy status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !selectedEndpointCalled {
		t.Fatal("expected proxy to use the endpoint selected from the Agent Card")
	}
	if staleEndpointCalled {
		t.Fatal("proxy used the stale configured endpoint")
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
	cfg.Agents = []service.AgentConfig{
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
			writeJSON(w, map[string]any{
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
	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		configPath,
		nil,
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

func TestAgentEndpointPatchesEnabledStateInYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		configPath,
		nil,
	)

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/agents/house",
		bytes.NewBufferString(`{"enabled":false}`),
	)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "house" || body.Enabled {
		t.Fatalf("unexpected patched agent: %+v", body)
	}

	reloaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Agents) != 2 || reloaded.Agents[0].Enabled {
		t.Fatalf("patched state was not persisted: %+v", reloaded.Agents)
	}
}

func TestAgentEndpointDeletesAgentFromYAMLConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		configPath,
		nil,
	)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/agents/house", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Deleted bool `json:"deleted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Deleted {
		t.Fatalf("unexpected delete response: %+v", body)
	}

	reloaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Agents) != 1 || reloaded.Agents[0].ID != "energy" {
		t.Fatalf("agent deletion was not persisted: %+v", reloaded.Agents)
	}
}

func TestAgentEndpointRefreshesCardMetadata(t *testing.T) {
	agentCardServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, map[string]any{
				"name":        "House Concierge",
				"description": "Refreshed card",
				"version":     "1.0.0",
				"supportedInterfaces": []map[string]string{
					{
						"url":             "http://127.0.0.1:9797/refreshed",
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

	cfg := testConfig()
	cfg.Agents = cfg.Agents[:1]
	cfg.Agents[0].CardURL = agentCardServer.URL
	allowAgentCardURL(&cfg, agentCardServer.URL)
	handler := New(cfg, "test")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/house/refresh-card", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID                      string `json:"id"`
		CardStatus              string `json:"cardStatus"`
		SelectedEndpointURL     string `json:"selectedEndpointUrl"`
		SelectedProtocolVersion string `json:"selectedProtocolVersion"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "house" ||
		body.CardStatus != "available" ||
		body.SelectedEndpointURL != "http://127.0.0.1:9797/refreshed" ||
		body.SelectedProtocolVersion != "1.0" {
		t.Fatalf("unexpected refreshed agent: %+v", body)
	}
}

func TestAgentSubroutesValidateRequests(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "jute.yaml")
	cfg := testConfig()
	if err := SaveYAML(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	handler := NewServer(
		cfg,
		"test",
		SetupStatus{Complete: true},
		nil,
		nil,
		nil,
		configPath,
		nil,
	)
	tests := []struct {
		name      string
		method    string
		path      string
		body      string
		wantCode  int
		wantAllow string
		wantError string
	}{
		{
			name:      "unsupported agent method",
			method:    http.MethodGet,
			path:      "/api/v1/agents/house",
			wantCode:  http.StatusMethodNotAllowed,
			wantAllow: "PATCH, DELETE",
			wantError: "method not allowed",
		},
		{
			name:      "invalid patch JSON",
			method:    http.MethodPatch,
			path:      "/api/v1/agents/house",
			body:      "{",
			wantCode:  http.StatusBadRequest,
			wantError: "invalid JSON request body",
		},
		{
			name:      "missing enabled value",
			method:    http.MethodPatch,
			path:      "/api/v1/agents/house",
			body:      "{}",
			wantCode:  http.StatusBadRequest,
			wantError: "enabled is required",
		},
		{
			name:      "delete unknown agent",
			method:    http.MethodDelete,
			path:      "/api/v1/agents/missing",
			wantCode:  http.StatusNotFound,
			wantError: "agent not found",
		},
		{
			name:      "unknown agent",
			method:    http.MethodPost,
			path:      "/api/v1/agents/missing/refresh-card",
			wantCode:  http.StatusNotFound,
			wantError: "agent not found",
		},
		{
			name:      "unsupported refresh method",
			method:    http.MethodGet,
			path:      "/api/v1/agents/house/refresh-card",
			wantCode:  http.StatusMethodNotAllowed,
			wantAllow: http.MethodPost,
			wantError: "method not allowed",
		},
		{
			name:      "unknown agent route",
			method:    http.MethodPost,
			path:      "/api/v1/agents/house/unknown",
			wantCode:  http.StatusNotFound,
			wantError: "agent route not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("expected status %d, got %d: %s", tt.wantCode, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Fatalf("expected Allow %q, got %q", tt.wantAllow, got)
			}
			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body["error"] != tt.wantError {
				t.Fatalf("expected error %q, got %+v", tt.wantError, body)
			}
		})
	}
}

func testConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.Agents = []service.AgentConfig{
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	runtimeStore, err := Open(filepath.Join(t.TempDir(), "jute.db"), logger)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := runtimeStore.Initialize(context.Background(), testConfig(), true); err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	return runtimeStore
}

type fixtureActiveSTTVoiceStore struct {
	*repository.MemoryVoiceRepository

	provider service.STTProvider
}

func (s *fixtureActiveSTTVoiceStore) ActiveSTTProvider(context.Context, string) (service.STTProvider, error) {
	return s.provider, nil
}

type fixtureActiveWakeSTTVoiceStore struct {
	*fixtureActiveSTTVoiceStore

	wake *fixtureAppWakeProvider
}

func (s *fixtureActiveWakeSTTVoiceStore) ActiveWakeProvider(
	context.Context,
	string,
	string,
) (service.WakeProvider, error) {
	return s.wake, nil
}

type fixtureAppSTTProvider struct {
	result service.STTResult
	err    error
	seen   service.CapturedUtterance
	calls  int
}

func (p *fixtureAppSTTProvider) Transcribe(
	_ context.Context,
	utterance service.CapturedUtterance,
) (service.STTResult, error) {
	p.calls++
	p.seen = utterance
	if p.err != nil {
		return service.STTResult{}, p.err
	}
	return p.result, nil
}

type fixtureAppWakeProvider struct {
	detected bool
	seen     bool
}

func (p *fixtureAppWakeProvider) DetectWake(
	context.Context,
	service.CapturedUtterance,
) (service.WakeDetection, error) {
	p.seen = true
	return service.WakeDetection{Detected: p.detected}, nil
}

type fixtureAppCapture struct {
	frames []service.AudioFrame
}

func (c fixtureAppCapture) Capture(ctx context.Context) (<-chan service.AudioFrame, <-chan error) {
	frames := make(chan service.AudioFrame)
	errs := make(chan error)
	go func() {
		defer close(frames)
		defer close(errs)
		for _, frame := range c.frames {
			select {
			case <-ctx.Done():
				return
			case frames <- frame:
			}
		}
	}()
	return frames, errs
}

type fixtureAppVAD struct {
	threshold byte
}

func (v fixtureAppVAD) Speech(frame service.AudioFrame) bool {
	for _, sample := range frame.PCM {
		if sample >= v.threshold {
			return true
		}
	}
	return false
}

func fixtureAppFrame(start time.Time, offset time.Duration, sample byte) service.AudioFrame {
	return service.AudioFrame{
		PCM:         []byte{sample},
		SampleRate:  16000,
		SampleWidth: 2,
		Channels:    1,
		Timestamp:   start.Add(offset),
		Duration:    100 * time.Millisecond,
	}
}

func waitForSentMessages(t *testing.T, client *a2a.InMemoryClient, count int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(client.SentMessages) >= count {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d sent message(s), got %d", count, len(client.SentMessages))
}

func waitForVoiceStateEvent(
	t *testing.T,
	events <-chan displayactions.Event,
	state string,
) displayactions.Event {
	t.Helper()
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before voice state %q", state)
			}
			if event.Type != service.EventVoiceStateChanged {
				continue
			}
			voiceEvent, ok := event.Data.(service.VoiceEvent)
			if !ok {
				t.Fatalf("unexpected voice state event data: %+v", event.Data)
			}
			payload, ok := voiceEvent.Payload.(service.VoiceStatePayload)
			if !ok {
				t.Fatalf("unexpected voice state payload: %+v", voiceEvent.Payload)
			}
			if payload.State == state {
				return event
			}
		case <-timeout:
			t.Fatalf("timed out waiting for voice state %q", state)
		}
	}
}

// Removed fakeMessageSender, fakeTaskHistorySender, fakeStreamingSender structs in favor of a2a.InMemoryClient

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(value)
}
