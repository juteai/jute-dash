package mcpbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/displayactions"
	"jute-dash/internal/store"
	"jute-dash/internal/widgetskills"
	_ "jute-dash/widgets/chathistory"
	_ "jute-dash/widgets/datetime"
	weather "jute-dash/widgets/weather"
)

const weatherSkillID = "jute.weather.current"

func TestInitializeReturnsCapabilities(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})

	rec := postRPC(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body rpcEnvelope
	decodeJSON(t, rec.Body.Bytes(), &body)
	result := body.Result
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("unexpected initialize result: %+v", result)
	}
	capabilities := result["capabilities"].(map[string]any)
	if capabilities["resources"] == nil || capabilities["tools"] == nil || capabilities["prompts"] == nil {
		t.Fatalf("missing capabilities: %+v", capabilities)
	}
}

func TestResourceAndToolMethodsExposeWidgetSkills(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})

	resources := postRPC(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "resources/list"})
	if resources.Code != http.StatusOK {
		t.Fatalf("resources/list status = %d", resources.Code)
	}
	if !bytes.Contains(resources.Body.Bytes(), []byte("jute://skills")) {
		t.Fatalf("resources/list did not include skills: %s", resources.Body.String())
	}
	if !bytes.Contains(resources.Body.Bytes(), []byte("jute://widgets/visible")) || !bytes.Contains(resources.Body.Bytes(), []byte("jute://widgets/weather/context")) {
		t.Fatalf("resources/list did not include widget resources: %s", resources.Body.String())
	}

	read := postRPC(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "resources/read",
		"params":  map[string]any{"uri": "jute://skills"},
	})
	if read.Code != http.StatusOK {
		t.Fatalf("resources/read status = %d: %s", read.Code, read.Body.String())
	}
	if !bytes.Contains(read.Body.Bytes(), []byte(weatherSkillID)) {
		t.Fatalf("resources/read did not include weather skill: %s", read.Body.String())
	}

	widgetContext := postRPC(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "resources/read",
		"params":  map[string]any{"uri": "jute://widgets/weather/context"},
	})
	if widgetContext.Code != http.StatusOK || !bytes.Contains(widgetContext.Body.Bytes(), []byte("London")) {
		t.Fatalf("widget context read failed: %d %s", widgetContext.Code, widgetContext.Body.String())
	}

	tools := postRPC(t, handler, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/list"})
	if tools.Code != http.StatusOK {
		t.Fatalf("tools/list status = %d", tools.Code)
	}
	if !bytes.Contains(tools.Body.Bytes(), []byte("jute_skill_read_context")) {
		t.Fatalf("tools/list did not include skill read tool: %s", tools.Body.String())
	}
}

func TestToolCallReadsSkillContextAndInvokesAction(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})

	read := rpcRequestWithHeaders(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "jute_skill_read_context",
			"arguments": map[string]any{"skillId": weatherSkillID},
		},
	}, agentHeader("house"))
	if read.Code != http.StatusOK || !bytes.Contains(read.Body.Bytes(), []byte("London")) {
		t.Fatalf("skill read failed: %d %s", read.Code, read.Body.String())
	}

	action := rpcRequestWithHeaders(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "jute_skill_invoke_action",
			"arguments": map[string]any{
				"skillId":  weatherSkillID,
				"actionId": "refresh",
			},
		},
	}, agentHeader("house"))
	if action.Code != http.StatusOK || !bytes.Contains(action.Body.Bytes(), []byte("completed")) {
		t.Fatalf("skill action failed: %d %s", action.Code, action.Body.String())
	}
}

func TestDisplayActionToolsQueueEvents(t *testing.T) {
	actions := &fakeDisplayActions{}
	handler := testHandlerWithActions(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}}, actions)

	tools := rpcRequestWithHeaders(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, agentHeader("house"))
	if tools.Code != http.StatusOK || !bytes.Contains(tools.Body.Bytes(), []byte("jute_display_notification")) || !bytes.Contains(tools.Body.Bytes(), []byte("jute_display_focus_widget")) {
		t.Fatalf("tools/list did not include display action tools: %d %s", tools.Code, tools.Body.String())
	}

	notification := rpcRequestWithHeaders(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "jute_display_notification",
			"arguments": map[string]any{"message": "Hello display", "severity": "success"},
		},
	}, agentHeader("house"))
	if notification.Code != http.StatusOK || !bytes.Contains(notification.Body.Bytes(), []byte("display.notification")) {
		t.Fatalf("notification tool failed: %d %s", notification.Code, notification.Body.String())
	}
	if actions.notificationMessage != "Hello display" || actions.notificationSeverity != "success" {
		t.Fatalf("notification action not called: %+v", actions)
	}

	focus := rpcRequestWithHeaders(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "jute_display_focus_widget",
			"arguments": map[string]any{"widgetInstanceId": "weather", "reason": "weather question"},
		},
	}, agentHeader("house"))
	if focus.Code != http.StatusOK || !bytes.Contains(focus.Body.Bytes(), []byte("display.focus_widget")) {
		t.Fatalf("focus widget tool failed: %d %s", focus.Code, focus.Body.String())
	}
	if actions.focusWidgetID != "weather" || actions.focusReason != "weather question" {
		t.Fatalf("focus action not called: %+v", actions)
	}
}

func TestDisplayFocusRejectsHiddenWidget(t *testing.T) {
	actions := &fakeDisplayActions{}
	handler := testHandlerWithActions(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}}, actions)

	focus := rpcRequestWithHeaders(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "jute_display_focus_widget",
			"arguments": map[string]any{"widgetInstanceId": "not-visible"},
		},
	}, agentHeader("house"))
	if focus.Code != http.StatusOK || !bytes.Contains(focus.Body.Bytes(), []byte("skill or action not found")) {
		t.Fatalf("expected hidden widget rejection, got %d %s", focus.Code, focus.Body.String())
	}
	if actions.focusWidgetID != "" {
		t.Fatalf("focus action should not be called for hidden widget: %+v", actions)
	}
}

func TestAnonymousCallerIsReadOnly(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})

	resources := postRPC(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "resources/list"})
	if resources.Code != http.StatusOK || !bytes.Contains(resources.Body.Bytes(), []byte("jute://skills")) {
		t.Fatalf("anonymous caller should see read resources: %d %s", resources.Code, resources.Body.String())
	}

	action := postRPC(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "jute_skill_invoke_action",
			"arguments": map[string]any{"skillId": weatherSkillID, "actionId": "refresh"},
		},
	})
	if action.Code != http.StatusOK || !bytes.Contains(action.Body.Bytes(), []byte("missing MCP scope: skills:action_invoke")) {
		t.Fatalf("anonymous caller should not invoke actions: %d %s", action.Code, action.Body.String())
	}

	prompt := postRPC(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "prompts/get",
		"params":  map[string]any{"name": "jute_home_assistant_guidance"},
	})
	if prompt.Code != http.StatusOK || !bytes.Contains(prompt.Body.Bytes(), []byte("missing MCP scope: skills:prompt_read")) {
		t.Fatalf("anonymous caller should not read prompts: %d %s", prompt.Code, prompt.Body.String())
	}
}

func TestUnknownAgentCallerRejected(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})

	rec := rpcRequestWithHeaders(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "resources/list"}, agentHeader("missing"))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("mcp caller is not authorized")) {
		t.Fatalf("expected unknown caller rejection, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestTokenAuthRequiresCallerIdentityForScopedCalls(t *testing.T) {
	t.Setenv("TEST_JUTE_MCP_TOKEN", "secret")
	handler := testHandler(config.MCPConfig{
		Auth: config.MCPAuthConfig{Mode: "local-token", EnvToken: "TEST_JUTE_MCP_TOKEN"},
	})

	rec := rpcRequestWithHeaders(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "resources/list"}, map[string]string{
		"Authorization": "Bearer secret",
	})
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("mcp caller identity is required")) {
		t.Fatalf("expected missing caller identity rejection, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPrompts(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})

	list := rpcRequestWithHeaders(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "prompts/list"}, agentHeader("house"))
	if list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte("jute_home_assistant_guidance")) {
		t.Fatalf("prompts/list failed: %d %s", list.Code, list.Body.String())
	}
	get := rpcRequestWithHeaders(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "prompts/get",
		"params":  map[string]any{"name": "jute_home_assistant_guidance"},
	}, agentHeader("house"))
	if get.Code != http.StatusOK || !bytes.Contains(get.Body.Bytes(), []byte("Widget Skills")) {
		t.Fatalf("prompts/get failed: %d %s", get.Code, get.Body.String())
	}
}

func TestAuthAndOrigin(t *testing.T) {
	t.Setenv("TEST_JUTE_MCP_TOKEN", "secret")
	cfg := config.MCPConfig{
		Auth: config.MCPAuthConfig{Mode: "local-token", EnvToken: "TEST_JUTE_MCP_TOKEN"},
	}
	handler := testHandler(cfg)

	unauthorized := postRPC(t, handler, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", unauthorized.Code)
	}

	authorized := rpcRequestWithHeaders(t, handler, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"}, map[string]string{
		"Authorization":   "Bearer secret",
		"Origin":          "http://localhost:5173",
		"X-Jute-Agent-ID": "house",
	})
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected authorized status 200, got %d: %s", authorized.Code, authorized.Body.String())
	}

	rejectedOrigin := rpcRequestWithHeaders(t, handler, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/list"}, map[string]string{
		"Authorization": "Bearer secret",
		"Origin":        "http://evil.example",
	})
	if rejectedOrigin.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden origin, got %d", rejectedOrigin.Code)
	}
}

func TestNotificationAccepted(t *testing.T) {
	handler := testHandler(config.MCPConfig{Auth: config.MCPAuthConfig{Mode: "none"}})
	rec := postRPC(t, handler, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func testHandler(cfg config.MCPConfig) http.Handler {
	return testHandlerWithActions(cfg, nil)
}

func testHandlerWithActions(cfg config.MCPConfig, actions DisplayActions) http.Handler {
	if cfg.Auth.Mode == "" {
		cfg.Auth.Mode = "none"
	}
	if actions == nil {
		return NewHandler(cfg, "test", staticProvider{snapshot: testSnapshot()})
	}
	return NewHandler(cfg, "test", staticProvider{snapshot: testSnapshot()}, actions)
}

func postRPC(t *testing.T, handler http.Handler, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return rpcRequestWithHeaders(t, handler, payload, nil)
}

func rpcRequestWithHeaders(t *testing.T, handler http.Handler, payload map[string]any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func agentHeader(agentID string) map[string]string {
	return map[string]string{callerAgentHeader: agentID}
}

func decodeJSON(t *testing.T, body []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, string(body))
	}
}

type rpcEnvelope struct {
	Result map[string]any `json:"result"`
}

type staticProvider struct {
	snapshot widgetskills.Snapshot
}

func (p staticProvider) Snapshot(context.Context) (widgetskills.Snapshot, error) {
	return p.snapshot, nil
}

type fakeDisplayActions struct {
	notificationMessage  string
	notificationSeverity string
	focusWidgetID        string
	focusReason          string
}

func (f *fakeDisplayActions) Notify(message, severity string) (displayactions.Notification, error) {
	f.notificationMessage = message
	f.notificationSeverity = severity
	return displayactions.Notification{
		ID:        "notification-test",
		Message:   message,
		Severity:  severity,
		CreatedAt: time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		ExpiresAt: time.Date(2026, 5, 20, 10, 0, 6, 0, time.UTC).Format(time.RFC3339Nano),
	}, nil
}

func (f *fakeDisplayActions) FocusWidget(widgetInstanceID, reason string) (displayactions.FocusWidget, error) {
	f.focusWidgetID = widgetInstanceID
	f.focusReason = reason
	return displayactions.FocusWidget{
		ID:               "focus-test",
		WidgetInstanceID: widgetInstanceID,
		Reason:           reason,
		CreatedAt:        time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}, nil
}

func testSnapshot() widgetskills.Snapshot {
	cfg := config.Default()
	cfg.Home.Timezone = "Europe/London"
	cfg.Home.Locale = "en-GB"
	cfg.Voice.PreferredAgentID = "house"
	cfg.Agents = []config.AgentConfig{{
		ID:              "house",
		Name:            "House",
		CardURL:         "http://127.0.0.1:9797/.well-known/agent-card.json",
		EndpointURL:     "http://127.0.0.1:9797/invoke",
		ProtocolBinding: a2a.ProtocolJSONRPC,
		Enabled:         true,
		MCPScopes: []string{
			config.MCPScopeDashboardRead,
			config.MCPScopeWidgetsRead,
			config.MCPScopeSkillsRead,
			config.MCPScopeSkillsContextRead,
			config.MCPScopeSkillsPromptRead,
			config.MCPScopeSkillsActionInvoke,
			config.MCPScopeDisplayWrite,
			config.MCPScopeDisplayFocusWidget,
		},
	}}
	layout := store.DefaultWidgetLayout()
	temp := 18.5
	for i := range layout.Widgets {
		if layout.Widgets[i].Kind == "weather" {
			layout.Widgets[i].Data = weather.State{
				LocationName:    "London",
				Temperature:     &temp,
				TemperatureUnit: "°C",
				Condition:       "Clear sky",
				Source:          weather.ProviderOpenMeteo,
				Status:          weather.StatusAvailable,
			}
		}
	}
	return widgetskills.Snapshot{
		Config: cfg,
		Layout: layout,
		Agents: []widgetskills.Agent{
			{ID: "house", Name: "House", ProtocolBinding: a2a.ProtocolJSONRPC, Enabled: true, Capabilities: []string{"conversation"}},
		},
		GeneratedAt: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
	}
}
