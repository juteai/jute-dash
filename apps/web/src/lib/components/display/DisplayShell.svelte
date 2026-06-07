<script lang="ts">
  import { browser } from '$app/environment';
  import { onMount } from 'svelte';
  import { Mic, MicOff, X } from 'lucide-svelte';
  import ChatView from '$lib/components/chat/ChatView.svelte';
  import DashboardView from '$lib/components/display/DashboardView.svelte';
  import ConversationOrb from '$lib/components/chat/ConversationOrb.svelte';
  import SettingsPanel from '$lib/components/settings/SettingsPanel.svelte';
  import { fade } from 'svelte/transition';
  import OfflineState from '$lib/components/display/OfflineState.svelte';
  import StatusRibbon from '$lib/components/display/StatusRibbon.svelte';
  import {
    addAgent,
    backgroundImageURL,
    createConversation,
    deleteAgent,
    eventsURL,
    getConversation,
    getConversations,
    getDashboard,
    getHouseholdSettings,
    getRoomSettings,
    getTileSettings,
    getWidgetCatalog,
    muteVoice,
    refreshAgentCard,
    resetWidgetLayout,
    saveHouseholdSettings,
    saveRoomSettings,
    saveTileSettings,
    saveWidgetLayout,
    sendConversationTurn,
    sendConversationTurnStream,
    setAgentEnabled,
    cancelVoice,
    unmuteVoice
  } from '$lib/api';
  import { logger } from '$lib/logger';
  import {
    firstAvailableAgent,
    getAgentAvailability,
    isAgentAvailable
  } from '$lib/agents';
  import { displayThemeStyle, resolveColorMode } from '$lib/themes';
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
    HouseholdSettings,
    Room,
    Tile,
    UserFacingIssue,
    WidgetCatalogItem,
    WidgetLayout
  } from '$lib/types';
  import { cn } from '$lib/utils';
  import {
    cloneLayout,
    addWidget as editorAddWidget,
    moveWidget as editorMoveWidget,
    resizeWidget as editorResizeWidget,
    removeWidget as editorRemoveWidget,
    setWidgetMode as editorSetWidgetMode,
    reorderWidget as editorReorderWidget,
    updateWidget as editorUpdateWidget
  } from '$lib/layout-editor';
  import WidgetSettingsSheet from '$lib/components/display/WidgetSettingsSheet.svelte';

  export let data: DashboardData;

  type VoiceDisplayEvent = {
    id: string;
    type: string;
    createdAt: string;
    deviceId: string;
    conversationId?: string;
    payload?: Record<string, unknown>;
  };

  let dashboard: DashboardData = data;
  let dashboardForView: DashboardData = data;
  let draftLayout: WidgetLayout | undefined;
  let configuringWidgetId = '';
  let widgetCatalog: WidgetCatalogItem[] = [];
  let editIssue = '';
  let savingLayout = false;
  let mode: DisplayMode = 'dashboard';
  let chatState: ChatState = 'idle';
  let assistantStatusText = '';
  let activeChatTurn: AbortController | undefined;
  let messages: ChatMessage[] = [];
  let conversations: Conversation[] = [];
  let selectedConversationId = '';
  let showAgentManager = false;
  let agentCardUrl = '';
  let agentIssue = '';
  let savingAgent = false;
  let householdSettings: HouseholdSettings | undefined;
  let roomSettings: Room[] = [];
  let tileSettings: Tile[] = [];
  let settingsIssue = '';
  let savingSettings = false;
  let savingRooms = false;
  let savingTiles = false;
  let activeSettingsSection:
    | 'household'
    | 'rooms'
    | 'tiles'
    | 'agents'
    | 'mcp'
    | 'voice'
    | 'about' = 'household';
  let mounted = false;
  let historyAgentId = '';
  let voiceIssue = '';
  let selectedAgentId = '';
  let prefersDark = false;
  let hasConnected = data.connectionState === 'connected';
  let retrying = false;
  let longPressTimer: number | undefined;
  let slideshowIndex = 0;
  let slideshowTimer: number | undefined;
  let eventSource: EventSource | undefined;
  let pollingTimer: number | undefined;
  let displayNotifications: DisplayNotification[] = [];
  let notificationTimers: number[] = [];
  let focusedWidgetId = '';
  let focusTimer: number | undefined;

  let voiceOrbState:
    | 'idle'
    | 'listening'
    | 'thinking'
    | 'speaking'
    | 'followup' = 'idle';
  let voiceTranscript = '';
  let assistantSpeech = '';
  let showVoiceOverlay = false;
  let voiceEndedTimeout: number | undefined;

  $: agents = dashboard.agents;
  $: availableAgent = firstAvailableAgent(agents);
  $: fallbackAgent =
    availableAgent ?? agents.find((agent) => agent.enabled) ?? agents[0];
  $: if (!selectedAgentId && agents.length > 0) {
    selectedAgentId = fallbackAgent?.id ?? '';
  }
  $: if (
    selectedAgentId &&
    agents.length > 0 &&
    !agents.some((agent) => agent.id === selectedAgentId)
  ) {
    selectedAgentId = fallbackAgent?.id ?? '';
  }
  $: selectedAgent = agents.find((agent) => agent.id === selectedAgentId);
  $: selectedAvailability = getAgentAvailability(selectedAgent);
  $: hasConnected = hasConnected || dashboard.connectionState === 'connected';
  $: showStartupOffline =
    !hasConnected && dashboard.connectionState === 'offline';
  $: showStartupLoading =
    !hasConnected && dashboard.connectionState === 'starting';
  $: activeTheme = resolveColorMode(dashboard.config.display, prefersDark);
  $: backgroundConfig = dashboard.config.display.background;
  $: slideshowImages =
    backgroundConfig?.kind === 'slideshow'
      ? (backgroundConfig.images ?? [])
      : [];
  $: currentBackgroundImage = resolveBackgroundImage(
    backgroundConfig,
    slideshowIndex
  );
  $: displayStyle = displayThemeStyle(
    dashboard.config.display,
    activeTheme,
    currentBackgroundImage
  );
  $: manageSlideshow(
    slideshowImages,
    backgroundConfig?.intervalSeconds,
    dashboard.config.display.motion
  );

  function resolveBackgroundImage(
    bg: typeof backgroundConfig,
    index: number
  ): string {
    if (!bg) {
      return '';
    }
    if (bg.kind === 'file' && bg.value) {
      return backgroundImageURL(bg.value);
    }
    if (bg.kind === 'asset' && bg.value) {
      return bg.value;
    }
    if (bg.kind === 'slideshow') {
      const images = bg.images ?? [];
      if (images.length > 0) {
        return backgroundImageURL(images[index % images.length]);
      }
    }
    return '';
  }

  function manageSlideshow(
    images: string[],
    intervalSeconds: number | undefined,
    motion: string
  ) {
    if (!browser) {
      return;
    }
    if (slideshowTimer) {
      window.clearInterval(slideshowTimer);
      slideshowTimer = undefined;
    }
    if (images.length > 1 && motion !== 'none') {
      const delay = Math.max(3, intervalSeconds || 30) * 1000;
      slideshowTimer = window.setInterval(() => {
        slideshowIndex = (slideshowIndex + 1) % images.length;
      }, delay);
    }
  }
  $: dashboardForView = {
    ...dashboard,
    layout: mode === 'edit' && draftLayout ? draftLayout : dashboard.layout
  };
  $: configuringWidget =
    configuringWidgetId && draftLayout
      ? draftLayout.widgets.find((widget) => widget.id === configuringWidgetId)
      : undefined;
  $: configuringCatalogItem = configuringWidget
    ? widgetCatalog.find((item) => item.kind === configuringWidget?.kind)
    : undefined;
  $: if (
    mounted &&
    selectedAgent &&
    isAgentAvailable(selectedAgent) &&
    selectedAgentId !== historyAgentId
  ) {
    // eslint-disable-next-line svelte/infinite-reactive-loop
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
    void retryDashboard();
    startPolling();

    return () => {
      mounted = false;
      query.removeEventListener('change', updateTheme);
      clearLongPress();
      disconnectDisplayEvents();
      stopPolling();
      clearFocusTimer();
      if (slideshowTimer) {
        window.clearInterval(slideshowTimer);
      }
    };
  });

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

  async function loadConversationHistory(
    preferredConversationId = selectedConversationId,
    agentOverride = selectedAgentId
  ) {
    try {
      const agentId =
        agentOverride ||
        selectedAgentId ||
        firstAvailableAgent(dashboard.agents)?.id ||
        dashboard.agents.find((agent) => agent.enabled)?.id ||
        dashboard.agents[0]?.id ||
        '';
      historyAgentId = agentId;
      const agent = dashboard.agents.find((item) => item.id === agentId);
      if (!agentId || !agent || !isAgentAvailable(agent)) {
        conversations = [];
        messages = [];
        selectedConversationId = '';
        return;
      }
      const loaded = await getConversations(fetch, agentId);
      conversations = loaded;
      const candidate =
        loaded.find(
          (conversation) => conversation.id === preferredConversationId
        ) ?? loaded.find((conversation) => !conversation.historyUnsupported);
      if (candidate) {
        // eslint-disable-next-line svelte/infinite-reactive-loop
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

  async function loadConversation(
    conversationId: string,
    agentId = selectedAgentId
  ) {
    const detail = await getConversation(fetch, conversationId, agentId);
    // eslint-disable-next-line svelte/infinite-reactive-loop
    applyConversationDetail(detail);
  }

  function applyConversationDetail(detail: ConversationDetail) {
    selectedConversationId = detail.conversation.id;
    // eslint-disable-next-line svelte/infinite-reactive-loop
    selectedAgentId = detail.conversation.agentId || selectedAgentId;
    messages = detail.messages.map(conversationMessageToChatMessage);
    chatState =
      detail.conversation.status === 'streaming'
        ? 'streaming'
        : detail.conversation.status === 'failed'
          ? 'error'
          : 'idle';
    conversations = upsertConversation(conversations, detail.conversation);
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
      agentId: message.agentId
    };
  }

  function upsertConversation(
    existing: Conversation[],
    conversation: Conversation
  ) {
    const withoutCurrent = existing.filter(
      (item) => item.id !== conversation.id
    );
    return [conversation, ...withoutCurrent].sort((a, b) =>
      b.updatedAt.localeCompare(a.updatedAt)
    );
  }

  async function ensureConversation(agent: Agent) {
    if (selectedConversationId) {
      const current = conversations.find(
        (conversation) => conversation.id === selectedConversationId
      );
      if (current?.agentId === agent.id) {
        return selectedConversationId;
      }
    }
    const detail = await createConversation(fetch, agent.id);
    applyConversationDetail(detail);
    return detail.conversation.id;
  }

  async function createNewConversation() {
    const agent =
      agents.find((item) => item.id === selectedAgentId) ?? availableAgent;
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
    settingsIssue = '';
    try {
      const agent = await addAgent(fetch, cardUrl);
      selectedAgentId = agent.id;
      agentCardUrl = '';
      dashboard = await getDashboard(fetch);
      await loadConversationHistory();
      markConnected();
    } catch {
      agentIssue =
        'Agent was not added. Check the Agent Card URL and that the hub was started with a YAML config.';
      settingsIssue = agentIssue;
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
      settingsIssue = agentIssue;
    }
  }

  async function removeAgent(agent: Agent) {
    try {
      await deleteAgent(fetch, agent.id);
      dashboard = await getDashboard(fetch);
      if (selectedAgentId === agent.id) {
        selectedAgentId =
          firstAvailableAgent(dashboard.agents)?.id ??
          dashboard.agents[0]?.id ??
          '';
        selectedConversationId = '';
        messages = [];
        conversations = [];
      }
      markConnected();
    } catch {
      agentIssue = 'Agent could not be removed.';
      settingsIssue = agentIssue;
    }
  }

  async function openSettings(
    section: typeof activeSettingsSection = 'household'
  ) {
    activeSettingsSection = section;
    showAgentManager = true;
    settingsIssue = '';
    try {
      const [household, rooms, tiles] = await Promise.all([
        getHouseholdSettings(fetch),
        getRoomSettings(fetch),
        getTileSettings(fetch)
      ]);
      householdSettings = household;
      roomSettings = rooms;
      tileSettings = tiles;
      markConnected();
    } catch {
      settingsIssue =
        'Settings are unavailable. Check that the hub is running.';
      markIssue('degraded', {
        code: 'settings_unavailable',
        severity: 'warning',
        title: 'Settings unavailable',
        message: 'Jute could not load editable settings.'
      });
    }
  }

  async function saveSettings(settings: HouseholdSettings) {
    if (savingSettings) {
      return;
    }
    savingSettings = true;
    settingsIssue = '';
    try {
      householdSettings = await saveHouseholdSettings(fetch, settings);
      dashboard = await getDashboard(fetch);
      markConnected();
    } catch {
      settingsIssue =
        'Settings were not saved. Check required fields and try again.';
      markIssue('degraded', {
        code: 'settings_save_failed',
        severity: 'warning',
        title: 'Settings not saved',
        message: 'Jute could not save household settings.'
      });
    } finally {
      savingSettings = false;
    }
  }

  async function saveRooms(rooms: Room[]) {
    if (savingRooms) {
      return;
    }
    savingRooms = true;
    settingsIssue = '';
    try {
      roomSettings = await saveRoomSettings(fetch, rooms);
      dashboard = await getDashboard(fetch);
      markConnected();
    } catch {
      settingsIssue =
        'Rooms were not saved. Check required fields and try again.';
      markIssue('degraded', {
        code: 'rooms_save_failed',
        severity: 'warning',
        title: 'Rooms not saved',
        message: 'Jute could not save room settings.'
      });
    } finally {
      savingRooms = false;
    }
  }

  async function saveTiles(tiles: Tile[]) {
    if (savingTiles) {
      return;
    }
    savingTiles = true;
    settingsIssue = '';
    try {
      tileSettings = await saveTileSettings(fetch, tiles);
      dashboard = await getDashboard(fetch);
      markConnected();
    } catch {
      settingsIssue =
        'Tiles were not saved. Check required fields and try again.';
      markIssue('degraded', {
        code: 'tiles_save_failed',
        severity: 'warning',
        title: 'Tiles not saved',
        message: 'Jute could not save dashboard tile settings.'
      });
    } finally {
      savingTiles = false;
    }
  }

  async function refreshSelectedAgentCard(agentId: string) {
    if (!agentId) {
      return;
    }
    try {
      const refreshed = await refreshAgentCard(fetch, agentId);
      dashboard = {
        ...dashboard,
        agents: dashboard.agents.map((agent) =>
          agent.id === refreshed.id ? refreshed : agent
        ),
        connectionState: 'connected',
        stale: false,
        issue: undefined,
        loadedAt: new Date().toISOString()
      };
      markConnected();
    } catch {
      markIssue('degraded', {
        code: 'agent_card_refresh_failed',
        severity: 'warning',
        title: 'Agent Card refresh failed',
        message: 'Jute could not refresh the selected agent card.'
      });
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
        editIssue =
          'Widget catalog is unavailable. Existing widgets can still be adjusted.';
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
      const saved = await saveWidgetLayout(fetch, draftLayout);
      dashboard = {
        ...dashboard,
        layout: saved,
        connectionState: 'connected',
        stale: false,
        issue: undefined,
        loadedAt: new Date().toISOString()
      };
      draftLayout = undefined;
      configuringWidgetId = '';
      mode = 'dashboard';
    } catch {
      editIssue =
        'Layout was not saved. Check that the hub is running, then try again.';
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
    configuringWidgetId = '';
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
      const reset = await resetWidgetLayout(
        fetch,
        dashboardForView.layout.profileId
      );
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

  function addWidget(kind: string, mode: 'ui' | 'headless' = 'ui') {
    if (!draftLayout) {
      return;
    }
    const item = widgetCatalog.find((candidate) => candidate.kind === kind);
    if (!item) {
      editIssue = 'That widget is not available in this display build.';
      return;
    }
    const res = editorAddWidget(draftLayout, item, mode);
    draftLayout = res.layout;
    editIssue = res.error || '';
  }

  function setWidgetHeadless(widgetId: string) {
    if (!draftLayout) {
      return;
    }
    draftLayout = editorSetWidgetMode(draftLayout, widgetId, 'headless');
  }

  function restoreWidget(widgetId: string) {
    if (!draftLayout) {
      return;
    }
    draftLayout = editorSetWidgetMode(draftLayout, widgetId, 'ui');
  }

  function reorderWidget(widgetId: string, direction: -1 | 1) {
    if (!draftLayout) {
      return;
    }
    draftLayout = editorReorderWidget(draftLayout, widgetId, direction);
  }

  function openWidgetConfig(widgetId: string) {
    configuringWidgetId = widgetId;
  }

  function saveWidgetConfig(patch: {
    title: string;
    settings: Record<string, unknown>;
    mode: 'ui' | 'headless';
  }) {
    if (!draftLayout || !configuringWidgetId) {
      return;
    }
    let next = editorUpdateWidget(draftLayout, configuringWidgetId, {
      title: patch.title,
      settings: patch.settings
    });
    next = editorSetWidgetMode(next, configuringWidgetId, patch.mode);
    draftLayout = next;
    configuringWidgetId = '';
  }

  function moveWidget(widgetId: string, x: number, y: number) {
    if (!draftLayout) {
      return;
    }
    draftLayout = editorMoveWidget(draftLayout, widgetId, x, y);
  }

  function resizeWidget(widgetId: string, w: number, h: number) {
    if (!draftLayout) {
      return;
    }
    draftLayout = editorResizeWidget(draftLayout, widgetId, w, h);
  }

  function removeWidget(widgetId: string) {
    if (!draftLayout) {
      return;
    }
    draftLayout = editorRemoveWidget(draftLayout, widgetId);
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

  function startPolling() {
    if (!browser || pollingTimer) {
      return;
    }
    pollingTimer = window.setInterval(async () => {
      await pollStatus();
    }, 10000);
  }

  function stopPolling() {
    if (pollingTimer) {
      window.clearInterval(pollingTimer);
      pollingTimer = undefined;
    }
  }

  async function pollStatus() {
    try {
      const fresh = await getDashboard(fetch);
      if (
        dashboard.connectionState === 'offline' ||
        dashboard.connectionState === 'reconnecting'
      ) {
        dashboard = fresh;
        markConnected();
      } else {
        dashboard = {
          ...fresh,
          connectionState: dashboard.connectionState,
          issue: dashboard.issue
        };
      }
    } catch {
      if (hasConnected) {
        markIssue('reconnecting', {
          code: 'hub_unreachable',
          severity: 'error',
          title: 'Hub not reachable',
          message: `Jute Dash cannot connect to the local hub at ${dashboard.hubUrl}.`,
          action: {
            label: 'Retry',
            target: 'retry'
          }
        });
      } else {
        markIssue('offline', {
          code: 'hub_unreachable',
          severity: 'error',
          title: 'Hub not reachable',
          message: `Jute Dash cannot connect to the local hub at ${dashboard.hubUrl}.`,
          action: {
            label: 'Retry',
            target: 'retry'
          }
        });
      }
    }
  }

  function connectDisplayEvents() {
    if (!browser || eventSource) {
      return;
    }
    eventSource = new EventSource(eventsURL());
    eventSource.addEventListener('open', async () => {
      logger.sse('Connected');
      if (hasConnected) {
        try {
          const fresh = await getDashboard(fetch);
          dashboard = fresh;
        } catch {
          // ignore
        }
        markConnected();
      }
    });
    eventSource.addEventListener('error', async () => {
      logger.sseError('Event stream connection lost or failed');
      if (mounted && hasConnected) {
        try {
          const fresh = await getDashboard(fetch);
          dashboard = {
            ...fresh,
            connectionState: 'degraded',
            stale: false,
            issue: {
              code: 'event_stream_disconnected',
              severity: 'warning',
              title: 'Event stream disconnected',
              message:
                'Jute lost the live display event stream. Dashboard data may be stale.'
            }
          };
        } catch {
          markIssue('reconnecting', {
            code: 'hub_unreachable',
            severity: 'error',
            title: 'Hub not reachable',
            message: `Jute Dash cannot connect to the local hub at ${dashboard.hubUrl}.`,
            action: {
              label: 'Retry',
              target: 'retry'
            }
          });
        }
      }
    });
    eventSource.addEventListener('display.notification', (event) => {
      logger.sse(event.type);
      const notification = parseDisplayEvent<DisplayNotification>(event);
      if (notification) {
        addDisplayNotification(notification);
      }
    });
    eventSource.addEventListener('display.focus_widget', (event) => {
      logger.sse(event.type);
      const focus = parseDisplayEvent<DisplayFocusWidget>(event);
      if (focus) {
        focusWidget(focus);
      }
    });
    eventSource.addEventListener('voice.state_changed', (event) => {
      const e = parseDisplayEvent<VoiceDisplayEvent>(event);
      logger.sse(
        event.type,
        e?.payload ? `state=${e.payload.state}` : undefined
      );
      if (e?.payload) {
        const payload = e.payload;
        dashboard = {
          ...dashboard,
          voice: {
            ...dashboard.voice,
            enabled: Boolean(payload.enabled),
            muted: Boolean(payload.muted),
            state:
              typeof payload.state === 'string'
                ? (payload.state as DashboardData['voice']['state'])
                : dashboard.voice.state,
            serviceStatus:
              typeof payload.serviceStatus === 'string'
                ? (payload.serviceStatus as DashboardData['voice']['serviceStatus'])
                : dashboard.voice.serviceStatus
          }
        };
      }
    });
    eventSource.addEventListener('voice.wake_detected', (event) => {
      logger.sse(event.type);
      if (voiceEndedTimeout) {
        window.clearTimeout(voiceEndedTimeout);
        voiceEndedTimeout = undefined;
      }
      voiceOrbState = 'listening';
      voiceTranscript = '';
      assistantSpeech = '';
      showVoiceOverlay = true;
    });
    eventSource.addEventListener('voice.transcript.partial', (event) => {
      const e = parseDisplayEvent<VoiceDisplayEvent>(event);
      const text = e?.payload?.text;
      logger.sse(event.type, text ? `text="${text}"` : undefined);
      if (typeof text === 'string') {
        voiceTranscript = text;
      }
      voiceOrbState = 'listening';
    });
    eventSource.addEventListener('voice.transcript.final', (event) => {
      const e = parseDisplayEvent<VoiceDisplayEvent>(event);
      const text = e?.payload?.text;
      logger.sse(event.type, text ? `text="${text}"` : undefined);
      if (typeof text === 'string') {
        voiceTranscript = text;
      }
      voiceOrbState = 'listening';
    });
    eventSource.addEventListener('conversation.turn_started', (event) => {
      logger.sse(event.type);
      voiceOrbState = 'thinking';
    });
    eventSource.addEventListener('conversation.turn_completed', (event) => {
      const e = parseDisplayEvent<VoiceDisplayEvent>(event);
      const speech = e?.payload?.speech;
      const text = e?.payload?.text;
      logger.sse(event.type, text ? `text="${text}"` : undefined);
      if (typeof speech === 'string') {
        assistantSpeech = speech;
      } else if (typeof text === 'string') {
        assistantSpeech = text;
      }
      voiceOrbState = 'speaking';
    });
    eventSource.addEventListener('conversation.followup_started', (event) => {
      logger.sse(event.type);
      voiceOrbState = 'followup';
      voiceTranscript = '';
    });
    eventSource.addEventListener('conversation.ended', (event) => {
      logger.sse(event.type);
      voiceOrbState = 'idle';
      if (voiceEndedTimeout) {
        window.clearTimeout(voiceEndedTimeout);
      }
      voiceEndedTimeout = window.setTimeout(() => {
        if (voiceOrbState === 'idle') {
          showVoiceOverlay = false;
          voiceTranscript = '';
          assistantSpeech = '';
        }
      }, 4000);
    });
    eventSource.addEventListener('hub.connected', (event) => {
      logger.sse(event.type);
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
    if (voiceEndedTimeout) {
      window.clearTimeout(voiceEndedTimeout);
      voiceEndedTimeout = undefined;
    }
  }

  function parseDisplayEvent<T>(event: Event): T | undefined {
    try {
      return JSON.parse((event as MessageEvent).data) as T;
    } catch {
      return undefined;
    }
  }

  function addDisplayNotification(notification: DisplayNotification) {
    displayNotifications = [
      notification,
      ...displayNotifications.filter((item) => item.id !== notification.id)
    ].slice(0, 3);
    const expiry = Date.parse(notification.expiresAt);
    const delay = Number.isFinite(expiry)
      ? Math.max(2500, expiry - Date.now())
      : 6000;
    const timer = window.setTimeout(() => {
      displayNotifications = displayNotifications.filter(
        (item) => item.id !== notification.id
      );
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
      document
        .querySelector(
          `[data-widget-id="${cssEscape(focus.widgetInstanceId)}"]`
        )
        ?.scrollIntoView({
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
    return typeof CSS !== 'undefined' && CSS.escape
      ? CSS.escape(value)
      : value.replace(/"/g, '\\"');
  }

  async function submitMessage(text: string, retryMessageId?: string) {
    const agent = agents.find((item) => item.id === selectedAgentId);
    if (!agent || !isAgentAvailable(agent)) {
      chatState = 'error';
      messages = [
        ...messages,
        systemMessage('No available agent is connected yet.')
      ];
      return;
    }

    const temporaryMessageId = retryMessageId ?? makeID();
    chatState = 'thinking';
    assistantStatusText = '';
    let completed = false;
    let canceled = false;
    let streamFailure: Error | undefined;
    const assistantMessageId = makeID();
    activeChatTurn?.abort();
    const turnController = new AbortController();
    activeChatTurn = turnController;

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
      if (agent.streaming) {
        await sendConversationTurnStream(
          fetch,
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
            if (event.type === 'turn_started') {
              chatState = 'thinking';
              return;
            }
            if (event.type === 'assistant_delta') {
              chatState = 'streaming';
              upsertAssistantDelta(assistantMessageId, event);
              return;
            }
            if (event.type === 'status_changed') {
              chatState =
                event.status === 'completed' ? 'streaming' : 'thinking';
              assistantStatusText = event.text || '';
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
              streamFailure = new Error(event.message);
              return;
            }
            if (event.type === 'turn_canceled') {
              canceled = true;
              chatState = 'idle';
              assistantStatusText = '';
            }
          },
          { signal: turnController.signal }
        );
      } else {
        const detail = await sendConversationTurn(
          fetch,
          conversationId,
          agent.id,
          text,
          { signal: turnController.signal }
        );
        completed = true;
        applyConversationDetail(detail);
      }
      if (canceled || turnController.signal.aborted) {
        return;
      }
      if (streamFailure) {
        throw streamFailure;
      }
      if (!completed) {
        throw new Error('stream ended before completion');
      }
      markConnected();
    } catch (err) {
      if (canceled || turnController.signal.aborted) {
        return;
      }
      messages = messages.map((message) =>
        message.id === temporaryMessageId
          ? { ...message, status: 'failed', retryText: text }
          : message
      );
      messages = [...messages, systemMessage(chatFailureMessage(err))];
      markIssue('degraded', {
        code: 'message_failed',
        severity: 'warning',
        title: 'Message not sent',
        message: chatFailureIssueMessage(err)
      });
      chatState = 'error';
    } finally {
      if (activeChatTurn === turnController) {
        activeChatTurn = undefined;
      }
    }
  }

  function upsertAssistantDelta(
    messageId: string,
    event: Extract<ConversationStreamEvent, { type: 'assistant_delta' }>
  ) {
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
    activeChatTurn?.abort();
    assistantStatusText = '';
    chatState = 'idle';
  }

  async function toggleVoiceMute() {
    if (dashboard.voice.serviceStatus !== 'ready') {
      voiceIssue =
        'Voice is not configured yet. Add an STT provider before using microphone controls.';
      return;
    }
    try {
      const voice = dashboard.voice.muted
        ? await unmuteVoice(fetch)
        : await muteVoice(fetch);
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
      voiceIssue =
        'Voice state could not be updated. Check that the hub is running, then try again.';
      markIssue('degraded', {
        code: 'voice_update_failed',
        severity: 'warning',
        title: 'Voice not updated',
        message: 'Jute could not update the voice mute state.'
      });
    }
  }

  async function cancelVoiceSession() {
    try {
      await cancelVoice(window.fetch);
      voiceOrbState = 'idle';
      showVoiceOverlay = false;
    } catch (err) {
      console.error('Failed to cancel voice session:', err);
    }
  }

  async function retryDashboard() {
    if (!browser || retrying) {
      return;
    }
    retrying = true;
    try {
      dashboard = await getDashboard(fetch);
      const agent = firstAvailableAgent(dashboard.agents);
      if (agent) {
        await loadConversationHistory('', agent.id);
      }
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
    connectDisplayEvents();
  }

  function markIssue(
    connectionState: DashboardData['connectionState'],
    issue: UserFacingIssue
  ) {
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

  function makeID() {
    if (
      browser &&
      'crypto' in window &&
      typeof window.crypto.randomUUID === 'function'
    ) {
      return window.crypto.randomUUID();
    }
    return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
</script>

<svelte:head>
  <title>{dashboard.config.home.name} · Jute Dash</title>
</svelte:head>

<main
  class={cn(
    'display-root',
    mode === 'chat' && 'display-root--chat',
    dashboard.stale && 'display-root--stale'
  )}
  data-theme={activeTheme}
  data-theme-id={dashboard.config.display.themeId}
  data-background-overlay={dashboard.config.display.background?.overlay ??
    'none'}
  style={displayStyle}
  on:pointerdown={startLongPress}
  on:pointerup={clearLongPress}
  on:pointercancel={clearLongPress}
  on:pointerleave={clearLongPress}
>
  {#if showStartupLoading}
    <section class="startup-state" aria-label="Connecting to Jute hub">
      <div class="startup-mark">{dashboard.config.home.name}</div>
      <div>
        <strong>Connecting to local hub</strong>
        <span>{dashboard.hubUrl}</span>
      </div>
    </section>
  {:else if showStartupOffline}
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
      stale={dashboard.stale}
      {selectedAgent}
      {selectedAvailability}
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
      onConfigureWidget={openWidgetConfig}
      onSetHeadless={setWidgetHeadless}
      onRestoreWidget={restoreWidget}
      onReorderWidget={reorderWidget}
      onManageAgents={() => openSettings('household')}
    />

    {#if configuringWidget}
      <WidgetSettingsSheet
        widget={configuringWidget}
        catalogItem={configuringCatalogItem}
        onClose={() => (configuringWidgetId = '')}
        onSave={saveWidgetConfig}
      />
    {/if}

    {#if showAgentManager}
      <SettingsPanel
        {agents}
        status={dashboard.status}
        voice={dashboard.voice}
        settings={householdSettings}
        rooms={roomSettings}
        tiles={tileSettings}
        issue={settingsIssue}
        saving={savingSettings}
        {savingRooms}
        {savingTiles}
        {savingAgent}
        {agentCardUrl}
        bind:activeSection={activeSettingsSection}
        onClose={() => (showAgentManager = false)}
        onSaveHousehold={saveSettings}
        onSaveRooms={saveRooms}
        onSaveTiles={saveTiles}
        onAddAgent={(cardUrl) => {
          agentCardUrl = cardUrl;
          return saveAgentFromCard();
        }}
        onToggleAgent={toggleAgent}
        onRemoveAgent={removeAgent}
        onRefreshAgentCard={refreshSelectedAgentCard}
      />
    {/if}

    {#if mode === 'chat'}
      <div class="chat-layer">
        <ChatView
          {agents}
          {messages}
          {conversations}
          state={chatState}
          statusText={assistantStatusText}
          voice={dashboard.voice}
          {voiceIssue}
          {selectedAgentId}
          {selectedConversationId}
          {selectedAvailability}
          status={dashboard.status}
          onAgentChange={(agentId) => {
            selectedAgentId = agentId;
            selectedConversationId = '';
            messages = [];
            chatState = 'idle';
            assistantStatusText = '';
            void loadConversationHistory('');
          }}
          onConversationSelect={(conversationId) =>
            loadConversation(conversationId)}
          onNewConversation={createNewConversation}
          onManageAgents={() => openSettings('agents')}
          onRefreshAgentCard={refreshSelectedAgentCard}
          onSubmit={(value) => submitMessage(value)}
          onRetry={retryMessage}
          onClose={closeChat}
          onCancel={cancelChatTurn}
          onToggleVoiceMute={toggleVoiceMute}
        />
      </div>
    {/if}

    {#if showVoiceOverlay}
      <div class="voice-overlay-container" transition:fade={{ duration: 300 }}>
        <div class="voice-card">
          <div class="voice-content">
            {#if voiceTranscript}
              <div class="bubble user-bubble">
                <span class="bubble-label">You</span>
                <p class="bubble-text">{voiceTranscript}</p>
              </div>
            {/if}

            {#if assistantSpeech}
              <div class="bubble assistant-bubble">
                <span class="bubble-label">Assistant</span>
                <p class="bubble-text">{assistantSpeech}</p>
              </div>
            {/if}

            {#if !voiceTranscript && !assistantSpeech}
              <div class="status-tip">
                {#if voiceOrbState === 'listening'}
                  <span class="status-pulse-dot cyan"></span> Listening...
                {:else if voiceOrbState === 'followup'}
                  <span class="status-pulse-dot yellow"></span> Follow-up listening...
                {:else if voiceOrbState === 'thinking'}
                  <span class="status-pulse-dot purple"></span> Thinking...
                {:else if voiceOrbState === 'speaking'}
                  <span class="status-pulse-dot green"></span> Speaking...
                {/if}
              </div>
            {/if}
          </div>

          <div class="voice-footer">
            <ConversationOrb state={voiceOrbState} />

            <div class="voice-controls">
              <button
                type="button"
                class="control-btn mute-btn {dashboard.voice.muted
                  ? 'muted'
                  : ''}"
                on:click={toggleVoiceMute}
                aria-label={dashboard.voice.muted
                  ? 'Unmute Microphone'
                  : 'Mute Microphone'}
              >
                {#if dashboard.voice.muted}
                  <MicOff size={18} />
                {:else}
                  <Mic size={18} />
                {/if}
              </button>

              <button
                type="button"
                class="control-btn cancel-btn"
                on:click={cancelVoiceSession}
                aria-label="Cancel Voice Session"
              >
                <X size={18} />
              </button>
            </div>
          </div>
        </div>
      </div>
    {/if}

    {#if displayNotifications.length > 0}
      <div
        class="display-notification-stack"
        aria-live="polite"
        aria-label="Display notifications"
      >
        {#each displayNotifications as notification (notification.id)}
          <section
            class={`display-notification display-notification--${notification.severity}`}
          >
            <strong>{notification.severity}</strong>
            <span>{notification.message}</span>
          </section>
        {/each}
      </div>
    {/if}
  {/if}
</main>

<style>
  .voice-overlay-container {
    position: fixed;
    bottom: 24px;
    left: 50%;
    transform: translateX(-50%);
    width: 90%;
    max-width: 480px;
    z-index: 100;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
  }

  .voice-card {
    background: rgba(18, 18, 18, 0.75);
    backdrop-filter: blur(16px) saturate(180%);
    -webkit-backdrop-filter: blur(16px) saturate(180%);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 24px;
    padding: 20px;
    box-shadow:
      0 12px 40px rgba(0, 0, 0, 0.5),
      0 0 0 1px rgba(255, 255, 255, 0.05);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  :global([data-theme='light']) .voice-card {
    background: rgba(255, 255, 255, 0.75);
    border: 1px solid rgba(0, 0, 0, 0.08);
    box-shadow:
      0 12px 40px rgba(0, 0, 0, 0.15),
      0 0 0 1px rgba(0, 0, 0, 0.03);
  }

  .voice-content {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 50px;
    justify-content: center;
  }

  .bubble {
    display: flex;
    flex-direction: column;
    padding: 12px 16px;
    border-radius: 16px;
    font-size: 14px;
    line-height: 1.5;
    max-width: 100%;
    animation: fade-in-up 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  .user-bubble {
    background: rgba(6, 182, 212, 0.12);
    border-left: 3px solid #06b6d4;
    align-self: flex-start;
  }

  :global([data-theme='light']) .user-bubble {
    background: rgba(6, 182, 212, 0.08);
  }

  .assistant-bubble {
    background: rgba(255, 255, 255, 0.06);
    border-left: 3px solid #a855f7;
    align-self: flex-start;
  }

  :global([data-theme='light']) .assistant-bubble {
    background: rgba(0, 0, 0, 0.03);
    border-left: 3px solid #7e22ce;
  }

  .bubble-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    opacity: 0.6;
    margin-bottom: 4px;
  }

  .bubble-text {
    margin: 0;
    font-weight: 500;
    color: #ffffff;
  }

  :global([data-theme='light']) .bubble-text {
    color: #111111;
  }

  .status-tip {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    font-size: 13px;
    color: rgba(255, 255, 255, 0.7);
    font-weight: 500;
  }

  :global([data-theme='light']) .status-tip {
    color: rgba(0, 0, 0, 0.7);
  }

  .status-pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    animation: dot-pulse 1.5s ease-in-out infinite;
  }

  .status-pulse-dot.cyan {
    background-color: #06b6d4;
    box-shadow: 0 0 8px #06b6d4;
  }

  .status-pulse-dot.yellow {
    background-color: #eab308;
    box-shadow: 0 0 8px #eab308;
  }

  .status-pulse-dot.purple {
    background-color: #a855f7;
    box-shadow: 0 0 8px #a855f7;
  }

  .status-pulse-dot.green {
    background-color: #10b981;
    box-shadow: 0 0 8px #10b981;
  }

  .voice-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    padding-top: 12px;
  }

  :global([data-theme='light']) .voice-footer {
    border-top: 1px solid rgba(0, 0, 0, 0.06);
  }

  .voice-controls {
    display: flex;
    gap: 8px;
  }

  .control-btn {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  :global([data-theme='light']) .control-btn {
    border: 1px solid rgba(0, 0, 0, 0.08);
    background: rgba(0, 0, 0, 0.03);
    color: rgba(0, 0, 0, 0.8);
  }

  .control-btn:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #ffffff;
    transform: scale(1.05);
  }

  :global([data-theme='light']) .control-btn:hover {
    background: rgba(0, 0, 0, 0.06);
    color: #000000;
  }

  .mute-btn.muted {
    background: rgba(239, 68, 68, 0.2);
    border-color: rgba(239, 68, 68, 0.4);
    color: #ef4444;
  }

  .mute-btn.muted:hover {
    background: rgba(239, 68, 68, 0.3);
  }

  @keyframes fade-in-up {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @keyframes dot-pulse {
    0%,
    100% {
      opacity: 1;
      transform: scale(1);
    }
    50% {
      opacity: 0.4;
      transform: scale(0.85);
    }
  }
</style>
