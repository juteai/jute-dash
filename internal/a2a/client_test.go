package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONRPCClientSendsA2A10SendMessageRequest(t *testing.T) {
	var got struct {
		Method string `json:"method"`
		Params struct {
			Message struct {
				ContextID string         `json:"contextId"`
				Role      string         `json:"role"`
				Metadata  map[string]any `json:"metadata"`
				Parts     []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"message"`
			Configuration struct {
				ReturnImmediately *bool `json:"returnImmediately"`
			} `json:"configuration"`
		} `json:"params"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer dev-token" {
			t.Fatalf("unexpected auth header %q", auth)
		}
		if version := r.Header.Get("A2A-Version"); version != "1.0" {
			t.Fatalf("unexpected A2A-Version %q", version)
		}
		if extensions := r.Header.Get("A2A-Extensions"); extensions != DashboardContextExtensionURI {
			t.Fatalf("unexpected A2A-Extensions %q", extensions)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(t, w, `{"message":{"messageId":"msg-1","contextId":"ctx-1","role":"ROLE_AGENT","parts":[{"text":"Hello from A2A"}]}}`)
	}))
	defer server.Close()

	client := NewJSONRPCClient()
	result, err := client.SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
		BearerToken:     "dev-token",
		ConversationID:  "ctx-existing",
		Extensions:      []string{DashboardContextExtensionURI},
		Metadata: map[string]any{
			DashboardContextExtensionURI: map[string]any{"dashboard": "safe"},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if got.Method != "SendMessage" {
		t.Fatalf("method = %q, want SendMessage", got.Method)
	}
	if got.Params.Configuration.ReturnImmediately == nil || *got.Params.Configuration.ReturnImmediately {
		t.Fatal("expected blocking request with returnImmediately=false")
	}
	if got.Params.Message.ContextID != "ctx-existing" {
		t.Fatalf("unexpected contextId %q", got.Params.Message.ContextID)
	}
	if got.Params.Message.Role != "ROLE_USER" || len(got.Params.Message.Parts) != 1 || got.Params.Message.Parts[0].Text != "Hello" {
		t.Fatalf("unexpected message payload: %+v", got.Params.Message)
	}
	if got.Params.Message.Metadata[DashboardContextExtensionURI] == nil {
		t.Fatalf("expected dashboard metadata, got %+v", got.Params.Message.Metadata)
	}
	if result.ConversationID != "ctx-1" || result.Status != "completed" || result.Text != "Hello from A2A" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientDoesNotRetryLegacyMethodNames(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		method = req.Method
		writeRPCError(t, w, -32601, "not found")
	}))
	defer server.Close()

	_, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err == nil {
		t.Fatal("expected method-not-found error")
	}
	if method != "SendMessage" {
		t.Fatalf("method = %q, want SendMessage", method)
	}
}

func TestJSONRPCClientExtractsTaskStatusText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"task":{
				"id":"task-1",
				"contextId":"ctx-1",
				"status":{
					"state":"completed",
					"message":{"messageId":"msg-3","role":"ROLE_AGENT","parts":[{"text":"Task complete"}]}
				}
			}
		}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Status",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.ConversationID != "ctx-1" || result.TaskID != "task-1" || result.Status != "completed" || result.Text != "Task complete" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientExtractsLatestTaskHistoryText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"kind":"task",
			"id":"task-2",
			"contextId":"ctx-2",
			"status":{"state":"completed"},
			"history":[
				{"kind":"message","messageId":"msg-4","role":"ROLE_USER","parts":[{"text":"Question"}]},
				{"kind":"message","messageId":"msg-5","role":"ROLE_AGENT","parts":[{"text":"Latest answer"}]}
			]
		}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "History",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if result.Text != "Latest answer" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientMapsRPCErrorSafely(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCError(t, w, -32000, "secret backend stack trace")
	}))
	defer server.Close()

	_, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret backend stack trace") {
		t.Fatalf("error leaked remote detail: %v", err)
	}
}

func TestJSONRPCClientStreamsA2AEvents(t *testing.T) {
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		method = req.Method
		w.Header().Set("Content-Type", "text/event-stream")
		writeRPCSSE(t, w, `{"task":{"id":"task-1","contextId":"ctx-1","status":{"state":"working"}}}`)
		writeRPCSSE(t, w, `{"artifactUpdate":{"taskId":"task-1","contextId":"ctx-1","artifact":{"parts":[{"text":"Hel"}]},"append":true}}`)
		writeRPCSSE(t, w, `{"artifactUpdate":{"taskId":"task-1","contextId":"ctx-1","artifact":{"parts":[{"text":"lo"}]},"append":true,"lastChunk":true}}`)
		writeRPCSSE(t, w, `{"statusUpdate":{"taskId":"task-1","contextId":"ctx-1","status":{"state":"completed"},"final":true}}`)
	}))
	defer server.Close()

	var events []StreamEvent
	err := NewJSONRPCClient().StreamMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamMessage() error = %v", err)
	}
	if method != "SendStreamingMessage" {
		t.Fatalf("method = %q, want SendStreamingMessage", method)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %+v", events)
	}
	if events[1].Text != "Hel" || !events[1].Append || events[2].Text != "lo" || !events[2].Terminal || !events[3].Terminal {
		t.Fatalf("unexpected stream events: %+v", events)
	}
}

func writeRPCResult(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"test","result":` + result + `}`))
}

func writeRPCSSE(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	_, _ = w.Write([]byte(`data: {"jsonrpc":"2.0","id":"test","result":` + result + "}\n\n"))
}

func writeRPCError(t *testing.T, w http.ResponseWriter, code int, message string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      "test",
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
