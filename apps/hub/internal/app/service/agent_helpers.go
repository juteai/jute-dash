package service

import "jute-dash/apps/hub/internal/pkg/registry"

type selectedAgentInterface struct {
	EndpointURL     string
	ProtocolBinding string
	ProtocolVersion string
	Streaming       bool
	Extensions      []string
	Metadata        map[string]any
}

func agentAuthAvailableFromPublic(agent registry.Agent) bool {
	return !agent.AuthConfigured || agent.AuthAvailable
}
