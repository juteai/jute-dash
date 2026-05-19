<script lang="ts">
  import { browser } from '$app/environment';
  import { onMount } from 'svelte';
  import ChatView from '$lib/components/chat/ChatView.svelte';
  import DashboardView from '$lib/components/display/DashboardView.svelte';
  import OfflineState from '$lib/components/display/OfflineState.svelte';
  import StatusRibbon from '$lib/components/display/StatusRibbon.svelte';
  import { getDashboard, getWidgetCatalog, muteVoice, resetWidgetLayout, saveWidgetLayout, sendMessage, unmuteVoice } from '$lib/api';
  import { firstAvailableAgent, getAgentAvailability, isAgentAvailable } from '$lib/agents';
  import type {
    Agent,
    ChatMessage,
    ChatState,
    DashboardData,
    DisplayMode,
    UserFacingIssue,
    WidgetCatalogItem,
    WidgetInstance,
    WidgetLayout
  } from '$lib/types';
  import { cn } from '$lib/utils';

  export let data: DashboardData;

  let dashboard: DashboardData = data;
  let dashboardForView: DashboardData = data;
  let draftLayout: WidgetLayout | undefined;
  let widgetCatalog: WidgetCatalogItem[] = [];
  let editIssue = '';
  let savingLayout = false;
  let mode: DisplayMode = 'dashboard';
  let chatState: ChatState = 'idle';
  let messages: ChatMessage[] = [];
  let voiceIssue = '';
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
  $: dashboardForView = {
    ...dashboard,
    layout: mode === 'edit' && draftLayout ? draftLayout : dashboard.layout
  };

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

  async function enterEdit() {
    draftLayout = cloneLayout(dashboard.layout);
    editIssue = '';
    mode = 'edit';
    if (widgetCatalog.length === 0) {
      try {
        widgetCatalog = await getWidgetCatalog(fetch);
        markConnected();
      } catch {
        editIssue = 'Widget catalog is unavailable. Existing widgets can still be adjusted.';
        markIssue('degraded', {
          code: 'widget_catalog_unavailable',
          severity: 'warning',
          title: 'Widget catalog unavailable',
          message: 'Jute could not load the widget catalog.'
        });
      }
    }
  }

  async function saveEdit() {
    if (!draftLayout || savingLayout || dashboard.stale) {
      return;
    }
    savingLayout = true;
    editIssue = '';
    try {
      const saved = await saveWidgetLayout(fetch, packLayout(draftLayout));
      dashboard = {
        ...dashboard,
        layout: saved,
        connectionState: 'connected',
        stale: false,
        issue: undefined,
        loadedAt: new Date().toISOString()
      };
      draftLayout = undefined;
      mode = 'dashboard';
    } catch {
      editIssue = 'Layout was not saved. Check that the hub is running, then try again.';
      markIssue('degraded', {
        code: 'layout_save_failed',
        severity: 'warning',
        title: 'Layout not saved',
        message: 'Jute could not save the dashboard layout.'
      });
    } finally {
      savingLayout = false;
    }
  }

  function cancelEdit() {
    draftLayout = undefined;
    editIssue = '';
    mode = 'dashboard';
  }

  async function resetLayout() {
    if (savingLayout || dashboard.stale) {
      return;
    }
    savingLayout = true;
    editIssue = '';
    try {
      const reset = await resetWidgetLayout(fetch, dashboardForView.layout.profileId);
      dashboard = {
        ...dashboard,
        layout: reset,
        connectionState: 'connected',
        stale: false,
        issue: undefined,
        loadedAt: new Date().toISOString()
      };
      draftLayout = cloneLayout(reset);
    } catch {
      editIssue = 'Default layout could not be restored.';
      markIssue('degraded', {
        code: 'layout_reset_failed',
        severity: 'warning',
        title: 'Layout not reset',
        message: 'Jute could not reset the dashboard layout.'
      });
    } finally {
      savingLayout = false;
    }
  }

  function addWidget(kind: string) {
    if (!draftLayout) {
      return;
    }
    const item = widgetCatalog.find((candidate) => candidate.kind === kind);
    if (!item) {
      editIssue = 'That widget is not available in this display build.';
      return;
    }

    const layout = cloneLayout(draftLayout);
    const targetRow = nextWidgetRow(layout);
    let widget = layout.widgets.find((candidate) => candidate.kind === item.kind);
    if (widget && !item.allowMultiple) {
      widget.visible = true;
      widget.title = widget.title || item.defaultTitle;
      widget.w = item.defaultW;
      widget.h = item.defaultH;
      widget.minW = item.minW;
      widget.minH = item.minH;
      widget.size = item.defaultSize;
    } else {
      widget = {
        id: uniqueWidgetId(layout, item.kind),
        kind: item.kind,
        title: item.defaultTitle,
        x: 0,
        y: nextWidgetRow(layout),
        w: item.defaultW,
        h: item.defaultH,
        minW: item.minW,
        minH: item.minH,
        size: item.defaultSize,
        settings: {},
        visible: true
      };
      layout.widgets = [...layout.widgets, widget];
    }
    widget.x = 0;
    widget.y = targetRow;
    draftLayout = packLayout(layout, widget.id);
    editIssue = '';
  }

  function moveWidget(widgetId: string, x: number, y: number) {
    if (!draftLayout) {
      return;
    }
    const layout = cloneLayout(draftLayout);
    const widget = layout.widgets.find((item) => item.id === widgetId);
    if (!widget) {
      return;
    }
    widget.x = x;
    widget.y = y;
    draftLayout = packLayout(layout, widgetId);
  }

  function resizeWidget(widgetId: string, w: number, h: number) {
    if (!draftLayout) {
      return;
    }
    const layout = cloneLayout(draftLayout);
    const widget = layout.widgets.find((item) => item.id === widgetId);
    if (!widget) {
      return;
    }
    widget.w = w;
    widget.h = h;
    widget.size = sizeFromDimensions(w, h);
    draftLayout = packLayout(layout, widgetId);
  }

  function removeWidget(widgetId: string) {
    if (!draftLayout) {
      return;
    }
    const layout = cloneLayout(draftLayout);
    const widget = layout.widgets.find((item) => item.id === widgetId);
    if (!widget) {
      return;
    }
    widget.visible = false;
    draftLayout = packLayout(layout);
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
      void enterEdit();
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

  async function toggleVoiceMute() {
    if (dashboard.voice.serviceStatus !== 'ready') {
      voiceIssue = 'Voice is not configured yet. Add an STT provider before using microphone controls.';
      return;
    }
    try {
      const voice = dashboard.voice.muted ? await unmuteVoice(fetch) : await muteVoice(fetch);
      dashboard = {
        ...dashboard,
        voice,
        connectionState: 'connected',
        stale: false,
        issue: undefined,
        loadedAt: new Date().toISOString()
      };
      voiceIssue = '';
    } catch {
      voiceIssue = 'Voice state could not be updated. Check that the hub is running, then try again.';
      markIssue('degraded', {
        code: 'voice_update_failed',
        severity: 'warning',
        title: 'Voice not updated',
        message: 'Jute could not update the voice mute state.'
      });
    }
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

  function cloneLayout(layout: WidgetLayout): WidgetLayout {
    return JSON.parse(JSON.stringify(layout)) as WidgetLayout;
  }

  function uniqueWidgetId(layout: WidgetLayout, kind: string) {
    const base = kind.replace(/[^a-z0-9-]/gi, '-').toLowerCase();
    if (!layout.widgets.some((widget) => widget.id === base)) {
      return base;
    }
    let counter = 2;
    while (layout.widgets.some((widget) => widget.id === `${base}-${counter}`)) {
      counter += 1;
    }
    return `${base}-${counter}`;
  }

  function nextWidgetRow(layout: WidgetLayout) {
    return layout.widgets.reduce((row, widget) => (widget.visible ? Math.max(row, widget.y + widget.h) : row), 0);
  }

  function sizeFromDimensions(w: number, h: number): WidgetInstance['size'] {
    if (w >= 3 || h >= 3) {
      return 'large';
    }
    if (w >= 2 && h >= 2) {
      return 'medium';
    }
    if (w >= 2) {
      return 'wide';
    }
    return 'small';
  }

  function packLayout(layout: WidgetLayout, activeId = ''): WidgetLayout {
    const next = cloneLayout(layout);
    const visible = next.widgets.filter((widget) => widget.visible);
    const ordered = visible.sort((a, b) => {
      if (a.id === activeId) {
        return -1;
      }
      if (b.id === activeId) {
        return 1;
      }
      return a.y - b.y || a.x - b.x || a.id.localeCompare(b.id);
    });
    const occupied: boolean[][] = [];

    for (const widget of ordered) {
      clampWidget(widget);
      if (widget.id === activeId) {
        occupy(occupied, widget);
        continue;
      }
      const spot = firstOpenSpot(occupied, widget.w, widget.h);
      widget.x = spot.x;
      widget.y = spot.y;
      occupy(occupied, widget);
    }
    return next;
  }

  function clampWidget(widget: WidgetInstance) {
    const columns = 4;
    widget.minW = Math.min(Math.max(widget.minW || 1, 1), columns);
    widget.minH = Math.min(Math.max(widget.minH || 1, 1), 6);
    widget.w = Math.min(Math.max(widget.w || widget.minW, widget.minW), columns);
    widget.h = Math.min(Math.max(widget.h || widget.minH, widget.minH), 6);
    widget.x = Math.min(Math.max(widget.x, 0), columns - widget.w);
    widget.y = Math.min(Math.max(widget.y, 0), 99 - widget.h + 1);
    widget.size = sizeFromDimensions(widget.w, widget.h);
    widget.settings = widget.settings ?? {};
  }

  function firstOpenSpot(occupied: boolean[][], w: number, h: number) {
    for (let y = 0; y < 100; y += 1) {
      for (let x = 0; x <= 4 - w; x += 1) {
        if (canPlace(occupied, x, y, w, h)) {
          return { x, y };
        }
      }
    }
    return { x: 0, y: 99 - h + 1 };
  }

  function canPlace(occupied: boolean[][], x: number, y: number, w: number, h: number) {
    for (let row = y; row < y + h; row += 1) {
      for (let column = x; column < x + w; column += 1) {
        if (occupied[row]?.[column]) {
          return false;
        }
      }
    }
    return true;
  }

  function occupy(occupied: boolean[][], widget: WidgetInstance) {
    for (let row = widget.y; row < widget.y + widget.h; row += 1) {
      occupied[row] = occupied[row] ?? [];
      for (let column = widget.x; column < widget.x + widget.w; column += 1) {
        occupied[row][column] = true;
      }
    }
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
      data={dashboardForView}
      editMode={mode === 'edit'}
      {messages}
      theme={activeTheme}
      stale={dashboard.stale}
      selectedAgent={selectedAgent}
      selectedAvailability={selectedAvailability}
      voice={dashboard.voice}
      {widgetCatalog}
      {editIssue}
      {savingLayout}
      onOpenChat={() => openChat()}
      onToggleVoiceMute={toggleVoiceMute}
      onEnterEdit={enterEdit}
      onSaveEdit={saveEdit}
      onCancelEdit={cancelEdit}
      onResetLayout={resetLayout}
      onAddWidget={addWidget}
      onMoveWidget={moveWidget}
      onResizeWidget={resizeWidget}
      onRemoveWidget={removeWidget}
    />

    {#if mode === 'chat'}
      <div class="chat-layer">
        <ChatView
          {agents}
          {messages}
          state={chatState}
          voice={dashboard.voice}
          {voiceIssue}
          {selectedAgentId}
          {selectedAvailability}
          onAgentChange={(agentId) => {
            selectedAgentId = agentId;
          }}
          onSubmit={(value) => submitMessage(value)}
          onRetry={retryMessage}
          onClose={closeChat}
          onCancel={cancelChatTurn}
          onToggleVoiceMute={toggleVoiceMute}
        />
      </div>
    {/if}
  {/if}
</main>
