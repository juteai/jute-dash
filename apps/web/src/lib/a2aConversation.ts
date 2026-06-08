import {
  Client,
  JsonRpcTransportFactory,
  type Transport
} from '@a2a-js/sdk/client';
import {
  TaskState,
  type AgentCard,
  type Part as A2APart,
  type Task as A2ATask
} from '@a2a-js/sdk';
import type {
  Conversation,
  ConversationDetail,
  ConversationStreamEvent
} from '$lib/types';
import { API_BASE } from '$lib/hubClient';
import {
  getPartData,
  getPartText,
  isReasoningArtifact,
  isStructuredArtifact,
  looksLikeToolInvocation,
  sanitizeDisplayText,
  textFromParts,
  textFromReasoningParts
} from '$lib/displaySanitizer';
import {
  statusFromTask,
  statusFromState,
  isTerminalTaskState,
  terminalTaskFailureMessage,
  parseTasksToConversationDetail,
  newUserMessage,
  localConversationDetail
} from '$lib/a2aParser';

type TurnRequestOptions = {
  signal?: AbortSignal;
};

type PartWithData = A2APart & {
  data?: {
    id?: string;
    name?: string;
    response?: {
      output?: unknown;
    };
  };
};

const proxyAgentCard = {
  capabilities: { streaming: true }
} as AgentCard;

async function getTransport(
  fetcher: typeof fetch,
  agentId: string
): Promise<Transport> {
  return new JsonRpcTransportFactory({ fetchImpl: fetcher }).create(
    `${API_BASE}/api/v1/proxy/agents/${encodeURIComponent(agentId)}`,
    proxyAgentCard
  );
}

async function getClient(fetcher: typeof fetch, agentId: string) {
  const transport = await getTransport(fetcher, agentId);
  return new Client(transport, proxyAgentCard);
}

function isHistoryUnsupportedError(error: unknown): boolean {
  if (!(error instanceof Error)) {
    return false;
  }
  const message = error.message.toLowerCase();
  return (
    message.includes('status: 501') ||
    message.includes('code: -32601') ||
    message.includes('method not found')
  );
}

function isAbortError(error: unknown): boolean {
  return (
    (error instanceof DOMException && error.name === 'AbortError') ||
    (error instanceof Error && error.name === 'AbortError')
  );
}

export async function getConversations(
  fetcher: typeof fetch,
  agentId: string
): Promise<Conversation[]> {
  if (!agentId) {
    return [];
  }
  try {
    const client = await getClient(fetcher, agentId);
    const result = await client.listTasks({
      tenant: '',
      contextId: '',
      status: TaskState.TASK_STATE_UNSPECIFIED,
      pageSize: 50,
      pageToken: '',
      statusTimestampAfter: undefined
    });
    const tasks = result.tasks;

    const byContext: Record<string, Conversation> = {};
    for (const task of tasks) {
      const contextId = task.contextId || task.id;
      if (!contextId) continue;

      let title = '';
      const history = task.history || (task as any).messages;
      if (history && history.length > 0) {
        const firstUser = history.find(
          (message: any) => message.role === 1 || message.role === 'user'
        );
        if (firstUser && typeof firstUser.role === 'number') {
          title = textFromParts(firstUser.parts);
        } else if (firstUser) {
          title = (firstUser as any).text ?? '';
        }
      }
      if (!title) {
        title = (task as any).text || 'Conversation';
      }

      const updatedAt =
        task.status?.timestamp ||
        (task as any).updatedAt ||
        new Date().toISOString();
      const conversation: Conversation = byContext[contextId] || {
        id: contextId,
        agentId,
        title,
        status: statusFromTask(task),
        a2aContextId: contextId,
        latestTaskId: task.id || '',
        createdAt: updatedAt,
        updatedAt: updatedAt
      };

      if (updatedAt && updatedAt >= conversation.updatedAt) {
        conversation.updatedAt = updatedAt;
        conversation.latestTaskId = task.id || '';
        conversation.status = statusFromTask(task);
      }
      byContext[contextId] = conversation;
    }

    return Object.values(byContext);
  } catch (err) {
    if (!isHistoryUnsupportedError(err)) {
      throw err;
    }
    return [
      {
        id: `history-unsupported-${agentId}`,
        agentId,
        title: 'History unavailable',
        status: 'unavailable',
        a2aContextId: '',
        latestTaskId: '',
        createdAt: '',
        updatedAt: '',
        historyUnsupported: true
      }
    ];
  }
}

export async function createConversation(
  fetcher: typeof fetch,
  agentId: string,
  title?: string
): Promise<ConversationDetail> {
  const contextId = 'ctx-' + Math.random().toString(36).substring(7);
  const now = new Date().toISOString();
  return {
    conversation: {
      id: contextId,
      agentId,
      title: title || 'New Conversation',
      status: 'idle',
      a2aContextId: contextId,
      latestTaskId: '',
      createdAt: now,
      updatedAt: now
    },
    messages: []
  };
}

export async function getConversation(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  options: TurnRequestOptions = {}
): Promise<ConversationDetail> {
  const client = await getClient(fetcher, agentId);
  const result = await client.listTasks(
    {
      tenant: '',
      contextId: conversationId,
      status: TaskState.TASK_STATE_UNSPECIFIED,
      pageSize: 50,
      pageToken: '',
      statusTimestampAfter: undefined
    },
    { signal: options.signal }
  );
  const tasks = result.tasks;
  const fullTasks: A2ATask[] = [];

  for (const task of tasks) {
    let record = task;
    if (task.id) {
      try {
        const fullTask = await client.getTask(
          {
            tenant: '',
            id: task.id,
            historyLength: 50
          },
          { signal: options.signal }
        );
        if (fullTask) {
          record = fullTask;
        }
      } catch (e) {
        if (isAbortError(e)) {
          throw e;
        }
        console.warn('Failed to get full task details for', task.id, e);
      }
    }
    fullTasks.push(record);
  }

  return parseTasksToConversationDetail(fullTasks, conversationId, agentId);
}

export async function* executeConversationTurn(
  fetcher: typeof fetch,
  conversationId: string,
  agent: { id: string; streaming?: boolean },
  text: string,
  options: TurnRequestOptions = {}
): AsyncGenerator<ConversationStreamEvent> {
  const agentId = agent.id;
  let assistantText = '';
  let latestTaskId = '';
  let latestStatus = 'completed';

  yield {
    type: 'turn_started',
    conversationId,
    agentId
  };

  try {
    const client = await getClient(fetcher, agentId);

    if (agent.streaming) {
      const emittedLengths = new Map<string, number>();
      const accumulatedRawTexts = new Map<string, string>();

      for await (const event of client.sendMessageStream(
        {
          tenant: '',
          message: newUserMessage(conversationId, text),
          configuration: undefined,
          metadata: undefined
        },
        { signal: options.signal }
      )) {
        const payload = event.payload;
        if (payload?.$case === 'message') {
          const content = textFromParts(payload.value.parts);
          if (content) {
            assistantText += content;
            yield {
              type: 'assistant_delta',
              conversationId,
              agentId,
              text: content,
              append: true
            };
          }
        } else if (payload?.$case === 'statusUpdate') {
          const statusText = textFromParts(
            payload.value.status?.message?.parts
          );
          const status = payload.value.status
            ? statusFromState(payload.value.status.state)
            : 'working';
          latestTaskId = payload.value.taskId;
          latestStatus = status;
          yield {
            type: 'status_changed',
            conversationId,
            agentId,
            taskId: payload.value.taskId,
            status,
            text: statusText,
            terminal: payload.value.status
              ? isTerminalTaskState(payload.value.status.state)
              : false
          };
          const failureMessage = terminalTaskFailureMessage(status);
          if (failureMessage) {
            yield {
              type: 'turn_failed',
              conversationId,
              agentId,
              message: failureMessage
            };
            return;
          }
        } else if (payload?.$case === 'artifactUpdate') {
          const artifact = payload.value.artifact;
          if (artifact) {
            latestTaskId = payload.value.taskId;
            const isReasoningArt = isReasoningArtifact(artifact);
            const artifactID = artifact.artifactId || 'streamed-artifact';
            const parts = artifact.parts ?? [];

            // 1. Process reasoning parts together
            const reasoningParts = parts.filter((part) => {
              if (part.metadata?.adk_thought === true) return true;
              if (isReasoningArt) {
                const pt = getPartText(part);
                const isToolText =
                  looksLikeToolInvocation(pt) ||
                  pt.includes('<tool_call>') ||
                  pt.includes('<tool_response>');
                return !isToolText;
              }
              return false;
            });

            if (reasoningParts.length > 0) {
              const combinedText = textFromReasoningParts(reasoningParts);
              if (combinedText) {
                const key = `${artifactID}:reasoning`;
                const prevRaw = accumulatedRawTexts.get(key) || '';
                const accumulatedRaw = payload.value.append
                  ? prevRaw + combinedText
                  : combinedText;
                accumulatedRawTexts.set(key, accumulatedRaw);

                if (accumulatedRaw) {
                  const prevLen = emittedLengths.get(key) || 0;
                  if (accumulatedRaw.length > prevLen) {
                    const delta = accumulatedRaw.slice(prevLen);
                    yield {
                      type: 'artifact_update',
                      conversationId,
                      agentId,
                      taskId: latestTaskId,
                      artifactId: isReasoningArt
                        ? artifactID
                        : `${artifactID}-thought`,
                      name: isReasoningArt
                        ? artifact.name
                        : `${artifact.name || artifactID} (Thinking)`,
                      text: delta,
                      append: prevLen > 0 || payload.value.append,
                      isStructured: false,
                      isReasoning: true
                    };
                    emittedLengths.set(key, accumulatedRaw.length);
                  }
                }
              }
            }

            // 2. Process non-reasoning parts in loop
            for (const [pIdx, part] of parts.entries()) {
              const data = getPartData(part) as PartWithData['data'];
              const isFunctionCall =
                part.metadata?.adk_type === 'function_call' ||
                (data && !data.response && (data.name || data.id));
              const isFunctionResponse =
                part.metadata?.adk_type === 'function_response' ||
                (data && data.response);

              if (isFunctionCall) {
                const toolName = data?.name || 'agent tool';
                const toolCallId = data?.id || `${latestTaskId}:tool:${pIdx}`;
                const key = `${artifactID}:${pIdx}:func_call`;
                if (!emittedLengths.has(key)) {
                  yield {
                    type: 'status_changed',
                    conversationId,
                    agentId,
                    taskId: toolCallId,
                    status: 'working',
                    text: `Calling tool: ${toolName}`,
                    args: (data as any)?.args
                  };
                  emittedLengths.set(key, 1);
                }
                continue;
              }

              if (isFunctionResponse) {
                const toolName = data?.name || 'agent tool';
                const toolCallId = data?.id || `${latestTaskId}:tool:${pIdx}`;
                const key = `${artifactID}:${pIdx}:func_response`;
                if (!emittedLengths.has(key)) {
                  yield {
                    type: 'status_changed',
                    conversationId,
                    agentId,
                    taskId: toolCallId,
                    status: 'completed',
                    text: `Called tool: ${toolName}`,
                    output: (data as any)?.response?.output
                  };
                  emittedLengths.set(key, 1);
                }
                continue;
              }

              const partText = getPartText(part);
              const isToolText =
                looksLikeToolInvocation(partText) ||
                partText.includes('<tool_call>') ||
                partText.includes('<tool_response>');
              const isPartReasoning =
                part.metadata?.adk_thought === true ||
                (isReasoningArt && !isToolText);
              if (isPartReasoning) {
                continue;
              }

              if (isToolText) {
                const key = `${artifactID}:${pIdx}:tool_call`;
                if (!emittedLengths.has(key)) {
                  const nameMatch =
                    partText.match(/"name"\s*:\s*"([^"]+)"/) ||
                    partText.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
                  const toolName = nameMatch ? nameMatch[1] : 'agent tool';
                  let args: any = undefined;
                  try {
                    const jsonStart = partText.indexOf('{');
                    if (jsonStart > -1) {
                      args = JSON.parse(partText.slice(jsonStart));
                    }
                  } catch {
                    // ignore
                  }
                  yield {
                    type: 'status_changed',
                    conversationId,
                    agentId,
                    taskId: `${latestTaskId}:tool_call:${pIdx}`,
                    status: 'working',
                    text: `Calling tool: ${toolName}`,
                    args
                  };
                  emittedLengths.set(key, partText.length);
                }
              } else {
                const key = `${artifactID}:${pIdx}:content`;
                const prevRaw = accumulatedRawTexts.get(key) || '';
                const accumulatedRaw = payload.value.append
                  ? prevRaw + partText
                  : partText;
                accumulatedRawTexts.set(key, accumulatedRaw);

                if (accumulatedRaw) {
                  const cleanText = sanitizeDisplayText(accumulatedRaw);
                  const prevCleanLen = emittedLengths.get(key) || 0;
                  if (cleanText.length > prevCleanLen) {
                    const delta = cleanText.slice(prevCleanLen);
                    const isStructured = isStructuredArtifact([part]);
                    yield {
                      type: 'artifact_update',
                      conversationId,
                      agentId,
                      taskId: latestTaskId,
                      artifactId: artifactID,
                      name: artifact.name,
                      text: delta,
                      append: prevCleanLen > 0 || payload.value.append,
                      isStructured,
                      isReasoning: false
                    };
                    emittedLengths.set(key, cleanText.length);
                  }
                }
              }
            }
          }
        } else if (payload?.$case === 'task') {
          const statusText = textFromParts(
            payload.value.status?.message?.parts
          );
          const status = statusFromTask(payload.value);
          latestTaskId = payload.value.id;
          latestStatus = status;
          yield {
            type: 'status_changed',
            conversationId,
            agentId,
            taskId: payload.value.id,
            status,
            text: statusText,
            terminal: payload.value.status
              ? isTerminalTaskState(payload.value.status.state)
              : false
          };
          const failureMessage = terminalTaskFailureMessage(status);
          if (failureMessage) {
            yield {
              type: 'turn_failed',
              conversationId,
              agentId,
              message: failureMessage
            };
            return;
          }
        }
      }
    } else {
      // Non-streaming path
      const result = await client.sendMessage(
        {
          tenant: '',
          message: newUserMessage(conversationId, text),
          configuration: undefined,
          metadata: undefined
        },
        { signal: options.signal }
      );
      if ('messageId' in result) {
        assistantText = textFromParts(result.parts);
        if (!assistantText.trim()) {
          throw new Error('Agent response contained no displayable text');
        }
        yield {
          type: 'assistant_delta',
          conversationId,
          agentId,
          text: assistantText,
          append: false
        };
        yield {
          type: 'turn_completed',
          ...localConversationDetail(
            conversationId,
            agentId,
            text,
            assistantText,
            result.taskId || '',
            'completed'
          )
        };
        return;
      } else {
        const status = statusFromTask(result);
        const failureMessage = terminalTaskFailureMessage(status);
        if (failureMessage) {
          throw new Error(failureMessage);
        }
      }
    }

    let detail: ConversationDetail;
    try {
      detail = await getConversation(fetcher, conversationId, agentId, options);
    } catch (err) {
      if (!isHistoryUnsupportedError(err)) {
        throw err;
      }
      detail = localConversationDetail(
        conversationId,
        agentId,
        text,
        assistantText,
        latestTaskId,
        latestStatus
      );
    }
    yield {
      type: 'turn_completed',
      ...detail
    };
  } catch (err: unknown) {
    if (isAbortError(err)) {
      yield {
        type: 'turn_canceled',
        conversationId,
        agentId
      };
      return;
    }
    const errMsg = err instanceof Error ? err.message : 'Unknown error';
    yield {
      type: 'turn_failed',
      conversationId,
      agentId,
      message: errMsg || 'Failed to complete stream turn'
    };
  }
}
