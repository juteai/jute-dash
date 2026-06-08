import { writable } from 'svelte/store';
import {
  getConversations,
  getConversation,
  createConversation,
  sendConversationTurn,
  sendConversationTurnStream
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
  UserFacingIssue
} from '$lib/types';
import { createStreamHandler } from '$lib/turnEngine';
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

      if (agent.streaming) {
        const { handler, result } = createStreamHandler(
          assistantMessageId,
          turnStartedAt,
          {
            updateMessages: (fn) =>
              update((s) => ({ ...s, messages: fn(s.messages) })),
            setChatState: (state) =>
              update((s) => ({ ...s, chatState: state })),
            setAssistantStatus: (statusText) =>
              update((s) => ({ ...s, assistantStatusText: statusText })),
            applyConversationDetail
          }
        );

        await sendConversationTurnStream(
          fetcher,
          conversationId,
          agent.id,
          text,
          (event) => {
            if (
              turnController.signal.aborted &&
              event.type !== 'turn_canceled'
            ) {
              return;
            }
            handler(event);
          },
          { signal: turnController.signal }
        );

        const turnResult = result();
        if (turnController.signal.aborted || turnResult.canceled) {
          return;
        }
        if (turnResult.failure) {
          throw turnResult.failure;
        }
        if (!turnResult.completed) {
          throw new Error('stream ended before completion');
        }
      } else {
        const detail = await sendConversationTurn(
          fetcher,
          conversationId,
          agent.id,
          text,
          { signal: turnController.signal }
        );
        const duration = Date.now() - turnStartedAt;
        const lastMsg = detail.messages
          .filter((m) => m.role === 'assistant')
          .pop();
        if (lastMsg) {
          lastMsg.thinkingDurationMs = duration;
        }
        applyConversationDetail(detail);
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
    openChat: async (
      agents: Agent[],
      agent?: Agent,
      fetcher: typeof fetch = window.fetch
    ) => {
      let targetAgentId = '';
      update((s) => {
        if (agent) {
          targetAgentId = agent.id;
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

      try {
        const detail = await createConversation(fetcher, selectedAgent.id);
        applyConversationDetail(detail);
        resetDismissTimer();
      } catch {
        update((s) => ({
          ...s,
          messages: [
            systemMessage('Jute could not start a new saved conversation.')
          ]
        }));
      }
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

export const chatStore = createChatStore();
