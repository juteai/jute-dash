<script lang="ts">
  import { browser } from '$app/environment';
  import { onMount } from 'svelte';
  import ChatView from '$lib/components/chat/ChatView.svelte';
  import DashboardView from '$lib/components/display/DashboardView.svelte';
  import OfflineState from '$lib/components/display/OfflineState.svelte';
  import StatusRibbon from '$lib/components/display/StatusRibbon.svelte';
  import {
    addAgent,
    createConversation,
    deleteAgent,
    eventsURL,
    getConversation,
    getConversations,
    getDashboard,
    getWidgetCatalog,
    muteVoice,
    resetWidgetLayout,
    saveWidgetLayout,
    sendConversationTurn,
    sendConversationTurnStream,
    setAgentEnabled,
    unmuteVoice
  } from '$lib/api';
  import { firstAvailableAgent, getAgentAvailability, isAgentAvailable } from '$lib/agents';
  import type {
    Agent,
    ChatMessage,
    ChatState,
    Conversation,
    ConversationDetail,
    ConversationMessage,
    ConversationStreamEvent,
    DashboardData,
    DisplayFocusWidget,
    DisplayNotification,
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
  let conversations: Conversation[] = [];
  let selectedConversationId = '';
  let showAgentManager = false;
  let agentCardUrl = '';
  let agentIssue = '';
  let savingAgent = false;
  let mounted = false;
  let historyAgentId = '';
  let voiceIssue = '';
  let selectedAgentId = '';
  let prefersDark = false;
  let hasConnected = data.connectionState === 'connected';
  let retrying = false;
  let longPressTimer: number | undefined;
  let eventSource: EventSource | undefined;
  let displayNotifications: DisplayNotification[] = [];
  let notificationTimers: number[] = [];
  let focusedWidgetId = '';
  let focusTimer: number | undefined;

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
  $: if (mounted && selectedAgentId && selectedAgentId !== historyAgentId) {
    void loadConversationHistory('', selectedAgentId);
  }

  onMount(() => {
    mounted = true;
    const query = window.matchMedia('(prefers-color-scheme: dark)');
    const updateTheme = () => {
      prefersDark = query.matches;
    };
    updateTheme();
    query.addEventListener('change', updateTheme);
    void loadConversationHistory('', selectedAgentId);
    connectDisplayEvents();

    return () => {
      mounted = false;
      query.removeEventListener('change', updateTheme);
      clearLongPress();
      disconnectDisplayEvents();
      clearFocusTimer();
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

  async function loadConversationHistory(preferredConversationId = selectedConversationId, agentOverride = selectedAgentId) {
    try {
      const agentId =
        agentOverride ||
        selectedAgentId ||
        firstAvailableAgent(dashboard.agents)?.id ||
        dashboard.agents.find((agent) => agent.enabled)?.id ||
        dashboard.agents[0]?.id ||
        '';
      historyAgentId = agentId;
      if (!agentId) {
        conversations = [];
        messages = [];
        selectedConversationId = '';
        return;
      }
      const loaded = await getConversations(fetch, agentId);
      conversations = loaded;
      const candidate =
        loaded.find((conversation) => conversation.id === preferredConversationId) ??
        loaded[0];
      if (candidate) {
        await loadConversation(candidate.id, candidate.agentId);
      } else {
        selectedConversationId = '';
        messages = [];
      }
      markConnected();
    } catch {
      markIssue('degraded', {
        code: 'conversation_history_unavailable',
        severity: 'warning',
        title: 'Conversation history unavailable',
        message: 'Jute could not load agent-backed conversation history.'
      });
    }
  }

  async function loadConversation(conversationId: string, agentId = selectedAgentId) {
    const detail = await getConversation(fetch, conversationId, agentId);
    applyConversationDetail(detail);
  }

  function applyConversationDetail(detail: ConversationDetail) {
    selectedConversationId = detail.conversation.id;
    selectedAgentId = detail.conversation.agentId || selectedAgentId;
    messages = detail.messages.map(conversationMessageToChatMessage);
    chatState = detail.conversation.status === 'streaming' ? 'streaming' : detail.conversation.status === 'failed' ? 'error' : 'idle';
    conversations = upsertConversation(conversations, detail.conversation);
  }

  function conversationMessageToChatMessage(message: ConversationMessage): ChatMessage {
    return {
      id: message.id,
      conversationId: message.conversationId,
      role: message.role,
      content: message.content,
      createdAt: message.createdAt,
      status: message.status === 'streaming' ? 'streaming' : message.status === 'failed' ? 'failed' : 'sent',
      retryText: message.status === 'failed' && message.role === 'user' ? message.content : undefined,
      agentId: message.agentId
    };
  }

  function upsertConversation(existing: Conversation[], conversation: Conversation) {
    const withoutCurrent = existing.filter((item) => item.id !== conversation.id);
    return [conversation, ...withoutCurrent].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
  }

  async function ensureConversation(agent: Agent) {
    if (selectedConversationId) {
      const current = conversations.find((conversation) => conversation.id === selectedConversationId);
      if (current?.agentId === agent.id) {
        return selectedConversationId;
      }
    }
    const detail = await createConversation(fetch, agent.id);
    applyConversationDetail(detail);
    return detail.conversation.id;
  }

  async function createNewConversation() {
    const agent = agents.find((item) => item.id === selectedAgentId) ?? availableAgent;
    if (!agent || !isAgentAvailable(agent)) {
      chatState = 'error';
      messages = [systemMessage('No available agent is connected yet.')];
      return;
    }
    try {
      const detail = await createConversation(fetch, agent.id);
      applyConversationDetail(detail);
      mode = 'chat';
      markConnected();
    } catch {
      markIssue('degraded', {
        code: 'conversation_create_failed',
        severity: 'warning',
        title: 'Conversation not created',
        message: 'Jute could not start a new saved conversation.'
      });
    }
  }

  async function saveAgentFromCard() {
    const cardUrl = agentCardUrl.trim();
    if (!cardUrl || savingAgent) {
      return;
    }
    savingAgent = true;
    agentIssue = '';
    try {
      const agent = await addAgent(fetch, cardUrl);
      selectedAgentId = agent.id;
      agentCardUrl = '';
      dashboard = await getDashboard(fetch);
      await loadConversationHistory();
      markConnected();
    } catch {
      agentIssue = 'Agent was not added. Check the Agent Card URL and that the hub was started with a YAML config.';
      markIssue('degraded', {
        code: 'agent_add_failed',
        severity: 'warning',
        title: 'Agent not added',
        message: 'Jute could not add that A2A agent.'
      });
    } finally {
      savingAgent = false;
    }
  }

  async function toggleAgent(agent: Agent) {
    try {
      await setAgentEnabled(fetch, agent.id, !agent.enabled);
      dashboard = await getDashboard(fetch);
      markConnected();
    } catch {
      agentIssue = 'Agent state could not be updated.';
    }
  }

  async function removeAgent(agent: Agent) {
    try {
      await deleteAgent(fetch, agent.id);
      dashboard = await getDashboard(fetch);
      if (selectedAgentId === agent.id) {
        selectedAgentId = firstAvailableAgent(dashboard.agents)?.id ?? dashboard.agents[0]?.id ?? '';
        selectedConversationId = '';
        messages = [];
        conversations = [];
      }
      markConnected();
    } catch {
      agentIssue = 'Agent could not be removed.';
    }
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

  function connectDisplayEvents() {
    if (!browser || eventSource) {
      return;
    }
    eventSource = new EventSource(eventsURL());
    eventSource.addEventListener('open', () => {
      if (hasConnected) {
        markConnected();
      }
    });
    eventSource.addEventListener('error', () => {
      if (mounted && hasConnected) {
        markIssue('reconnecting', {
          code: 'event_stream_disconnected',
          severity: 'warning',
          title: 'Reconnecting',
          message: 'Jute lost the live display event stream. Dashboard data may be stale.'
        });
      }
    });
    eventSource.addEventListener('display.notification', (event) => {
      const notification = parseDisplayEvent<DisplayNotification>(event);
      if (notification) {
        addDisplayNotification(notification);
      }
    });
    eventSource.addEventListener('display.focus_widget', (event) => {
      const focus = parseDisplayEvent<DisplayFocusWidget>(event);
      if (focus) {
        focusWidget(focus);
      }
    });
    eventSource.addEventListener('hub.connected', () => {
      if (hasConnected) {
        markConnected();
      }
    });
  }

  function disconnectDisplayEvents() {
    eventSource?.close();
    eventSource = undefined;
    for (const timer of notificationTimers) {
      window.clearTimeout(timer);
    }
    notificationTimers = [];
  }

  function parseDisplayEvent<T>(event: Event): T | undefined {
    try {
      return JSON.parse((event as MessageEvent).data) as T;
    } catch {
      return undefined;
    }
  }

  function addDisplayNotification(notification: DisplayNotification) {
    displayNotifications = [notification, ...displayNotifications.filter((item) => item.id !== notification.id)].slice(0, 3);
    const expiry = Date.parse(notification.expiresAt);
    const delay = Number.isFinite(expiry) ? Math.max(2500, expiry - Date.now()) : 6000;
    const timer = window.setTimeout(() => {
      displayNotifications = displayNotifications.filter((item) => item.id !== notification.id);
    }, delay);
    notificationTimers = [...notificationTimers, timer];
  }

  function focusWidget(focus: DisplayFocusWidget) {
    focusedWidgetId = focus.widgetInstanceId;
    if (mode === 'chat') {
      mode = 'dashboard';
    }
    clearFocusTimer();
    focusTimer = window.setTimeout(() => {
      focusedWidgetId = '';
      focusTimer = undefined;
    }, 4500);
    window.setTimeout(() => {
      document.querySelector(`[data-widget-id="${cssEscape(focus.widgetInstanceId)}"]`)?.scrollIntoView({
        block: 'center',
        behavior: 'smooth'
      });
    }, 0);
  }

  function clearFocusTimer() {
    if (focusTimer) {
      window.clearTimeout(focusTimer);
      focusTimer = undefined;
    }
  }

  function cssEscape(value: string) {
    return typeof CSS !== 'undefined' && CSS.escape ? CSS.escape(value) : value.replace(/"/g, '\\"');
  }

  async function submitMessage(text: string, retryMessageId?: string) {
    const agent = agents.find((item) => item.id === selectedAgentId);
    if (!agent || !isAgentAvailable(agent)) {
      chatState = 'error';
      messages = [...messages, systemMessage('No available agent is connected yet.')];
      return;
    }

    const temporaryMessageId = retryMessageId ?? makeID();
    chatState = 'thinking';
    let streamedOutput = false;
    let completed = false;
    let failedFromStream = false;
    const assistantMessageId = makeID();

    try {
      const conversationId = await ensureConversation(agent);
      messages = [
        ...messages.filter((message) => message.id !== retryMessageId),
        makeMessage('user', text, {
          id: temporaryMessageId,
          conversationId,
          status: 'sent',
          retryText: text,
          agentId: agent.id
        })
      ];
      await sendConversationTurnStream(fetch, conversationId, agent.id, text, (event) => {
        if (event.type === 'turn_started') {
          chatState = 'thinking';
          return;
        }
        if (event.type === 'assistant_delta') {
          streamedOutput = true;
          chatState = 'streaming';
          upsertAssistantDelta(assistantMessageId, event);
          return;
        }
        if (event.type === 'status_changed') {
          chatState = event.status === 'completed' ? 'streaming' : 'thinking';
          return;
        }
        if (event.type === 'turn_completed') {
          completed = true;
          applyConversationDetail({
            conversation: event.conversation,
            messages: event.messages
          });
          return;
        }
        if (event.type === 'turn_failed') {
          failedFromStream = true;
          throw new Error(event.message);
        }
      });
      if (!completed) {
        throw new Error('stream ended before completion');
      }
      markConnected();
    } catch {
      if (!streamedOutput && !failedFromStream) {
        try {
          const conversationId = selectedConversationId || (await ensureConversation(agent));
          const detail = await sendConversationTurn(fetch, conversationId, agent.id, text);
          applyConversationDetail(detail);
          markConnected();
          return;
        } catch {
          // Fall through to the standard retryable failure state.
        }
      }
      messages = messages.map((message) =>
        message.id === temporaryMessageId ? { ...message, status: 'failed', retryText: text } : message
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

  function upsertAssistantDelta(messageId: string, event: Extract<ConversationStreamEvent, { type: 'assistant_delta' }>) {
    let found = false;
    messages = messages.map((message) => {
      if (message.id !== messageId) {
        return message;
      }
      found = true;
      return {
        ...message,
        conversationId: event.conversationId || message.conversationId,
        content: event.append ? message.content + event.text : event.text,
        status: 'streaming',
        agentId: event.agentId || message.agentId
      };
    });
    if (!found) {
      messages = [
        ...messages,
        makeMessage('assistant', event.text, {
          id: messageId,
          conversationId: event.conversationId,
          status: 'streaming',
          agentId: event.agentId
        })
      ];
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
      await loadConversationHistory();
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
      agentId: overrides.agentId,
      conversationId: overrides.conversationId
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
      {focusedWidgetId}
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
      onManageAgents={() => (showAgentManager = true)}
    />

    {#if showAgentManager}
      <div class="agent-manager-layer">
        <section class="agent-manager" aria-label="Agent settings">
          <header class="agent-manager-header">
            <div>
              <strong>Agents</strong>
              <span>Add A2A agents by Agent Card URL. Jute writes them to the active YAML config.</span>
            </div>
            <button type="button" class="agent-manager-close" on:click={() => (showAgentManager = false)}>Close</button>
          </header>
          <form class="agent-add-form" on:submit|preventDefault={saveAgentFromCard}>
            <input bind:value={agentCardUrl} placeholder="http://127.0.0.1:9797/.well-known/agent-card.json" />
            <button type="submit" disabled={savingAgent || !agentCardUrl.trim()}>{savingAgent ? 'Adding' : 'Add agent'}</button>
          </form>
          {#if agentIssue}
            <p class="agent-manager-issue">{agentIssue}</p>
          {/if}
          <div class="agent-manager-list">
            {#if agents.length === 0}
              <p>No agents configured yet.</p>
            {:else}
              {#each agents as agent}
                <article class="agent-manager-item">
                  <div>
                    <strong>{agent.name}</strong>
                    <span>{agent.cardUrl}</span>
                  </div>
                  <div class="agent-manager-actions">
                    <button type="button" on:click={() => toggleAgent(agent)}>{agent.enabled ? 'Disable' : 'Enable'}</button>
                    <button type="button" on:click={() => removeAgent(agent)}>Remove</button>
                  </div>
                </article>
              {/each}
            {/if}
          </div>
        </section>
      </div>
    {/if}

    {#if mode === 'chat'}
      <div class="chat-layer">
        <ChatView
          {agents}
          {messages}
          {conversations}
          state={chatState}
          voice={dashboard.voice}
          {voiceIssue}
          {selectedAgentId}
          {selectedConversationId}
          {selectedAvailability}
          onAgentChange={(agentId) => {
            selectedAgentId = agentId;
            selectedConversationId = '';
            messages = [];
            chatState = 'idle';
            void loadConversationHistory('');
          }}
          onConversationSelect={(conversationId) => loadConversation(conversationId)}
          onNewConversation={createNewConversation}
          onManageAgents={() => (showAgentManager = true)}
          onSubmit={(value) => submitMessage(value)}
          onRetry={retryMessage}
          onClose={closeChat}
          onCancel={cancelChatTurn}
          onToggleVoiceMute={toggleVoiceMute}
        />
      </div>
    {/if}

    {#if displayNotifications.length > 0}
      <div class="display-notification-stack" aria-live="polite" aria-label="Display notifications">
        {#each displayNotifications as notification (notification.id)}
          <section class={`display-notification display-notification--${notification.severity}`}>
            <strong>{notification.severity}</strong>
            <span>{notification.message}</span>
          </section>
        {/each}
      </div>
    {/if}
  {/if}
</main>
