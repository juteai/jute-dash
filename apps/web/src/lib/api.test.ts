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

  it('does not hide temporary history failures as unsupported capability', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        jsonResponse({ error: 'agent proxy unavailable' }, { status: 502 })
      );

    await expect(getConversations(fetcher, 'house')).rejects.toThrow(
      /Status: 502/
    );
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

  it('uses a direct blocking message without requiring task history', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'SendMessage') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: {
              message: message('ROLE_AGENT', 'Direct response', 'msg-2')
            }
          });
        }
        return jsonResponse(
          {
            jsonrpc: '2.0',
            id: body.id,
            error: { code: -32601, message: 'Method not found' }
          },
          { status: 501 }
        );
      });

    const result = await sendConversationTurn(
      fetcher,
      'ctx-1',
      'house',
      'Hello'
    );

    expect(result.conversation).toEqual(
      expect.objectContaining({
        id: 'ctx-1',
        status: 'completed',
        historyUnsupported: true
      })
    );
    expect(result.messages).toEqual([
      expect.objectContaining({ role: 'user', content: 'Hello' }),
      expect.objectContaining({
        role: 'assistant',
        content: 'Direct response'
      })
    ]);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('rejects a terminal failed task without loading history', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        return jsonResponse({
          jsonrpc: '2.0',
          id: body.id,
          result: {
            task: {
              ...task('task-1', 'ctx-1'),
              status: {
                state: 'TASK_STATE_FAILED',
                timestamp: '2026-06-02T10:01:00Z'
              }
            }
          }
        });
      });

    await expect(
      sendConversationTurn(fetcher, 'ctx-1', 'house', 'Hello')
    ).rejects.toThrow('Agent task failed');
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('renders task artifact text in blocking conversation results', async () => {
    const artifactTask = {
      ...task('task-1', 'ctx-1', [
        message('ROLE_USER', 'Summarize it', 'msg-1')
      ]),
      artifacts: [
        {
          artifactId: 'artifact-1',
          name: 'Summary',
          parts: [{ text: 'The summary result' }]
        }
      ]
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'SendMessage') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { task: artifactTask }
          });
        }
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [artifactTask] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: artifactTask
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await sendConversationTurn(
      fetcher,
      'ctx-1',
      'house',
      'Summarize it'
    );

    expect(result.messages).toContainEqual(
      expect.objectContaining({
        role: 'assistant',
        content: 'The summary result',
        a2aTaskId: 'task-1'
      })
    );
  });

  it('passes cancellation through a blocking turn', async () => {
    const controller = new AbortController();
    controller.abort();
    const fetcher = vi.fn<typeof fetch>().mockImplementation(
      async (url, options) =>
        new Promise<Response>((resolve, reject) => {
          if (options?.signal?.aborted) {
            reject(new DOMException('Aborted', 'AbortError'));
            return;
          }
          reject(new Error('request was not canceled'));
        })
    );

    await expect(
      sendConversationTurn(fetcher, 'ctx-1', 'house', 'Hello', {
        signal: controller.signal
      })
    ).rejects.toMatchObject({ name: 'AbortError' });
    expect(fetcher).toHaveBeenCalledTimes(1);
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

  it('stops after an agent reports a terminal failure', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
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
                        state: 'TASK_STATE_FAILED',
                        message: message(
                          'ROLE_AGENT',
                          'Unable to complete the request',
                          'status-1'
                        )
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
      });
    const events: unknown[] = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Hello',
      (event) => events.push(event)
    );

    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'turn_failed',
        conversationId: 'ctx-1',
        agentId: 'house',
        message: 'Agent task failed'
      })
    );
    expect(events).not.toContainEqual(
      expect.objectContaining({ type: 'turn_completed' })
    );
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('aborts an in-flight stream without reporting a failure', async () => {
    const controller = new AbortController();
    const fetcher = vi.fn<typeof fetch>().mockImplementation(
      async (url, options) =>
        new Promise<Response>((resolve, reject) => {
          const signal = options?.signal;
          if (!signal) {
            reject(new Error('missing abort signal'));
            return;
          }
          if (signal.aborted) {
            reject(new DOMException('Aborted', 'AbortError'));
            return;
          }
          signal.addEventListener(
            'abort',
            () => reject(new DOMException('Aborted', 'AbortError')),
            { once: true }
          );
        })
    );
    const events: unknown[] = [];

    const turn = sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Hello',
      (event) => events.push(event),
      { signal: controller.signal }
    );
    controller.abort();
    await turn;

    expect(events).toContainEqual({
      type: 'turn_canceled',
      conversationId: 'ctx-1',
      agentId: 'house'
    });
    expect(events).not.toContainEqual(
      expect.objectContaining({ type: 'turn_failed' })
    );
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('completes from streamed text when task history is unsupported', async () => {
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
                      message: message('ROLE_AGENT', 'Hello', 'msg-2')
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
        return jsonResponse(
          {
            jsonrpc: '2.0',
            id: body.id,
            error: { code: -32601, message: 'Method not found' }
          },
          { status: 501 }
        );
      });
    const events: unknown[] = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Hello?',
      (event) => events.push(event)
    );

    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'turn_completed',
        conversation: expect.objectContaining({
          id: 'ctx-1',
          historyUnsupported: true
        }),
        messages: [
          expect.objectContaining({ role: 'user', content: 'Hello?' }),
          expect.objectContaining({ role: 'assistant', content: 'Hello' })
        ]
      })
    );
    expect(events).not.toContainEqual(
      expect.objectContaining({ type: 'turn_failed' })
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
