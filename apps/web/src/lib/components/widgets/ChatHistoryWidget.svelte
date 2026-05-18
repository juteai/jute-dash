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
