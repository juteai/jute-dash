package server

import (
	"context"
	"sync"
	"time"

	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/registry"
)

const cardCacheTTL = 10 * time.Minute

// agentCardService owns the in-memory agent card cache and A2A card fetching
// logic. It is the single place where agent card state lives; no other part of
// the Server holds or mutates card cache entries directly.
//
// Callers pass in the config and token they need — the service has no opinion
// about where those come from.
type agentCardService struct {
	mu          sync.Mutex
	cards       map[string]agentCardCache
	cardFetcher *a2aclient.AgentCardFetcher
}

func newAgentCardService() *agentCardService {
	return &agentCardService{
		cards:       map[string]agentCardCache{},
		cardFetcher: a2aclient.NewAgentCardFetcher(),
	}
}

// load returns the cached card for agentID if it exists.
func (svc *agentCardService) load(agentID string) (agentCardCache, bool) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	c, ok := svc.cards[agentID]
	return c, ok
}

// remove deletes a card cache entry for the given agent ID.
func (svc *agentCardService) remove(agentID string) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	delete(svc.cards, agentID)
}

// save writes a card cache entry.
func (svc *agentCardService) save(c agentCardCache) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.cards[c.AgentID] = c
}

// current returns a valid cache entry, refreshing it first if necessary.
func (svc *agentCardService) current(ctx context.Context, agent registry.Agent, configuredAgent config.AgentConfig) agentCardCache {
	if c, ok := svc.load(agent.ID); ok && c.CardStatus == "available" {
		return c
	}
	return svc.refresh(ctx, agent, configuredAgent)
}

// refresh fetches the agent card from the network, selects an interface, and
// stores the result. It always returns a cache entry — on failure the entry
// has CardStatus "unavailable".
func (svc *agentCardService) refresh(ctx context.Context, agent registry.Agent, configuredAgent config.AgentConfig) agentCardCache {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	c := agentCardCache{
		AgentID:                 agent.ID,
		CardStatus:              "unavailable",
		CardError:               "agent card is unavailable",
		SelectedEndpointURL:     agent.EndpointURL,
		SelectedProtocolBinding: agent.ProtocolBinding,
		SelectedProtocolVersion: a2aclient.ProtocolVersion10,
		FetchedAt:               now,
		ExpiresAt:               now,
	}
	bearerToken, _ := agentBearerToken(configuredAgent)
	result, err := svc.cardFetcher.Fetch(ctx, agent.CardURL, bearerToken)
	if err != nil {
		c.CardError = "agent card could not be fetched"
		svc.save(c)
		return c
	}
	selected, err := a2aclient.SelectInterface(result.Card, agent.EndpointURL, agent.ProtocolBinding)
	if err != nil {
		c.CardJSON = result.Raw
		c.CardError = "agent card has no compatible A2A 1.0 interface"
		c.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
		c.ExpiresAt = result.FetchedAt.Add(cardCacheTTL).Format(time.RFC3339Nano)
		c.Skills = result.Card.Skills
		c.Streaming = result.Card.Capabilities.Streaming
		c.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
		svc.save(c)
		return c
	}
	c.CardJSON = result.Raw
	c.CardStatus = "available"
	c.CardError = ""
	c.SelectedEndpointURL = selected.EndpointURL
	c.SelectedProtocolBinding = selected.ProtocolBinding
	c.SelectedProtocolVersion = selected.ProtocolVersion
	c.Streaming = result.Card.Capabilities.Streaming
	c.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
	c.Skills = result.Card.Skills
	c.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
	c.ExpiresAt = result.FetchedAt.Add(cardCacheTTL).Format(time.RFC3339Nano)
	svc.save(c)
	return c
}
