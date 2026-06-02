import { describe, expect, it, vi } from 'vitest';
import {
  createConversation,
  fallbackDashboard,
  getConversations,
  initialDashboard,
  parseSSEEvent,
  sendConversationTurn,
  sendConversationTurnStream
} from './api';

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
    ...init
  });
}

describe('api conversation history', () => {
  it('does not fetch conversations until an agent is selected', async () => {
    const fetcher = vi.fn<typeof fetch>();

    await expect(getConversations(fetcher, '')).resolves.toEqual([]);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('loads agent-backed conversation history', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse({
        conversations: [
          {
            id: 'ctx-1',
            agentId: 'house',
            title: 'Hello',
            status: 'completed',
            a2aContextId: 'ctx-1',
            latestTaskId: 'task-1',
            createdAt: '2026-06-02T10:00:00Z',
            updatedAt: '2026-06-02T10:01:00Z'
          }
        ]
      })
    );

    const conversations = await getConversations(fetcher, 'house');

    expect(conversations).toHaveLength(1);
    expect(conversations[0].id).toBe('ctx-1');
    expect(String(fetcher.mock.calls[0][0])).toContain('/api/v1/conversations?agentId=house');
  });

  it('maps unsupported agent history to a calm placeholder conversation', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse({ error: 'agent history is unavailable' }, { status: 501 }));

    const conversations = await getConversations(fetcher, 'house');

    expect(conversations).toEqual([
      expect.objectContaining({
        id: 'history-unsupported-house',
        agentId: 'house',
        historyUnsupported: true
      })
    ]);
  });

  it('creates a conversation and sends turns through the hub API', async () => {
    const detail = {
      conversation: {
        id: 'ctx-1',
        agentId: 'house',
        title: 'House',
        status: 'completed',
        a2aContextId: 'ctx-1',
        latestTaskId: 'task-1',
        createdAt: '2026-06-02T10:00:00Z',
        updatedAt: '2026-06-02T10:01:00Z'
      },
      messages: [{ id: 'agent-1', conversationId: 'ctx-1', agentId: 'house', role: 'assistant', content: 'Hello', status: 'sent' }]
    };
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(detail, { status: 201 }));

    await expect(createConversation(fetcher, 'house', 'Kitchen')).resolves.toEqual(detail);

    expect(fetcher).toHaveBeenCalledWith(
      expect.stringContaining('/api/v1/conversations'),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ agentId: 'house', title: 'Kitchen' })
      })
    );

    fetcher.mockResolvedValueOnce(jsonResponse(detail));
    await expect(sendConversationTurn(fetcher, 'ctx-1', 'house', 'Hello')).resolves.toEqual(detail);
    expect(fetcher).toHaveBeenLastCalledWith(
      expect.stringContaining('/api/v1/conversations/ctx-1/turns'),
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ agentId: 'house', text: 'Hello' })
      })
    );
  });
});

describe('api conversation streaming', () => {
  it('parses SSE conversation events', () => {
    const event = parseSSEEvent(`event: assistant_delta
data: {"conversationId":"ctx-1","text":"Hi","append":true}

`);

    expect(event).toEqual({
      type: 'assistant_delta',
      conversationId: 'ctx-1',
      text: 'Hi',
      append: true
    });
  });

  it('streams turn events across chunk boundaries', async () => {
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        const encoder = new TextEncoder();
        controller.enqueue(encoder.encode(`event: assistant_delta
data: {"conversationId":"ctx-1","text":"Hel","append":true}

`));
        controller.enqueue(encoder.encode(`event: assistant_delta
data: {"conversationId":"ctx-1","text":"lo","append":true}

`));
        controller.close();
      }
    });
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(stream, { status: 200 }));
    const events: unknown[] = [];

    await sendConversationTurnStream(fetcher, 'ctx-1', 'house', 'Hello', (event) => events.push(event));

    expect(events).toEqual([
      { type: 'assistant_delta', conversationId: 'ctx-1', text: 'Hel', append: true },
      { type: 'assistant_delta', conversationId: 'ctx-1', text: 'lo', append: true }
    ]);
  });
});

describe('fallback dashboard', () => {
  it('marks offline scaffolding as stale and agentless', () => {
    const fallback = fallbackDashboard();

    expect(fallback.connectionState).toBe('offline');
    expect(fallback.stale).toBe(true);
    expect(fallback.agents).toEqual([]);
    expect(fallback.layout.widgets.map((widget) => widget.kind)).toEqual(['date-time', 'weather', 'chat-history']);
    expect('weather' in fallback.home).toBe(false);
  });

  it('can create a neutral initial dashboard before client-side hub connection', () => {
    const initial = initialDashboard();

    expect(initial.connectionState).toBe('starting');
    expect(initial.issue).toBeUndefined();
  });
});
