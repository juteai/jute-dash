package agents

import (
	"context"
	"sync"
	"time"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

const CardCacheTTL = 10 * time.Minute

type CardService struct {
	mu          sync.Mutex
	cards       map[string]AgentCardCacheEntry
	cardFetcher *a2aclient.AgentCardFetcher
	cardPolicy  a2aclient.AgentCardURLPolicy
}

func NewCardService(policy ...a2aclient.AgentCardURLPolicy) *CardService {
	cardPolicy := a2aclient.DefaultAgentCardURLPolicy()
	if len(policy) > 0 {
		cardPolicy = policy[0]
	}
	return &CardService{
		cards:       map[string]AgentCardCacheEntry{},
		cardFetcher: a2aclient.NewAgentCardFetcher(),
		cardPolicy:  cardPolicy,
	}
}

// Load returns the cached card for agentID if it exists.
func (svc *CardService) Load(agentID string) (AgentCardCacheEntry, bool) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	c, ok := svc.cards[agentID]
	return c, ok
}

// Remove deletes a card cache entry for the given agent ID.
func (svc *CardService) Remove(agentID string) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	delete(svc.cards, agentID)
}

// Save writes a card cache entry.
func (svc *CardService) Save(c AgentCardCacheEntry) {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.cards[c.AgentID] = c
}

// Current returns a valid cache entry, refreshing it first if necessary.
func (svc *CardService) Current(
	ctx context.Context,
	agent registry.Agent,
	configuredAgent AgentConfig,
) AgentCardCacheEntry {
	if c, ok := svc.Load(agent.ID); ok && c.CardStatus == "available" {
		return c
	}
	return svc.Refresh(ctx, agent, configuredAgent)
}

// Refresh fetches the agent card from the network, selects an interface, and
// stores the result. It always returns a cache entry — on failure the entry
// has CardStatus "unavailable".
func (svc *CardService) Refresh(
	ctx context.Context,
	agent registry.Agent,
	configuredAgent AgentConfig,
) AgentCardCacheEntry {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	c := AgentCardCacheEntry{
		AgentID:                 agent.ID,
		CardStatus:              "unavailable",
		CardError:               "agent card is unavailable",
		SelectedEndpointURL:     agent.EndpointURL,
		SelectedProtocolBinding: agent.ProtocolBinding,
		SelectedProtocolVersion: a2aclient.ProtocolVersion10,
		FetchedAt:               now,
		ExpiresAt:               now,
	}
	cardURL, err := svc.cardPolicy.Authorize(agent.CardURL)
	if err != nil {
		c.CardError = "agent card url is not allowed"
		svc.Save(c)
		return c
	}
	bearerToken, _ := AgentBearerToken(configuredAgent)
	result, err := svc.cardFetcher.Fetch(ctx, cardURL, bearerToken)
	if err != nil {
		c.CardError = "agent card could not be fetched"
		svc.Save(c)
		return c
	}
	selected, err := a2aclient.SelectInterface(result.Card)
	if err != nil {
		c.CardJSON = result.Raw
		c.CardError = "agent card has no compatible A2A 1.0 interface"
		c.FetchedAt = result.FetchedAt.Format(time.RFC3339Nano)
		c.ExpiresAt = result.FetchedAt.Add(CardCacheTTL).Format(time.RFC3339Nano)
		c.Skills = result.Card.Skills
		c.Streaming = result.Card.Capabilities.Streaming
		c.DashboardContextSupported = a2aclient.SupportsDashboardContext(result.Card)
		svc.Save(c)
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
	c.ExpiresAt = result.FetchedAt.Add(CardCacheTTL).Format(time.RFC3339Nano)
	svc.Save(c)
	return c
}
