import { writable } from 'svelte/store';
import {
  getConversations,
  getConversation,
  createConversation,
  executeConversationTurn
} from '$lib/a2aConversation';
import { isAgentAvailable } from '$lib/agents';
import { navigationStore } from '$lib/navigationStore';
import type {
  Agent,
  ChatMessage,
  ChatState,
  Conversation,
  ConversationDetail,
  ConversationMessage,
  UserFacingIssue,
  ConversationStreamEvent,
  VoiceConversationMessage
} from '$lib/types';
import { createMessageQueue } from '$lib/messageQueue';

export interface ChatStoreState {
  chatState: ChatState;
  assistantStatusText: string;
  messageQueue: { id: string; text: string }[];
  timerProgress: number;
  showTimer: boolean;
  dismissTimeRemaining: number;
  messages: ChatMessage[];
  conversations: Conversation[];
  selectedConversationId: string;
  selectedAgentId: string;
  historyAgentId: string;
}

const initialState: ChatStoreState = {
  chatState: 'idle',
  assistantStatusText: '',
  messageQueue: [],
  timerProgress: 0,
  showTimer: false,
  dismissTimeRemaining: 60,
  messages: [],
  conversations: [],
  selectedConversationId: '',
  selectedAgentId: '',
  historyAgentId: ''
};

function makeID() {
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

function systemMessage(content: string): ChatMessage {
  return makeMessage('system', content);
}

function conversationMessageToChatMessage(
  message: ConversationMessage
): ChatMessage {
  return {
    id: message.id,
    conversationId: message.conversationId,
    role: message.role,
    content: message.content,
    createdAt: message.createdAt,
    status:
      message.status === 'streaming'
        ? 'streaming'
        : message.status === 'failed'
          ? 'failed'
          : 'sent',
    retryText:
      message.status === 'failed' && message.role === 'user'
        ? message.content
        : undefined,
    agentId: message.agentId,
    interimSteps: message.interimSteps,
    thinkingDurationMs: message.thinkingDurationMs,
    artifact: message.artifact
  };
}

function upsertConversation(
  existing: Conversation[],
  conversation: Conversation
) {
  const withoutCurrent = existing.filter((item) => item.id !== conversation.id);
  return [conversation, ...withoutCurrent].sort((a, b) =>
    b.updatedAt.localeCompare(a.updatedAt)
  );
}

function chatFailureMessage(error: unknown) {
  const message = error instanceof Error ? error.message.toLowerCase() : '';
  if (message.includes('disabled')) {
    return 'Message not sent. The selected agent is disabled.';
  }
  if (message.includes('credentials')) {
    return 'Message not sent. The selected agent needs credentials before Jute can call it.';
  }
  if (
    message.includes('protocol binding') ||
    message.includes('not implemented')
  ) {
    return 'Message not sent. The selected agent does not expose a supported A2A JSON-RPC binding.';
  }
  if (message.includes('agent card')) {
    return 'Message not sent. Jute could not refresh that agent’s Agent Card.';
  }
  if (message.includes('empty')) {
    return 'The agent responded, but Jute could not find displayable text.';
  }
  return 'Message not sent. Check that the hub and local A2A agent are running, then retry.';
}

function chatFailureIssueMessage(error: unknown) {
  const message = error instanceof Error ? error.message.toLowerCase() : '';
  if (message.includes('credentials')) {
    return 'The selected agent is missing credentials.';
  }
  if (
    message.includes('protocol binding') ||
    message.includes('not implemented')
  ) {
    return 'The selected agent does not expose a supported A2A binding.';
  }
  if (message.includes('agent card')) {
    return 'Jute could not refresh the selected agent card.';
  }
  return 'The selected agent did not complete the request.';
}

function createChatStore() {
  const { subscribe, update } = writable<ChatStoreState>(initialState);

  let activeChatTurn: AbortController | undefined;
  let dismissTimerInterval: ReturnType<typeof setInterval> | undefined;
  // eslint-disable-next-line prefer-const
  let messageQueueManager: ReturnType<typeof createMessageQueue>;

  function stopDismissTimer() {
    if (dismissTimerInterval) {
      clearInterval(dismissTimerInterval);
      dismissTimerInterval = undefined;
    }
    update((s) => ({ ...s, showTimer: false, timerProgress: 0 }));
  }

  function startDismissTimer() {
    stopDismissTimer();
    update((s) => ({
      ...s,
      dismissTimeRemaining: 60,
      showTimer: true,
      timerProgress: 1
    }));

    dismissTimerInterval = setInterval(() => {
      let currentVal = 0;
      update((s) => {
        const nextTime = Math.max(0, s.dismissTimeRemaining - 0.1);
        currentVal = nextTime;
        return {
          ...s,
          dismissTimeRemaining: nextTime,
          timerProgress: nextTime / 60
        };
      });

      if (currentVal <= 0) {
        stopDismissTimer();
        navigationStore.closeChat();
      }
    }, 100);
  }

  function resetDismissTimer() {
    let shouldStart = false;
    update((s) => {
      if (
        (s.chatState === 'idle' || s.chatState === 'error') &&
        s.messageQueue.length === 0
      ) {
        shouldStart = true;
      }
      return s;
    });

    if (shouldStart) {
      startDismissTimer();
    } else {
      stopDismissTimer();
    }
  }

  function applyConversationDetail(detail: ConversationDetail) {
    update((s) => ({
      ...s,
      selectedConversationId: detail.conversation.id,
      selectedAgentId: detail.conversation.agentId || s.selectedAgentId,
      messages: detail.messages.map(conversationMessageToChatMessage),
      chatState:
        detail.conversation.status === 'streaming'
          ? 'streaming'
          : detail.conversation.status === 'failed'
            ? 'error'
            : 'idle',
      conversations: upsertConversation(s.conversations, detail.conversation)
    }));
  }

  function applyVoiceConversation(
    conversationId: string,
    agentId: string,
    voiceMessages: VoiceConversationMessage[],
    voiceState: 'thinking' | 'idle' | 'error' = 'idle'
  ) {
    const now = new Date().toISOString();
    const messages = voiceMessages.map((message) =>
      makeMessage(message.role, message.text, {
        id: message.id,
        conversationId,
        agentId,
        createdAt: message.createdAt,
        status:
          message.status === 'error'
            ? 'failed'
            : message.status === 'speaking'
              ? 'sent'
              : message.status === 'partial'
                ? 'streaming'
                : 'sent',
        retryText:
          message.role === 'user' && message.status === 'error'
            ? message.text
            : undefined
      })
    );
    const firstUser = messages.find((message) => message.role === 'user');
    const latestMessage = messages[messages.length - 1];
    const updatedAt = latestMessage?.createdAt || now;
    const conversation: Conversation = {
      id: conversationId,
      agentId,
      title: firstUser?.content || 'Voice conversation',
      status:
        voiceState === 'thinking'
          ? 'streaming'
          : voiceState === 'error'
            ? 'failed'
            : 'completed',
      a2aContextId: conversationId,
      latestTaskId: '',
      createdAt: messages[0]?.createdAt || now,
      updatedAt
    };

    update((s) => ({
      ...s,
      selectedConversationId: conversationId,
      selectedAgentId: agentId || s.selectedAgentId,
      messages,
      chatState: voiceState,
      conversations: upsertConversation(s.conversations, conversation)
    }));
  }

  async function ensureConversation(agent: Agent, fetcher: typeof fetch) {
    let convId = '';
    let conversationsList: Conversation[] = [];
    update((s) => {
      convId = s.selectedConversationId;
      conversationsList = s.conversations;
      return s;
    });

    if (convId) {
      const current = conversationsList.find((c) => c.id === convId);
      if (current?.agentId === agent.id) {
        return convId;
      }
    }

    const detail = await createConversation(fetcher, agent.id);
    applyConversationDetail(detail);
    return detail.conversation.id;
  }

  async function submitMessage(
    text: string,
    agents: Agent[],
    retryMessageId?: string,
    fetcher: typeof fetch = window.fetch,
    onMarkConnected?: () => void,
    onMarkIssue?: (issue: UserFacingIssue) => void
  ) {
    let agentId = '';
    let selectedConvId = '';
    let currentChatState = 'idle';
    update((s) => {
      agentId = s.selectedAgentId;
      selectedConvId = s.selectedConversationId;
      currentChatState = s.chatState;
      return s;
    });

    const agent = agents.find((item) => item.id === agentId);
    if (!agent || !isAgentAvailable(agent)) {
      update((s) => ({
        ...s,
        chatState: 'error',
        messages: [
          ...s.messages,
          systemMessage('No available agent is connected yet.')
        ]
      }));
      return;
    }

    if (
      (currentChatState === 'thinking' || currentChatState === 'streaming') &&
      !retryMessageId
    ) {
      const tempId = makeID();
      update((s) => ({
        ...s,
        messages: [
          ...s.messages,
          makeMessage('user', text, {
            id: tempId,
            status: 'queued',
            agentId,
            conversationId: selectedConvId
          })
        ]
      }));
      messageQueueManager.enqueue(tempId, text);
      resetDismissTimer();
      return;
    }

    stopDismissTimer();

    const temporaryMessageId = retryMessageId ?? makeID();
    update((s) => ({
      ...s,
      chatState: 'thinking',
      assistantStatusText: ''
    }));

    const turnStartedAt = Date.now();
    const assistantMessageId = makeID();

    if (activeChatTurn) {
      activeChatTurn.abort();
    }
    const turnController = new AbortController();
    activeChatTurn = turnController;

    try {
      const conversationId = await ensureConversation(agent, fetcher);
      update((s) => {
        const filtered = s.messages.filter((msg) => msg.id !== retryMessageId);
        return {
          ...s,
          chatState: 'thinking',
          messages: [
            ...filtered,
            makeMessage('user', text, {
              id: temporaryMessageId,
              conversationId,
              status: 'sent',
              retryText: text,
              agentId: agent.id
            })
          ]
        };
      });

      let completed = false;
      let canceled = false;
      let failure: Error | undefined;
      let thinkingDurationMs: number | undefined;

      const eventStream = executeConversationTurn(
        fetcher,
        conversationId,
        agent,
        text,
        { signal: turnController.signal }
      );

      for await (const event of eventStream) {
        if (turnController.signal.aborted && event.type !== 'turn_canceled') {
          break;
        }

        if (event.type === 'turn_started') {
          update((s) => ({ ...s, chatState: 'thinking' }));
          continue;
        }
        if (event.type === 'assistant_delta') {
          if (thinkingDurationMs === undefined) {
            thinkingDurationMs = Date.now() - turnStartedAt;
          }
          update((s) => ({
            ...s,
            chatState: 'streaming',
            messages: upsertAssistantDelta(
              s.messages,
              assistantMessageId,
              event,
              thinkingDurationMs
            )
          }));
          continue;
        }
        if (event.type === 'artifact_update') {
          if (event.isReasoning) {
            update((s) => ({
              ...s,
              chatState: 'thinking',
              messages: upsertReasoningStep(
                s.messages,
                assistantMessageId,
                event
              )
            }));
          } else if (event.isStructured) {
            if (thinkingDurationMs === undefined) {
              thinkingDurationMs = Date.now() - turnStartedAt;
            }
            update((s) => ({
              ...s,
              chatState: 'streaming',
              messages: upsertArtifactUpdate(
                s.messages,
                event,
                thinkingDurationMs
              )
            }));
          } else {
            if (thinkingDurationMs === undefined) {
              thinkingDurationMs = Date.now() - turnStartedAt;
            }
            update((s) => ({
              ...s,
              chatState: 'streaming',
              messages: upsertAssistantDelta(
                s.messages,
                assistantMessageId,
                {
                  type: 'assistant_delta',
                  conversationId: event.conversationId,
                  agentId: event.agentId,
                  taskId: event.taskId,
                  text: event.text,
                  append: event.append
                },
                thinkingDurationMs
              )
            }));
          }
          continue;
        }
        if (event.type === 'status_changed') {
          update((s) => ({
            ...s,
            chatState: event.status === 'completed' ? 'streaming' : 'thinking',
            assistantStatusText: event.text || '',
            messages: upsertInterimStep(s.messages, assistantMessageId, event)
          }));
          continue;
        }
        if (event.type === 'turn_completed') {
          completed = true;
          applyConversationDetail({
            conversation: event.conversation,
            messages: event.messages
          });
          continue;
        }
        if (event.type === 'turn_failed') {
          failure = new Error(event.message);
          continue;
        }
        if (event.type === 'turn_canceled') {
          canceled = true;
          update((s) => ({ ...s, chatState: 'idle', assistantStatusText: '' }));
        }
      }

      if (turnController.signal.aborted || canceled) {
        return;
      }
      if (failure) {
        throw failure;
      }
      if (!completed) {
        throw new Error('turn ended before completion');
      }

      if (onMarkConnected) onMarkConnected();
    } catch (err) {
      if (turnController.signal.aborted) {
        return;
      }
      update((s) => {
        const updated = s.messages.map((msg) =>
          msg.id === temporaryMessageId
            ? { ...msg, status: 'failed' as const, retryText: text }
            : msg
        );
        return {
          ...s,
          chatState: 'error',
          messages: [...updated, systemMessage(chatFailureMessage(err))]
        };
      });

      if (onMarkIssue) {
        onMarkIssue({
          code: 'message_failed',
          severity: 'warning',
          title: 'Message not sent',
          message: chatFailureIssueMessage(err)
        });
      }
    } finally {
      if (activeChatTurn === turnController) {
        activeChatTurn = undefined;
      }
    }
  }

  // Initialize queue manager
  messageQueueManager = createMessageQueue({
    submitMessage,
    startDismissTimer,
    stopDismissTimer,
    isSettled: () => {
      let settled = false;
      update((s) => {
        settled = s.chatState === 'idle' || s.chatState === 'error';
        return s;
      });
      return settled;
    },
    onQueueChange: (q) => {
      update((s) => ({ ...s, messageQueue: q }));
    },
    onMessageSending: (id) => {
      update((s) => ({
        ...s,
        messages: s.messages.map((msg) =>
          msg.id === id ? { ...msg, status: 'sending' as const } : msg
        )
      }));
    }
  });

  return {
    subscribe,
    setAgentId: (agentId: string) => {
      update((s) => ({
        ...s,
        selectedAgentId: agentId,
        selectedConversationId: '',
        messages: [],
        chatState: 'idle',
        assistantStatusText: ''
      }));
    },
    openChat: async (agents: Agent[], agent?: Agent) => {
      let targetAgentId = '';
      let preserveCurrentConversation = false;
      update((s) => {
        preserveCurrentConversation =
          !agent && Boolean(s.selectedConversationId) && s.messages.length > 0;
        if (agent) {
          targetAgentId = agent.id;
        } else if (preserveCurrentConversation) {
          targetAgentId = s.selectedAgentId;
        } else if (!s.selectedAgentId) {
          const available = agents.find((item) => isAgentAvailable(item));
          targetAgentId = available?.id ?? '';
        } else {
          targetAgentId = s.selectedAgentId;
        }
        return {
          ...s,
          selectedAgentId: targetAgentId,
          messageQueue: []
        };
      });

      stopDismissTimer();

      const selectedAgent = agents.find((item) => item.id === targetAgentId);
      if (!selectedAgent || !isAgentAvailable(selectedAgent)) {
        update((s) => ({
          ...s,
          chatState: 'error',
          messages: [systemMessage('No available agent is connected yet.')]
        }));
        return;
      }

      if (preserveCurrentConversation) {
        resetDismissTimer();
        return;
      }

      update((s) => ({
        ...s,
        selectedConversationId: '',
        messages: [],
        chatState: 'idle',
        assistantStatusText: ''
      }));
      resetDismissTimer();
    },
    closeChat: () => {
      stopDismissTimer();
      update((s) => ({ ...s, chatState: 'idle' }));
      navigationStore.closeChat();
    },
    loadHistory: async (
      agents: Agent[],
      preferredConversationId = '',
      agentOverride = '',
      fetcher: typeof fetch = window.fetch,
      onMarkConnected?: () => void,
      onMarkIssue?: (issue: UserFacingIssue) => void
    ) => {
      let agentId = '';
      update((s) => {
        agentId =
          agentOverride ||
          s.selectedAgentId ||
          agents.find((item) => isAgentAvailable(item))?.id ||
          agents.find((agent) => agent.enabled)?.id ||
          agents[0]?.id ||
          '';
        return { ...s, historyAgentId: agentId, selectedAgentId: agentId };
      });

      const agent = agents.find((item) => item.id === agentId);
      if (!agentId || !agent || !isAgentAvailable(agent)) {
        update((s) => ({
          ...s,
          conversations: [],
          messages: [],
          selectedConversationId: ''
        }));
        return;
      }

      try {
        const loaded = await getConversations(fetcher, agentId);
        update((s) => ({ ...s, conversations: loaded }));

        const candidate =
          loaded.find((conv) => conv.id === preferredConversationId) ??
          loaded.find((conv) => !conv.historyUnsupported);

        if (candidate) {
          const detail = await getConversation(
            fetcher,
            candidate.id,
            candidate.agentId
          );
          applyConversationDetail(detail);
        } else {
          update((s) => ({ ...s, selectedConversationId: '', messages: [] }));
        }

        if (onMarkConnected) onMarkConnected();
      } catch {
        if (onMarkIssue) {
          onMarkIssue({
            code: 'conversation_history_unavailable',
            severity: 'warning',
            title: 'Conversation history unavailable',
            message: 'Jute could not load agent-backed conversation history.'
          });
        }
      }
    },
    loadConversation: async (
      conversationId: string,
      agentId = '',
      fetcher: typeof fetch = window.fetch
    ) => {
      let currentAgentId = '';
      update((s) => {
        currentAgentId = agentId || s.selectedAgentId;
        return s;
      });

      try {
        const detail = await getConversation(
          fetcher,
          conversationId,
          currentAgentId
        );
        applyConversationDetail(detail);
      } catch {
        // ignore
      }
    },
    applyVoiceConversation,
    submit: async (
      text: string,
      agents: Agent[],
      retryMessageId?: string,
      fetcher: typeof fetch = window.fetch,
      onMarkConnected?: () => void,
      onMarkIssue?: (issue: UserFacingIssue) => void
    ) => {
      await submitMessage(
        text,
        agents,
        retryMessageId,
        fetcher,
        onMarkConnected,
        onMarkIssue
      );
      void messageQueueManager.drain(
        agents,
        fetcher,
        onMarkConnected,
        onMarkIssue
      );
    },
    retry: async (
      message: ChatMessage,
      agents: Agent[],
      fetcher: typeof fetch = window.fetch,
      onMarkConnected?: () => void,
      onMarkIssue?: (issue: UserFacingIssue) => void
    ) => {
      const text = message.retryText ?? message.content;
      if (!text.trim()) return;
      await submitMessage(
        text,
        agents,
        message.id,
        fetcher,
        onMarkConnected,
        onMarkIssue
      );
      void messageQueueManager.drain(
        agents,
        fetcher,
        onMarkConnected,
        onMarkIssue
      );
    },
    cancel: () => {
      if (activeChatTurn) {
        activeChatTurn.abort();
        activeChatTurn = undefined;
      }
      const canceledIds = messageQueueManager.cancel();
      const canceledSet = new Set(canceledIds);
      update((s) => {
        const filtered = s.messages.filter((msg) => !canceledSet.has(msg.id));
        return {
          ...s,
          assistantStatusText: '',
          messageQueue: [],
          messages: filtered,
          chatState: 'idle'
        };
      });
    },
    newConversation: async (
      agents: Agent[],
      fetcher: typeof fetch = window.fetch,
      onMarkConnected?: () => void,
      onMarkIssue?: (issue: UserFacingIssue) => void
    ) => {
      let agentId = '';
      update((s) => {
        agentId = s.selectedAgentId;
        return {
          ...s,
          messageQueue: []
        };
      });
      stopDismissTimer();

      const agent =
        agents.find((item) => item.id === agentId) ||
        agents.find((item) => isAgentAvailable(item));
      if (!agent || !isAgentAvailable(agent)) {
        update((s) => ({
          ...s,
          chatState: 'error',
          messages: [systemMessage('No available agent is connected yet.')]
        }));
        return;
      }

      try {
        const detail = await createConversation(fetcher, agent.id);
        applyConversationDetail(detail);
        if (onMarkConnected) onMarkConnected();
        resetDismissTimer();
      } catch {
        if (onMarkIssue) {
          onMarkIssue({
            code: 'conversation_create_failed',
            severity: 'warning',
            title: 'Conversation not created',
            message: 'Jute could not start a new saved conversation.'
          });
        }
      }
    },
    resetTimer: () => {
      resetDismissTimer();
    },
    stopTimer: () => {
      stopDismissTimer();
    },
    clearHistory: () => {
      update((s) => ({
        ...s,
        messages: [],
        conversations: [],
        selectedConversationId: ''
      }));
    }
  };
}

function upsertAssistantDelta(
  messages: ChatMessage[],
  messageId: string,
  event: Extract<ConversationStreamEvent, { type: 'assistant_delta' }>,
  thinkingDurationMs?: number
): ChatMessage[] {
  let found = false;
  const updatedMessages = messages.map((message) => {
    if (message.id !== messageId) {
      return message;
    }
    found = true;
    const steps = (message.interimSteps || []).map((step) => {
      if (step.status === 'thinking') {
        return { ...step, status: 'completed' as const };
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
}

function upsertArtifactUpdate(
  messages: ChatMessage[],
  event: Extract<ConversationStreamEvent, { type: 'artifact_update' }>,
  thinkingDurationMs?: number
): ChatMessage[] {
  if (event.isReasoning) {
    return messages;
  }
  const messageId = `${event.taskId || 'stream'}:artifact:${event.artifactId}`;

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
        return { ...step, status: 'completed' as const };
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
}

function upsertReasoningStep(
  messages: ChatMessage[],
  assistantMessageId: string,
  event: Extract<ConversationStreamEvent, { type: 'artifact_update' }>
): ChatMessage[] {
  const stepId = `${event.taskId || 'stream'}:reasoning:${event.artifactId}`;
  const stepText = event.text;

  let foundMessage = false;
  const updatedMessages = messages.map((message) => {
    if (message.id !== assistantMessageId) {
      return message;
    }
    foundMessage = true;
    const steps = (message.interimSteps || []).map((step) => {
      if (step.status === 'thinking' && step.id !== stepId) {
        return { ...step, status: 'completed' as const };
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
        status: 'thinking' as const
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
          status: 'thinking' as const
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
            status: 'thinking' as const
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
}

function upsertInterimStep(
  messages: ChatMessage[],
  assistantMessageId: string,
  event: Extract<ConversationStreamEvent, { type: 'status_changed' }>
): ChatMessage[] {
  if (!event.text || !event.text.trim()) {
    return messages;
  }
  const stepId = event.taskId || makeID();

  let foundMessage = false;
  const updatedMessages = messages.map((message) => {
    if (message.id !== assistantMessageId) {
      return message;
    }
    foundMessage = true;
    const steps = (message.interimSteps || []).map((step) => {
      if (step.status === 'thinking' && step.id !== stepId) {
        return { ...step, status: 'completed' as const };
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
        status: event.status as any,
        args:
          event.args !== undefined
            ? event.args
            : updatedSteps[existingIndex].args,
        output:
          event.output !== undefined
            ? event.output
            : updatedSteps[existingIndex].output
      };
    } else {
      updatedSteps = [
        ...steps,
        {
          id: stepId,
          text: event.text || '',
          status: event.status as any,
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
          status: event.status as any,
          args: event.args,
          output: event.output
        }
      ]
    });
  }
  return updatedMessages;
}

export const chatStore = createChatStore();
