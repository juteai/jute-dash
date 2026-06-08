import {
  Client,
  JsonRpcTransportFactory,
  type Transport
} from '@a2a-js/sdk/client';
import {
  Role,
  TaskState,
  taskStateToJSON,
  type AgentCard,
  type Message as A2AMessage,
  type Part as A2APart,
  type Task as A2ATask
} from '@a2a-js/sdk';
import type {
  Conversation,
  ConversationDetail,
  ConversationStreamEvent,
  InterimStep
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

interface LegacyMessage {
  id?: string;
  messageId?: string;
  role: string;
  text?: string;
  parts?: Array<{ kind: string; text?: string }>;
}

interface LegacyTask {
  messages?: LegacyMessage[];
  text?: string;
  updatedAt?: string;
}

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

function statusFromTask(task: A2ATask): string {
  return task.status
    ? taskStateToJSON(task.status.state)
        .replace(/^TASK_STATE_/, '')
        .toLowerCase()
    : 'completed';
}

function isTerminalTaskState(state: TaskState): boolean {
  return [
    TaskState.TASK_STATE_COMPLETED,
    TaskState.TASK_STATE_FAILED,
    TaskState.TASK_STATE_CANCELED,
    TaskState.TASK_STATE_REJECTED
  ].includes(state);
}

function terminalTaskFailureMessage(status: string): string | undefined {
  switch (status) {
    case 'failed':
      return 'Agent task failed';
    case 'rejected':
      return 'Agent rejected the request';
    case 'canceled':
      return 'Agent canceled the request';
    default:
      return undefined;
  }
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

function newUserMessage(conversationId: string, text: string): A2AMessage {
  return {
    messageId: crypto.randomUUID(),
    contextId: conversationId,
    taskId: '',
    role: Role.ROLE_USER,
    parts: [
      {
        content: { $case: 'text', value: text },
        metadata: undefined,
        filename: '',
        mediaType: 'text/plain'
      }
    ],
    metadata: undefined,
    extensions: [],
    referenceTaskIds: []
  };
}

function localConversationDetail(
  conversationId: string,
  agentId: string,
  userText: string,
  assistantText: string,
  taskId: string,
  status: string
): ConversationDetail {
  const now = new Date().toISOString();
  return {
    conversation: {
      id: conversationId,
      agentId,
      title: userText,
      status,
      a2aContextId: conversationId,
      latestTaskId: taskId,
      createdAt: now,
      updatedAt: now,
      historyUnsupported: true
    },
    messages: [
      {
        id: crypto.randomUUID(),
        conversationId,
        agentId,
        role: 'user',
        content: userText,
        status: 'sent',
        a2aMessageId: '',
        a2aTaskId: taskId,
        createdAt: now,
        updatedAt: now
      },
      {
        id: crypto.randomUUID(),
        conversationId,
        agentId,
        role: 'assistant',
        content: assistantText,
        status: 'sent',
        a2aMessageId: '',
        a2aTaskId: taskId,
        createdAt: now,
        updatedAt: now
      }
    ]
  };
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
      const history = (task.history ||
        (task as unknown as LegacyTask).messages) as
        | Array<A2AMessage | LegacyMessage>
        | undefined;
      if (history && history.length > 0) {
        const firstUser = history.find(
          (message) =>
            message.role === Role.ROLE_USER || message.role === 'user'
        );
        if (firstUser && typeof firstUser.role === 'number') {
          title = textFromParts((firstUser as A2AMessage).parts);
        } else if (firstUser) {
          title = firstUser.text ?? '';
        }
      }
      if (!title) {
        title = (task as unknown as LegacyTask).text || 'Conversation';
      }

      const updatedAt =
        task.status?.timestamp ||
        (task as unknown as LegacyTask).updatedAt ||
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

  const detail: ConversationDetail = {
    conversation: {
      id: conversationId,
      agentId,
      title: 'Conversation',
      status: 'idle',
      a2aContextId: conversationId,
      latestTaskId: '',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString()
    },
    messages: []
  };

  tasks.sort((a, b) => {
    const timeA =
      a.status?.timestamp || (a as unknown as LegacyTask).updatedAt || '';
    const timeB =
      b.status?.timestamp || (b as unknown as LegacyTask).updatedAt || '';
    return timeA.localeCompare(timeB);
  });

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

    const recordUpdatedAt =
      record.status?.timestamp ||
      (record as unknown as LegacyTask).updatedAt ||
      detail.conversation.updatedAt;
    detail.conversation.latestTaskId = record.id || '';
    detail.conversation.status = statusFromTask(record);
    detail.conversation.updatedAt = recordUpdatedAt;

    const recordInterimSteps: InterimStep[] = [];
    if (record.status?.message?.parts) {
      const reasoningParts = record.status.message.parts.filter(
        (p) => p.metadata?.adk_thought === true
      );
      if (reasoningParts.length > 0) {
        const text = textFromReasoningParts(reasoningParts);
        if (text) {
          recordInterimSteps.push({
            id: `${record.id}:status-thought`,
            text,
            status: 'completed'
          });
        }
      }
    }
    for (const [index, artifact] of (record.artifacts ?? []).entries()) {
      const isReasoningArt = isReasoningArtifact(artifact);
      const reasoningParts = isReasoningArt
        ? (artifact.parts ?? []).filter((p) => {
            if (p.metadata?.adk_thought === true) return true;
            const pt = getPartText(p);
            // Exclude tool-call and tool-response text from reasoning display
            return (
              !pt.includes('<tool_call>') &&
              !pt.includes('<tool_response>') &&
              !looksLikeToolInvocation(pt)
            );
          })
        : (artifact.parts ?? []).filter(
            (p) => p.metadata?.adk_thought === true
          );
      if (reasoningParts.length > 0) {
        const text = textFromReasoningParts(reasoningParts);
        if (text) {
          recordInterimSteps.push({
            id: `${record.id}:thought:${artifact.artifactId || index}`,
            text,
            status: 'completed'
          });
        }
      }
      for (const [pIdx, part] of (artifact.parts ?? []).entries()) {
        if (part.metadata?.adk_thought === true) continue;

        const data = getPartData(part) as PartWithData['data'];
        const isFunctionCall =
          part.metadata?.adk_type === 'function_call' ||
          (data && !data.response && (data.name || data.id));
        const isFunctionResponse =
          part.metadata?.adk_type === 'function_response' ||
          (data && data.response);

        if (isFunctionCall) {
          const toolName = data?.name || 'agent tool';
          const toolCallId = data?.id || `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`;
          recordInterimSteps.push({
            id: toolCallId,
            text: `Called tool: ${toolName}`,
            status: 'completed',
            args: (data as any)?.args
          });
          continue;
        }

        if (isFunctionResponse) {
          const toolName = data?.name || 'agent tool';
          const toolCallId = data?.id || `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`;
          const existing = data?.id ? recordInterimSteps.find((s) => s.id === data.id) : undefined;
          if (existing) {
            existing.output = (data as any)?.response?.output;
          } else {
            recordInterimSteps.push({
              id: toolCallId,
              text: `Called tool: ${toolName}`,
              status: 'completed',
              output: (data as any)?.response?.output
            });
          }
          continue;
        }

        const text = getPartText(part);
        if (
          text &&
          (text.includes('<tool_call>') || looksLikeToolInvocation(text))
        ) {
          const nameMatch =
            text.match(/"name"\s*:\s*"([^"]+)"/) ||
            text.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
          const toolName = nameMatch ? nameMatch[1] : 'agent tool';
          let args: any = undefined;
          try {
            const jsonStart = text.indexOf('{');
            if (jsonStart > -1) {
              args = JSON.parse(text.slice(jsonStart));
            }
          } catch {}
          recordInterimSteps.push({
            id: `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`,
            text: `Called tool: ${toolName}`,
            status: 'completed',
            args
          });
        }
      }
    }

    const history = (record.history ||
      (record as unknown as LegacyTask).messages) as
      | Array<A2AMessage | LegacyMessage>
      | undefined;
    if (history) {
      for (const msg of history) {
        const isA2AMessage = 'messageId' in msg && typeof msg.role === 'number';
        const legacyMessage = msg as LegacyMessage;
        const content = isA2AMessage
          ? textFromParts(msg.parts as A2APart[])
          : legacyMessage.text || '';

        const messageThoughts: Array<{
          id: string;
          text: string;
          status: string;
          args?: any;
          output?: any;
        }> = [];
        if (isA2AMessage && msg.parts) {
          for (const [idx, part] of msg.parts.entries()) {
            if (part.metadata?.adk_thought === true) {
              const text = textFromReasoningParts([part]);
              if (text) {
                messageThoughts.push({
                  id: `${msg.messageId || 'msg'}:thought:${idx}`,
                  text,
                  status: 'completed'
                });
              }
            } else {
              // Check for tool calls in history message parts (structured and text/XML/ADK-style)
              const data = getPartData(part) as PartWithData['data'];
              const isFunctionCall =
                part.metadata?.adk_type === 'function_call' ||
                (data && !data.response && (data.name || data.id));
              const isFunctionResponse =
                part.metadata?.adk_type === 'function_response' ||
                (data && data.response);

              if (isFunctionCall) {
                const toolName = data?.name || 'agent tool';
                messageThoughts.push({
                  id: data?.id || `${msg.messageId || 'msg'}:tool:${idx}`,
                  text: `Called tool: ${toolName}`,
                  status: 'completed',
                  args: (data as any)?.args
                });
              } else if (isFunctionResponse) {
                const toolName = data?.name || 'agent tool';
                const toolCallId = data?.id || `${msg.messageId || 'msg'}:tool:${idx}`;
                const existing = data?.id ? messageThoughts.find((s) => s.id === data.id) : undefined;
                if (existing) {
                  existing.output = (data as any)?.response?.output;
                } else {
                  messageThoughts.push({
                    id: toolCallId,
                    text: `Called tool: ${toolName}`,
                    status: 'completed',
                    output: (data as any)?.response?.output
                  });
                }
              } else {
                const text = (
                  'text' in part && part.text
                    ? part.text
                    : part.content?.$case === 'text'
                      ? part.content.value
                      : ''
                ) as string;
                if (
                  text &&
                  (text.includes('<tool_call>') ||
                    looksLikeToolInvocation(text))
                ) {
                  const nameMatch =
                    text.match(/"name"\s*:\s*"([^"]+)"/) ||
                    text.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
                  const toolName = nameMatch ? nameMatch[1] : 'agent tool';
                  let args: any = undefined;
                  try {
                    const jsonStart = text.indexOf('{');
                    if (jsonStart > -1) {
                      args = JSON.parse(text.slice(jsonStart));
                    }
                  } catch {}
                  messageThoughts.push({
                    id: `${msg.messageId || 'msg'}:tool:${idx}`,
                    text: `Called tool: ${toolName}`,
                    status: 'completed',
                    args
                  });
                }
              }
            }
          }
        }

        const isAssistant =
          msg.role === Role.ROLE_AGENT ||
          msg.role === 'agent' ||
          msg.role === 'assistant';
        const combinedSteps = isAssistant
          ? [...recordInterimSteps, ...messageThoughts]
          : [];

        detail.messages.push({
          id: legacyMessage.id || msg.messageId || Math.random().toString(),
          conversationId,
          agentId,
          role:
            msg.role === Role.ROLE_USER || msg.role === 'user'
              ? 'user'
              : 'assistant',
          content,
          status: 'sent',
          a2aMessageId: legacyMessage.id || msg.messageId || '',
          a2aTaskId: record.id || '',
          createdAt: recordUpdatedAt,
          updatedAt: recordUpdatedAt,
          interimSteps: combinedSteps.length > 0 ? combinedSteps : undefined
        });
      }
    }
    for (const [index, artifact] of (record.artifacts ?? []).entries()) {
      if (isReasoningArtifact(artifact)) {
        continue;
      }
      const content = textFromParts(artifact.parts);
      const isStructured = isStructuredArtifact(artifact.parts);
      if (!content && !isStructured) {
        continue;
      }
      const artifactID = artifact.artifactId || String(index);
      const title = artifact.name || artifact.artifactId || 'Artifact';

      // Avoid duplicating plain-text artifacts if history already has an assistant reply
      if (!isStructured) {
        const hasAssistantReply = detail.messages.some(
          (m) => m.a2aTaskId === record.id && m.role === 'assistant'
        );
        if (hasAssistantReply) {
          continue;
        }
      }

      detail.messages.push({
        id: `${record.id}:artifact:${artifactID}`,
        conversationId,
        agentId,
        role: 'assistant',
        content,
        status: 'sent',
        a2aMessageId: artifact.artifactId,
        a2aTaskId: record.id || '',
        createdAt: recordUpdatedAt,
        updatedAt: recordUpdatedAt,
        artifact: isStructured
          ? {
              id: artifactID,
              title,
              content
            }
          : undefined,
        interimSteps:
          recordInterimSteps.length > 0 ? recordInterimSteps : undefined
      });
    }

    if (recordInterimSteps.length > 0) {
      const hasAssistant = detail.messages.some(
        (m) => m.role === 'assistant' && m.a2aTaskId === record.id
      );
      if (!hasAssistant) {
        detail.messages.push({
          id: `${record.id}:fallback-assistant`,
          conversationId,
          agentId,
          role: 'assistant',
          content: '',
          status: 'sent',
          a2aMessageId: '',
          a2aTaskId: record.id || '',
          createdAt: recordUpdatedAt,
          updatedAt: recordUpdatedAt,
          interimSteps: recordInterimSteps
        });
      }
    }
  }

  const firstUser = detail.messages.find((m) => m.role === 'user');
  if (firstUser && firstUser.content) {
    detail.conversation.title = firstUser.content;
  }

  return detail;
}

export async function sendConversationTurn(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string,
  options: TurnRequestOptions = {}
): Promise<ConversationDetail> {
  const client = await getClient(fetcher, agentId);
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
    const assistantText = textFromParts(result.parts);
    if (!assistantText.trim()) {
      throw new Error('Agent response contained no displayable text');
    }
    return localConversationDetail(
      result.contextId || conversationId,
      agentId,
      text,
      assistantText,
      result.taskId,
      'completed'
    );
  }
  const failureMessage = terminalTaskFailureMessage(statusFromTask(result));
  if (failureMessage) {
    throw new Error(failureMessage);
  }

  return getConversation(fetcher, conversationId, agentId, options);
}

export async function sendConversationTurnStream(
  fetcher: typeof fetch,
  conversationId: string,
  agentId: string,
  text: string,
  onEvent: (event: ConversationStreamEvent) => void,
  options: TurnRequestOptions = {}
): Promise<void> {
  let assistantText = '';
  let latestTaskId = '';
  let latestStatus = 'completed';
  const emittedLengths = new Map<string, number>();
  const accumulatedRawTexts = new Map<string, string>();

  onEvent({
    type: 'turn_started',
    conversationId,
    agentId
  });

  try {
    const client = await getClient(fetcher, agentId);

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
          onEvent({
            type: 'assistant_delta',
            conversationId,
            agentId,
            text: content,
            append: true
          });
        }
      } else if (payload?.$case === 'statusUpdate') {
        const statusText = textFromParts(payload.value.status?.message?.parts);
        const status = payload.value.status
          ? taskStateToJSON(payload.value.status.state)
              .replace(/^TASK_STATE_/, '')
              .toLowerCase()
          : 'working';
        latestTaskId = payload.value.taskId;
        latestStatus = status;
        onEvent({
          type: 'status_changed',
          conversationId,
          agentId,
          taskId: payload.value.taskId,
          status,
          text: statusText,
          terminal: payload.value.status
            ? isTerminalTaskState(payload.value.status.state)
            : false
        });
        const failureMessage = terminalTaskFailureMessage(status);
        if (failureMessage) {
          onEvent({
            type: 'turn_failed',
            conversationId,
            agentId,
            message: failureMessage
          });
          return;
        }
      } else if (payload?.$case === 'artifactUpdate') {
        const artifact = payload.value.artifact;
        if (artifact) {
          latestTaskId = payload.value.taskId;
          const isReasoningArt = isReasoningArtifact(artifact);
          const artifactID = artifact.artifactId || 'streamed-artifact';

          const parts = artifact.parts ?? [];

          // 1. Process all reasoning parts together to prevent separate part indexes from overwriting each other
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
                  onEvent({
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
                  });
                  emittedLengths.set(key, accumulatedRaw.length);
                }
              }
            }
          }

          // 2. Process non-reasoning parts in the loop
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
                onEvent({
                  type: 'status_changed',
                  conversationId,
                  agentId,
                  taskId: toolCallId,
                  status: 'working',
                  text: `Calling tool: ${toolName}`,
                  args: (data as any)?.args
                });
                emittedLengths.set(key, 1);
              }
              continue;
            }

            if (isFunctionResponse) {
              const toolName = data?.name || 'agent tool';
              const toolCallId = data?.id || `${latestTaskId}:tool:${pIdx}`;
              const key = `${artifactID}:${pIdx}:func_response`;
              if (!emittedLengths.has(key)) {
                onEvent({
                  type: 'status_changed',
                  conversationId,
                  agentId,
                  taskId: toolCallId,
                  status: 'completed',
                  text: `Called tool: ${toolName}`,
                  output: (data as any)?.response?.output
                });
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
              // Streamed tool call or tool response — show as status update only
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
                } catch {}
                onEvent({
                  type: 'status_changed',
                  conversationId,
                  agentId,
                  taskId: `${latestTaskId}:tool_call:${pIdx}`,
                  status: 'working',
                  text: `Calling tool: ${toolName}`,
                  args
                });
                emittedLengths.set(key, partText.length);
              }
            } else {
              // Regular content — emit as artifact update
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
                  onEvent({
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
                  });
                  emittedLengths.set(key, cleanText.length);
                }
              }
            }
          }
        }
      } else if (payload?.$case === 'task') {
        const statusText = textFromParts(payload.value.status?.message?.parts);
        const status = statusFromTask(payload.value);
        latestTaskId = payload.value.id;
        latestStatus = status;
        onEvent({
          type: 'status_changed',
          conversationId,
          agentId,
          taskId: payload.value.id,
          status,
          text: statusText,
          terminal: payload.value.status
            ? isTerminalTaskState(payload.value.status.state)
            : false
        });
        const failureMessage = terminalTaskFailureMessage(status);
        if (failureMessage) {
          onEvent({
            type: 'turn_failed',
            conversationId,
            agentId,
            message: failureMessage
          });
          return;
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
      if (!assistantText.trim()) {
        throw new Error('Agent response contained no displayable text', {
          cause: err
        });
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
    onEvent({
      type: 'turn_completed',
      ...detail
    });
  } catch (err: unknown) {
    if (isAbortError(err)) {
      onEvent({
        type: 'turn_canceled',
        conversationId,
        agentId
      });
      return;
    }
    const errMsg = err instanceof Error ? err.message : 'Unknown error';
    onEvent({
      type: 'turn_failed',
      conversationId,
      agentId,
      message: errMsg || 'Failed to complete stream turn'
    });
  }
}
