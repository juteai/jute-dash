import type { Agent, AgentAvailability } from '$lib/types';

export function getAgentAvailability(
  agent: Agent | undefined
): AgentAvailability {
  if (!agent) {
    return 'unknown';
  }
  if (!agent.enabled) {
    return 'disabled';
  }
  const binding = agent.selectedProtocolBinding || agent.protocolBinding;
  const version = agent.selectedProtocolVersion;
  if (binding !== 'JSONRPC' || (version && version !== '1.0')) {
    return 'unsupported_binding';
  }
  if (agent.authConfigured && agent.authAvailable === false) {
    return 'missing_credentials';
  }
  if (agent.cardStatus && agent.cardStatus !== 'available') {
    return 'offline';
  }
  if (!(agent.selectedEndpointUrl || agent.endpointUrl) || !agent.cardUrl) {
    return 'unknown';
  }
  return 'available';
}

export function isAgentAvailable(agent: Agent | undefined) {
  return getAgentAvailability(agent) === 'available';
}

export function availabilityLabel(availability: AgentAvailability) {
  switch (availability) {
    case 'available':
      return 'available';
    case 'disabled':
      return 'disabled';
    case 'missing_credentials':
      return 'missing credentials';
    case 'unsupported_binding':
      return 'unsupported binding';
    case 'unhealthy':
      return 'unhealthy';
    case 'offline':
      return 'offline';
    default:
      return 'unknown';
  }
}

export function availabilityDescription(availability: AgentAvailability) {
  switch (availability) {
    case 'available':
      return 'Ready for A2A 1.0 chat.';
    case 'disabled':
      return 'This agent is configured but disabled.';
    case 'missing_credentials':
      return 'This agent needs credentials before Jute can send messages.';
    case 'unsupported_binding':
      return 'Jute can send JSON-RPC A2A 1.0 turns right now.';
    case 'unhealthy':
      return 'The agent health check failed.';
    case 'offline':
      return 'The agent endpoint is not reachable.';
    default:
      return 'Agent health has not been checked yet.';
  }
}

export function availabilityTone(
  availability: AgentAvailability
): 'neutral' | 'active' | 'warning' | 'danger' {
  if (availability === 'available') {
    return 'active';
  }
  if (availability === 'disabled' || availability === 'unknown') {
    return 'neutral';
  }
  if (availability === 'offline' || availability === 'unhealthy') {
    return 'danger';
  }
  return 'warning';
}

export function firstAvailableAgent(agents: Agent[]) {
  return agents.find(isAgentAvailable);
}
