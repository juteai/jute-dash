package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
