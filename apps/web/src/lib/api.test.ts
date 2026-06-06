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

function message(
  role: 'ROLE_USER' | 'ROLE_AGENT',
  text: string,
  messageId: string
) {
  return {
    messageId,
    role,
    parts: [{ text }]
  };
}

function task(
  id: string,
  contextId: string,
  history: ReturnType<typeof message>[] = []
) {
  return {
    id,
    contextId,
    status: {
      state: 'TASK_STATE_COMPLETED',
      timestamp: '2026-06-02T10:01:00Z'
    },
    artifacts: [],
    history
  };
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
                task('task-1', 'ctx-1', [
                  message('ROLE_USER', 'Hello', 'msg-1')
                ])
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
    expect(fetcher.mock.calls[0][1]).toEqual(
      expect.objectContaining({
        headers: expect.objectContaining({
          'A2A-Version': '1.0'
        }),
        body: expect.stringContaining('"ListTasks"')
      })
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
        if (body.method === 'SendMessage') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              task: task('task-1', 'ctx-1')
            }
          });
        }
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              tasks: [task('task-1', 'ctx-1')]
            }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: task('task-1', 'ctx-1', [
              message('ROLE_USER', 'Hello', 'msg-1'),
              message('ROLE_AGENT', 'Hi', 'msg-2')
            ])
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
        headers: expect.objectContaining({
          'A2A-Version': '1.0'
        }),
        body: expect.stringContaining('"SendMessage"')
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
                encoder.encode(`data: {"jsonrpc":"2.0","id":${requestId},"result":{"message":{"messageId":"msg-1","role":"ROLE_AGENT","parts":[{"text":"Hel"}]}}}

`)
              );
              controller.enqueue(
                encoder.encode(`data: {"jsonrpc":"2.0","id":${requestId},"result":{"message":{"messageId":"msg-2","role":"ROLE_AGENT","parts":[{"text":"lo"}]}}}

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
              tasks: [task('task-1', 'ctx-1')]
            }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: task('task-1', 'ctx-1', [
              message('ROLE_USER', 'Hello', 'msg-1'),
              message('ROLE_AGENT', 'Hello', 'msg-2')
            ])
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

  it('maps status, artifact, and task stream payloads to display events', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        const accept = (
          options?.headers as Record<string, string> | undefined
        )?.['Accept'];
        if (accept === 'text/event-stream') {
          const events = [
            {
              statusUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                status: {
                  state: 'TASK_STATE_WORKING',
                  message: message('ROLE_AGENT', 'Searching', 'status-1')
                }
              }
            },
            {
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'artifact-1',
                  parts: [{ text: 'Result' }]
                },
                append: true,
                lastChunk: false
              }
            },
            {
              task: task('task-1', 'ctx-1', [
                message('ROLE_AGENT', 'Finished', 'task-message-1')
              ])
            }
          ];
          const stream = new ReadableStream<Uint8Array>({
            start(controller) {
              const encoder = new TextEncoder();
              for (const result of events) {
                controller.enqueue(
                  encoder.encode(
                    `data: ${JSON.stringify({
                      jsonrpc: '2.0',
                      id: body.id,
                      result
                    })}\n\n`
                  )
                );
              }
              controller.close();
            }
          });
          return new Response(stream, {
            status: 200,
            headers: { 'Content-Type': 'text/event-stream' }
          });
        }
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [task('task-1', 'ctx-1')] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: task('task-1', 'ctx-1')
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });
    const events: unknown[] = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Find it',
      (event) => events.push(event)
    );

    expect(events).toContainEqual({
      type: 'status_changed',
      conversationId: 'ctx-1',
      agentId: 'house',
      taskId: 'task-1',
      status: 'working',
      text: 'Searching',
      terminal: false
    });
    expect(events).toContainEqual({
      type: 'assistant_delta',
      conversationId: 'ctx-1',
      agentId: 'house',
      text: 'Result',
      append: true
    });
    expect(events).toContainEqual({
      type: 'status_changed',
      conversationId: 'ctx-1',
      agentId: 'house',
      taskId: 'task-1',
      status: 'completed',
      text: '',
      terminal: true
    });
  });

  it('emits a failed turn without loading history when streaming fails', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        jsonResponse({ error: 'upstream unavailable' }, { status: 502 })
      );
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
    expect(events[1]).toEqual(
      expect.objectContaining({
        type: 'turn_failed',
        conversationId: 'ctx-1',
        agentId: 'house'
      })
    );
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('marks terminal A2A status updates as terminal display events', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        const accept = (
          options?.headers as Record<string, string> | undefined
        )?.['Accept'];
        if (accept === 'text/event-stream') {
          const stream = new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(
                new TextEncoder().encode(
                  `data: ${JSON.stringify({
                    jsonrpc: '2.0',
                    id: body.id,
                    result: {
                      statusUpdate: {
                        taskId: 'task-1',
                        contextId: 'ctx-1',
                        status: {
                          state: 'TASK_STATE_COMPLETED',
                          message: message('ROLE_AGENT', 'Complete', 'status-1')
                        }
                      }
                    }
                  })}\n\n`
                )
              );
              controller.close();
            }
          });
          return new Response(stream, {
            status: 200,
            headers: { 'Content-Type': 'text/event-stream' }
          });
        }
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [task('task-1', 'ctx-1')] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: task('task-1', 'ctx-1')
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

    expect(events).toContainEqual({
      type: 'status_changed',
      conversationId: 'ctx-1',
      agentId: 'house',
      taskId: 'task-1',
      status: 'completed',
      text: 'Complete',
      terminal: true
    });
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
