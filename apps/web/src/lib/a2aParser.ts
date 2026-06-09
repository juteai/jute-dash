import {
  Role,
  TaskState,
  taskStateToJSON,
  type Message as A2AMessage,
  type Part as A2APart,
  type Task as A2ATask
} from '@a2a-js/sdk';
import type { ConversationDetail, InterimStep } from '$lib/types';
import {
  getPartData,
  getPartText,
  isReasoningArtifact,
  isStructuredArtifact,
  looksLikeToolInvocation,
  looksLikeReasoningParagraph,
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

type PartWithData = A2APart & {
  data?: {
    id?: string;
    name?: string;
    response?: {
      output?: unknown;
    };
  };
};

export function statusFromTask(task: A2ATask): string {
  return task.status ? statusFromState(task.status.state) : 'completed';
}

export function statusFromState(state: TaskState): string {
  return taskStateToJSON(state)
    .replace(/^TASK_STATE_/, '')
    .toLowerCase();
}

export function isTerminalTaskState(state: TaskState): boolean {
  return [
    TaskState.TASK_STATE_COMPLETED,
    TaskState.TASK_STATE_FAILED,
    TaskState.TASK_STATE_CANCELED,
    TaskState.TASK_STATE_REJECTED
  ].includes(state);
}

export function terminalTaskFailureMessage(status: string): string | undefined {
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

function isFinalUserFacingPart(
  part: A2APart,
  parentIsReasoningArt: boolean
): boolean {
  if (part.metadata?.adk_thought === true) return false;
  if (
    part.metadata?.adk_type === 'function_call' ||
    part.metadata?.adk_type === 'function_response'
  )
    return false;

  const data = getPartData(part) as PartWithData['data'];
  if (data && (data.name || data.id || data.response)) return false;

  const text = getPartText(part);
  if (!text) return false;

  const trimmed = text.trim();
  if (!trimmed) return false;

  if (
    trimmed.startsWith('<tool_call>') ||
    trimmed.endsWith('</tool_call>') ||
    trimmed.startsWith('<tool_response>') ||
    trimmed.endsWith('</tool_response>') ||
    trimmed.includes('<tool_call>') ||
    trimmed.includes('<tool_response>') ||
    looksLikeToolInvocation(trimmed)
  ) {
    return false;
  }

  if (parentIsReasoningArt && looksLikeReasoningParagraph(trimmed)) {
    return false;
  }

  return true;
}

export function parseTasksToConversationDetail(
  tasks: A2ATask[],
  conversationId: string,
  agentId: string
): ConversationDetail {
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

  const sortedTasks = [...tasks].sort((a, b) => {
    const timeA =
      a.status?.timestamp || (a as unknown as LegacyTask).updatedAt || '';
    const timeB =
      b.status?.timestamp || (b as unknown as LegacyTask).updatedAt || '';
    return timeA.localeCompare(timeB);
  });

  for (const record of sortedTasks) {
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
        if (text && text.trim() !== '') {
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
      let count = 0;
      let currentThoughtParts: A2APart[] = [];

      const flushThought = () => {
        if (currentThoughtParts.length > 0) {
          const text = textFromReasoningParts(currentThoughtParts);
          if (text && text.trim() !== '') {
            recordInterimSteps.push({
              id: isReasoningArt
                ? `${record.id}:thought:${artifact.artifactId || index}${count > 0 ? `-${count}` : ''}`
                : `${record.id}:thought:${artifact.artifactId || index}`,
              text,
              status: 'completed'
            });
          }
          currentThoughtParts = [];
        }
      };

      for (const [pIdx, part] of (artifact.parts ?? []).entries()) {
        const data = getPartData(part) as PartWithData['data'];
        const isFunctionCall =
          part.metadata?.adk_type === 'function_call' ||
          (data && !data.response && (data.name || data.id));
        const isFunctionResponse =
          part.metadata?.adk_type === 'function_response' ||
          (data && data.response);

        const pt = getPartText(part);
        const isToolText =
          looksLikeToolInvocation(pt) ||
          pt.includes('<tool_call>') ||
          pt.includes('<tool_response>');

        const isPartReasoning =
          part.metadata?.adk_thought === true ||
          (isReasoningArt &&
            !isToolText &&
            !isFunctionCall &&
            !isFunctionResponse &&
            (pt.trim() === '' || looksLikeReasoningParagraph(pt)));

        if (isPartReasoning) {
          currentThoughtParts.push(part);
          continue;
        }

        flushThought();

        if (isFunctionCall) {
          const toolName = data?.name || 'agent tool';
          const toolCallId =
            data?.id ||
            `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`;
          recordInterimSteps.push({
            id: toolCallId,
            text: `Called tool: ${toolName}`,
            status: 'completed',
            args: (data as any)?.args
          });
          count++;
          continue;
        }

        if (isFunctionResponse) {
          const toolName = data?.name || 'agent tool';
          const toolCallId =
            data?.id ||
            `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`;
          const existing = data?.id
            ? recordInterimSteps.find((s) => s.id === data.id)
            : undefined;
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

        if (isToolText) {
          const nameMatch =
            pt.match(/"name"\s*:\s*"([^"]+)"/) ||
            pt.match(/^([a-zA-Z_]\w*)\s*-\s*\{/);
          const toolName = nameMatch ? nameMatch[1] : 'agent tool';
          let args: any = undefined;
          try {
            const jsonStart = pt.indexOf('{');
            if (jsonStart > -1) {
              args = JSON.parse(pt.slice(jsonStart));
            }
          } catch {
            // ignore
          }
          recordInterimSteps.push({
            id: `${record.id}:tool:${artifact.artifactId || index}:${pIdx}`,
            text: `Called tool: ${toolName}`,
            status: 'completed',
            args
          });
          count++;
        }
      }

      flushThought();
    }

    const history = (record.history ||
      (record as unknown as LegacyTask).messages) as
      | Array<A2AMessage | LegacyMessage>
      | undefined;
    const messagesToProcess: Array<A2AMessage | LegacyMessage> = [];
    const recordInput = (record as any).input;
    if (recordInput) {
      messagesToProcess.push(recordInput);
    }
    if (history) {
      for (const msg of history) {
        if (
          recordInput &&
          msg.messageId &&
          msg.messageId === recordInput.messageId
        ) {
          continue;
        }
        messagesToProcess.push(msg);
      }
    }
    for (const msg of messagesToProcess) {
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
            if (text && text.trim() !== '') {
              messageThoughts.push({
                id: `${msg.messageId || 'msg'}:thought:${idx}`,
                text,
                status: 'completed'
              });
            }
          } else {
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
              const toolCallId =
                data?.id || `${msg.messageId || 'msg'}:tool:${idx}`;
              const existing = data?.id
                ? messageThoughts.find((s) => s.id === data.id)
                : undefined;
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
                } catch {
                  // ignore
                }
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
    for (const [index, artifact] of (record.artifacts ?? []).entries()) {
      const isReasoningArt = isReasoningArtifact(artifact);
      if (isReasoningArt) {
        const hasUserFacing = (artifact.parts ?? []).some((p) =>
          isFinalUserFacingPart(p, true)
        );
        if (!hasUserFacing) {
          continue;
        }
      }
      const content = textFromParts(artifact.parts);
      const isStructured = isStructuredArtifact(artifact.parts);
      if (!content && !isStructured) {
        continue;
      }
      const artifactID = artifact.artifactId || String(index);
      const title = artifact.name || artifact.artifactId || 'Artifact';

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

export function newUserMessage(
  conversationId: string,
  text: string
): A2AMessage {
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

export function localConversationDetail(
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
