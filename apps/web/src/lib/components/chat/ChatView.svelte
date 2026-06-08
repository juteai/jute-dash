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
        pressed={voiceReady && !voice.muted}
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
