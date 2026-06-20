package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jute-dash/apps/hub/internal/app/model"
	"jute-dash/apps/hub/pkg/widgetskills"

	_ "jute-dash/widgets/spotify/hub"
)

const testTokenEnv = "JUTE_MCP_TEST_TOKEN"

func TestHandlerRejectsCrossOriginRequestsBeforeAuth(t *testing.T) {
	cfg := testMCPConfig(t)
	handler := NewHandler(cfg, "test", nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
	))
	req.Header.Set("Origin", "https://dashboard.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	response := decodeRPCResponse(t, rec)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", rec.Code)
	}
	if response.Error == nil ||
		response.Error.Code != -32000 ||
		response.Error.Message != "origin is not allowed" {
		t.Fatalf("unexpected RPC error: %+v", response.Error)
	}
}

func TestHandlerRequiresConfiguredBearerToken(t *testing.T) {
	cfg := testMCPConfig(t)
	handler := NewHandler(cfg, "test", nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
	))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	response := decodeRPCResponse(t, rec)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	if response.Error == nil ||
		response.Error.Code != -32001 ||
		response.Error.Message != "unauthorized" {
		t.Fatalf("unexpected RPC error: %+v", response.Error)
	}
}

func TestHandlerAdvertisesPOSTForUnsupportedMethods(t *testing.T) {
	cfg := testMCPConfig(t)
	handler := NewHandler(cfg, "test", nil)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
	if rec.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("expected Allow POST, got %q", rec.Header().Get("Allow"))
	}
}

func TestHandlerInitializesAuthenticatedClient(t *testing.T) {
	cfg := testMCPConfig(t)
	handler := NewHandler(cfg, "test-version", nil)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
	))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Origin", "http://127.0.0.1:4173")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	response := decodeRPCResponse(t, rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if response.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", response.Error)
	}
	result, ok := response.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected initialize result: %#v", response.Result)
	}
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("expected protocol version %q, got %#v", ProtocolVersion, result["protocolVersion"])
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "jute-dash" || serverInfo["version"] != "test-version" {
		t.Fatalf("unexpected server info: %#v", result["serverInfo"])
	}
}

func TestHandlerInvokesSpotifyActionThroughDispatcher(t *testing.T) {
	cfg := testMCPConfig(t)
	dispatcher := &recordingActionDispatcher{
		result: map[string]any{"status": "ok"},
	}
	handler := NewHandlerWithActions(
		cfg,
		"test-version",
		staticSnapshotProvider{snapshot: spotifyActionSnapshot()},
		nil,
		dispatcher,
	)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"jute_skill_invoke_action","arguments":{"skillId":"jute.spotify.control","widgetInstanceId":"spotify","actionId":"play_track","arguments":{"query":"True Colours The Weeknd"}}}}`,
	))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set(callerAgentHeader, "kronk-agent")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	response := decodeRPCResponse(t, rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if response.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", response.Error)
	}
	if dispatcher.widgetInstanceID != "spotify" || dispatcher.actionID != "play_track" || dispatcher.actor != "mcp" {
		t.Fatalf("unexpected dispatched action: %+v", dispatcher)
	}
	if dispatcher.arguments["query"] != "True Colours The Weeknd" {
		t.Fatalf("expected action arguments to pass through, got %#v", dispatcher.arguments)
	}
	if _, exists := dispatcher.arguments["skillId"]; exists {
		t.Fatalf("expected routing arguments to be stripped, got %#v", dispatcher.arguments)
	}
}

func TestHandlerRejectsInventedAggregateMusicSkill(t *testing.T) {
	cfg := testMCPConfig(t)
	dispatcher := &recordingActionDispatcher{
		result: map[string]any{"status": "ok"},
	}
	handler := NewHandlerWithActions(
		cfg,
		"test-version",
		staticSnapshotProvider{snapshot: spotifyActionSnapshot()},
		nil,
		dispatcher,
	)
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"jute_skill_invoke_action","arguments":{"skillId":"music_player","widgetInstanceId":"default_music_widget","actionId":"next"}}}`,
	))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set(callerAgentHeader, "kronk-agent")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	response := decodeRPCResponse(t, rec)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if response.Error == nil || response.Error.Message != "skill or action not found" {
		t.Fatalf("expected skill/action not found error, got %+v", response.Error)
	}
	if dispatcher.widgetInstanceID != "" || dispatcher.actionID != "" {
		t.Fatalf("invented aggregate skill should not dispatch, got %+v", dispatcher)
	}
}

func testMCPConfig(t *testing.T) Config {
	t.Helper()
	t.Setenv(testTokenEnv, "test-token")
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Auth.EnvToken = testTokenEnv
	return cfg
}

func decodeRPCResponse(t *testing.T, rec *httptest.ResponseRecorder) rpcResponse {
	t.Helper()
	var response rpcResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode RPC response: %v", err)
	}
	return response
}

type staticSnapshotProvider struct {
	snapshot widgetskills.Snapshot
}

func (p staticSnapshotProvider) Snapshot(context.Context) (widgetskills.Snapshot, error) {
	return p.snapshot, nil
}

type recordingActionDispatcher struct {
	widgetInstanceID string
	actionID         string
	arguments        map[string]any
	actor            string
	confirmed        bool
	result           map[string]any
}

func (d *recordingActionDispatcher) InvokeWidgetAction(
	_ context.Context,
	widgetInstanceID string,
	actionID string,
	arguments map[string]any,
	actor string,
	confirmed bool,
) (map[string]any, error) {
	d.widgetInstanceID = widgetInstanceID
	d.actionID = actionID
	d.arguments = arguments
	d.actor = actor
	d.confirmed = confirmed
	return d.result, nil
}

func spotifyActionSnapshot() widgetskills.Snapshot {
	return widgetskills.Snapshot{
		Layout: widgetskills.WidgetLayout{
			Widgets: []widgetskills.WidgetInstance{
				{
					ID:      "spotify",
					Kind:    "spotify",
					Title:   "Spotify",
					Visible: true,
					Size:    "wide",
					Data: map[string]any{
						"track_title": "Test Track",
						"artist_name": "Test Artist",
						"is_playing":  true,
						"volume":      70,
					},
				},
			},
		},
		Agents: []widgetskills.Agent{
			{
				ID:      "kronk-agent",
				Enabled: true,
				MCPScopes: []string{
					model.MCPScopeSkillsRead,
					model.MCPScopeSkillsContextRead,
					model.MCPScopeSkillsActionInvoke,
				},
			},
		},
	}
}
