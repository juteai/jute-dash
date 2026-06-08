import { describe, expect, it, vi } from 'vitest';
import {
  createConversation,
  getConversations,
  getConversation,
  sendConversationTurn,
  sendConversationTurnStream
} from './a2aConversation';

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

  it('filters out reasoning artifacts and populates artifact field for non-reasoning structured ones in getConversation', async () => {
    const taskData = {
      ...task('task-1', 'ctx-1', [message('ROLE_USER', 'Run task', 'msg-1')]),
      artifacts: [
        {
          artifactId: 'reasoning',
          name: 'Agent Thinking',
          description: 'internal thought process',
          parts: [{ text: 'I should call a tool...' }]
        },
        {
          artifactId: 'summary',
          name: 'Final Summary',
          parts: [{ text: 'Done summary text', mediaType: 'application/json' }]
        }
      ]
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [taskData] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: taskData
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-1', 'house');

    // The reasoning artifact should be filtered out
    const reasoningMessage = result.messages.find(
      (m) => m.a2aMessageId === 'reasoning'
    );
    expect(reasoningMessage).toBeUndefined();

    // The summary artifact should be present and have the artifact field set
    const summaryMessage = result.messages.find(
      (m) => m.a2aMessageId === 'summary'
    );
    expect(summaryMessage).toBeDefined();
    expect(summaryMessage?.artifact).toEqual({
      id: 'summary',
      title: 'Final Summary',
      content: 'Done summary text'
    });
  });

  it('filters out reasoning/tool artifacts containing only whitespace, thoughts, and function calls/responses in getConversation', async () => {
    const taskData = {
      ...task('task-1', 'ctx-1', [message('ROLE_USER', 'Run task', 'msg-1')]),
      artifacts: [
        {
          artifactId: '019ea64b-020c-7a12-b7ae-27d98b98910a',
          parts: [
            { text: '\n', metadata: { adk_thought: true } },
            { text: '\n\n' },
            {
              data: {
                id: 'call_cbc0c43a-a5cd-4b43-85d3-cdfdc6686740',
                name: 'jute_skill_read_context'
              },
              metadata: { adk_type: 'function_call' }
            }
          ]
        },
        {
          artifactId: '019ea64b-023a-70a0-b60f-19121f6c8467',
          parts: [
            {
              data: {
                id: 'call_cbc0c43a-a5cd-4b43-85d3-cdfdc6686740',
                name: 'jute_skill_read_context',
                response: { output: 'result' }
              },
              metadata: { adk_type: 'function_response' }
            }
          ]
        }
      ]
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [taskData] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: taskData
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-1', 'house');

    // Both reasoning/tool artifacts should be filtered out
    const art1 = result.messages.find(
      (m) => m.a2aMessageId === '019ea64b-020c-7a12-b7ae-27d98b98910a'
    );
    expect(art1).toBeUndefined();

    const art2 = result.messages.find(
      (m) => m.a2aMessageId === '019ea64b-023a-70a0-b60f-19121f6c8467'
    );
    expect(art2).toBeUndefined();
  });

  it('treats non-reasoning plain-text artifacts as normal replies without artifact field in getConversation', async () => {
    const taskData = {
      ...task('task-1', 'ctx-1', [message('ROLE_USER', 'Run task', 'msg-1')]),
      artifacts: [
        {
          artifactId: 'summary-text',
          name: 'Final Summary Text',
          parts: [{ text: 'Done summary text', mediaType: 'text/plain' }]
        }
      ]
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [taskData] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: taskData
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-1', 'house');

    const summaryMessage = result.messages.find(
      (m) => m.a2aMessageId === 'summary-text'
    );
    expect(summaryMessage).toBeDefined();
    expect(summaryMessage?.artifact).toBeUndefined();
    expect(summaryMessage?.content).toBe('Done summary text');
  });

  it('filters out parts with adk_thought true in metadata from artifacts in getConversation', async () => {
    const taskData = {
      ...task('task-1', 'ctx-1', [message('ROLE_USER', 'Run task', 'msg-1')]),
      artifacts: [
        {
          artifactId: 'summary',
          name: 'Final Summary',
          parts: [
            { text: 'Okay, thinking...', metadata: { adk_thought: true } },
            { text: 'Done summary text', mediaType: 'text/plain' }
          ]
        }
      ]
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [taskData] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: taskData
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-1', 'house');

    const summaryMessage = result.messages.find(
      (m) => m.a2aMessageId === 'summary'
    );
    expect(summaryMessage).toBeDefined();
    expect(summaryMessage?.artifact).toBeUndefined();
    expect(summaryMessage?.content).toBe('Done summary text');
    expect(summaryMessage?.interimSteps).toEqual([
      {
        id: 'task-1:thought:summary',
        text: 'Okay, thinking...',
        status: 'completed'
      }
    ]);
  });

  it('correctly filters out adk_thought parts from actual A2A assistant turn response payload', async () => {
    const rawTurnResponse = {
      id: '019ea32d-0aa9-7111-9754-e9d9f99f689a',
      artifacts: [
        {
          artifactId: '019ea32d-0f51-7e19-a581-fbd1cf3fef9d',
          metadata: {
            adk_app_name: 'kronk_a2a_assistant',
            adk_author: 'kronk_a2a_assistant',
            adk_custom_metadata: { kronk_model_id: 'Qwen3-0.6B-Q8_0' },
            adk_invocation_id: 'e-bb38bd7a-bcef-4150-aba7-0b57d002a0e7',
            adk_session_id: 'ctx-2m8ktd',
            adk_usage_metadata: {
              candidatesTokenCount: 11,
              promptTokenCount: 975,
              totalTokenCount: 1076
            },
            adk_user_id: 'jute-local-dev'
          },
          parts: [
            {
              text: 'Okay, the user just sent "test". I need to respond correctly. Since the user is testing, maybe they want to see if I can handle it. But according to the instructions, I should return the final answer without using tools or analysis. The user might be testing my ability or providing a prompt. I should acknowledge their message and confirm that I\'m here to help. No tool calls are needed here. Just a simple response.\n',
              metadata: { adk_thought: true }
            },
            { text: 'Hello! How can I assist you today?' }
          ]
        }
      ],
      contextId: 'ctx-2m8ktd',
      history: [
        {
          messageId: '69006a71-dc13-4a86-9a30-fe943fcb2582',
          contextId: 'ctx-2m8ktd',
          parts: [{ text: 'test', mediaType: 'text/plain' }],
          role: 'ROLE_USER'
        }
      ],
      metadata: {
        adk_app_name: 'kronk_a2a_assistant',
        adk_session_id: 'ctx-2m8ktd',
        adk_user_id: 'jute-local-dev'
      },
      status: {
        state: 'TASK_STATE_COMPLETED',
        timestamp: '2026-06-07T18:41:39.887174+01:00'
      }
    };

    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [rawTurnResponse] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: rawTurnResponse
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-2m8ktd', 'house');

    // Expected 2 messages: user turn ("test"), and assistant turn ("Hello! How can I assist you today?")
    expect(result.messages).toHaveLength(2);
    expect(result.messages[0].role).toBe('user');
    expect(result.messages[0].content).toBe('test');
    expect(result.messages[1].role).toBe('assistant');
    expect(result.messages[1].content).toBe(
      'Hello! How can I assist you today?'
    );
    expect(result.messages[1].artifact).toBeUndefined();
    expect(result.messages[1].interimSteps).toBeDefined();
    expect(result.messages[1].interimSteps?.[0].text).toContain(
      'Okay, the user just sent "test". I need to respond correctly.'
    );
    expect(result.messages[1].interimSteps?.[0].status).toBe('completed');
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
    expect(events).toContainEqual(
      expect.objectContaining({
        type: 'artifact_update',
        conversationId: 'ctx-1',
        agentId: 'house',
        taskId: 'task-1',
        artifactId: 'artifact-1',
        text: 'Result',
        append: true
      })
    );
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

  it('filters out streamed artifact updates that represent reasoning artifacts', async () => {
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
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'reasoning',
                  name: 'Thinking',
                  parts: [{ text: 'I should think...' }]
                },
                append: true,
                lastChunk: false
              }
            },
            {
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'actual-result',
                  name: 'Result',
                  parts: [{ text: 'Here is the answer' }]
                },
                append: true,
                lastChunk: false
              }
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
    const events: Array<{
      type: string;
      artifactId?: string;
      text?: string;
      isReasoning?: boolean;
    }> = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Find it',
      (event) => events.push(event)
    );

    // The 'reasoning' artifact update should be emitted with isReasoning: true
    const reasoningEvent = events.find((e) => e.artifactId === 'reasoning');
    expect(reasoningEvent).toBeDefined();
    expect(reasoningEvent?.isReasoning).toBe(true);
    expect(reasoningEvent?.text).toBe('I should think...');

    // The 'actual-result' artifact update SHOULD be emitted
    const resultEvent = events.find((e) => e.artifactId === 'actual-result');
    expect(resultEvent).toBeDefined();
    expect(resultEvent?.text).toBe('Here is the answer');
  });

  it('splits streamed artifact updates that contain parts with adk_thought true into reasoning and non-reasoning events', async () => {
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
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'mixed-result',
                  name: 'Result',
                  parts: [
                    {
                      text: 'I am thinking...',
                      metadata: { adk_thought: true }
                    },
                    { text: 'Here is the answer' }
                  ]
                },
                append: true,
                lastChunk: false
              }
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
    const events: Array<{
      type: string;
      artifactId?: string;
      text?: string;
      isReasoning?: boolean;
    }> = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Find it',
      (event) => events.push(event)
    );

    const resultEvent = events.find((e) => e.artifactId === 'mixed-result');
    expect(resultEvent).toBeDefined();
    expect(resultEvent?.text).toBe('Here is the answer');

    const thoughtEvent = events.find(
      (e) => e.artifactId === 'mixed-result-thought'
    );
    expect(thoughtEvent).toBeDefined();
    expect(thoughtEvent?.text).toBe('I am thinking...');
    expect(thoughtEvent?.isReasoning).toBe(true);
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

  it('detects tool calls in getConversation and parses them as interim steps', async () => {
    const taskData = {
      ...task('task-1', 'ctx-1', [
        message('ROLE_USER', 'Weather info', 'msg-1')
      ]),
      artifacts: [
        {
          artifactId: 'tool-use',
          name: 'Tool Call',
          parts: [
            {
              text: 'Okay, checking weather...',
              metadata: { adk_thought: true }
            },
            {
              text: '<tool_call>\n{"name": "jute_skill_read_context", "arguments": {"skillId": "jute.weather.current"}}\n</tool_call>'
            }
          ]
        }
      ]
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [taskData] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: taskData
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-1', 'house');
    const msg = result.messages.find(
      (m) => m.a2aTaskId === 'task-1' && m.role === 'assistant'
    );
    expect(msg).toBeDefined();
    expect(msg?.content).toBe('');
    expect(msg?.interimSteps).toEqual([
      {
        id: 'task-1:thought:tool-use',
        text: 'Okay, checking weather...',
        status: 'completed'
      },
      {
        id: 'task-1:tool:tool-use:1',
        text: 'Called tool: jute_skill_read_context',
        status: 'completed'
      }
    ]);
  });

  it('detects both thought processes and tool calls in getConversation', async () => {
    const taskData = {
      id: 'task-123',
      contextId: 'ctx-123',
      status: {
        state: 'TASK_STATE_COMPLETED',
        timestamp: '2026-06-08T09:50:22.073488+01:00'
      },
      artifacts: [
        {
          artifactId: '019ea66c-b874-7753-b56e-3168d15403da',
          metadata: {
            adk_app_name: 'kronk_a2a_assistant',
            adk_author: 'kronk_a2a_assistant'
          },
          parts: [
            {
              text: 'Okay, the user is asking about the weather...',
              metadata: { adk_thought: true }
            },
            {
              text: '\n\n'
            },
            {
              data: {
                args: { skillId: 'jute.weather.current' },
                id: 'call_6f7b97fb-7dfa-4b79-b584-3d141f63f119',
                name: 'jute_skill_read_context'
              },
              metadata: { adk_type: 'function_call' }
            }
          ]
        },
        {
          artifactId: '019ea66c-c819-7bf2-890d-bd24388bec4c',
          parts: [
            {
              data: {
                id: 'call_6f7b97fb-7dfa-4b79-b584-3d141f63f119',
                name: 'jute_skill_read_context',
                response: { output: 'Weather is nice' }
              },
              metadata: { adk_type: 'function_response' }
            }
          ]
        },
        {
          artifactId: '019ea66c-f82c-7c7f-aa86-f15b1f63d8d6',
          parts: [
            {
              text: 'Okay, user asked about weather...',
              metadata: { adk_thought: true }
            },
            {
              text: 'The weather is nice.'
            }
          ]
        }
      ],
      history: [
        {
          messageId: 'msg-user',
          role: 1, // USER
          parts: [
            {
              text: 'What is the weather?'
            }
          ]
        },
        {
          messageId: 'msg-agent',
          role: 2, // AGENT
          parts: [
            {
              text: 'Okay, user asked about weather...',
              metadata: { adk_thought: true }
            },
            {
              text: 'The weather is nice.'
            }
          ]
        }
      ]
    };

    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, options) => {
        const body = JSON.parse(options?.body as string);
        if (body.method === 'ListTasks') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: { tasks: [taskData] }
          });
        }
        if (body.method === 'GetTask') {
          return jsonResponse({
            jsonrpc: '2.0',
            id: body.id,
            result: taskData
          });
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });

    const result = await getConversation(fetcher, 'ctx-123', 'house');
    const msg = result.messages.find((m) => m.role === 'assistant');
    expect(msg?.interimSteps).toEqual([
      {
        id: 'task-123:thought:019ea66c-b874-7753-b56e-3168d15403da',
        text: 'Okay, the user is asking about the weather...\n\n',
        status: 'completed'
      },
      {
        id: 'call_6f7b97fb-7dfa-4b79-b584-3d141f63f119',
        text: 'Called tool: jute_skill_read_context',
        status: 'completed',
        args: { skillId: 'jute.weather.current' },
        output: 'Weather is nice'
      },
      {
        id: 'task-123:thought:019ea66c-f82c-7c7f-aa86-f15b1f63d8d6',
        text: 'Okay, user asked about weather...',
        status: 'completed'
      },
      {
        id: 'msg-agent:thought:0',
        text: 'Okay, user asked about weather...',
        status: 'completed'
      }
    ]);
  });

  it('detects streamed tool calls and emits status_changed events', async () => {
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
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'tool-use-stream',
                  parts: [
                    {
                      text: '<tool_call>\n{"name": "jute_skill_read_context"}\n</tool_call>'
                    }
                  ]
                },
                append: true,
                lastChunk: false
              }
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
    const events: Array<{ type: string; status?: string; text?: string }> = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Check context',
      (event) => events.push(event)
    );

    const toolCallEvent = events.find(
      (e) =>
        e.type === 'status_changed' &&
        e.text?.includes('jute_skill_read_context')
    );
    expect(toolCallEvent).toBeDefined();
    expect(toolCallEvent?.text).toBe('Calling tool: jute_skill_read_context');
    expect(toolCallEvent?.status).toBe('working');
  });

  it('extracts args and response output from streamed function call and function response', async () => {
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
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'tool-use-stream',
                  parts: [
                    {
                      data: {
                        id: 'call-1',
                        name: 'jute_skill_list',
                        args: { filter: 'active' }
                      },
                      metadata: { adk_type: 'function_call' }
                    }
                  ]
                },
                append: true,
                lastChunk: false
              }
            },
            {
              artifactUpdate: {
                taskId: 'task-1',
                contextId: 'ctx-1',
                artifact: {
                  artifactId: 'tool-use-stream',
                  parts: [
                    {
                      data: {
                        id: 'call-1',
                        name: 'jute_skill_list',
                        response: { output: 'success output' }
                      },
                      metadata: { adk_type: 'function_response' }
                    }
                  ]
                },
                append: true,
                lastChunk: false
              }
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
    const events: Array<{ type: string; status?: string; text?: string; args?: any; output?: any }> = [];

    await sendConversationTurnStream(
      fetcher,
      'ctx-1',
      'house',
      'Check skills',
      (event) => events.push(event)
    );

    const callEvent = events.find((e) => e.type === 'status_changed' && e.status === 'working');
    expect(callEvent).toBeDefined();
    expect(callEvent?.text).toBe('Calling tool: jute_skill_list');
    expect(callEvent?.args).toEqual({ filter: 'active' });

    const responseEvent = events.find((e) => e.type === 'status_changed' && e.status === 'completed');
    expect(responseEvent).toBeDefined();
    expect(responseEvent?.text).toBe('Called tool: jute_skill_list');
    expect(responseEvent?.output).toBe('success output');
  });
});
