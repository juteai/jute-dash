package main

import (
	"bytes"
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

func TestKronkA2AServerPublishesA2A10Card(t *testing.T) {
	server := newTestA2AServer(t)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()

	server.handleAgentCard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var card struct {
		SupportedInterfaces []struct {
			URL             string `json:"url"`
			ProtocolBinding string `json:"protocolBinding"`
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"supportedInterfaces"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&card); err != nil {
		t.Fatalf("decode card: %v", err)
	}
	if len(card.SupportedInterfaces) != 1 {
		t.Fatalf("supportedInterfaces len = %d, want 1", len(card.SupportedInterfaces))
	}
	if got := card.SupportedInterfaces[0].ProtocolVersion; got != "1.0" {
		t.Fatalf("protocolVersion = %q, want 1.0", got)
	}
	if got := card.SupportedInterfaces[0].ProtocolBinding; got != "JSONRPC" {
		t.Fatalf("protocolBinding = %q, want JSONRPC", got)
	}
}

func TestKronkA2AServerSendMessage(t *testing.T) {
	server := newTestA2AServer(t)
	body := bytes.NewBufferString(`{"jsonrpc":"2.0","id":"1","method":"SendMessage","params":{"message":{"messageId":"msg-1","contextId":"ctx-1","role":"ROLE_USER","parts":[{"text":"hello"}]}}}`)
	req := httptest.NewRequest(http.MethodPost, "/invoke", body)
	req.Header.Set("A2A-Version", "1.0")
	rec := httptest.NewRecorder()

	server.handleInvoke(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp struct {
		Error  *rpcError `json:"error"`
		Result struct {
			Message a2aMessage `json:"message"`
		} `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}
	if got := textFromParts(resp.Result.Message.Parts); got != "fake Kronk reply" {
		t.Fatalf("message text = %q, want fake Kronk reply", got)
	}
}

func newTestA2AServer(t *testing.T) *kronkA2AServer {
	t.Helper()
	a, err := agent.New(agent.Config{
		Name:        "fake_kronk",
		Description: "Fake Kronk agent",
		Run: func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				yield(&session.Event{
					LLMResponse: model.LLMResponse{
						Content: genai.NewContentFromText("fake Kronk reply", genai.RoleModel),
					},
				}, nil)
			}
		},
	})
	if err != nil {
		t.Fatalf("create fake agent: %v", err)
	}
	server, err := newKronkA2AServer(a, "http://127.0.0.1:9797")
	if err != nil {
		t.Fatalf("new A2A server: %v", err)
	}
	return server
}
