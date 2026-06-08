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

<style>
  .settings-add-form {
    display: flex;
    align-items: stretch;
    gap: 10px;
    margin-bottom: 12px;
  }

  .settings-add-form input {
    flex: 1;
    min-width: 0;
    min-height: 42px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 10px;
  }

  .settings-list {
    display: grid;
    gap: 8px;
  }

  .settings-list-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 12px;
  }

  .settings-list-item strong {
    display: block;
    color: var(--foreground);
  }

  .settings-list-item span {
    display: block;
    margin-top: 3px;
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
    overflow-wrap: anywhere;
  }

  .settings-badges {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 8px;
  }

  .settings-item-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 10px;
  }

  .settings-empty {
    margin: 12px 0 0;
    line-height: 1.4;
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
  }

  @media (max-width: 640px) {
    .settings-list-item,
    .settings-add-form {
      align-items: stretch;
      flex-direction: column;
    }
  }
</style>
