import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { chatStore } from './chatStore';
import { navigationStore } from './navigationStore';
import type { Agent } from './types';
import type { ChatStoreState } from './chatStore';

function createMockAgent(overrides: Partial<Agent> = {}): Agent {
  return {
    id: 'agent-1',
    name: 'Test Agent',
    description: 'Test agent description',
    cardUrl: 'http://localhost/agent-card.json',
    endpointUrl: 'http://localhost/invoke',
    protocolBinding: 'JSONRPC',
    enabled: true,
    capabilities: ['conversation'],
    authConfigured: false,
    cardStatus: 'available',
    streaming: false,
    ...overrides
  };
}

describe('chatStore unit tests', () => {
  beforeEach(() => {
    chatStore.clearHistory();
    chatStore.setAgentId('');
    chatStore.stopTimer();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('verifies initial state is correct', () => {
    let state: ChatStoreState | undefined;
    const unsubscribe = chatStore.subscribe((s) => {
      state = s;
    });
    unsubscribe();

    expect(state).toBeDefined();
    expect(state!.chatState).toBe('idle');
    expect(state!.messages).toEqual([]);
    expect(state!.messageQueue).toEqual([]);
    expect(state!.showTimer).toBe(false);
    expect(state!.timerProgress).toBe(0);
  });

  it('updates selectedAgentId and clears history on setAgentId', () => {
    chatStore.setAgentId('new-agent');

    let state: ChatStoreState | undefined;
    const unsubscribe = chatStore.subscribe((s) => {
      state = s;
    });
    unsubscribe();

    expect(state).toBeDefined();
    expect(state!.selectedAgentId).toBe('new-agent');
    expect(state!.messages).toEqual([]);
    expect(state!.chatState).toBe('idle');
  });

  it('queues messages when chatState is thinking', async () => {
    // Manually force store into thinking state
    // (Simulating an active sendConversationTurn call)
    let state: ChatStoreState | undefined;
    chatStore.setAgentId('agent-1');

    let resolveFetch: (res: Response) => void = () => {};
    const fetchPromise = new Promise<Response>((resolve) => {
      resolveFetch = resolve;
    });

    const mockFetch = vi.fn().mockImplementation(() => {
      return fetchPromise;
    });

    // We start a turn submission
    const submitPromise = chatStore.submit(
      'hello',
      [createMockAgent()],
      undefined,
      mockFetch
    );

    // Yield control to let the first submit resolve ensureConversation and append the first message
    await Promise.resolve();
    await Promise.resolve();
    await Promise.resolve();

    // Immediately submit a second message before mockFetch resolves (chatState is 'thinking')
    chatStore.submit(
      'second message',
      [createMockAgent()],
      undefined,
      mockFetch
    );

    const unsubscribe = chatStore.subscribe((s) => {
      state = s;
    });
    unsubscribe();

    expect(state).toBeDefined();
    expect(state!.messageQueue).toHaveLength(1);
    expect(state!.messageQueue[0].text).toBe('second message');
    expect(state!.messages).toHaveLength(2);
    expect(state!.messages[0].content).toBe('hello');
    expect(state!.messages[1].content).toBe('second message');
    expect(state!.messages[1].status).toBe('queued');

    // Resolve the fetch to finish the first turn and process the queue
    resolveFetch(
      new Response(
        JSON.stringify({
          conversation: {
            id: 'ctx-1',
            agentId: 'agent-1',
            status: 'completed',
            updatedAt: new Date().toISOString()
          },
          messages: []
        }),
        { status: 200, headers: { 'content-type': 'application/json' } }
      )
    );

    await submitPromise;
  });

  it('controls the 60-second dismiss timer progress and callbacks', () => {
    let state: ChatStoreState | undefined;
    const unsubscribe = chatStore.subscribe((s) => {
      state = s;
    });

    const closeSpy = vi.spyOn(navigationStore, 'closeChat');

    // Start dismiss timer
    chatStore.resetTimer();

    expect(state).toBeDefined();
    expect(state!.showTimer).toBe(true);
    expect(state!.timerProgress).toBe(1);

    // Fast-forward timer by 30 seconds
    vi.advanceTimersByTime(30000);
    expect(state!.dismissTimeRemaining).toBeCloseTo(30, 1);
    expect(state!.timerProgress).toBeCloseTo(0.5, 1);
    expect(closeSpy).not.toHaveBeenCalled();

    // Fast-forward by remaining 30 seconds (total 60s)
    vi.advanceTimersByTime(30000);
    expect(state!.showTimer).toBe(false);
    expect(state!.timerProgress).toBe(0);
    expect(closeSpy).toHaveBeenCalled();

    unsubscribe();
    closeSpy.mockRestore();
  });
});
