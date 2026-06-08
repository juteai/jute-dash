import type {
  ChatMessage,
  ChatState,
  ConversationDetail,
  ConversationStreamEvent
} from '$lib/types';

export interface TurnCallbacks {
  updateMessages: (fn: (messages: ChatMessage[]) => ChatMessage[]) => void;
  setChatState: (state: ChatState) => void;
  setAssistantStatus: (text: string) => void;
  applyConversationDetail: (detail: ConversationDetail) => void;
}

export interface TurnResult {
  completed: boolean;
  canceled: boolean;
  failure?: Error;
}

function makeID(): string {
  if (
    typeof window !== 'undefined' &&
    'crypto' in window &&
    typeof window.crypto.randomUUID === 'function'
  ) {
    return window.crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function makeMessage(
  role: ChatMessage['role'],
  content: string,
  overrides: Partial<ChatMessage> = {}
): ChatMessage {
  return {
    id: overrides.id ?? makeID(),
    role,
    content,
    createdAt: overrides.createdAt ?? new Date().toISOString(),
    status: overrides.status,
    retryText: overrides.retryText,
    agentId: overrides.agentId,
    conversationId: overrides.conversationId
  };
}

function upsertAssistantDelta(
  messageId: string,
  event: Extract<ConversationStreamEvent, { type: 'assistant_delta' }>,
  updateMessages: TurnCallbacks['updateMessages'],
  thinkingDurationMs?: number
) {
  updateMessages((messages) => {
    let found = false;
    const updatedMessages = messages.map((message) => {
      if (message.id !== messageId) {
        return message;
      }
      found = true;
      const steps = (message.interimSteps || []).map((step) => {
        if (step.status === 'thinking') {
          return { ...step, status: 'completed' };
        }
        return step;
      });
      return {
        ...message,
        conversationId: event.conversationId || message.conversationId,
        content: event.append ? message.content + event.text : event.text,
        status: 'streaming' as const,
        agentId: event.agentId || message.agentId,
        thinkingDurationMs: thinkingDurationMs ?? message.thinkingDurationMs,
        interimSteps: steps.length > 0 ? steps : undefined
      };
    });

    if (!found) {
      updatedMessages.push(
        makeMessage('assistant', event.text, {
          id: messageId,
          conversationId: event.conversationId,
          status: 'streaming',
          agentId: event.agentId,
          thinkingDurationMs
        })
      );
    }
    return updatedMessages;
  });
}

function upsertArtifactUpdate(
  event: Extract<ConversationStreamEvent, { type: 'artifact_update' }>,
  updateMessages: TurnCallbacks['updateMessages'],
  thinkingDurationMs?: number
) {
  if (event.isReasoning) {
    return;
  }
  const messageId = `${event.taskId || 'stream'}:artifact:${event.artifactId}`;

  updateMessages((messages) => {
    let found = false;
    const updatedMessages = messages.map((message) => {
      if (message.id !== messageId) {
        return message;
      }
      found = true;
      const prevContent = message.content || '';
      const newContent = event.append ? prevContent + event.text : event.text;

      const steps = (message.interimSteps || []).map((step) => {
        if (step.status === 'thinking') {
          return { ...step, status: 'completed' };
        }
        return step;
      });

      return {
        ...message,
        status: 'streaming' as const,
        content: newContent,
        thinkingDurationMs: thinkingDurationMs ?? message.thinkingDurationMs,
        interimSteps: steps.length > 0 ? steps : undefined,
        artifact: {
          id: event.artifactId,
          title: event.name || event.artifactId || 'Artifact',
          content: newContent
        }
      };
    });

    if (!found) {
      updatedMessages.push({
        id: messageId,
        conversationId: event.conversationId,
        agentId: event.agentId,
        role: 'assistant',
        content: event.text,
        status: 'streaming',
        createdAt: new Date().toISOString(),
        thinkingDurationMs,
        artifact: {
          id: event.artifactId,
          title: event.name || event.artifactId || 'Artifact',
          content: event.text
        }
      });
    }
    return updatedMessages;
  });
}

function upsertReasoningStep(
  assistantMessageId: string,
  event: Extract<ConversationStreamEvent, { type: 'artifact_update' }>,
  updateMessages: TurnCallbacks['updateMessages']
) {
  const stepId = `${event.taskId || 'stream'}:reasoning:${event.artifactId}`;
  const stepText = event.text;

  updateMessages((messages) => {
    let foundMessage = false;
    const updatedMessages = messages.map((message) => {
      if (message.id !== assistantMessageId) {
        return message;
      }
      foundMessage = true;
      const steps = (message.interimSteps || []).map((step) => {
        if (step.status === 'thinking' && step.id !== stepId) {
          return { ...step, status: 'completed' };
        }
        return step;
      });
      const existingIndex = steps.findIndex((s) => s.id === stepId);
      let updatedSteps;
      if (existingIndex > -1) {
        updatedSteps = [...steps];
        updatedSteps[existingIndex] = {
          ...updatedSteps[existingIndex],
          text: event.append
            ? updatedSteps[existingIndex].text + stepText
            : stepText,
          status: 'thinking'
        };
      } else {
        if (!stepText.trim()) {
          return message;
        }
        updatedSteps = [
          ...steps,
          {
            id: stepId,
            text: stepText,
            status: 'thinking'
          }
        ];
      }
      return {
        ...message,
        interimSteps: updatedSteps
      };
    });

    if (!foundMessage) {
      if (stepText.trim()) {
        updatedMessages.push({
          id: assistantMessageId,
          conversationId: event.conversationId,
          agentId: event.agentId,
          role: 'assistant',
          content: '',
          status: 'streaming',
          createdAt: new Date().toISOString(),
          interimSteps: [
            {
              id: stepId,
              text: stepText,
              status: 'thinking'
            }
          ]
        });
      } else {
        updatedMessages.push({
          id: assistantMessageId,
          conversationId: event.conversationId,
          agentId: event.agentId,
          role: 'assistant',
          content: '',
          status: 'streaming',
          createdAt: new Date().toISOString()
        });
      }
    }
    return updatedMessages;
  });
}

function upsertInterimStep(
  assistantMessageId: string,
  event: Extract<ConversationStreamEvent, { type: 'status_changed' }>,
  updateMessages: TurnCallbacks['updateMessages']
) {
  if (!event.text || !event.text.trim()) {
    return;
  }
  const stepId = event.taskId || makeID();

  updateMessages((messages) => {
    let foundMessage = false;
    const updatedMessages = messages.map((message) => {
      if (message.id !== assistantMessageId) {
        return message;
      }
      foundMessage = true;
      const steps = (message.interimSteps || []).map((step) => {
        if (step.status === 'thinking' && step.id !== stepId) {
          return { ...step, status: 'completed' };
        }
        return step;
      });
      const existingIndex = steps.findIndex((s) => s.id === stepId);
      let updatedSteps;
      if (existingIndex > -1) {
        updatedSteps = [...steps];
        updatedSteps[existingIndex] = {
          ...updatedSteps[existingIndex],
          text: event.text || updatedSteps[existingIndex].text,
          status: event.status,
          args: event.args !== undefined ? event.args : updatedSteps[existingIndex].args,
          output: event.output !== undefined ? event.output : updatedSteps[existingIndex].output
        };
      } else {
        updatedSteps = [
          ...steps,
          {
            id: stepId,
            text: event.text || '',
            status: event.status,
            args: event.args,
            output: event.output
          }
        ];
      }
      return {
        ...message,
        interimSteps: updatedSteps
      };
    });

    if (!foundMessage) {
      updatedMessages.push({
        id: assistantMessageId,
        conversationId: event.conversationId,
        agentId: event.agentId,
        role: 'assistant',
        content: '',
        status: 'sending',
        createdAt: new Date().toISOString(),
        interimSteps: [
          {
            id: stepId,
            text: event.text || '',
            status: event.status,
            args: event.args,
            output: event.output
          }
        ]
      });
    }
    return updatedMessages;
  });
}

export function createStreamHandler(
  assistantMessageId: string,
  turnStartedAt: number,
  callbacks: TurnCallbacks
): {
  handler: (event: ConversationStreamEvent) => void;
  result: () => TurnResult;
} {
  let completed = false;
  let canceled = false;
  let failure: Error | undefined;
  let thinkingDurationMs: number | undefined;

  const handler = (event: ConversationStreamEvent) => {
    if (event.type === 'turn_started') {
      callbacks.setChatState('thinking');
      return;
    }
    if (event.type === 'assistant_delta') {
      if (thinkingDurationMs === undefined) {
        thinkingDurationMs = Date.now() - turnStartedAt;
      }
      callbacks.setChatState('streaming');
      upsertAssistantDelta(
        assistantMessageId,
        event,
        callbacks.updateMessages,
        thinkingDurationMs
      );
      return;
    }
    if (event.type === 'artifact_update') {
      if (event.isReasoning) {
        callbacks.setChatState('thinking');
        upsertReasoningStep(
          assistantMessageId,
          event,
          callbacks.updateMessages
        );
      } else if (event.isStructured) {
        if (thinkingDurationMs === undefined) {
          thinkingDurationMs = Date.now() - turnStartedAt;
        }
        callbacks.setChatState('streaming');
        upsertArtifactUpdate(
          event,
          callbacks.updateMessages,
          thinkingDurationMs
        );
      } else {
        if (thinkingDurationMs === undefined) {
          thinkingDurationMs = Date.now() - turnStartedAt;
        }
        callbacks.setChatState('streaming');
        upsertAssistantDelta(
          assistantMessageId,
          {
            type: 'assistant_delta',
            conversationId: event.conversationId,
            agentId: event.agentId,
            taskId: event.taskId,
            text: event.text,
            append: event.append
          },
          callbacks.updateMessages,
          thinkingDurationMs
        );
      }
      return;
    }
    if (event.type === 'status_changed') {
      callbacks.setChatState(
        event.status === 'completed' ? 'streaming' : 'thinking'
      );
      callbacks.setAssistantStatus(event.text || '');
      upsertInterimStep(assistantMessageId, event, callbacks.updateMessages);
      return;
    }
    if (event.type === 'turn_completed') {
      completed = true;
      callbacks.applyConversationDetail({
        conversation: event.conversation,
        messages: event.messages
      });
      return;
    }
    if (event.type === 'turn_failed') {
      failure = new Error(event.message);
      return;
    }
    if (event.type === 'turn_canceled') {
      canceled = true;
      callbacks.setChatState('idle');
      callbacks.setAssistantStatus('');
    }
  };

  return {
    handler,
    result: () => ({ completed, canceled, failure })
  };
}
