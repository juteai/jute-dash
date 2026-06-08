<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { RefreshCw } from 'lucide-svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import type { Agent, MCPStatus } from '$lib/types';

  export let selectedAgent: Agent | undefined = undefined;
  export let mcpStatus: MCPStatus | undefined = undefined;
  export let refreshingCard = false;

  const dispatch = createEventDispatcher<{
    refreshCard: void;
    manageAgents: void;
  }>();

  $: selectedBinding =
    selectedAgent?.selectedProtocolBinding || selectedAgent?.protocolBinding;
  $: selectedVersion = selectedAgent?.selectedProtocolVersion || '1.0';
  $: selectedEndpoint =
    selectedAgent?.selectedEndpointUrl || selectedAgent?.endpointUrl || '';

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
</script>

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
          >{mcpStatus?.enabled ? mcpStatus.serviceStatus : 'disabled'}</strong
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
        on:click={() => dispatch('refreshCard')}
        disabled={refreshingCard}
      >
        <RefreshCw size={15} />
        <span>{refreshingCard ? 'Refreshing' : 'Refresh Agent Card'}</span>
      </Button>
      <Button
        size="sm"
        variant="ghost"
        on:click={() => dispatch('manageAgents')}>Manage agents</Button
      >
    </div>
  {:else}
    <p class="agent-diagnostics-empty">Add an A2A agent to see diagnostics.</p>
  {/if}
</section>

<style>
  .agent-diagnostics {
    display: grid;
    gap: 10px;
    padding: 12px 14px;
    border-bottom: 1px solid var(--border);
    background: var(--surface-muted);
  }

  .agent-diagnostics-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
  }

  .agent-diagnostics-grid > div,
  .agent-diagnostics-skills {
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    padding: 10px;
  }

  .agent-diagnostics span,
  .agent-diagnostics small {
    display: block;
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 700;
  }

  .agent-diagnostics strong {
    display: block;
    margin-top: 4px;
    overflow: hidden;
    color: var(--foreground);
    font-size: 0.84rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .agent-diagnostics small {
    margin-top: 4px;
    overflow-wrap: anywhere;
  }

  .agent-diagnostics-skills {
    display: grid;
    gap: 8px;
  }

  .agent-diagnostics-skills > div,
  .agent-diagnostics-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .agent-diagnostics-empty {
    margin: 0;
    color: var(--muted);
    font-size: 0.84rem;
    font-weight: 650;
  }

  @media (max-width: 920px) {
    .agent-diagnostics-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (max-width: 640px) {
    .agent-diagnostics-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
