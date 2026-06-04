package agents

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	a2aclient "jute-dash/apps/hub/internal/pkg/a2a"
	"jute-dash/apps/hub/internal/pkg/registry"
)

var (
	errYAMLConfigRequired = errors.New("YAML config file is required")
)

type AgentManager struct {
	mu         sync.RWMutex
	cards      *CardService
	configPath string
	getConfig  func() []AgentConfig
	saveConfig func([]AgentConfig) error
	registry   registry.Registry
}

func NewAgentManager(
	getConfig func() []AgentConfig,
	saveConfig func([]AgentConfig) error,
	cards *CardService,
	configPath string,
) *AgentManager {
	initialConfigs := getConfig()
	m := &AgentManager{
		cards:      cards,
		configPath: configPath,
		getConfig:  getConfig,
		saveConfig: saveConfig,
		registry:   registry.New(mapToRegistryAgentConfigs(initialConfigs)),
	}
	return m
}

func (m *AgentManager) ActiveRegistry() registry.Registry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registry
}

func (m *AgentManager) List(ctx context.Context, triggerDiscovery bool) []registry.Agent {
	m.mu.Lock()
	agentsList := m.registry.List()
	m.mu.Unlock()

	out := make([]registry.Agent, len(agentsList))
	for i, agent := range agentsList {
		cache, ok := m.cards.Load(agent.ID)
		if !ok && triggerDiscovery && agent.Enabled {
			// Trigger a discovery refresh synchronously
			configured, _ := m.ConfiguredAgent(agent.ID)
			cache = m.cards.Refresh(ctx, agent, configured)
		}
		out[i] = m.enrichAgent(agent, cache)
	}
	return out
}

func (m *AgentManager) Find(id string) (registry.Agent, bool) {
	m.mu.RLock()
	agent, ok := m.registry.Find(id)
	m.mu.RUnlock()

	if !ok {
		return registry.Agent{}, false
	}
	var cache AgentCardCacheEntry
	if c, ok := m.cards.Load(agent.ID); ok {
		cache = c
	}
	return m.enrichAgent(agent, cache), true
}

func (m *AgentManager) ConfiguredAgent(id string) (AgentConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	configs := m.getConfig()
	for _, cfg := range configs {
		if cfg.ID == id {
			return cfg, true
		}
	}
	return AgentConfig{}, false
}

func (m *AgentManager) Add(ctx context.Context, cardURL string) (registry.Agent, error) {
	if err := m.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	cardURL = strings.TrimSpace(cardURL)
	if cardURL == "" {
		return registry.Agent{}, errors.New("cardUrl is required")
	}
	result, err := m.cards.cardFetcher.Fetch(ctx, cardURL, "")
	if err != nil {
		return registry.Agent{}, err
	}
	selected, err := a2aclient.SelectInterface(result.Card)
	if err != nil {
		return registry.Agent{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agents := m.getConfig()
	for _, existing := range agents {
		if existing.CardURL == cardURL {
			regAgent := registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(existing)}).List()[0]
			cache := cardCacheFromCard(existing.ID, result, selected)
			m.cards.Save(cache)
			return m.enrichAgentLocked(regAgent, cache), nil
		}
	}

	id := uniqueAgentID(agents, slug(result.Card.Name))
	agent := AgentConfig{
		ID:              id,
		Name:            result.Card.Name,
		Description:     result.Card.Description,
		CardURL:         cardURL,
		EndpointURL:     selected.EndpointURL,
		ProtocolBinding: selected.ProtocolBinding,
		Enabled:         true,
		Capabilities:    []string{"conversation"},
		MCPScopes:       DefaultMCPReadScopes(),
	}

	nextAgents := append(append([]AgentConfig(nil), agents...), agent)
	if err := m.saveConfig(nextAgents); err != nil {
		return registry.Agent{}, err
	}

	m.registry = registry.New(mapToRegistryAgentConfigs(nextAgents))

	cache := cardCacheFromCard(agent.ID, result, selected)
	m.cards.Save(cache)

	regAgent := registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(agent)}).List()[0]
	return m.enrichAgentLocked(regAgent, cache), nil
}

func (m *AgentManager) Patch(id string, enabled *bool) (registry.Agent, error) {
	if err := m.requireWritableYAMLConfig(); err != nil {
		return registry.Agent{}, err
	}
	if enabled == nil {
		return registry.Agent{}, errors.New("enabled is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agents := m.getConfig()
	nextAgents := append([]AgentConfig(nil), agents...)
	for i := range nextAgents {
		if nextAgents[i].ID != id {
			continue
		}
		nextAgents[i].Enabled = *enabled
		if err := m.saveConfig(nextAgents); err != nil {
			return registry.Agent{}, err
		}

		m.registry = registry.New(mapToRegistryAgentConfigs(nextAgents))

		agent := registry.New([]registry.AgentConfig{mapToRegistryAgentConfig(nextAgents[i])}).List()[0]
		var cache AgentCardCacheEntry
		if c, ok := m.cards.Load(agent.ID); ok {
			cache = c
		}
		return m.enrichAgentLocked(agent, cache), nil
	}
	return registry.Agent{}, errors.New("agent not found")
}

func (m *AgentManager) Delete(id string) error {
	if err := m.requireWritableYAMLConfig(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	agents := m.getConfig()
	nextAgents := make([]AgentConfig, 0, len(agents))
	found := false
	for _, agent := range agents {
		if agent.ID == id {
			found = true
			continue
		}
		nextAgents = append(nextAgents, agent)
	}
	if !found {
		return errors.New("agent not found")
	}
	if err := m.saveConfig(nextAgents); err != nil {
		return err
	}

	m.registry = registry.New(mapToRegistryAgentConfigs(nextAgents))
	m.cards.Remove(id)
	return nil
}

func (m *AgentManager) RefreshCard(ctx context.Context, id string) (registry.Agent, error) {
	m.mu.RLock()
	agent, ok := m.registry.Find(id)
	m.mu.RUnlock()

	if !ok {
		return registry.Agent{}, errors.New("agent not found")
	}

	configured, _ := m.ConfiguredAgent(id)
	cache := m.cards.Refresh(ctx, agent, configured)
	return m.agentWithDiscovery(agent, cache), nil
}

func (m *AgentManager) requireWritableYAMLConfig() error {
	ext := strings.ToLower(filepath.Ext(m.configPath))
	if strings.TrimSpace(m.configPath) == "" || (ext != ".yaml" && ext != ".yml") {
		return errYAMLConfigRequired
	}
	return nil
}

func (m *AgentManager) agentWithDiscovery(agent registry.Agent, cache AgentCardCacheEntry) registry.Agent {
	agent.CardStatus = cache.CardStatus
	agent.CardFetchedAt = cache.FetchedAt
	agent.CardError = cache.CardError
	agent.SelectedEndpointURL = cache.SelectedEndpointURL
	agent.SelectedProtocolBinding = cache.SelectedProtocolBinding
	agent.SelectedProtocolVersion = cache.SelectedProtocolVersion
	agent.Skills = append([]a2aclient.AgentSkill(nil), cache.Skills...)
	agent.Streaming = cache.Streaming
	agent.DashboardContextSupported = cache.DashboardContextSupported
	if agent.SelectedEndpointURL != "" {
		agent.EndpointURL = agent.SelectedEndpointURL
	}
	if agent.SelectedProtocolBinding != "" {
		agent.ProtocolBinding = agent.SelectedProtocolBinding
	}
	return agent
}

// Global mapping helpers

func mapToRegistryAgentConfig(agent AgentConfig) registry.AgentConfig {
	return registry.AgentConfig{
		ID:              agent.ID,
		Name:            agent.Name,
		Description:     agent.Description,
		CardURL:         agent.CardURL,
		EndpointURL:     agent.EndpointURL,
		ProtocolBinding: agent.ProtocolBinding,
		Enabled:         agent.Enabled,
		Capabilities:    append([]string(nil), agent.Capabilities...),
		MCPScopes:       append([]string(nil), agent.MCPScopes...),
		AuthConfigured:  agent.Auth != nil,
	}
}

func mapToRegistryAgentConfigs(agents []AgentConfig) []registry.AgentConfig {
	out := make([]registry.AgentConfig, len(agents))
	for i, a := range agents {
		out[i] = mapToRegistryAgentConfig(a)
	}
	return out
}

func uniqueAgentID(agents []AgentConfig, base string) string {
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

func cardCacheFromCard(
	agentID string,
	result a2aclient.AgentCardFetchResult,
	selected a2aclient.SelectedInterface,
) AgentCardCacheEntry {
	fetchedAt := result.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	return AgentCardCacheEntry{
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
		ExpiresAt:                 fetchedAt.Add(CardCacheTTL).Format(time.RFC3339Nano),
	}
}

func (m *AgentManager) enrichAgent(agent registry.Agent, cache AgentCardCacheEntry) registry.Agent {
	if configured, ok := m.ConfiguredAgent(agent.ID); ok {
		agent.AuthConfigured = configured.Auth != nil
		agent.AuthAvailable = agentAuthAvailable(configured)
	}
	return m.agentWithDiscovery(agent, cache)
}

func (m *AgentManager) configuredAgentLocked(id string) (AgentConfig, bool) {
	configs := m.getConfig()
	for _, cfg := range configs {
		if cfg.ID == id {
			return cfg, true
		}
	}
	return AgentConfig{}, false
}

func (m *AgentManager) enrichAgentLocked(agent registry.Agent, cache AgentCardCacheEntry) registry.Agent {
	if configured, ok := m.configuredAgentLocked(agent.ID); ok {
		agent.AuthConfigured = configured.Auth != nil
		agent.AuthAvailable = agentAuthAvailable(configured)
	}
	return m.agentWithDiscovery(agent, cache)
}

//nolint:gochecknoglobals // allow global test seam for os.Getenv
var osGetenv = os.Getenv

func SetEnvReader(reader func(string) string) {
	osGetenv = reader
}

func agentAuthAvailable(agent AgentConfig) bool {
	if agent.Auth == nil {
		return true
	}
	return strings.TrimSpace(osGetenv(agent.Auth.EnvToken)) != ""
}

func (m *AgentManager) StatusSummary(ctx context.Context) AgentStatusSummary {
	agents := m.List(ctx, false)
	summary := AgentStatusSummary{Total: len(agents)}
	for _, agent := range agents {
		if agent.Enabled {
			summary.Enabled++
		} else {
			summary.Disabled++
		}
		if agent.Enabled && agent.CardStatus == "available" && agentAuthAvailableFromPublic(agent) {
			summary.Available++
		}
		if agent.Enabled && agent.CardStatus != "" && agent.CardStatus != "available" {
			summary.Unavailable++
		}
		if agent.DashboardContextSupported {
			summary.DashboardContextSupported++
		}
		if len(agent.MCPScopes) > 0 {
			summary.MCPScoped++
		}
	}
	return summary
}
