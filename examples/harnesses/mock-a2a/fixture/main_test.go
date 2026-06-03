package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessageReportsMCPNotConfigured(t *testing.T) {
	t.Setenv("JUTE_MCP_URL", "")

	rec := sendTestMessage(t, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MCP not configured") {
		t.Fatalf("expected MCP not configured response, got %s", rec.Body.String())
	}
}

func TestSendMessageReadsMCPContext(t *testing.T) {
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode MCP request: %v", err)
		}
		switch req.Method {
		case "initialize":
			writeMCPResult(t, w, `{"protocolVersion":"2025-11-25"}`)
		case "resources/read":
			writeMCPResult(
				t,
				w,
				`{"contents":[{"uri":"jute://skills","mimeType":"application/json","text":"{\"skills\":[{\"skillId\":\"jute.weather.current\",\"displayName\":\"Weather\",\"actions\":[\"refresh\"],\"context\":{\"locationName\":\"London\",\"condition\":\"Clear sky\",\"temperature\":18.5,\"temperatureUnit\":\"°C\",\"status\":\"available\"}}]}"}]}`,
			)
		case "tools/call":
			writeMCPResult(
				t,
				w,
				`{"content":[{"type":"text","text":"ok"}],"structuredContent":{"status":"completed"},"isError":false}`,
			)
		case "prompts/get":
			writeMCPResult(
				t,
				w,
				`{"messages":[{"role":"user","content":{"type":"text","text":"Use Widget Skills safely."}}]}`,
			)
		default:
			t.Fatalf("unexpected MCP method %q", req.Method)
		}
	}))
	defer mcp.Close()
	t.Setenv("JUTE_MCP_URL", mcp.URL)

	rec := sendTestMessage(t, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MCP saw 1 widget skills") || !strings.Contains(body, "weather: London Clear sky") {
		t.Fatalf("expected MCP context response, got %s", body)
	}
}

func TestSendMessageHandlesMCPFailureGracefully(t *testing.T) {
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusServiceUnavailable)
	}))
	defer mcp.Close()
	t.Setenv("JUTE_MCP_URL", mcp.URL)

	rec := sendTestMessage(t, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "MCP initialize failed") {
		t.Fatalf("expected graceful MCP failure, got %s", rec.Body.String())
	}
}

func sendTestMessage(t *testing.T, metadata map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	if metadata == nil {
		metadata = map[string]any{}
	}
	body := `{"jsonrpc":"2.0","id":"1","method":"SendMessage","params":{"message":{"role":"ROLE_USER","parts":[{"text":"Hello"}],"metadata":` + marshalTestJSON(
		t,
		metadata,
	) + `}}}`
	req := httptest.NewRequest(http.MethodPost, "/invoke", strings.NewReader(body))
	req.Header.Set("A2A-Version", "1.0")
	rec := httptest.NewRecorder()
	handleInvoke(rec, req)
	return rec
}

func marshalTestJSON(t *testing.T, value any) string {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return string(bytes)
}

func writeMCPResult(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":` + result + `}`))
}
