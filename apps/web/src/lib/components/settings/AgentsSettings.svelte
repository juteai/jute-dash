<script lang="ts">
  import { RefreshCw } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import { hubStream } from '$lib/hubStream';
  import { availabilityLabel, getAgentAvailability } from '$lib/agents';
  import type { Agent } from '$lib/types';

  let localAgentCardUrl = '';
  let refreshingAgentId = '';

  async function addAgent() {
    const cardUrl = localAgentCardUrl.trim();
    if (!cardUrl || $settingsStore.savingAgent) {
      return;
    }
    try {
      await settingsStore.addAgent(cardUrl);
      localAgentCardUrl = '';
    } catch {
      // Error is set in settingsStore.issue
    }
  }

  async function refreshAgent(agentId: string) {
    if (!agentId || refreshingAgentId) {
      return;
    }
    refreshingAgentId = agentId;
    try {
      await settingsStore.refreshAgentCard(agentId);
    } catch {
      // ignore
    } finally {
      refreshingAgentId = '';
    }
  }

  async function toggleAgent(agent: Agent) {
    try {
      await settingsStore.toggleAgent(agent);
    } catch {
      // ignore
    }
  }

  async function removeAgent(agent: Agent) {
    try {
      await settingsStore.removeAgent(agent);
    } catch {
      // ignore
    }
  }
</script>

<form class="settings-add-form" on:submit|preventDefault={addAgent}>
  <input
    bind:value={localAgentCardUrl}
    placeholder="http://127.0.0.1:9797/.well-known/agent-card.json"
  />
  <Button
    type="submit"
    disabled={$settingsStore.savingAgent || !localAgentCardUrl.trim()}
    >{$settingsStore.savingAgent ? 'Adding' : 'Add agent'}</Button
  >
</form>
<div class="settings-list">
  {#if $hubStream.dashboard.agents.length === 0}
    <p class="settings-empty">No agents configured yet.</p>
  {:else}
    {#each $hubStream.dashboard.agents as agent (agent.id)}
      <article class="settings-list-item">
        <div>
          <strong>{agent.name}</strong>
          <span>{agent.cardUrl}</span>
          <div class="settings-badges">
            <Badge tone="neutral"
              >{availabilityLabel(getAgentAvailability(agent))}</Badge
            >
            <Badge tone="neutral"
              >{agent.selectedProtocolBinding || agent.protocolBinding}</Badge
            >
            {#if agent.dashboardContextSupported}
              <Badge tone="active">screen context</Badge>
            {/if}
          </div>
        </div>
        <div class="settings-item-actions">
          <Button
            size="sm"
            variant="outline"
            on:click={() => refreshAgent(agent.id)}
            disabled={refreshingAgentId === agent.id}
          >
            <RefreshCw size={15} />
            <span
              >{refreshingAgentId === agent.id ? 'Refreshing' : 'Refresh'}</span
            >
          </Button>
          <Button
            size="sm"
            variant="outline"
            on:click={() => toggleAgent(agent)}
            >{agent.enabled ? 'Disable' : 'Enable'}</Button
          >
          <Button size="sm" variant="ghost" on:click={() => removeAgent(agent)}
            >Remove</Button
          >
        </div>
      </article>
    {/each}
  {/if}
</div>
