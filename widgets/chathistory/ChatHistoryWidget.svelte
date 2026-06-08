<script lang="ts">
  import { MessageCircle, Plus } from 'lucide-svelte';
  import { availabilityLabel, availabilityTone, getAgentAvailability } from '$lib/agents';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import type { Agent, AgentAvailability, ChatMessage } from '$lib/types';

  export let agents: Agent[] = [];
  export let messages: ChatMessage[] = [];
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let onOpenChat: () => void = () => {};

  $: availableAgents = agents.filter((agent) => getAgentAvailability(agent) === 'available');
  $: activeAgent = selectedAgent ?? availableAgents[0] ?? agents.find((agent) => agent.enabled) ?? agents[0];
  $: activeAvailability = activeAgent ? getAgentAvailability(activeAgent) : selectedAvailability;
  $: recent = messages.slice(-3).reverse();
</script>

<div class="chat-history-widget">
  <div class="chat-history-summary">
    <MessageCircle size={28} aria-hidden="true" />
    <div>
      <div class="chat-history-count">{messages.length}</div>
      <div class="chat-history-label">
        {#if activeAgent}
          {activeAgent.name}
        {:else}
          No agent connected
        {/if}
      </div>
    </div>
  </div>

  <div class="chat-history-agent">
    {#if activeAgent}
      <Badge tone={availabilityTone(activeAvailability)}>{availabilityLabel(activeAvailability)}</Badge>
      <span>{activeAgent.protocolBinding}</span>
    {:else}
      <Badge tone="warning">setup needed</Badge>
      <span>Add an A2A agent to start conversations.</span>
    {/if}
  </div>

  <div class="chat-history-list">
    {#if recent.length === 0}
      <p>No recent chat yet. Start with a quick household request.</p>
    {:else}
      {#each recent as message}
        <div class={`chat-history-item chat-history-item--${message.role}`}>
          <span>{message.role}</span>
          <p>{message.content}</p>
        </div>
      {/each}
    {/if}
  </div>

  <div class="chat-history-footer">
    <span>
      {#if availableAgents.length > 0}
        {availableAgents.length} available agents
      {:else}
        No agent connected
      {/if}
    </span>
    <Button size="sm" on:click={onOpenChat}>
      <Plus size={16} />
      <span>Chat</span>
    </Button>
  </div>
</div>

<style>
  .chat-history-widget {
    display: flex;
    flex-direction: column;
    min-height: 100%;
    gap: 10px;
    font-size: var(--widget-body-size);
  }

  .chat-history-summary {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .chat-history-agent {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 680;
  }

  .chat-history-agent span:last-child {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chat-history-count {
    font-size: calc(var(--widget-display-size) * 0.68);
    font-weight: 780;
    line-height: 1;
  }

  .chat-history-label,
  .chat-history-footer,
  .chat-history-list p {
    color: var(--muted);
    font-size: var(--widget-body-size);
  }

  .chat-history-list {
    display: grid;
    gap: 8px;
    min-height: 0;
  }

  .chat-history-item {
    min-width: 0;
    padding: 8px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
  }

  .chat-history-item span {
    color: var(--muted);
    font-size: 0.7rem;
    font-weight: 760;
    text-transform: uppercase;
  }

  .chat-history-item p {
    display: -webkit-box;
    margin: 4px 0 0;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .chat-history-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
</style>
