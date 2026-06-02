package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	a2aclient "jute-dash/internal/a2a"
	"jute-dash/internal/config"
	"jute-dash/internal/registry"
)

var errYAMLConfigRequired = errors.New("YAML config file is required")

func (s *Server) addAgentFromCard(ctx context.Context, cardURL string) (registry.Agent, error) {
	if err := s.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	cardURL = strings.TrimSpace(cardURL)
	if cardURL == "" {
		return registry.Agent{}, errors.New("cardUrl is required")
	}
	result, err := s.agentCards.cardFetcher.Fetch(ctx, cardURL, "")
	if err != nil {
		return registry.Agent{}, err
	}
	selected, err := a2aclient.SelectInterface(result.Card, "", a2aclient.ProtocolJSONRPC)
	if err != nil {
		return registry.Agent{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.cfg.Agents {
		if existing.CardURL == cardURL {
			return s.agentWithDiscovery(registry.New([]config.AgentConfig{existing}).List()[0], cardCacheFromCard(existing.ID, result, selected)), nil
		}
	}

	id := uniqueAgentID(s.cfg.Agents, slug(result.Card.Name))
	agent := config.AgentConfig{
		ID:              id,
		Name:            result.Card.Name,
		Description:     result.Card.Description,
		CardURL:         cardURL,
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		Enabled:         true,
		Capabilities:    []string{"conversation"},
		MCPScopes:       config.DefaultMCPReadScopes(),
	}
	next := s.cfg
	next.Agents = append(append([]config.AgentConfig(nil), s.cfg.Agents...), agent)
	if err := config.SaveYAML(s.configPath, next); err != nil {
		return registry.Agent{}, err
	}
	s.cfg = next
	s.registry = registry.New(s.cfg.Agents)
	cache := cardCacheFromCard(agent.ID, result, selected)
	s.agentCards.save(cache)
	return s.agentWithDiscovery(registry.New([]config.AgentConfig{agent}).List()[0], cache), nil
}

func (s *Server) patchAgent(agentID string, enabled *bool) (registry.Agent, error) {
	if err := s.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	if enabled == nil {
		return registry.Agent{}, errors.New("enabled is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.Agents = append([]config.AgentConfig(nil), s.cfg.Agents...)
	for i := range next.Agents {
		if next.Agents[i].ID != agentID {
			continue
		}
		next.Agents[i].Enabled = *enabled
		if err := config.SaveYAML(s.configPath, next); err != nil {
			return registry.Agent{}, err
		}
		s.cfg = next
		s.registry = registry.New(s.cfg.Agents)
		agent := registry.New([]config.AgentConfig{next.Agents[i]}).List()[0]
		if cache, ok := s.agentCards.load(agent.ID); ok {
			agent = s.agentWithDiscovery(agent, cache)
		}
		return agent, nil
	}
	return registry.Agent{}, errors.New("agent not found")
}

func (s *Server) deleteAgent(agentID string) error {
	if err := s.requireWritableYAMLConfig(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.cfg
	next.Agents = make([]config.AgentConfig, 0, len(s.cfg.Agents))
	found := false
	for _, agent := range s.cfg.Agents {
		if agent.ID == agentID {
			found = true
			continue
		}
		next.Agents = append(next.Agents, agent)
	}
	if !found {
		return errors.New("agent not found")
	}
	if err := config.SaveYAML(s.configPath, next); err != nil {
		return err
	}
	s.cfg = next
	s.registry = registry.New(s.cfg.Agents)
	s.agentCards.remove(agentID)
	return nil
}

func (s *Server) requireWritableYAMLConfig() error {
	ext := strings.ToLower(filepath.Ext(s.configPath))
	if strings.TrimSpace(s.configPath) == "" || (ext != ".yaml" && ext != ".yml") {
		return errYAMLConfigRequired
	}
	return nil
}

func writeAgentConfigError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errYAMLConfigRequired):
		writeError(w, http.StatusConflict, "YAML config file is required to add agents")
	case errors.Is(err, a2aclient.ErrAgentCardUnavailable):
		writeError(w, http.StatusBadGateway, "agent card could not be fetched")
	case errors.Is(err, a2aclient.ErrNoSupportedInterface):
		writeError(w, http.StatusBadRequest, "agent card has no compatible A2A 1.0 JSON-RPC interface")
	case strings.Contains(err.Error(), "required"):
		writeError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "not found"):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "agent configuration could not be updated")
	}
}

func cardCacheFromCard(agentID string, result a2aclient.AgentCardFetchResult, selected a2aclient.SelectedInterface) agentCardCache {
	fetchedAt := result.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	return agentCardCache{
		AgentID:                   agentID,
		CardJSON:                  result.Raw,
		CardStatus:                "available",
		SelectedEndpointURL:       selected.EndpointURL,
		SelectedProtocolBinding:   selected.ProtocolBinding,
		SelectedProtocolVersion:   selected.ProtocolVersion,
		Streaming:                 result.Card.Capabilities.Streaming,
		DashboardContextSupported: a2aclient.SupportsDashboardContext(result.Card),
		Skills:                    result.Card.Skills,
		FetchedAt:                 fetchedAt.Format(time.RFC3339Nano),
		ExpiresAt:                 fetchedAt.Add(10 * time.Minute).Format(time.RFC3339Nano),
	}
}

func uniqueAgentID(agents []config.AgentConfig, base string) string {
	if base == "" {
		base = "agent"
	}
	used := map[string]struct{}{}
	for _, agent := range agents {
		used[agent.ID] = struct{}{}
	}
	if _, ok := used[base]; !ok {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = slugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "agent"
	}
	return value
}
