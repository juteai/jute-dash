package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONRPCClientSendsMessageSendRequest(t *testing.T) {
	var got struct {
		Method string `json:"method"`
		Params struct {
			Message struct {
				Role  string `json:"role"`
				Parts []struct {
					Kind string `json:"kind"`
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"message"`
			Configuration struct {
				Blocking bool `json:"blocking"`
			} `json:"configuration"`
		} `json:"params"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer dev-token" {
			t.Fatalf("unexpected auth header %q", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRPCResult(t, w, `{"kind":"message","messageId":"msg-1","role":"agent","parts":[{"kind":"text","text":"Hello from Kronk"}]}`)
	}))
	defer server.Close()

	client := NewJSONRPCClient()
	result, err := client.SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
		BearerToken:     "dev-token",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if got.Method != "message/send" {
		t.Fatalf("method = %q, want message/send", got.Method)
	}
	if !got.Params.Configuration.Blocking {
		t.Fatal("expected blocking request")
	}
	if got.Params.Message.Role != "user" || len(got.Params.Message.Parts) != 1 || got.Params.Message.Parts[0].Text != "Hello" {
		t.Fatalf("unexpected message payload: %+v", got.Params.Message)
	}
	if result.Status != "completed" || result.Text != "Hello from Kronk" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientRetriesSendMessageOnMethodNotFound(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		methods = append(methods, req.Method)
		if req.Method == "message/send" {
			writeRPCError(t, w, -32601, "not found")
			return
		}
		writeRPCResult(t, w, `{"kind":"message","messageId":"msg-2","role":"agent","parts":[{"kind":"text","text":"Retried"}]}`)
	}))
	defer server.Close()

	result, err := NewJSONRPCClient().SendMessage(t.Context(), SendMessageRequest{
		EndpointURL:     server.URL,
		ProtocolBinding: ProtocolJSONRPC,
		Text:            "Hello",
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if strings.Join(methods, ",") != "message/send,SendMessage" {
		t.Fatalf("unexpected methods: %v", methods)
	}
	if result.Text != "Retried" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestJSONRPCClientExtractsTaskStatusText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeRPCResult(t, w, `{
			"kind":"task",
			"id":"task-1",
			"contextId":"ctx-1",
			"status":{
				"state":"completed",
				"message":{"kind":"message","messageId":"msg-3","role":"agent","parts":[{"kind":"text","text":"Task complete"}]}
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
	if result.ConversationID != "ctx-1" || result.Status != "completed" || result.Text != "Task complete" {
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
				{"kind":"message","messageId":"msg-4","role":"user","parts":[{"kind":"text","text":"Question"}]},
				{"kind":"message","messageId":"msg-5","role":"agent","parts":[{"kind":"text","text":"Latest answer"}]}
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

func writeRPCResult(t *testing.T, w http.ResponseWriter, result string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"test","result":` + result + `}`))
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
