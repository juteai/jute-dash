package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentCardFetcherFetchesA2A10Card(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("unexpected Accept header %q", accept)
		}
		_ = json.NewEncoder(w).Encode(AgentCard{
			Name:        "Dev Agent",
			Description: "Local dev agent",
			Version:     "1.0.0",
			SupportedInterfaces: []AgentInterface{
				{URL: "http://127.0.0.1:9797/invoke", ProtocolBinding: ProtocolJSONRPC, ProtocolVersion: ProtocolVersion10},
			},
			Capabilities: AgentCapabilities{
				Streaming: false,
				Extensions: []AgentExtension{
					{URI: DashboardContextExtensionURI},
				},
			},
			Skills: []AgentSkill{{ID: "chat", Name: "Chat", Description: "Talk", Tags: []string{"chat"}}},
		})
	}))
	defer server.Close()

	result, err := NewAgentCardFetcher().Fetch(t.Context(), server.URL, "")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.Card.Name != "Dev Agent" || !SupportsDashboardContext(result.Card) || result.Raw == "" {
		t.Fatalf("unexpected fetch result: %+v", result)
	}
}

func TestSelectInterfacePrefersA2A10JSONRPC(t *testing.T) {
	card := AgentCard{
		Name: "Dev Agent",
		SupportedInterfaces: []AgentInterface{
			{URL: "http://127.0.0.1:9797/rest", ProtocolBinding: ProtocolHTTPJSON, ProtocolVersion: ProtocolVersion10},
			{URL: "http://127.0.0.1:9797/invoke", ProtocolBinding: ProtocolJSONRPC, ProtocolVersion: ProtocolVersion10},
			{URL: "http://127.0.0.1:9797/legacy", ProtocolBinding: ProtocolJSONRPC, ProtocolVersion: "0.3"},
		},
	}

	selected, err := SelectInterface(card, "", "")
	if err != nil {
		t.Fatalf("SelectInterface() error = %v", err)
	}
	if selected.EndpointURL != "http://127.0.0.1:9797/invoke" || selected.ProtocolBinding != ProtocolJSONRPC || selected.ProtocolVersion != ProtocolVersion10 {
		t.Fatalf("unexpected selected interface: %+v", selected)
	}
}

func TestSelectInterfaceRejectsUnsupportedVersions(t *testing.T) {
	card := AgentCard{
		Name: "Legacy Agent",
		SupportedInterfaces: []AgentInterface{
			{URL: "http://127.0.0.1:9797/invoke", ProtocolBinding: ProtocolJSONRPC, ProtocolVersion: "0.3"},
		},
	}

	if _, err := SelectInterface(card, "", ""); err == nil {
		t.Fatal("expected unsupported interface error")
	}
}
