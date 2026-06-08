<script lang="ts">
  import { browser } from '$app/environment';
  import { onDestroy, onMount } from 'svelte';
  import ChatView from '$lib/components/chat/ChatView.svelte';
  import DashboardView from '$lib/components/display/DashboardView.svelte';
  import SettingsPanel from '$lib/components/settings/SettingsPanel.svelte';
  import VoiceOverlay from '$lib/components/display/VoiceOverlay.svelte';
  import OfflineState from '$lib/components/display/OfflineState.svelte';
  import StatusRibbon from '$lib/components/display/StatusRibbon.svelte';
  import { backgroundImageURL } from '$lib/hubClient';
  import {
    firstAvailableAgent,
    getAgentAvailability,
    isAgentAvailable
  } from '$lib/agents';
  import { displayThemeStyle, resolveColorMode } from '$lib/themes';
  import type { Agent, DashboardData } from '$lib/types';
  import { cn } from '$lib/utils';
  import WidgetSettingsSheet from '$lib/components/display/WidgetSettingsSheet.svelte';
  import BackgroundRenderer from '$lib/components/display/BackgroundRenderer.svelte';
  import { layoutStore } from '$lib/layoutStore';
  import { hubStream } from '$lib/hubStream';
  import { chatStore } from '$lib/chatStore';
  import { settingsStore } from '$lib/settingsStore';
  import { navigationStore } from '$lib/navigationStore';

  export let data: DashboardData;

  let showAgentManager = false;
  let activeSettingsSection:
    | 'household'
    | 'rooms'
    | 'tiles'
    | 'agents'
    | 'mcp'
    | 'voice'
    | 'appearance'
    | 'about' = 'household';
  let mounted = false;
  let prefersDark = false;
  let longPressTimer: number | undefined;
  let slideshowIndex = 0;
  let slideshowTimer: number | undefined;

  /* eslint-disable no-useless-assignment */
  let lastData: DashboardData | undefined;
  $: if (data && data !== lastData) {
    lastData = data;
    hubStream.init(data);
  }
  /* eslint-enable no-useless-assignment */

  $: availableAgent = firstAvailableAgent($hubStream.dashboard.agents);
  $: fallbackAgent =
    availableAgent ??
    $hubStream.dashboard.agents.find((agent) => agent.enabled) ??
    $hubStream.dashboard.agents[0];
  $: selectedAgent =
    $hubStream.dashboard.agents.find(
      (agent) => agent.id === $chatStore.selectedAgentId
    ) || fallbackAgent;
  $: selectedAvailability = getAgentAvailability(selectedAgent);

  $: activeTheme = resolveColorMode(
    $hubStream.dashboard.config.display,
    prefersDark
  );
  $: backgroundConfig = $hubStream.dashboard.config.display.background;
  $: slideshowImages =
    backgroundConfig?.kind === 'slideshow'
      ? (backgroundConfig.images ?? [])
      : [];
  $: currentBackgroundImage = resolveBackgroundImage(
    backgroundConfig,
    slideshowIndex
  );
  $: displayStyle = displayThemeStyle(
    $hubStream.dashboard.config.display,
    activeTheme,
    currentBackgroundImage
  );
  $: manageSlideshow(
    slideshowImages,
    backgroundConfig?.intervalSeconds,
    $hubStream.dashboard.config.display.motion
  );
  $: weatherData = $hubStream.dashboard.layout.widgets.find(
    (w) => w.kind === 'weather'
  )?.data;

  $: dashboardForView = {
    ...$hubStream.dashboard,
    layout:
      $layoutStore.editMode && $layoutStore.draftLayout
        ? $layoutStore.draftLayout
        : $hubStream.dashboard.layout
  };

  $: configuringWidget =
    $layoutStore.configuringWidgetId && $layoutStore.draftLayout
      ? $layoutStore.draftLayout.widgets.find(
          (widget) => widget.id === $layoutStore.configuringWidgetId
        )
      : undefined;
  $: configuringCatalogItem = configuringWidget
    ? $layoutStore.widgetCatalog.find(
        (item) => item.kind === configuringWidget?.kind
      )
    : undefined;

  $: if (
    mounted &&
    selectedAgent &&
    isAgentAvailable(selectedAgent) &&
    selectedAgent.id !== $chatStore.historyAgentId
  ) {
    void chatStore.loadHistory(
      $hubStream.dashboard.agents,
      '',
      selectedAgent.id,
      fetch
    );
  }

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

  onMount(() => {
    mounted = true;
    const query = window.matchMedia('(prefers-color-scheme: dark)');
    const updateTheme = () => {
      prefersDark = query.matches;
    };
    updateTheme();
    query.addEventListener('change', updateTheme);

    // Initialize layout catalog and stores
    void layoutStore.initCatalog(fetch);
    hubStream.connect(fetch);
    void retryDashboard();

    return () => {
      mounted = false;
      query.removeEventListener('change', updateTheme);
      clearLongPress();
      hubStream.disconnect();
      if (slideshowTimer) {
        window.clearInterval(slideshowTimer);
      }
    };
  });

  onDestroy(() => {
    chatStore.stopTimer();
  });

  async function openChat(agent?: Agent) {
    navigationStore.openChat();
    await chatStore.openChat($hubStream.dashboard.agents, agent, fetch);
  }

  function closeChat() {
    chatStore.closeChat();
  }

  function handleInteraction() {
    chatStore.resetTimer();
  }

  function openSettings(section: typeof activeSettingsSection = 'household') {
    activeSettingsSection = section;
    showAgentManager = true;
  }

  function startLongPress(event: PointerEvent) {
    if (!browser || $navigationStore.mode !== 'dashboard') {
      return;
    }
    const target = event.target as HTMLElement | null;
    if (target?.closest('button, a, input, textarea, select')) {
      return;
    }
    clearLongPress();
    longPressTimer = window.setTimeout(() => {
      layoutStore.enterEdit($hubStream.dashboard.layout);
    }, 650);
  }

  function clearLongPress() {
    if (longPressTimer) {
      window.clearTimeout(longPressTimer);
      longPressTimer = undefined;
    }
  }

  async function submitMessage(text: string, retryMessageId?: string) {
    await chatStore.submit(
      text,
      $hubStream.dashboard.agents,
      retryMessageId,
      fetch,
      () => {
        // markConnected
        hubStream.updateDashboard({
          ...$hubStream.dashboard,
          connectionState: 'connected' as const,
          stale: false,
          issue: undefined
        });
      },
      (issue) => {
        // markIssue
        hubStream.updateDashboard({
          ...$hubStream.dashboard,
          connectionState: 'degraded' as const,
          stale: true,
          issue
        });
      }
    );
  }

  async function toggleVoiceMute() {
    try {
      await hubStream.toggleVoiceMute(fetch);
    } catch (err) {
      console.error('Failed to toggle voice mute:', err);
    }
  }

  async function cancelVoiceSession() {
    await hubStream.cancelVoiceSession(fetch);
  }

  async function retryDashboard() {
    try {
      const fresh = await hubStream.retryDashboard(fetch);
      if (fresh) {
        const agent = firstAvailableAgent(fresh.agents);
        if (agent) {
          await chatStore.loadHistory(fresh.agents, '', agent.id, fetch);
        }
      }
    } catch {
      // ignore
    }
  }
</script>

<svelte:head>
  <title>{$hubStream.dashboard.config.home.name} · Jute Dash</title>
</svelte:head>

<main
  class={cn(
    'display-root',
    $navigationStore.mode === 'chat' && 'display-root--chat',
    $hubStream.dashboard.stale && 'display-root--stale'
  )}
  data-theme={activeTheme}
  data-theme-id={$hubStream.dashboard.config.display.themeId}
  data-background-overlay={$hubStream.dashboard.config.display.background
    ?.overlay ?? 'none'}
  style={displayStyle}
  on:pointerdown={startLongPress}
  on:pointerup={clearLongPress}
  on:pointercancel={clearLongPress}
  on:pointerleave={clearLongPress}
>
  <BackgroundRenderer
    {backgroundConfig}
    motion={$hubStream.dashboard.config.display.motion}
    {weatherData}
  />

  {#if $hubStream.dashboard.connectionState === 'starting'}
    <section class="startup-state" aria-label="Connecting to Jute hub">
      <div class="startup-mark">{$hubStream.dashboard.config.home.name}</div>
      <div>
        <strong>Connecting to local hub</strong>
        <span>{$hubStream.dashboard.hubUrl}</span>
      </div>
    </section>
  {:else if $hubStream.dashboard.connectionState === 'offline'}
    <OfflineState
      theme={activeTheme}
      hubUrl={$hubStream.dashboard.hubUrl}
      issue={$hubStream.dashboard.issue}
      retrying={$hubStream.retrying}
      onRetry={retryDashboard}
    />
  {:else}
    <StatusRibbon
      state={$hubStream.dashboard.connectionState}
      stale={$hubStream.dashboard.stale}
      issue={$hubStream.dashboard.issue}
      retrying={$hubStream.retrying}
      onRetry={retryDashboard}
    />

    <DashboardView
      data={dashboardForView}
      editMode={$layoutStore.editMode}
      messages={$chatStore.messages}
      stale={$hubStream.dashboard.stale}
      {selectedAgent}
      {selectedAvailability}
      focusedWidgetId={$hubStream.focusedWidgetId}
      voice={$hubStream.dashboard.voice}
      widgetCatalog={$layoutStore.widgetCatalog}
      editIssue={$layoutStore.editIssue}
      savingLayout={$layoutStore.saving}
      onOpenChat={() => openChat()}
      onToggleVoiceMute={toggleVoiceMute}
      onEnterEdit={() => layoutStore.enterEdit($hubStream.dashboard.layout)}
      onSaveEdit={() =>
        layoutStore.saveEdit($hubStream.dashboard.stale, fetch, (saved) => {
          hubStream.updateLayout(saved);
        })}
      onCancelEdit={layoutStore.cancelEdit}
      onResetLayout={() =>
        layoutStore.resetLayout(
          $hubStream.dashboard.layout.profileId,
          fetch,
          (reset) => {
            hubStream.updateLayout(reset);
          }
        )}
      onManageAgents={() => openSettings('household')}
    />

    {#if configuringWidget}
      <WidgetSettingsSheet
        widget={configuringWidget}
        catalogItem={configuringCatalogItem}
        onClose={layoutStore.closeWidgetConfig}
        onSave={layoutStore.saveWidgetConfig}
      />
    {/if}

    {#if showAgentManager}
      <SettingsPanel
        bind:activeSection={activeSettingsSection}
        onClose={() => (showAgentManager = false)}
      />
    {/if}

    {#if $navigationStore.mode === 'chat'}
      <div
        class="chat-layer"
        role="presentation"
        on:pointerdown={handleInteraction}
        on:keydown={handleInteraction}
        on:scroll|capture={handleInteraction}
      >
        <ChatView
          agents={$hubStream.dashboard.agents}
          messages={$chatStore.messages}
          conversations={$chatStore.conversations}
          state={$chatStore.chatState}
          voice={$hubStream.dashboard.voice}
          selectedAgentId={$chatStore.selectedAgentId}
          selectedConversationId={$chatStore.selectedConversationId}
          {selectedAvailability}
          status={$hubStream.dashboard.status}
          timerProgress={$chatStore.timerProgress}
          showTimer={$chatStore.showTimer}
          onAgentChange={(agentId) => {
            chatStore.setAgentId(agentId);
            void chatStore.loadHistory(
              $hubStream.dashboard.agents,
              '',
              agentId,
              fetch
            );
          }}
          onConversationSelect={(conversationId) =>
            chatStore.loadConversation(
              conversationId,
              $chatStore.selectedAgentId,
              fetch
            )}
          onNewConversation={() =>
            chatStore.newConversation($hubStream.dashboard.agents, fetch)}
          onManageAgents={() => openSettings('agents')}
          onRefreshAgentCard={(agentId) =>
            settingsStore.refreshAgentCard(agentId, fetch)}
          onSubmit={(value) => submitMessage(value)}
          onRetry={(msg) =>
            chatStore.retry(msg, $hubStream.dashboard.agents, fetch)}
          onClose={closeChat}
          onCancel={chatStore.cancel}
          onToggleVoiceMute={toggleVoiceMute}
        />
      </div>
    {/if}

    {#if $hubStream.showVoiceOverlay}
      <VoiceOverlay
        voice={$hubStream.dashboard.voice}
        voiceOrbState={$hubStream.voiceOrbState}
        voiceTranscript={$hubStream.voiceTranscript}
        assistantSpeech={$hubStream.assistantSpeech}
        on:toggleMute={toggleVoiceMute}
        on:cancel={cancelVoiceSession}
      />
    {/if}

    {#if $hubStream.displayNotifications.length > 0}
      <div
        class="display-notification-stack"
        aria-live="polite"
        aria-label="Display notifications"
      >
        {#each $hubStream.displayNotifications as notification (notification.id)}
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
