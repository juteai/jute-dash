<script lang="ts">
  import { ChevronDown, Info, Mic, VolumeX, History } from 'lucide-svelte';
  import MessageComposer from '$lib/components/chat/MessageComposer.svelte';
  import MessageList from '$lib/components/chat/MessageList.svelte';
  import StardustCanvas from '$lib/components/chat/StardustCanvas.svelte';
  import ChatDiagnostics from '$lib/components/chat/ChatDiagnostics.svelte';
  import ConversationSidebar from '$lib/components/chat/ConversationSidebar.svelte';
  import ArtifactPreview from '$lib/components/chat/ArtifactPreview.svelte';
  import { getAgentAvailability, availabilityLabel } from '$lib/agents';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import type {
    Agent,
    AgentAvailability,
    AppStatus,
    ChatMessage,
    ChatState,
    Conversation,
    VoiceStatus
  } from '$lib/types';

  export let agents: Agent[] = [];
  export let messages: ChatMessage[] = [];
  export let conversations: Conversation[] = [];
  export let state: ChatState = 'idle';
  export let voice: VoiceStatus;
  export let selectedAgentId = '';
  export let selectedConversationId = '';
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let status: AppStatus | undefined;
  export let timerProgress = 0;
  export let showTimer = false;
  export let onAgentChange: (agentId: string) => void = () => {};
  export let onConversationSelect: (
    conversationId: string
  ) => Promise<void> | void = () => {};
  export let onNewConversation: () => Promise<void> | void = () => {};
  export let onManageAgents: () => void = () => {};
  export let onRefreshAgentCard: (
    agentId: string
  ) => Promise<void> | void = () => {};
  export let onSubmit: (value: string) => Promise<void> | void = () => {};
  export let onRetry: (message: ChatMessage) => Promise<void> | void = () => {};
  export let onClose: () => void = () => {};
  export let onCancel: () => void = () => {};
  export let onToggleVoiceMute: () => Promise<void> | void = () => {};

  let showDiagnostics = false;
  let showHistory = false;
  let refreshingCard = false;
  let selectedArtifact: { id: string; title: string; content: string } | null =
    null;

  $: if (selectedConversationId) {
    selectedArtifact = null;
  }

  $: selectedAgent =
    agents.find((agent) => agent.id === selectedAgentId) ?? agents[0];
  $: agentAvailability = selectedAgent
    ? getAgentAvailability(selectedAgent)
    : selectedAvailability;
  $: composerDisabled = agentAvailability !== 'available';
  $: mcpStatus = status?.mcp;
  $: selectedConversation = conversations.find(
    (conversation) => conversation.id === selectedConversationId
  );
  $: voiceReady = voice?.serviceStatus === 'ready';
  $: voiceLabel = voiceReady
    ? voice.muted
      ? 'Voice muted'
      : 'Wake listening'
    : 'Voice not configured';

  async function handleRefreshCard() {
    if (!selectedAgent || refreshingCard) {
      return;
    }
    refreshingCard = true;
    try {
      await onRefreshAgentCard(selectedAgent.id);
    } finally {
      refreshingCard = false;
    }
  }
</script>

<section class="chat-view" aria-label="Agent conversation">
  <!-- Encapsulated Particles Background -->
  <StardustCanvas {state} />

  <header class="chat-header">
    <div class="chat-agent">
      <div class="chat-agent-title">
        {selectedAgent?.name ?? 'No agent configured'}
      </div>
      {#if showDiagnostics}
        <div class="chat-agent-meta">
          <span>{state}</span>
          {#if selectedAgent}
            <span>{availabilityLabel(agentAvailability)}</span>
            {#if selectedAgent.dashboardContextSupported}
              <span>screen context</span>
            {/if}
          {/if}
        </div>
      {/if}
    </div>

    <div class="chat-controls">
      {#if showDiagnostics && agents.length > 1}
        <label class="agent-select-label">
          <span>Agent</span>
          <select
            bind:value={selectedAgentId}
            on:change={(event) => onAgentChange(event.currentTarget.value)}
          >
            {#each agents as agent (agent.id)}
              <option value={agent.id}
                >{agent.name} · {availabilityLabel(
                  getAgentAvailability(agent)
                )}</option
              >
            {/each}
          </select>
        </label>
      {/if}
      <IconButton
        label="Agent diagnostics"
        variant="outline"
        pressed={showDiagnostics}
        on:click={() => (showDiagnostics = !showDiagnostics)}
      >
        <Info size={19} />
      </IconButton>
      <IconButton
        label="Toggle history"
        variant="outline"
        pressed={showHistory}
        on:click={() => (showHistory = !showHistory)}
      >
        <History size={19} />
      </IconButton>
      <IconButton
        label={voiceLabel}
        variant="outline"
        pressed={voiceReady && voice.muted}
        disabled={!voiceReady}
        on:click={onToggleVoiceMute}
      >
        {#if voiceReady && !voice.muted}
          <Mic size={19} />
        {:else}
          <VolumeX size={19} />
        {/if}
      </IconButton>
      <div class="close-timer-container">
        {#if showTimer}
          <svg class="close-timer-svg">
            <circle class="close-timer-track" cx="22" cy="22" r="18" />
            <circle
              class="close-timer-bar"
              cx="22"
              cy="22"
              r="18"
              stroke-dasharray="113.1"
              stroke-dashoffset={113.1 * (1 - timerProgress)}
            />
          </svg>
        {/if}
        <IconButton label="Close chat" variant="outline" on:click={onClose}>
          <ChevronDown size={20} />
        </IconButton>
      </div>
    </div>
  </header>
  <!-- Encapsulated Diagnostics Dashboard Panel -->
  {#if showDiagnostics}
    <ChatDiagnostics
      {selectedAgent}
      {mcpStatus}
      {refreshingCard}
      on:refreshCard={handleRefreshCard}
      on:manageAgents={onManageAgents}
    />
  {/if}

  <div
    class="chat-body"
    class:chat-view--with-history={showHistory}
    class:chat-view--minimal={!showHistory}
    class:chat-view--with-preview={selectedArtifact}
  >
    <!-- Encapsulated Saved History Sidebar -->
    {#if showHistory}
      <ConversationSidebar
        {conversations}
        {selectedConversationId}
        {composerDisabled}
        on:select={(e) => onConversationSelect(e.detail.conversationId)}
        on:new={onNewConversation}
        on:manageAgents={onManageAgents}
      />
    {/if}

    <div class="chat-thread">
      <div class="conversation-thread-header">
        <div>
          <strong>{selectedConversation?.title || 'New conversation'}</strong>
          <span>
            {#if selectedConversation?.historyUnsupported}
              History unavailable for this agent
            {:else if selectedConversation}
              {selectedConversation.status}
            {:else if selectedAgent}
              Ready to start with {selectedAgent.name}
            {:else}
              No agent connected
            {/if}
          </span>
        </div>
      </div>

      <MessageList
        {messages}
        {state}
        emptyTitle={selectedAgent ? 'Ask Jute anything' : 'No agent connected'}
        emptyMessage={selectedAgent
          ? 'Choose an agent and start with a short request.'
          : 'Add an A2A agent to start conversations.'}
        {onRetry}
        onSelectArtifact={(artifact) => (selectedArtifact = artifact)}
      />

      <MessageComposer
        {state}
        disabled={composerDisabled}
        {voice}
        {onSubmit}
        {onCancel}
        onVoiceClick={onToggleVoiceMute}
      />
    </div>

    <!-- Encapsulated sliding markdown artifact details preview -->
    {#if selectedArtifact}
      <ArtifactPreview
        artifact={selectedArtifact}
        on:close={() => (selectedArtifact = null)}
      />
    {/if}
  </div>
</section>

<style>
  :global(.chat-layer) {
    position: absolute;
    inset: 0;
    z-index: 50;
    background: radial-gradient(
      circle at 50% 50%,
      color-mix(in srgb, var(--active) 8%, transparent) 0%,
      color-mix(in srgb, var(--background) 95%, transparent) 70%,
      var(--background) 100%
    );
    background-size: 180% 180%;
    animation:
      ambient-gradient-morph 12s ease-in-out infinite alternate,
      chat-layer-in 300ms ease-out;
    backdrop-filter: blur(40px) saturate(220%);
    -webkit-backdrop-filter: blur(40px) saturate(220%);
    display: flex;
    flex-direction: column;
  }

  .chat-view {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    overflow: hidden;
    background: transparent;
    animation: chat-panel-in 300ms ease-out;
  }

  .chat-header {
    display: flex;
    align-items: center;
    position: relative;
    z-index: 10;
    justify-content: space-between;
    gap: 12px;
    padding: 14px;
    border-bottom: 1px solid var(--border);
  }

  .chat-agent {
    min-width: 0;
  }

  .chat-agent-title {
    overflow: hidden;
    color: var(--foreground);
    font-weight: 760;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chat-agent-meta {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 4px;
    color: var(--muted);
    font-size: 0.78rem;
    font-weight: 650;
  }

  .chat-controls {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
  }

  .agent-select-label {
    display: grid;
    gap: 4px;
    color: var(--muted);
    font-size: 0.74rem;
    font-weight: 700;
  }

  .chat-controls select {
    min-height: 40px;
    max-width: 220px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 10px;
  }

  .close-timer-container {
    position: relative;
    width: 44px;
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .close-timer-svg {
    position: absolute;
    inset: 0;
    width: 44px;
    height: 44px;
    transform: rotate(-90deg);
    pointer-events: none;
  }

  .close-timer-track {
    fill: none;
    stroke: color-mix(in srgb, var(--border) 40%, transparent);
    stroke-width: 2px;
  }

  .close-timer-bar {
    fill: none;
    stroke: var(--active);
    stroke-width: 2.5px;
    stroke-linecap: round;
    transition:
      stroke-dashoffset 0.1s linear,
      stroke 0.3s ease;
  }

  .chat-body {
    position: relative;
    z-index: 10;
    display: grid;
    flex: 1;
    min-height: 0;
  }

  .chat-thread {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    min-width: 0;
    min-height: 0;
  }

  .conversation-thread-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 52px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border);
  }

  .conversation-thread-header strong {
    display: block;
    color: var(--foreground);
    font-size: 0.92rem;
  }

  .conversation-thread-header span {
    display: block;
    margin-top: 2px;
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 650;
  }

  @media (min-width: 641px) {
    .chat-body.chat-view--minimal {
      grid-template-columns: minmax(0, 1fr);
    }

    .chat-body.chat-view--minimal.chat-view--with-preview {
      grid-template-columns: minmax(0, 1.2fr) minmax(320px, 480px);
    }

    .chat-body.chat-view--with-history {
      grid-template-columns: minmax(220px, 280px) minmax(0, 1fr);
    }

    .chat-body.chat-view--with-history.chat-view--with-preview {
      grid-template-columns: minmax(220px, 280px) minmax(0, 1.2fr) minmax(
          320px,
          480px
        );
    }
  }

  .chat-view--minimal .chat-thread {
    max-width: 800px;
    width: 100%;
    margin: 0 auto;
  }

  @keyframes -global-chat-layer-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes -global-chat-panel-in {
    from {
      opacity: 0;
      transform: scale(0.98) translateY(12px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }

  @keyframes -global-ambient-gradient-morph {
    0% {
      background-position: 0% 50%;
    }
    50% {
      background-position: 100% 50%;
    }
    100% {
      background-position: 0% 50%;
    }
  }

  @media (max-width: 920px) {
    .chat-view {
      height: 100%;
    }
  }

  @media (max-width: 640px) {
    :global(.chat-layer) {
      position: fixed;
      padding: 8px;
    }

    .chat-header {
      align-items: flex-start;
      flex-direction: column;
    }

    .chat-controls {
      width: 100%;
      justify-content: flex-start;
    }

    .chat-view {
      width: 100%;
      height: 100%;
    }

    .chat-body {
      grid-template-columns: 1fr;
      grid-template-rows: auto minmax(0, 1fr);
    }
  }
</style>
