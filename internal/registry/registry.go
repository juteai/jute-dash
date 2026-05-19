package registry

import (
	"jute-dash/internal/a2a"
	"jute-dash/internal/config"
)

type Agent struct {
	ID                        string           `json:"id"`
	Name                      string           `json:"name"`
	Description               string           `json:"description"`
	CardURL                   string           `json:"cardUrl"`
	EndpointURL               string           `json:"endpointUrl"`
	ProtocolBinding           string           `json:"protocolBinding"`
	Enabled                   bool             `json:"enabled"`
	Capabilities              []string         `json:"capabilities"`
	AuthConfigured            bool             `json:"authConfigured"`
	CardStatus                string           `json:"cardStatus,omitempty"`
	CardFetchedAt             string           `json:"cardFetchedAt,omitempty"`
	CardError                 string           `json:"cardError,omitempty"`
	SelectedEndpointURL       string           `json:"selectedEndpointUrl,omitempty"`
	SelectedProtocolBinding   string           `json:"selectedProtocolBinding,omitempty"`
	SelectedProtocolVersion   string           `json:"selectedProtocolVersion,omitempty"`
	Skills                    []a2a.AgentSkill `json:"skills,omitempty"`
	Streaming                 bool             `json:"streaming"`
	DashboardContextSupported bool             `json:"dashboardContextSupported"`
}

type Registry struct {
	agents []Agent
	byID   map[string]Agent
}

func New(configured []config.AgentConfig) Registry {
	agents := make([]Agent, 0, len(configured))
	byID := make(map[string]Agent, len(configured))

	for _, item := range configured {
		agent := Agent{
			ID:              item.ID,
			Name:            item.Name,
			Description:     item.Description,
			CardURL:         item.CardURL,
			EndpointURL:     item.EndpointURL,
			ProtocolBinding: item.ProtocolBinding,
			Enabled:         item.Enabled,
			Capabilities:    append([]string(nil), item.Capabilities...),
			AuthConfigured:  item.Auth != nil,
		}
		agents = append(agents, agent)
		byID[agent.ID] = agent
	}

	return Registry{agents: agents, byID: byID}
}

func (r Registry) List() []Agent {
	return append([]Agent(nil), r.agents...)
}

func (r Registry) Enabled() []Agent {
	enabled := make([]Agent, 0, len(r.agents))
	for _, agent := range r.agents {
		if agent.Enabled {
			enabled = append(enabled, agent)
		}
	}
	return enabled
}

func (r Registry) Find(id string) (Agent, bool) {
	agent, ok := r.byID[id]
	return agent, ok
}
