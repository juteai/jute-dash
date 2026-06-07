<script lang="ts">
  import {
    ChevronDown,
    Info,
    Mic,
    Plus,
    RefreshCw,
    VolumeX
  } from 'lucide-svelte';
  import AssistantActivity from '$lib/components/chat/AssistantActivity.svelte';
  import MessageComposer from '$lib/components/chat/MessageComposer.svelte';
  import MessageList from '$lib/components/chat/MessageList.svelte';
  import {
    availabilityDescription,
    availabilityLabel,
    availabilityTone,
    getAgentAvailability
  } from '$lib/agents';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
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
  export let statusText = '';
  export let voice: VoiceStatus;
  export let voiceIssue = '';
  export let selectedAgentId = '';
  export let selectedConversationId = '';
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let status: AppStatus | undefined;
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

  type BadgeTone = 'danger' | 'neutral' | 'active';

  let statusTone: BadgeTone;
  let showDiagnostics = false;
  let refreshingCard = false;

  $: selectedAgent =
    agents.find((agent) => agent.id === selectedAgentId) ?? agents[0];
  $: agentAvailability = selectedAgent
    ? getAgentAvailability(selectedAgent)
    : selectedAvailability;
  $: composerDisabled = agentAvailability !== 'available';
  $: selectedBinding =
    selectedAgent?.selectedProtocolBinding || selectedAgent?.protocolBinding;
  $: selectedVersion = selectedAgent?.selectedProtocolVersion || '1.0';
  $: selectedEndpoint =
    selectedAgent?.selectedEndpointUrl || selectedAgent?.endpointUrl || '';
  $: mcpStatus = status?.mcp;
  $: mcpLabel = mcpStatus?.enabled
    ? `MCP ${mcpStatus.serviceStatus}`
    : 'MCP disabled';
  $: selectedConversation = conversations.find(
    (conversation) => conversation.id === selectedConversationId
  );
  $: voiceReady = voice?.serviceStatus === 'ready';
  $: voiceLabel = voiceReady
    ? voice.muted
      ? 'Voice muted'
      : 'Wake listening'
    : 'Voice not configured';
  $: statusTone =
    state === 'error' ? 'danger' : state === 'idle' ? 'neutral' : 'active';

  function formatConversationTime(value: string) {
    if (!value) {
      return '';
    }
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    }).format(new Date(value));
  }

  function endpointHost(value: string) {
    if (!value) {
      return 'not selected';
    }
    try {
      const url = new URL(value);
      return `${url.host}${url.pathname}`;
    } catch {
      return value.replace(/^https?:\/\//, '');
    }
  }

  async function refreshCard() {
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
  <header class="chat-header">
    <div class="chat-agent">
      <div class="chat-agent-title">
        {selectedAgent?.name ?? 'No agent configured'}
      </div>
      <div class="chat-agent-meta">
        <Badge tone={statusTone}>{state}</Badge>
        {#if selectedAgent}
          <Badge tone={availabilityTone(agentAvailability)}
            >{availabilityLabel(agentAvailability)}</Badge
          >
          {#if selectedAgent.dashboardContextSupported}
            <span>screen context</span>
          {/if}
          <span>{mcpLabel}</span>
        {/if}
      </div>
      {#if selectedAgent}
        <p class="chat-agent-description">
          {availabilityDescription(agentAvailability)}
        </p>
      {/if}
    </div>

    <div class="chat-controls">
      {#if agents.length > 1}
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
      <IconButton label="Close chat" variant="outline" on:click={onClose}>
        <ChevronDown size={20} />
      </IconButton>
    </div>
  </header>

  <div class="chat-status-row">
    <AssistantActivity {state} />
    <span>
      {#if !selectedAgent}
        No agent connected
      {:else if voiceIssue}
        {voiceIssue}
      {:else if !voiceReady}
        Voice input is not configured yet. Typed chat is available.
      {:else if agentAvailability !== 'available'}
        {availabilityDescription(agentAvailability)}
      {:else if state === 'thinking'}
        {statusText || 'Waiting for the agent'}
      {:else if state === 'streaming'}
        Response in progress
      {:else if state === 'error'}
        Something needs another try
      {:else}
        Ready
      {/if}
    </span>
  </div>

  {#if showDiagnostics}
    <section class="agent-diagnostics" aria-label="Agent diagnostics">
      {#if selectedAgent}
        <div class="agent-diagnostics-grid">
          <div>
            <span>Agent Card</span>
            <strong>{selectedAgent.cardStatus || 'unknown'}</strong>
            {#if selectedAgent.cardError}
              <small>{selectedAgent.cardError}</small>
            {:else if selectedAgent.cardFetchedAt}
              <small
                >Fetched {formatConversationTime(
                  selectedAgent.cardFetchedAt
                )}</small
              >
            {/if}
          </div>
          <div>
            <span>A2A binding</span>
            <strong>{selectedBinding || 'not selected'}</strong>
            <small>A2A {selectedVersion}</small>
          </div>
          <div>
            <span>Endpoint</span>
            <strong>{endpointHost(selectedEndpoint)}</strong>
            <small
              >{selectedAgent.streaming
                ? 'streaming supported'
                : 'blocking only'}</small
            >
          </div>
          <div>
            <span>Context</span>
            <strong
              >{selectedAgent.dashboardContextSupported
                ? 'dashboard context supported'
                : 'dashboard context unavailable'}</strong
            >
            <small>{selectedAgent.mcpScopes?.length ?? 0} MCP scopes</small>
          </div>
          <div>
            <span>MCP bridge</span>
            <strong
              >{mcpStatus?.enabled
                ? mcpStatus.serviceStatus
                : 'disabled'}</strong
            >
            <small
              >{mcpStatus?.enabled
                ? `${mcpStatus.transport} · ${mcpStatus.authMode || 'no auth mode'}`
                : 'A2A still works without MCP'}</small
            >
          </div>
          <div>
            <span>Credentials</span>
            <strong
              >{selectedAgent.authConfigured
                ? selectedAgent.authAvailable === false
                  ? 'missing'
                  : 'configured'
                : 'not required'}</strong
            >
            <small>Credential references stay inside the hub</small>
          </div>
        </div>
        {#if selectedAgent.skills && selectedAgent.skills.length > 0}
          <div class="agent-diagnostics-skills">
            <span>Skills</span>
            <div>
              {#each selectedAgent.skills as skill (skill.id ?? skill.name)}
                <Badge tone="neutral">{skill.name}</Badge>
              {/each}
            </div>
          </div>
        {:else}
          <p class="agent-diagnostics-empty">
            No Agent Card skills discovered yet.
          </p>
        {/if}
        <div class="agent-diagnostics-actions">
          <Button
            size="sm"
            variant="outline"
            on:click={refreshCard}
            disabled={refreshingCard}
          >
            <RefreshCw size={15} />
            <span>{refreshingCard ? 'Refreshing' : 'Refresh Agent Card'}</span>
          </Button>
          <Button size="sm" variant="ghost" on:click={onManageAgents}
            >Manage agents</Button
          >
        </div>
      {:else}
        <p class="agent-diagnostics-empty">
          Add an A2A agent to see diagnostics.
        </p>
      {/if}
    </section>
  {/if}

  <div class="chat-body">
    <aside class="conversation-sidebar" aria-label="Conversation history">
      <div class="conversation-sidebar-header">
        <div>
          <strong>History</strong>
          <span>{conversations.length} saved</span>
        </div>
        <IconButton
          label="New conversation"
          variant="outline"
          disabled={composerDisabled}
          on:click={onNewConversation}
        >
          <Plus size={17} />
        </IconButton>
      </div>

      <div class="conversation-list">
        {#if conversations.length === 0}
          <div class="conversation-empty">
            {#if composerDisabled}
              No available agent yet.
              <button
                type="button"
                class="conversation-link-button"
                on:click={onManageAgents}>Add agent</button
              >
            {:else}
              Agent-backed history is empty or unsupported.
            {/if}
          </div>
        {:else}
          {#each conversations as conversation (conversation.id)}
            <button
              type="button"
              class:conversation-item--active={conversation.id ===
                selectedConversationId}
              class="conversation-item"
              on:click={() => onConversationSelect(conversation.id)}
            >
              <span>{conversation.title || 'Conversation'}</span>
              <small>{formatConversationTime(conversation.updatedAt)}</small>
            </button>
          {/each}
        {/if}
      </div>
    </aside>

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
        emptyTitle={selectedAgent ? 'Ask Jute anything' : 'No agent connected'}
        emptyMessage={selectedAgent
          ? 'Choose an agent and start with a short request.'
          : 'Add an A2A agent to start conversations.'}
        {onRetry}
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
  </div>
</section>
