<script lang="ts">
  import { browser } from '$app/environment';
  import { onMount } from 'svelte';
  import ChatView from '$lib/components/chat/ChatView.svelte';
  import DashboardView from '$lib/components/display/DashboardView.svelte';
  import OfflineState from '$lib/components/display/OfflineState.svelte';
  import StatusRibbon from '$lib/components/display/StatusRibbon.svelte';
  import { getDashboard, sendMessage } from '$lib/api';
  import { firstAvailableAgent, getAgentAvailability, isAgentAvailable } from '$lib/agents';
  import type { Agent, ChatMessage, ChatState, DashboardData, DisplayMode, UserFacingIssue } from '$lib/types';
  import { cn } from '$lib/utils';

  export let data: DashboardData;

  let dashboard: DashboardData = data;
  let mode: DisplayMode = 'dashboard';
  let chatState: ChatState = 'idle';
  let messages: ChatMessage[] = [];
  let selectedAgentId = '';
  let prefersDark = false;
  let hasConnected = data.connectionState === 'connected';
  let retrying = false;
  let longPressTimer: number | undefined;

  $: agents = dashboard.agents;
  $: availableAgent = firstAvailableAgent(agents);
  $: fallbackAgent = availableAgent ?? agents.find((agent) => agent.enabled) ?? agents[0];
  $: if (!selectedAgentId && agents.length > 0) {
    selectedAgentId = fallbackAgent?.id ?? '';
  }
  $: if (selectedAgentId && agents.length > 0 && !agents.some((agent) => agent.id === selectedAgentId)) {
    selectedAgentId = fallbackAgent?.id ?? '';
  }
  $: selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
  $: selectedAvailability = getAgentAvailability(selectedAgent);
  $: hasConnected = hasConnected || dashboard.connectionState === 'connected';
  $: showStartupOffline = !hasConnected && dashboard.connectionState === 'offline';
  $: activeTheme = resolveTheme(dashboard.config.display.theme, prefersDark);

  onMount(() => {
    const query = window.matchMedia('(prefers-color-scheme: dark)');
    const updateTheme = () => {
      prefersDark = query.matches;
    };
    updateTheme();
    query.addEventListener('change', updateTheme);

    return () => {
      query.removeEventListener('change', updateTheme);
      clearLongPress();
    };
  });

  function resolveTheme(theme: string, systemPrefersDark: boolean): 'light' | 'dark' {
    if (theme === 'dark') {
      return 'dark';
    }
    if (theme === 'light') {
      return 'light';
    }
    return systemPrefersDark ? 'dark' : 'light';
  }

  function openChat(agent?: Agent) {
    if (agent) {
      selectedAgentId = agent.id;
    } else if (!selectedAgentId && availableAgent) {
      selectedAgentId = availableAgent.id;
    }
    mode = 'chat';
  }

  function closeChat() {
    chatState = 'idle';
    mode = 'dashboard';
  }

  function enterEdit() {
    mode = 'edit';
  }

  function exitEdit() {
    mode = 'dashboard';
  }

  function startLongPress(event: PointerEvent) {
    if (!browser || mode !== 'dashboard') {
      return;
    }
    const target = event.target as HTMLElement | null;
    if (target?.closest('button, a, input, textarea, select')) {
      return;
    }
    clearLongPress();
    longPressTimer = window.setTimeout(() => {
      mode = 'edit';
    }, 650);
  }

  function clearLongPress() {
    if (longPressTimer) {
      window.clearTimeout(longPressTimer);
      longPressTimer = undefined;
    }
  }

  async function submitMessage(text: string, retryMessageId?: string) {
    const agent = agents.find((item) => item.id === selectedAgentId);
    if (!agent || !isAgentAvailable(agent)) {
      chatState = 'error';
      messages = [...messages, systemMessage('No available agent is connected yet.')];
      return;
    }

    const userMessageId = retryMessageId ?? makeID();
    if (retryMessageId) {
      messages = messages.map((message) =>
        message.id === retryMessageId
          ? { ...message, status: 'sending', retryText: text, agentId: agent.id }
          : message
      );
    } else {
      messages = [...messages, makeMessage('user', text, { id: userMessageId, status: 'sending', retryText: text, agentId: agent.id })];
    }
    chatState = 'thinking';

    try {
      const response = await sendMessage(fetch, agent.id, text);
      messages = messages.map((message) =>
        message.id === userMessageId ? { ...message, status: 'sent', retryText: undefined } : message
      );
      messages = [...messages, makeMessage('assistant', response.message, { status: 'sent', agentId: agent.id })];
      markConnected();
      chatState = 'idle';
    } catch {
      messages = messages.map((message) =>
        message.id === userMessageId ? { ...message, status: 'failed', retryText: text } : message
      );
      messages = [...messages, systemMessage('Message not sent. Check that the hub and local A2A agent are running, then retry.')];
      markIssue('degraded', {
        code: 'message_failed',
        severity: 'warning',
        title: 'Message not sent',
        message: 'The selected agent did not complete the request.'
      });
      chatState = 'error';
    }
  }

  function retryMessage(message: ChatMessage) {
    const text = message.retryText ?? message.content;
    if (!text.trim()) {
      return;
    }
    void submitMessage(text, message.id);
  }

  function cancelChatTurn() {
    chatState = 'idle';
  }

  async function retryDashboard() {
    if (!browser || retrying) {
      return;
    }
    retrying = true;
    try {
      dashboard = await getDashboard(fetch);
      markConnected();
    } catch {
      markIssue(hasConnected ? 'reconnecting' : 'offline', {
        code: 'hub_unreachable',
        severity: 'error',
        title: 'Hub not reachable',
        message: `Jute Dash cannot connect to the local hub at ${dashboard.hubUrl}.`,
        action: {
          label: 'Retry',
          target: 'retry'
        }
      });
    } finally {
      retrying = false;
    }
  }

  function markConnected() {
    dashboard = {
      ...dashboard,
      connectionState: 'connected',
      stale: false,
      issue: undefined,
      loadedAt: new Date().toISOString()
    };
  }

  function markIssue(connectionState: DashboardData['connectionState'], issue: UserFacingIssue) {
    dashboard = {
      ...dashboard,
      connectionState,
      stale: true,
      issue
    };
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
      agentId: overrides.agentId
    };
  }

  function systemMessage(content: string): ChatMessage {
    return makeMessage('system', content);
  }

  function makeID() {
    if (browser && 'crypto' in window && typeof window.crypto.randomUUID === 'function') {
      return window.crypto.randomUUID();
    }
    return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
</script>

<svelte:head>
  <title>{dashboard.config.home.name} · Jute Dash</title>
</svelte:head>

<main
  class={cn('display-root', mode === 'chat' && 'display-root--chat', dashboard.stale && 'display-root--stale')}
  data-theme={activeTheme}
  on:pointerdown={startLongPress}
  on:pointerup={clearLongPress}
  on:pointercancel={clearLongPress}
  on:pointerleave={clearLongPress}
>
  {#if showStartupOffline}
    <OfflineState
      theme={activeTheme}
      hubUrl={dashboard.hubUrl}
      issue={dashboard.issue}
      {retrying}
      onRetry={retryDashboard}
    />
  {:else}
    <StatusRibbon
      state={dashboard.connectionState}
      stale={dashboard.stale}
      issue={dashboard.issue}
      {retrying}
      onRetry={retryDashboard}
    />

    <DashboardView
      data={dashboard}
      editMode={mode === 'edit'}
      {messages}
      theme={activeTheme}
      stale={dashboard.stale}
      selectedAgent={selectedAgent}
      selectedAvailability={selectedAvailability}
      onOpenChat={() => openChat()}
      onEnterEdit={enterEdit}
      onExitEdit={exitEdit}
    />

    {#if mode === 'chat'}
      <div class="chat-layer">
        <ChatView
          {agents}
          {messages}
          state={chatState}
          {selectedAgentId}
          {selectedAvailability}
          onAgentChange={(agentId) => {
            selectedAgentId = agentId;
          }}
          onSubmit={(value) => submitMessage(value)}
          onRetry={retryMessage}
          onClose={closeChat}
          onCancel={cancelChatTurn}
        />
      </div>
    {/if}
  {/if}
</main>
