package a2a

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
				{
					URL:             "http://127.0.0.1:9797/invoke",
					ProtocolBinding: ProtocolJSONRPC,
					ProtocolVersion: ProtocolVersion10,
				},
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

	cardURL, err := AgentCardURLPolicy{URLs: []string{server.URL}}.Authorize(server.URL)
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	result, err := NewAgentCardFetcher().Fetch(t.Context(), cardURL, "")
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

	selected, err := SelectInterface(card)
	if err != nil {
		t.Fatalf("SelectInterface() error = %v", err)
	}
	if selected.EndpointURL != "http://127.0.0.1:9797/invoke" || selected.ProtocolBinding != ProtocolJSONRPC ||
		selected.ProtocolVersion != ProtocolVersion10 {
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

	if _, err := SelectInterface(card); err == nil {
		t.Fatal("expected unsupported interface error")
	}
}

func TestSelectInterfaceRejectsLegacyCardWithoutSupportedInterfaces(t *testing.T) {
	card := AgentCard{
		Name:               "Legacy Kronk Agent",
		URL:                "http://127.0.0.1:9797/invoke",
		PreferredTransport: ProtocolJSONRPC,
		Capabilities:       AgentCapabilities{Streaming: true},
	}

	if _, err := SelectInterface(card); err == nil {
		t.Fatal("expected unsupported interface error")
	}
}

func TestAgentCardURLPolicyAllowsLoopbackByDefault(t *testing.T) {
	got, err := DefaultAgentCardURLPolicy().Authorize(" http://127.0.0.1:9797/.well-known/agent-card.json ")
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if got.String() != "http://127.0.0.1:9797/.well-known/agent-card.json" {
		t.Fatalf("unexpected authorized URL: %s", got.String())
	}
}

func TestAgentCardURLPolicyAllowsConfiguredExactURL(t *testing.T) {
	policy := AgentCardURLPolicy{
		URLs: []string{"https://kitchen.agents.example.com/.well-known/agent-card.json"},
	}
	got, err := policy.Authorize("https://kitchen.agents.example.com/.well-known/agent-card.json")
	if err != nil {
		t.Fatalf("Authorize() error = %v", err)
	}
	if got.String() != "https://kitchen.agents.example.com/.well-known/agent-card.json" {
		t.Fatalf("unexpected authorized URL: %s", got.String())
	}
}

func TestAgentCardURLPolicyRejectsConfiguredWildcardHost(t *testing.T) {
	problems := ValidateAgentCardURLPolicy(AgentCardURLPolicy{
		URLs: []string{"https://*.agents.example.com/.well-known/agent-card.json"},
	})
	if len(problems) != 1 || !strings.Contains(problems[0], "wildcards are not supported") {
		t.Fatalf("unexpected validation problems: %+v", problems)
	}
}

func TestAgentCardURLPolicyRejectsUnconfiguredRemoteHost(t *testing.T) {
	_, err := DefaultAgentCardURLPolicy().Authorize("https://agent.example.com/.well-known/agent-card.json")
	if !errors.Is(err, ErrAgentCardURLNotAllowed) {
		t.Fatalf("expected ErrAgentCardURLNotAllowed, got %v", err)
	}
}

func TestAgentCardURLPolicyRejectsURLConfusionParts(t *testing.T) {
	for _, raw := range []string{
		"https://agent.example.com@127.0.0.1/.well-known/agent-card.json",
		"https://agent.example.com/.well-known/agent-card.json?next=http://127.0.0.1",
		"https://agent.example.com/.well-known/agent-card.json#http://127.0.0.1",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := DefaultAgentCardURLPolicy().Authorize(raw); err == nil {
				t.Fatal("expected URL to be rejected")
			}
		})
	}
}
