package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	a2a "jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/store"
	"jute-dash/internal/weather"
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
	runtimeStore, err := store.Open(filepath.Join(t.TempDir(), "jute.db"))
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
	handler := newServer(result.Config, "test", weather.NewClient(), sender, result.Setup, store.DefaultWidgetLayout(), runtimeStore)

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
	handler := NewWithMessageSender(testConfig(), "test", &fakeMessageSender{err: errors.New("raw remote failure with internal details")})
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

func TestHomeEndpointIncludesWeather(t *testing.T) {
	handler := NewWithWeatherProvider(testConfig(), "test", weatherProviderFunc(func(ctx context.Context, cfg config.WeatherConfig) weather.State {
		temp := 18.4
		return weather.State{
			LocationName:    "Test Garden",
			Temperature:     &temp,
			TemperatureUnit: "°C",
			Condition:       "Clear sky",
			Icon:            "sun",
			Source:          weather.ProviderOpenMeteo,
			Status:          weather.StatusAvailable,
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/home", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body struct {
		Weather weather.State `json:"weather"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Weather.Status != weather.StatusAvailable {
		t.Fatalf("unexpected weather: %+v", body.Weather)
	}
	if body.Weather.Temperature == nil || *body.Weather.Temperature != 18.4 {
		t.Fatalf("unexpected temperature: %+v", body.Weather.Temperature)
	}
}

func TestSetupStatusEndpoint(t *testing.T) {
	handler := NewWithSetupStatus(testConfig(), "test", store.SetupStatus{
		Complete: false,
		Missing:  []string{"home.name"},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body store.SetupStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Complete || len(body.Missing) != 1 || body.Missing[0] != "home.name" {
		t.Fatalf("unexpected setup status: %+v", body)
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
		Providers []store.VoiceProviderPack `json:"providers"`
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
	layout := store.WidgetLayout{
		ProfileID: "default-dashboard",
		Widgets: []store.WidgetInstance{
			{ID: "date-time", Kind: "date-time", Title: "Date & Time", W: 2, H: 1, MinW: 1, MinH: 1, Size: "wide", Visible: true},
			{ID: "weather", Kind: "weather", Title: "Weather", X: 2, W: 2, H: 1, MinW: 1, MinH: 1, Size: "wide", Visible: true},
			{ID: "chat-history", Kind: "chat-history", Title: "Chat History", Y: 1, W: 2, H: 2, MinW: 1, MinH: 1, Size: "medium", Visible: true},
		},
	}
	handler := NewWithSetupStatusAndLayout(testConfig(), "test", store.SetupStatus{Complete: true}, layout)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/widgets/layout", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body store.WidgetLayout
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ProfileID != "default-dashboard" || len(body.Widgets) != 3 {
		t.Fatalf("unexpected layout response: %+v", body)
	}
	if body.Widgets[0].Kind != "date-time" || body.Widgets[1].Kind != "weather" || body.Widgets[2].Kind != "chat-history" {
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
		Widgets []store.WidgetCatalogItem `json:"widgets"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Widgets) != 3 || body.Widgets[0].Kind != "date-time" {
		t.Fatalf("unexpected catalog response: %+v", body.Widgets)
	}
}

func TestWidgetLayoutPutPersistsWithStore(t *testing.T) {
	runtimeStore := openInitializedServerStore(t)
	defer runtimeStore.Close()
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", store.SetupStatus{Complete: true}, runtimeStore)

	layout := store.DefaultWidgetLayout()
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
	payload := bytes.NewBufferString(`{"profileId":"default-dashboard","widgets":[{"id":"bad","kind":"missing","title":"Bad","x":0,"y":0,"w":1,"h":1,"minW":1,"minH":1,"size":"small","settings":{},"visible":true}]}`)
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
	layout := store.DefaultWidgetLayout()
	layout.Widgets[0].Visible = false
	if _, err := runtimeStore.SaveWidgetLayout(context.Background(), layout); err != nil {
		t.Fatalf("SaveWidgetLayout() error = %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", store.SetupStatus{Complete: true}, runtimeStore)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/widgets/layout/reset", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body store.WidgetLayout
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Widgets) != 3 || !body.Widgets[0].Visible {
		t.Fatalf("unexpected reset layout: %+v", body)
	}
}

func TestWidgetLayoutPutReturnsSafeStoreFailure(t *testing.T) {
	layout := store.DefaultWidgetLayout()
	handler := NewWithSetupStatusAndLayoutStore(testConfig(), "test", store.SetupStatus{Complete: true}, &failingLayoutStore{
		layout: layout,
		err:    errors.New("sqlite path /private/raw/details failed"),
	})
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
	runtimeStore, err := store.Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer runtimeStore.Close()

	result, err := runtimeStore.Initialize(context.Background(), testConfig(), true)
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	handler := NewWithWeatherProvider(result.Config, "test", weatherProviderFunc(func(ctx context.Context, cfg config.WeatherConfig) weather.State {
		return weather.State{Status: weather.StatusDisabled, LocationName: cfg.LocationName}
	}))

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
	if len(body.Agents) != 2 || body.Agents[0].ID != "energy" || body.Agents[1].ID != "house" {
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
	runtimeStore, err := store.Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer runtimeStore.Close()
	result, err := runtimeStore.Initialize(context.Background(), cfg, true)
	if err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	handler := NewWithSetupStatusAndLayoutStore(result.Config, "test", result.Setup, runtimeStore)

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
	if len(body.Agents) != 1 || body.Agents[0].CardStatus != "available" || body.Agents[0].SelectedEndpointURL != "http://agent.local/invoke" {
		t.Fatalf("unexpected agent discovery response: %+v", body.Agents)
	}
	if !body.Agents[0].DashboardContextSupported || !body.Agents[0].Streaming || len(body.Agents[0].Skills) != 1 {
		t.Fatalf("missing discovered metadata: %+v", body.Agents[0])
	}
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.Agents = []config.AgentConfig{
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

func openInitializedServerStore(t *testing.T) *store.Store {
	t.Helper()
	runtimeStore, err := store.Open(filepath.Join(t.TempDir(), "jute.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := runtimeStore.Initialize(context.Background(), testConfig(), true); err != nil {
		t.Fatalf("initialize store: %v", err)
	}
	return runtimeStore
}

type failingLayoutStore struct {
	layout store.WidgetLayout
	err    error
}

func (s *failingLayoutStore) WidgetLayout(ctx context.Context, profileID string) (store.WidgetLayout, error) {
	return s.layout, nil
}

func (s *failingLayoutStore) SaveWidgetLayout(ctx context.Context, layout store.WidgetLayout) (store.WidgetLayout, error) {
	return store.WidgetLayout{}, s.err
}

func (s *failingLayoutStore) ResetWidgetLayout(ctx context.Context, profileID string) (store.WidgetLayout, error) {
	return store.WidgetLayout{}, s.err
}

type weatherProviderFunc func(context.Context, config.WeatherConfig) weather.State

func (fn weatherProviderFunc) Current(ctx context.Context, cfg config.WeatherConfig) weather.State {
	return fn(ctx, cfg)
}

type fakeMessageSender struct {
	result a2a.SendMessageResult
	err    error
	last   a2a.SendMessageRequest
	called bool
}

func (s *fakeMessageSender) SendMessage(ctx context.Context, req a2a.SendMessageRequest) (a2a.SendMessageResult, error) {
	s.called = true
	s.last = req
	if s.err != nil {
		return a2a.SendMessageResult{}, s.err
	}
	return s.result, nil
}
