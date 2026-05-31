<script lang="ts">
  import { RefreshCw, X } from 'lucide-svelte';
  import { availabilityLabel, getAgentAvailability } from '$lib/agents';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import type { Agent, AppStatus, HouseholdSettings, VoiceStatus } from '$lib/types';

  export let agents: Agent[] = [];
  export let status: AppStatus | undefined;
  export let voice: VoiceStatus;
  export let settings: HouseholdSettings | undefined;
  export let issue = '';
  export let saving = false;
  export let savingAgent = false;
  export let agentCardUrl = '';
  export let activeSection: 'household' | 'agents' | 'mcp' | 'voice' | 'about' = 'household';
  export let onClose: () => void = () => {};
  export let onSaveHousehold: (settings: HouseholdSettings) => Promise<void> | void = () => {};
  export let onAddAgent: (cardUrl: string) => Promise<void> | void = () => {};
  export let onToggleAgent: (agent: Agent) => Promise<void> | void = () => {};
  export let onRemoveAgent: (agent: Agent) => Promise<void> | void = () => {};
  export let onRefreshAgentCard: (agentId: string) => Promise<void> | void = () => {};

  let draft: HouseholdSettings | undefined;
  let localAgentCardUrl = '';
  let refreshingAgentId = '';

  $: if (settings && (!draft || draft.setup !== settings.setup)) {
    draft = structuredClone(settings);
  }
  $: localAgentCardUrl = agentCardUrl || localAgentCardUrl;

  const sections = [
    ['household', 'Household'],
    ['agents', 'Agents'],
    ['mcp', 'MCP'],
    ['voice', 'Voice'],
    ['about', 'About']
  ] as const;

  function numeric(value: string) {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }

  async function saveHousehold() {
    if (!draft || saving) {
      return;
    }
    await onSaveHousehold(draft);
  }

  async function addAgent() {
    const cardUrl = localAgentCardUrl.trim();
    if (!cardUrl || savingAgent) {
      return;
    }
    await onAddAgent(cardUrl);
    localAgentCardUrl = '';
  }

  async function refreshAgent(agentId: string) {
    if (!agentId || refreshingAgentId) {
      return;
    }
    refreshingAgentId = agentId;
    try {
      await onRefreshAgentCard(agentId);
    } finally {
      refreshingAgentId = '';
    }
  }
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
      {#each sections as [id, label]}
        <button type="button" class:settings-tab--active={activeSection === id} on:click={() => (activeSection = id)}>
          {label}
        </button>
      {/each}
    </nav>

    {#if issue}
      <p class="settings-issue">{issue}</p>
    {/if}

    <div class="settings-body">
      {#if activeSection === 'household'}
        {#if draft}
          <div class="settings-form-grid">
            <label>
              <span>Home name</span>
              <input bind:value={draft.home.name} />
            </label>
            <label>
              <span>Timezone</span>
              <input bind:value={draft.home.timezone} placeholder="Europe/London" />
            </label>
            <label>
              <span>Locale</span>
              <input bind:value={draft.home.locale} placeholder="en-GB" />
            </label>
            <label>
              <span>Theme</span>
              <select bind:value={draft.display.theme}>
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </select>
            </label>
            <label>
              <span>Weather location</span>
              <input bind:value={draft.weather.locationName} />
            </label>
            <label>
              <span>Latitude</span>
              <input
                type="number"
                step="0.0001"
                value={draft.weather.latitude}
                on:input={(event) => (draft && (draft.weather.latitude = numeric(event.currentTarget.value)))}
              />
            </label>
            <label>
              <span>Longitude</span>
              <input
                type="number"
                step="0.0001"
                value={draft.weather.longitude}
                on:input={(event) => (draft && (draft.weather.longitude = numeric(event.currentTarget.value)))}
              />
            </label>
            <label>
              <span>Temperature unit</span>
              <select bind:value={draft.weather.temperatureUnit}>
                <option value="celsius">Celsius</option>
                <option value="fahrenheit">Fahrenheit</option>
              </select>
            </label>
          </div>
          <div class="settings-actions">
            <Button on:click={saveHousehold} disabled={saving}>{saving ? 'Saving' : 'Save household'}</Button>
          </div>
        {:else}
          <p class="settings-empty">Household settings are loading.</p>
        {/if}
      {:else if activeSection === 'agents'}
        <form class="settings-add-form" on:submit|preventDefault={addAgent}>
          <input bind:value={localAgentCardUrl} placeholder="http://127.0.0.1:9797/.well-known/agent-card.json" />
          <Button type="submit" disabled={savingAgent || !localAgentCardUrl.trim()}>{savingAgent ? 'Adding' : 'Add agent'}</Button>
        </form>
        <div class="settings-list">
          {#if agents.length === 0}
            <p class="settings-empty">No agents configured yet.</p>
          {:else}
            {#each agents as agent}
              <article class="settings-list-item">
                <div>
                  <strong>{agent.name}</strong>
                  <span>{agent.cardUrl}</span>
                  <div class="settings-badges">
                    <Badge tone="neutral">{availabilityLabel(getAgentAvailability(agent))}</Badge>
                    <Badge tone="neutral">{agent.selectedProtocolBinding || agent.protocolBinding}</Badge>
                    {#if agent.dashboardContextSupported}
                      <Badge tone="active">screen context</Badge>
                    {/if}
                  </div>
                </div>
                <div class="settings-item-actions">
                  <Button size="sm" variant="outline" on:click={() => refreshAgent(agent.id)} disabled={refreshingAgentId === agent.id}>
                    <RefreshCw size={15} />
                    <span>{refreshingAgentId === agent.id ? 'Refreshing' : 'Refresh'}</span>
                  </Button>
                  <Button size="sm" variant="outline" on:click={() => onToggleAgent(agent)}>{agent.enabled ? 'Disable' : 'Enable'}</Button>
                  <Button size="sm" variant="ghost" on:click={() => onRemoveAgent(agent)}>Remove</Button>
                </div>
              </article>
            {/each}
          {/if}
        </div>
      {:else if activeSection === 'mcp'}
        <div class="settings-status-grid">
          <div><span>Status</span><strong>{status?.mcp.enabled ? status.mcp.serviceStatus : 'disabled'}</strong></div>
          <div><span>Transport</span><strong>{status?.mcp.transport || 'streamable-http'}</strong></div>
          <div><span>Path</span><strong>{status?.mcp.path || '/mcp'}</strong></div>
          <div><span>Auth</span><strong>{status?.mcp.authMode || 'not configured'}</strong></div>
        </div>
        <p class="settings-help">MCP is configured at hub startup. Edit the harness or bootstrap config, then restart the stack to change it.</p>
      {:else if activeSection === 'voice'}
        <div class="settings-status-grid">
          <div><span>Status</span><strong>{voice.serviceStatus}</strong></div>
          <div><span>State</span><strong>{voice.state}</strong></div>
          <div><span>STT provider</span><strong>{voice.sttProviderId || 'not configured'}</strong></div>
          <div><span>TTS provider</span><strong>{voice.ttsProviderId || 'not configured'}</strong></div>
        </div>
        <p class="settings-help">Voice provider selection is planned next. This panel currently shows the safe hub status only.</p>
      {:else}
        <div class="settings-status-grid">
          <div><span>Hub version</span><strong>{status?.version || 'dev'}</strong></div>
          <div><span>Setup</span><strong>{status?.setup.complete ? 'complete' : 'incomplete'}</strong></div>
          <div><span>Config</span><strong>{status?.config.writableYaml ? 'writable YAML' : 'runtime store'}</strong></div>
          <div><span>Agents</span><strong>{status?.agents.enabled ?? agents.filter((agent) => agent.enabled).length} enabled</strong></div>
        </div>
      {/if}
    </div>
  </section>
</div>
