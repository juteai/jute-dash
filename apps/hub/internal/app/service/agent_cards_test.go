package service

import (
	"testing"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

func TestCardServiceCurrentUsesAvailableCache(t *testing.T) {
	cards := NewCardService(a2aclient.AgentCardURLPolicy{})
	cached := AgentCardCacheEntry{
		AgentID:                 "agent-1",
		CardStatus:              "available",
		SelectedEndpointURL:     "http://cached.example/a2a",
		SelectedProtocolBinding: a2aclient.ProtocolJSONRPC,
	}
	cards.Save(cached)

	got := cards.Current(t.Context(), registry.Agent{ID: "agent-1"}, AgentConfig{})

	if got.SelectedEndpointURL != cached.SelectedEndpointURL || got.CardStatus != "available" {
		t.Fatalf("expected cached card, got %+v", got)
	}
}
