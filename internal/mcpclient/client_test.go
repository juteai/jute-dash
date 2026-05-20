package mcpclient

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSendsJSONRPCAndBearerAuth(t *testing.T) {
	var got struct {
		Method string `json:"method"`
		Params struct {
			URI string `json:"uri"`
		} `json:"params"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret" {
			t.Fatalf("unexpected auth header %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(t, w, `{"contents":[{"uri":"jute://skills","mimeType":"application/json","text":"{\"skills\":[]}"}]}`)
	}))
	defer server.Close()

	client, err := New(Config{URL: server.URL, BearerToken: "secret"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	text, err := client.ReadResourceText(t.Context(), "jute://skills")
	if err != nil {
		t.Fatalf("ReadResourceText() error = %v", err)
	}
	if got.Method != "resources/read" || got.Params.URI != "jute://skills" {
		t.Fatalf("unexpected request: %+v", got)
	}
	if !strings.Contains(text, "skills") {
		t.Fatalf("unexpected resource text %q", text)
	}
}

func TestClientCallsToolAndExtractsStructuredContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"content":[{"type":"text","text":"ok"}],
			"structuredContent":{"status":"completed"},
			"isError":false
		}`)
	}))
	defer server.Close()

	client, err := New(Config{URL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := client.CallTool(t.Context(), "jute_skill_invoke_action", map[string]any{"skillId": WeatherSkillID})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError || !strings.Contains(string(result.StructuredContent), "completed") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClientMapsErrorsSafely(t *testing.T) {
	t.Run("rpc error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeRPCError(t, w, -32000, "secret stack")
		}))
		defer server.Close()

		client, err := New(Config{URL: server.URL})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		_, err = client.ListTools(t.Context())
		if !errors.Is(err, ErrRPCFailure) {
			t.Fatalf("expected ErrRPCFailure, got %v", err)
		}
		if strings.Contains(err.Error(), "secret stack") {
			t.Fatalf("leaked RPC message: %v", err)
		}
	})

	t.Run("http status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "secret", http.StatusUnauthorized)
		}))
		defer server.Close()

		client, err := New(Config{URL: server.URL})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		_, err = client.ListTools(t.Context())
		if !errors.Is(err, ErrTransport) {
			t.Fatalf("expected ErrTransport, got %v", err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatalf("leaked HTTP body: %v", err)
		}
	})
}

func TestCollectJuteContextReadsSkillsAndInvokesWeatherRefresh(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		methods = append(methods, req.Method)
		switch req.Method {
		case "initialize":
			writeRPCResult(t, w, `{"protocolVersion":"2025-11-25"}`)
		case "resources/read":
			if req.Params.URI == "jute://dashboard/current" {
				writeRPCResult(t, w, `{"contents":[{"uri":"jute://dashboard/current","mimeType":"application/json","text":"{\"skills\":[]}"}]}`)
				return
			}
			writeRPCResult(t, w, `{"contents":[{"uri":"jute://skills","mimeType":"application/json","text":"{\"skills\":[{\"skillId\":\"jute.weather.current\",\"displayName\":\"Weather\",\"actions\":[\"refresh\"],\"context\":{\"locationName\":\"London\",\"condition\":\"Clear sky\",\"temperature\":18.5,\"temperatureUnit\":\"°C\",\"status\":\"available\"}}]}"}]}`)
		case "tools/call":
			writeRPCResult(t, w, `{"content":[{"type":"text","text":"ok"}],"structuredContent":{"status":"completed"},"isError":false}`)
		case "prompts/get":
			writeRPCResult(t, w, `{"messages":[{"role":"user","content":{"type":"text","text":"Use Widget Skills safely."}}]}`)
		default:
			t.Fatalf("unexpected method %q", req.Method)
		}
	}))
	defer server.Close()

	client, err := New(Config{URL: server.URL})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	summary := client.CollectJuteContext(t.Context())
	if !summary.Available || !summary.DashboardRead || summary.SkillCount != 1 || summary.Weather.LocationName != "London" || summary.WeatherRefresh != "completed" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if strings.Join(methods, ",") != "initialize,resources/read,resources/read,tools/call,prompts/get" {
		t.Fatalf("unexpected methods: %+v", methods)
	}
}

func writeRPCResult(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":` + result + `}`))
}

func writeRPCError(t *testing.T, w http.ResponseWriter, code int, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	response, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"error":   map[string]any{"code": code, "message": message},
	})
	if err != nil {
		t.Fatalf("marshal error response: %v", err)
	}
	_, _ = w.Write(response)
}
