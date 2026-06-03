import { describe, expect, it } from 'vitest';
import {
  availabilityDescription,
  availabilityLabel,
  firstAvailableAgent,
  getAgentAvailability,
  isAgentAvailable
} from './agents';
import type { Agent } from './types';

function agent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'house',
    name: 'House',
    description: 'House assistant',
    cardUrl: 'http://127.0.0.1:9797/.well-known/agent-card.json',
    endpointUrl: 'http://127.0.0.1:9797/invoke',
    protocolBinding: 'JSONRPC',
    enabled: true,
    capabilities: ['conversation'],
    authConfigured: false,
    cardStatus: 'available',
    ...overrides
  };
}

describe('agent availability', () => {
  it('marks a JSON-RPC agent with a card and endpoint as available', () => {
    const available = agent();

    expect(getAgentAvailability(available)).toBe('available');
    expect(isAgentAvailable(available)).toBe(true);
    expect(availabilityLabel('available')).toBe('available');
    expect(availabilityDescription('available')).toContain('A2A');
  });

  it('distinguishes disabled, missing credential, unsupported, offline, and unknown states', () => {
    expect(getAgentAvailability(agent({ enabled: false }))).toBe('disabled');
    expect(
      getAgentAvailability(
        agent({ authConfigured: true, authAvailable: false })
      )
    ).toBe('missing_credentials');
    expect(getAgentAvailability(agent({ protocolBinding: 'HTTP+JSON' }))).toBe(
      'unsupported_binding'
    );
    expect(getAgentAvailability(agent({ cardStatus: 'unavailable' }))).toBe(
      'offline'
    );
    expect(
      getAgentAvailability(agent({ endpointUrl: '', selectedEndpointUrl: '' }))
    ).toBe('unknown');
  });

  it('uses discovered protocol binding and endpoint when present', () => {
    const discovered = agent({
      protocolBinding: 'HTTP+JSON',
      selectedProtocolBinding: 'JSONRPC',
      selectedEndpointUrl: 'http://127.0.0.1:9797/jsonrpc'
    });

    expect(getAgentAvailability(discovered)).toBe('available');
  });

  it('returns the first available agent from a mixed list', () => {
    const disabled = agent({ id: 'disabled', enabled: false });
    const available = agent({ id: 'available' });

    expect(firstAvailableAgent([disabled, available])?.id).toBe('available');
  });
});
