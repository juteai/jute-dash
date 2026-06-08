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

  export let activeSection:
    | 'household'
    | 'rooms'
    | 'tiles'
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
        <div class="settings-status-grid">
          <div>
            <span>Status</span><strong
              >{$hubStream.dashboard.voice.serviceStatus}</strong
            >
          </div>
          <div>
            <span>State</span><strong>{$hubStream.dashboard.voice.state}</strong
            >
          </div>
          <div>
            <span>STT provider</span><strong
              >{$hubStream.dashboard.voice.sttProviderId ||
                'not configured'}</strong
            >
          </div>
          <div>
            <span>TTS provider</span><strong
              >{$hubStream.dashboard.voice.ttsProviderId ||
                'not configured'}</strong
            >
          </div>
        </div>
        <p class="settings-help">
          Voice provider selection is planned next. This panel currently shows
          the safe hub status only.
        </p>
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
