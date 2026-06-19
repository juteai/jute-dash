package agents

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"

	"github.com/stretchr/testify/mock"
)

func newAgentSyncer(t *testing.T, configs *[]AgentConfig) *AgentSyncer {
	t.Helper()
	syncer := NewAgentSyncer(t)
	syncer.EXPECT().
		AgentsConfig(mock.Anything).
		RunAndReturn(func(context.Context) ([]AgentConfig, error) {
			return append([]AgentConfig(nil), (*configs)...), nil
		}).
		Maybe()
	syncer.EXPECT().
		SyncAgents(mock.Anything, mock.Anything).
		Run(func(_ context.Context, next []AgentConfig) {
			*configs = append([]AgentConfig(nil), next...)
		}).
		Return(nil).
		Maybe()
	return syncer
}

func TestAgentManager_New_InitializesRegistry(t *testing.T) {
	configs := []AgentConfig{
		{
			ID:              "agent-1",
			Name:            "Agent One",
			Description:     "First agent",
			CardURL:         "http://example.com/card.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
		},
	}

	syncer := newAgentSyncer(t, &configs)
	cards := NewCardService()

	mgr := NewAgentManager(syncer, cards, "config.yaml")
	reg := mgr.ActiveRegistry()

	agent, ok := reg.Find("agent-1")
	if !ok {
		t.Fatal("expected agent-1 to be found in registry")
	}
	if agent.Name != "Agent One" {
		t.Errorf("expected agent Name to be 'Agent One', got %q", agent.Name)
	}
}

func TestAgentManager_List_EnrichesAndTriggersDiscovery(t *testing.T) {
	configs := []AgentConfig{
		{
			ID:              "agent-1",
			Name:            "Agent One",
			CardURL:         "http://example.com/card.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
		},
	}

	var fetched bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetched = true
		card := a2aclient.AgentCard{
			Name:        "Agent One",
			Description: "First agent",
			SupportedInterfaces: []a2aclient.AgentInterface{
				{
					URL:             "http://example.com/api",
					ProtocolBinding: "JSONRPC",
					ProtocolVersion: "1.0",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	configs[0].CardURL = server.URL + "/card.json"

	syncer := newAgentSyncer(t, &configs)
	cards := NewCardService(a2aclient.AgentCardURLPolicy{URLs: []string{configs[0].CardURL}})

	mgr := NewAgentManager(syncer, cards, "config.yaml")

	// Verify list with triggerDiscovery=true fetching card from server
	agentsList := mgr.List(t.Context(), true)
	if len(agentsList) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agentsList))
	}
	if !fetched {
		t.Error("expected card fetch to be triggered via discovery")
	}

	// Verify enriched attributes
	if agentsList[0].CardStatus != "available" {
		t.Errorf("expected CardStatus to be 'available', got %q", agentsList[0].CardStatus)
	}
	if agentsList[0].SelectedEndpointURL != "http://example.com/api" {
		t.Errorf("expected SelectedEndpointURL to be http://example.com/api, got %q", agentsList[0].SelectedEndpointURL)
	}
}

func TestAgentManager_Find(t *testing.T) {
	configs := []AgentConfig{
		{
			ID:              "agent-1",
			Name:            "Agent One",
			CardURL:         "http://example.com/card.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
		},
	}

	syncer := newAgentSyncer(t, &configs)
	cards := NewCardService(a2aclient.AgentCardURLPolicy{URLs: []string{configs[0].CardURL}})

	mgr := NewAgentManager(syncer, cards, "config.yaml")

	agent, ok := mgr.Find("agent-1")
	if !ok {
		t.Fatal("expected to find agent-1")
	}
	if agent.Name != "Agent One" {
		t.Errorf("expected agent Name to be 'Agent One', got %q", agent.Name)
	}

	_, ok = mgr.Find("non-existent")
	if ok {
		t.Error("expected not to find non-existent agent")
	}
}

func TestAgentManager_Add(t *testing.T) {
	configs := []AgentConfig{}
	syncer := newAgentSyncer(t, &configs)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		card := a2aclient.AgentCard{
			Name:        "New Agent",
			Description: "Brand new agent",
			SupportedInterfaces: []a2aclient.AgentInterface{
				{
					URL:             "http://example.com/api-new",
					ProtocolBinding: "JSONRPC",
					ProtocolVersion: "1.0",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	cards := NewCardService(a2aclient.AgentCardURLPolicy{URLs: []string{server.URL}})
	mgr := NewAgentManager(syncer, cards, "config.yaml")

	// Try adding with empty url
	_, err := mgr.Add(t.Context(), "")
	if err == nil {
		t.Error("expected error when adding with empty URL")
	}

	// Try adding with invalid config path (not yaml)
	badMgr := NewAgentManager(syncer, cards, "config.json")
	_, err = badMgr.Add(t.Context(), server.URL)
	if !errors.Is(err, errYAMLConfigRequired) {
		t.Errorf("expected errYAMLConfigRequired, got %v", err)
	}

	// Successful add
	agent, err := mgr.Add(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error adding agent: %v", err)
	}

	if agent.ID != "new-agent" {
		t.Errorf("expected generated ID 'new-agent', got %q", agent.ID)
	}
	if agent.Name != "New Agent" {
		t.Errorf("expected name 'New Agent', got %q", agent.Name)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 saved config, got %d", len(configs))
	}
	if configs[0].CardURL != server.URL {
		t.Errorf("expected saved CardURL to match server URL, got %q", configs[0].CardURL)
	}

	// Add same agent again - should update cache but not append duplicates
	agent2, err := mgr.Add(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("unexpected error re-adding agent: %v", err)
	}
	if agent2.ID != "new-agent" {
		t.Errorf("expected agent ID to remain 'new-agent', got %q", agent2.ID)
	}
	if len(configs) != 1 {
		t.Errorf("expected config count to remain 1, got %d", len(configs))
	}
}

func TestAgentManager_AddRejectsUnallowedRemoteCardURL(t *testing.T) {
	cards := NewCardService()
	configs := []AgentConfig{}
	syncer := newAgentSyncer(t, &configs)
	mgr := NewAgentManager(
		syncer,
		cards,
		"config.yaml",
	)

	_, err := mgr.Add(t.Context(), "https://agent.example.com/.well-known/agent-card.json")
	if !errors.Is(err, a2aclient.ErrAgentCardURLNotAllowed) {
		t.Fatalf("expected ErrAgentCardURLNotAllowed, got %v", err)
	}
}

func TestAgentManager_Patch_And_Delete(t *testing.T) {
	configs := []AgentConfig{
		{
			ID:              "agent-1",
			Name:            "Agent One",
			CardURL:         "http://example.com/card.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
		},
	}

	syncer := newAgentSyncer(t, &configs)
	cards := NewCardService(a2aclient.AgentCardURLPolicy{URLs: []string{configs[0].CardURL}})

	mgr := NewAgentManager(syncer, cards, "config.yaml")

	// Patch enabled false
	enabled := false
	agent, err := mgr.Patch(t.Context(), "agent-1", &enabled)
	if err != nil {
		t.Fatalf("unexpected error patching: %v", err)
	}
	if agent.Enabled {
		t.Error("expected agent to be disabled")
	}

	// Try patch non-existent
	_, err = mgr.Patch(t.Context(), "non-existent", &enabled)
	if err == nil {
		t.Error("expected error patching non-existent agent")
	}

	// Delete agent-1
	err = mgr.Delete(t.Context(), "agent-1")
	if err != nil {
		t.Fatalf("unexpected error deleting agent: %v", err)
	}

	if len(configs) != 0 {
		t.Errorf("expected saved configs to be empty, got %d", len(configs))
	}

	// Verify not found in registry
	_, found := mgr.Find("agent-1")
	if found {
		t.Error("expected agent-1 to be deleted from registry")
	}

	// Try delete non-existent
	err = mgr.Delete(t.Context(), "agent-1")
	if err == nil {
		t.Error("expected error deleting non-existent agent")
	}
}

func TestAgentManager_StatusSummary(t *testing.T) {
	configs := []AgentConfig{
		{
			ID:              "agent-1",
			Name:            "Agent One",
			CardURL:         "http://example.com/card.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
			MCPScopes:       []string{MCPScopeDashboardRead},
		},
		{
			ID:              "agent-2",
			Name:            "Agent Two",
			CardURL:         "http://example.com/card2.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         false,
		},
	}

	syncer := newAgentSyncer(t, &configs)
	cards := NewCardService()

	// Pretend agent-1 has an available card in the cache
	cards.Save(AgentCardCacheEntry{
		AgentID:                   "agent-1",
		CardStatus:                "available",
		DashboardContextSupported: true,
	})

	mgr := NewAgentManager(syncer, cards, "config.yaml")

	summary := mgr.StatusSummary(t.Context())

	if summary.Total != 2 {
		t.Errorf("expected 2 total, got %d", summary.Total)
	}
	if summary.Enabled != 1 {
		t.Errorf("expected 1 enabled, got %d", summary.Enabled)
	}
	if summary.Disabled != 1 {
		t.Errorf("expected 1 disabled, got %d", summary.Disabled)
	}
	if summary.Available != 1 {
		t.Errorf("expected 1 available, got %d", summary.Available)
	}
	if summary.DashboardContextSupported != 1 {
		t.Errorf("expected 1 dashboard context supported, got %d", summary.DashboardContextSupported)
	}
	if summary.MCPScoped != 1 {
		t.Errorf("expected 1 mcp scoped, got %d", summary.MCPScoped)
	}
}

func TestAgentManager_RefreshCard(t *testing.T) {
	configs := []AgentConfig{
		{
			ID:              "agent-1",
			Name:            "Agent One",
			CardURL:         "http://example.com/card.json",
			EndpointURL:     "http://example.com/api",
			ProtocolBinding: "JSONRPC",
			Enabled:         true,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		card := a2aclient.AgentCard{
			Name:        "Agent One Refreshed",
			Description: "First agent",
			SupportedInterfaces: []a2aclient.AgentInterface{
				{
					URL:             "http://example.com/api-refreshed",
					ProtocolBinding: "JSONRPC",
					ProtocolVersion: "1.0",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(card)
	}))
	defer server.Close()

	configs[0].CardURL = server.URL + "/card.json"

	syncer := newAgentSyncer(t, &configs)
	cards := NewCardService(a2aclient.AgentCardURLPolicy{URLs: []string{configs[0].CardURL}})

	mgr := NewAgentManager(syncer, cards, "config.yaml")

	agent, err := mgr.RefreshCard(t.Context(), "agent-1")
	if err != nil {
		t.Fatalf("unexpected error refreshing card: %v", err)
	}

	if agent.SelectedEndpointURL != "http://example.com/api-refreshed" {
		t.Errorf("expected SelectedEndpointURL to match refreshed endpoint, got %q", agent.SelectedEndpointURL)
	}

	_, err = mgr.RefreshCard(t.Context(), "non-existent")
	if err == nil {
		t.Error("expected error refreshing card of non-existent agent")
	}
}
