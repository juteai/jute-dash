<script lang="ts">
  import { ChevronDown, Mic, VolumeX } from 'lucide-svelte';
  import AssistantActivity from '$lib/components/chat/AssistantActivity.svelte';
  import MessageComposer from '$lib/components/chat/MessageComposer.svelte';
  import MessageList from '$lib/components/chat/MessageList.svelte';
  import { availabilityDescription, availabilityLabel, availabilityTone, getAgentAvailability } from '$lib/agents';
  import Badge from '$lib/components/ui/Badge.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import type { Agent, AgentAvailability, ChatMessage, ChatState, VoiceStatus } from '$lib/types';

  export let agents: Agent[] = [];
  export let messages: ChatMessage[] = [];
  export let state: ChatState = 'idle';
  export let voice: VoiceStatus;
  export let voiceIssue = '';
  export let selectedAgentId = '';
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let onAgentChange: (agentId: string) => void = () => {};
  export let onSubmit: (value: string) => Promise<void> | void = () => {};
  export let onRetry: (message: ChatMessage) => Promise<void> | void = () => {};
  export let onClose: () => void = () => {};
  export let onCancel: () => void = () => {};
  export let onToggleVoiceMute: () => Promise<void> | void = () => {};

  type BadgeTone = 'danger' | 'neutral' | 'active';

  let statusTone: BadgeTone = 'neutral';

  $: selectedAgent = agents.find((agent) => agent.id === selectedAgentId) ?? agents[0];
  $: agentAvailability = selectedAgent ? getAgentAvailability(selectedAgent) : selectedAvailability;
  $: composerDisabled = agentAvailability !== 'available';
  $: voiceReady = voice?.serviceStatus === 'ready';
  $: voiceLabel = voiceReady
    ? voice.muted
      ? 'Voice muted'
      : 'Wake listening'
    : 'Voice not configured';
  $: statusTone = state === 'error' ? 'danger' : state === 'idle' ? 'neutral' : 'active';
</script>

<section class="chat-view" aria-label="Agent conversation">
  <header class="chat-header">
    <div class="chat-agent">
      <div class="chat-agent-title">{selectedAgent?.name ?? 'No agent configured'}</div>
      <div class="chat-agent-meta">
        <Badge tone={statusTone}>{state}</Badge>
        {#if selectedAgent}
          <Badge tone={availabilityTone(agentAvailability)}>{availabilityLabel(agentAvailability)}</Badge>
          <span>{selectedAgent.protocolBinding}</span>
          {#if selectedAgent.authConfigured}
            <span>auth</span>
          {/if}
        {/if}
      </div>
      {#if selectedAgent}
        <p class="chat-agent-description">{selectedAgent.description || availabilityDescription(agentAvailability)}</p>
      {/if}
    </div>

    <div class="chat-controls">
      {#if agents.length > 1}
        <label class="agent-select-label">
          <span>Agent</span>
          <select bind:value={selectedAgentId} on:change={(event) => onAgentChange(event.currentTarget.value)}>
            {#each agents as agent}
              <option value={agent.id}>{agent.name} · {availabilityLabel(getAgentAvailability(agent))}</option>
            {/each}
          </select>
        </label>
      {/if}
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
        Waiting for the agent
      {:else if state === 'streaming'}
        Response in progress
      {:else if state === 'error'}
        Something needs another try
      {:else}
        Ready
      {/if}
    </span>
  </div>

  <MessageList
    {messages}
    emptyTitle={selectedAgent ? 'Ask Jute anything' : 'No agent connected'}
    emptyMessage={selectedAgent ? 'Choose an agent and start with a short request.' : 'Add an A2A agent to start conversations.'}
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
</section>
