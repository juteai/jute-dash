<script lang="ts">
  import { browser } from '$app/environment';
  import { onDestroy, onMount } from 'svelte';
  import { Mic, MicOff, X } from 'lucide-svelte';
  import ChatView from '$lib/components/chat/ChatView.svelte';
  import DashboardView from '$lib/components/display/DashboardView.svelte';
  import ConversationOrb from '$lib/components/chat/ConversationOrb.svelte';
  import SettingsPanel from '$lib/components/settings/SettingsPanel.svelte';
  import { fade } from 'svelte/transition';
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
  let voiceIssue = '';
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
      voiceIssue = '';
    } catch (err) {
      voiceIssue = err instanceof Error ? err.message : String(err);
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
          statusText={$chatStore.assistantStatusText}
          voice={$hubStream.dashboard.voice}
          {voiceIssue}
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
      <div class="voice-overlay-container" transition:fade={{ duration: 300 }}>
        <div class="voice-card">
          <div class="voice-content">
            {#if $hubStream.voiceTranscript}
              <div class="bubble user-bubble">
                <span class="bubble-label">You</span>
                <p class="bubble-text">{$hubStream.voiceTranscript}</p>
              </div>
            {/if}

            {#if $hubStream.assistantSpeech}
              <div class="bubble assistant-bubble">
                <span class="bubble-label">Assistant</span>
                <p class="bubble-text">{$hubStream.assistantSpeech}</p>
              </div>
            {/if}

            {#if !$hubStream.voiceTranscript && !$hubStream.assistantSpeech}
              <div class="status-tip">
                {#if $hubStream.voiceOrbState === 'listening'}
                  <span class="status-pulse-dot cyan"></span> Listening...
                {:else if $hubStream.voiceOrbState === 'followup'}
                  <span class="status-pulse-dot yellow"></span> Follow-up listening...
                {:else if $hubStream.voiceOrbState === 'thinking'}
                  <span class="status-pulse-dot purple"></span> Thinking...
                {:else if $hubStream.voiceOrbState === 'speaking'}
                  <span class="status-pulse-dot green"></span> Speaking...
                {/if}
              </div>
            {/if}
          </div>

          <div class="voice-footer">
            <ConversationOrb state={$hubStream.voiceOrbState} />

            <div class="voice-controls">
              <button
                type="button"
                class="control-btn mute-btn {$hubStream.dashboard.voice.muted
                  ? 'muted'
                  : ''}"
                on:click={toggleVoiceMute}
                aria-label={$hubStream.dashboard.voice.muted
                  ? 'Unmute Microphone'
                  : 'Mute Microphone'}
              >
                {#if $hubStream.dashboard.voice.muted}
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
