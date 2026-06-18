<script lang="ts">
  import { onMount } from 'svelte';
  import { X } from 'lucide-svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import { hubStream } from '$lib/hubStream';
  import HouseholdSettings from './HouseholdSettings.svelte';
  import AppearanceSettings from './AppearanceSettings.svelte';
  import RoomsSettings from './RoomsSettings.svelte';
  import TilesSettings from './TilesSettings.svelte';
  import AgentsSettings from './AgentsSettings.svelte';
  import VoiceSettings from './VoiceSettings.svelte';
  import ConnectionsSettings from './ConnectionsSettings.svelte';

  export let activeSection:
    | 'household'
    | 'rooms'
    | 'tiles'
    | 'connections'
    | 'agents'
    | 'mcp'
    | 'voice'
    | 'appearance'
    | 'about' = 'household';
  export let onClose: () => void = () => {};

  const sections = [
    ['household', 'Household'],
    ['appearance', 'Appearance'],
    ['rooms', 'Rooms'],
    ['tiles', 'Tiles'],
    ['connections', 'Connections'],
    ['agents', 'Agents'],
    ['mcp', 'MCP'],
    ['voice', 'Voice'],
    ['about', 'About']
  ] as const;

  onMount(() => {
    void settingsStore.load(fetch);
  });
</script>

<div class="settings-layer">
  <section class="settings-panel" aria-label="Jute settings">
    <header class="settings-header">
      <div>
        <strong>Settings</strong>
        <span>Configure this local Jute Dash install.</span>
      </div>
      <IconButton label="Close settings" variant="outline" on:click={onClose}>
        <X size={18} />
      </IconButton>
    </header>

    <nav class="settings-tabs" aria-label="Settings sections">
      {#each sections as [id, label] (id)}
        <button
          type="button"
          class:settings-tab--active={activeSection === id}
          on:click={() => {
            activeSection = id;
            settingsStore.clearIssue();
          }}
        >
          {label}
        </button>
      {/each}
    </nav>

    {#if $settingsStore.issue}
      <p class="settings-issue">{$settingsStore.issue}</p>
    {/if}

    <div class="settings-body">
      {#if activeSection === 'household'}
        <HouseholdSettings />
      {:else if activeSection === 'appearance'}
        <AppearanceSettings />
      {:else if activeSection === 'rooms'}
        <RoomsSettings />
      {:else if activeSection === 'tiles'}
        <TilesSettings />
      {:else if activeSection === 'connections'}
        <ConnectionsSettings />
      {:else if activeSection === 'agents'}
        <AgentsSettings />
      {:else if activeSection === 'mcp'}
        <div class="settings-status-grid">
          <div>
            <span>Status</span><strong
              >{$hubStream.dashboard.status?.mcp.enabled
                ? $hubStream.dashboard.status.mcp.serviceStatus
                : 'disabled'}</strong
            >
          </div>
          <div>
            <span>Transport</span><strong
              >{$hubStream.dashboard.status?.mcp.transport ||
                'streamable-http'}</strong
            >
          </div>
          <div>
            <span>Path</span><strong
              >{$hubStream.dashboard.status?.mcp.path || '/mcp'}</strong
            >
          </div>
          <div>
            <span>Auth</span><strong
              >{$hubStream.dashboard.status?.mcp.authMode ||
                'not configured'}</strong
            >
          </div>
        </div>
        <p class="settings-help">
          MCP is configured at hub startup. Edit the harness or bootstrap
          config, then restart the stack to change it.
        </p>
      {:else if activeSection === 'voice'}
        <VoiceSettings />
      {:else}
        <div class="settings-status-grid">
          <div>
            <span>Home</span><strong
              >{$settingsStore.householdSettings?.home.name ||
                $hubStream.dashboard.config.home.name ||
                'Jute Dash'}</strong
            >
          </div>
          <div>
            <span>Hub version</span><strong
              >{$hubStream.dashboard.status?.version || 'dev'}</strong
            >
          </div>
          <div>
            <span>Setup</span><strong
              >{$hubStream.dashboard.status?.setup.complete
                ? 'complete'
                : 'incomplete'}</strong
            >
          </div>
          <div>
            <span>Config</span><strong
              >{$hubStream.dashboard.status?.config.writableYaml
                ? 'writable YAML'
                : 'runtime store'}</strong
            >
          </div>
          <div>
            <span>Agents</span><strong
              >{$hubStream.dashboard.status?.agents.enabled ??
                $hubStream.dashboard.agents.filter((agent) => agent.enabled)
                  .length} enabled</strong
            >
          </div>
        </div>
      {/if}
    </div>
  </section>
</div>

<style>
  :global(.settings-layer) {
    position: fixed;
    inset: 0;
    z-index: 30;
    display: grid;
    place-items: center;
    padding: 16px;
    background: color-mix(in srgb, var(--background) 72%, transparent);
  }

  .settings-panel {
    display: grid;
    grid-template-rows: auto auto auto minmax(0, 1fr);
    gap: 12px;
    width: min(100%, 860px);
    height: min(90vh, 760px);
    max-height: calc(100vh - 32px);
    overflow: hidden;
    border: 1px solid var(--border-strong);
    border-radius: 8px;
    background: var(--surface);
    padding: 16px;
    box-shadow: 0 20px 80px var(--shadow);
  }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .settings-header strong {
    display: block;
    color: var(--foreground);
  }

  .settings-header span {
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
  }

  .settings-tabs {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    border-bottom: 1px solid var(--border);
    padding-bottom: 10px;
  }

  .settings-tabs button {
    min-height: 36px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 12px;
    cursor: pointer;
    font-weight: 740;
  }

  .settings-tabs button.settings-tab--active {
    border-color: var(--foreground);
    background: var(--foreground);
    color: var(--background);
  }

  .settings-body {
    display: grid;
    min-height: 0;
    overflow-y: auto;
    scrollbar-gutter: stable;
  }

  .settings-status-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .settings-status-grid > div {
    display: grid;
    gap: 6px;
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .settings-status-grid strong {
    display: block;
    margin-top: 4px;
    overflow: hidden;
    color: var(--foreground);
    font-size: 0.84rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .settings-status-grid span {
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
  }

  .settings-help {
    margin: 12px 0 0;
    line-height: 1.4;
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
  }

  .settings-issue {
    margin: 0;
    border: 1px solid var(--warning);
    border-radius: 8px;
    padding: 10px 12px;
    color: var(--warning);
  }

  @media (max-width: 640px) {
    :global(.settings-layer) {
      position: fixed;
    }

    .settings-panel {
      width: 100%;
      height: calc(100vh - 16px);
      max-height: calc(100vh - 16px);
    }

    .settings-status-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
