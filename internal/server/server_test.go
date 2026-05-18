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
