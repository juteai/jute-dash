import { describe, expect, it, vi } from 'vitest';
import {
  createConversation,
  fallbackDashboard,
  getConversations,
  initialDashboard,
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
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              tasks: [
                {
                  id: 'task-1',
                  contextId: 'ctx-1',
                  text: 'Hello',
                  updatedAt: '2026-06-02T10:01:00Z',
                  status: { state: 'completed' },
                  messages: [
                    {
                      id: 'msg-1',
                      role: 'user',
                      text: 'Hello'
                    }
                  ]
                }
              ]
            }
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const conversations = await getConversations(fetcher, 'house');

    expect(conversations).toHaveLength(1);
    expect(conversations[0].id).toBe('ctx-1');
    expect(conversations[0].title).toBe('Hello');
    expect(String(fetcher.mock.calls[0][0])).toContain(
      '/api/v1/proxy/agents/house'
    );
  });

  it('maps unsupported agent history to a calm placeholder conversation', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        jsonResponse({ error: 'agent history is unavailable' }, { status: 501 })
      );

    const conversations = await getConversations(fetcher, 'house');

    expect(conversations).toEqual([
      expect.objectContaining({
        id: 'history-unsupported-house',
        agentId: 'house',
        historyUnsupported: true
      })
    ]);
  });

  it('creates a conversation detail locally', async () => {
    const fetcher = vi.fn<typeof fetch>();

    const detail = await createConversation(fetcher, 'house', 'Kitchen');
    expect(detail.conversation.agentId).toBe('house');
    expect(detail.conversation.title).toBe('Kitchen');
    expect(detail.conversation.id).toMatch(/^ctx-/);
    expect(detail.messages).toEqual([]);
    expect(fetcher).not.toHaveBeenCalled();
  });

  it('sends turns through the hub proxy API using JSON-RPC', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'message/send') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {}
          });
        }
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              tasks: [
                {
                  id: 'task-1',
                  contextId: 'ctx-1',
                  text: 'Hello',
                  updatedAt: '2026-06-02T10:01:00Z',
                  status: { state: 'completed' }
                }
              ]
            }
          });
        }
        if (body.method === 'tasks/get') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              id: 'task-1',
              contextId: 'ctx-1',
              text: 'Hello',
              updatedAt: '2026-06-02T10:01:00Z',
              status: { state: 'completed' },
              messages: [
                {
                  id: 'msg-1',
                  role: 'user',
                  text: 'Hello'
                },
                {
                  id: 'msg-2',
                  role: 'assistant',
                  text: 'Hi'
                }
              ]
            }
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await sendConversationTurn(
      fetcher,
      'ctx-1',
      'house',
      'Hello'
    );
    expect(result.conversation.id).toBe('ctx-1');
    expect(result.messages).toHaveLength(2);
    expect(result.messages[0].role).toBe('user');
    expect(result.messages[1].role).toBe('assistant');

    expect(fetcher).toHaveBeenCalledWith(
      expect.stringContaining('/api/v1/proxy/agents/house'),
      expect.objectContaining({
        method: 'POST',
        body: expect.stringContaining('"message/send"')
      })
    );
  });
});

describe('api conversation streaming', () => {
  it('streams turn events across chunk boundaries', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const accept = (
          options?.headers as Record<string, string> | undefined
        )?.['Accept'];
        if (accept === 'text/event-stream') {
          const body = JSON.parse(options?.body as string);
          const requestId = body.id;
          const stream = new ReadableStream<Uint8Array>({
            start(controller) {
              const encoder = new TextEncoder();
              controller.enqueue(
                encoder.encode(`data: {"jsonrpc":"2.0","id":${requestId},"result":{"kind":"message","role":"assistant","parts":[{"kind":"text","text":"Hel"}]}}

`)
              );
              controller.enqueue(
                encoder.encode(`data: {"jsonrpc":"2.0","id":${requestId},"result":{"kind":"message","role":"assistant","parts":[{"kind":"text","text":"lo"}]}}

`)
              );
              controller.close();
            }
          });
          return new Response(stream, {
            status: 200,
            headers: { 'Content-Type': 'text/event-stream' }
          });
        }
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              tasks: [
                {
                  id: 'task-1',
                  contextId: 'ctx-1',
                  text: 'Hello',
                  updatedAt: '2026-06-02T10:01:00Z',
                  status: { state: 'completed' }
                }
              ]
            }
          });
        }
        if (body.method === 'tasks/get') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              id: 'task-1',
              contextId: 'ctx-1',
              text: 'Hello',
              updatedAt: '2026-06-02T10:01:00Z',
              status: { state: 'completed' },
              messages: [
                {
                  id: 'msg-1',
                  role: 'user',
                  text: 'Hello'
                },
                {
                  id: 'msg-2',
                  role: 'assistant',
                  text: 'Hello'
                }
              ]
            }
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const events: unknown[] = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Hello',
      (event) => events.push(event)
    );

    expect(events[0]).toEqual({
      type: 'turn_started',
      conversationId: 'ctx-1',
      agentId: 'house'
    });
    expect(events[1]).toEqual({
      type: 'assistant_delta',
      conversationId: 'ctx-1',
      agentId: 'house',
      text: 'Hel',
      append: true
    });
    expect(events[2]).toEqual({
      type: 'assistant_delta',
      conversationId: 'ctx-1',
      agentId: 'house',
      text: 'lo',
      append: true
    });
    expect(events[3]).toEqual(
      expect.objectContaining({
        type: 'turn_completed'
      })
    );
  });
});

describe('fallback dashboard', () => {
  it('marks offline scaffolding as stale and agentless', () => {
    const fallback = fallbackDashboard();

    expect(fallback.connectionState).toBe('offline');
    expect(fallback.stale).toBe(true);
    expect(fallback.agents).toEqual([]);
    expect(fallback.layout.widgets.map((widget) => widget.kind)).toEqual([
      'date-time',
      'weather',
      'chat-history'
    ]);
    expect('weather' in fallback.home).toBe(false);
  });

  it('can create a neutral initial dashboard before client-side hub connection', () => {
    const initial = initialDashboard();

    expect(initial.connectionState).toBe('starting');
    expect(initial.issue).toBeUndefined();
  });
});
