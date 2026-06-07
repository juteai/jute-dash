<script lang="ts">
  import { onMount } from 'svelte';
  import { RefreshCw, X, Trash2, Upload } from 'lucide-svelte';
  import { availabilityLabel, getAgentAvailability } from '$lib/agents';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import { themeOptions } from '$lib/themes';
  import {
    backgroundImageURL,
    deleteBackgroundImage,
    getBackgroundImages,
    uploadBackgroundImage
  } from '$lib/api';
  import type {
    Agent,
    AppStatus,
    BackgroundImage,
    HouseholdSettings,
    Room,
    Tile,
    VoiceStatus
  } from '$lib/types';

  export let agents: Agent[] = [];
  export let status: AppStatus | undefined;
  export let voice: VoiceStatus;
  export let settings: HouseholdSettings | undefined;
  export let rooms: Room[] = [];
  export let tiles: Tile[] = [];
  export let issue = '';
  export let saving = false;
  export let savingRooms = false;
  export let savingTiles = false;
  export let savingAgent = false;
  export let agentCardUrl = '';
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
  export let onSaveHousehold: (
    settings: HouseholdSettings
  ) => Promise<void> | void = () => {};
  export let onSaveRooms: (rooms: Room[]) => Promise<void> | void = () => {};
  export let onSaveTiles: (tiles: Tile[]) => Promise<void> | void = () => {};
  export let onAddAgent: (cardUrl: string) => Promise<void> | void = () => {};
  export let onToggleAgent: (agent: Agent) => Promise<void> | void = () => {};
  export let onRemoveAgent: (agent: Agent) => Promise<void> | void = () => {};
  export let onRefreshAgentCard: (
    agentId: string
  ) => Promise<void> | void = () => {};

  let draft: HouseholdSettings | undefined;
  let roomDrafts: Room[] = [];
  let tileDrafts: Tile[] = [];
  let lastJSON = {
    settings: '',
    rooms: '',
    tiles: ''
  };
  let localAgentCardUrl = '';
  let refreshingAgentId = '';

  $: if (settings) {
    const next = JSON.stringify(settings);
    if (next !== lastJSON.settings) {
      draft = structuredClone(settings);
      if (!draft.display.background) {
        draft.display.background = {
          kind: 'theme',
          value: '',
          fit: 'cover',
          position: 'center',
          overlay: 'none'
        };
      }
      lastJSON.settings = next;
    }
  }
  $: localAgentCardUrl = agentCardUrl || localAgentCardUrl;
  $: {
    const next = JSON.stringify(rooms);
    if (next !== lastJSON.rooms) {
      roomDrafts = structuredClone(rooms);
      lastJSON.rooms = next;
    }
  }
  $: {
    const next = JSON.stringify(tiles);
    if (next !== lastJSON.tiles) {
      tileDrafts = structuredClone(tiles);
      lastJSON.tiles = next;
    }
  }

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

  let backgroundLibrary: BackgroundImage[] = [];
  let uploadingBackground = false;
  let backgroundError = '';

  const BACKGROUND_KINDS = ['theme', 'color', 'file', 'slideshow'];

  onMount(() => {
    void loadBackgroundLibrary();
  });

  async function loadBackgroundLibrary() {
    try {
      backgroundLibrary = await getBackgroundImages(fetch);
    } catch {
      backgroundError = 'Background library is unavailable.';
    }
  }

  function ensureBackground() {
    if (!draft) {
      return;
    }
    if (!draft.display.background) {
      draft.display.background = {
        kind: 'theme',
        value: '',
        fit: 'cover',
        position: 'center',
        overlay: 'none'
      };
    }
  }

  async function handleBackgroundUpload(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }
    uploadingBackground = true;
    backgroundError = '';
    try {
      const image = await uploadBackgroundImage(fetch, file);
      backgroundLibrary = [...backgroundLibrary, image].sort((a, b) =>
        a.name.localeCompare(b.name)
      );
    } catch (err) {
      backgroundError = err instanceof Error ? err.message : 'Upload failed.';
    } finally {
      uploadingBackground = false;
      input.value = '';
    }
  }

  async function removeBackgroundImage(name: string) {
    try {
      await deleteBackgroundImage(fetch, name);
      backgroundLibrary = backgroundLibrary.filter(
        (image) => image.name !== name
      );
    } catch {
      backgroundError = 'Could not delete image.';
    }
  }

  function selectSingleBackground(name: string) {
    if (!draft) {
      return;
    }
    ensureBackground();
    draft.display.background.kind = 'file';
    draft.display.background.value = name;
    draft = draft;
  }

  function toggleSlideshowImage(name: string) {
    if (!draft) {
      return;
    }
    ensureBackground();
    draft.display.background.kind = 'slideshow';
    const images = draft.display.background.images ?? [];
    draft.display.background.images = images.includes(name)
      ? images.filter((image) => image !== name)
      : [...images, name];
    draft = draft;
  }

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

  async function saveRoomDrafts() {
    if (savingRooms) {
      return;
    }
    await onSaveRooms(roomDrafts);
  }

  async function saveTileDrafts() {
    if (savingTiles) {
      return;
    }
    await onSaveTiles(tileDrafts);
  }

  function addRoom() {
    const id = uniqueId(
      'room',
      roomDrafts.map((room) => room.id)
    );
    roomDrafts = [
      ...roomDrafts,
      { id, name: 'New room', summary: '', status: 'Idle' }
    ];
  }

  function addTile() {
    const id = uniqueId(
      'tile',
      tileDrafts.map((tile) => tile.id)
    );
    tileDrafts = [
      ...tileDrafts,
      { id, kind: 'status', label: 'New tile', value: 'Value', detail: '' }
    ];
  }

  function uniqueId(prefix: string, existing: string[]) {
    const taken = new Set(existing);
    for (let index = 1; index < 1000; index += 1) {
      const candidate = `${prefix}-${index}`;
      if (!taken.has(candidate)) {
        return candidate;
      }
    }
    return `${prefix}-${Date.now()}`;
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
      {#each sections as [id, label] (id)}
        <button
          type="button"
          class:settings-tab--active={activeSection === id}
          on:click={() => (activeSection = id)}
        >
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
              <input
                bind:value={draft.home.timezone}
                placeholder="Europe/London"
              />
            </label>
            <label>
              <span>Locale</span>
              <input bind:value={draft.home.locale} placeholder="en-GB" />
            </label>
            <label>
              <span>Theme pack</span>
              <select bind:value={draft.display.themeId}>
                {#each themeOptions as option (option.id)}
                  <option value={option.id}>{option.name}</option>
                {/each}
              </select>
            </label>
            <label>
              <span>Color mode</span>
              <select
                bind:value={draft.display.colorMode}
                on:change={() =>
                  draft && (draft.display.theme = draft.display.colorMode)}
              >
                <option value="system">System</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </select>
            </label>
            <label>
              <span>Widget chrome</span>
              <select bind:value={draft.display.widgetChrome.default}>
                <option value="solid">Solid</option>
                <option value="clear">Clear</option>
                <option value="smoked">Smoked</option>
                <option value="frosted">Frosted</option>
                <option value="auto">Auto</option>
              </select>
            </label>
            <label class="settings-checkbox">
              <span>Weather</span>
              <input type="checkbox" bind:checked={draft.weather.enabled} />
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
                on:input={(event) =>
                  draft &&
                  (draft.weather.latitude = numeric(event.currentTarget.value))}
              />
            </label>
            <label>
              <span>Longitude</span>
              <input
                type="number"
                step="0.0001"
                value={draft.weather.longitude}
                on:input={(event) =>
                  draft &&
                  (draft.weather.longitude = numeric(
                    event.currentTarget.value
                  ))}
              />
            </label>
            <label>
              <span>Temperature unit</span>
              <select bind:value={draft.weather.temperatureUnit}>
                <option value="celsius">Celsius</option>
                <option value="fahrenheit">Fahrenheit</option>
              </select>
            </label>
            <label>
              <span>Wind speed unit</span>
              <select bind:value={draft.weather.windSpeedUnit}>
                <option value="kmh">km/h</option>
                <option value="mph">mph</option>
                <option value="ms">m/s</option>
                <option value="kn">knots</option>
              </select>
            </label>
          </div>
          <div class="settings-actions">
            <Button on:click={saveHousehold} disabled={saving}
              >{saving ? 'Saving' : 'Save household'}</Button
            >
          </div>
        {:else}
          <p class="settings-empty">Household settings are loading.</p>
        {/if}
      {:else if activeSection === 'rooms'}
        <div class="settings-list">
          {#if roomDrafts.length === 0}
            <p class="settings-empty">No rooms configured yet.</p>
          {:else}
            {#each roomDrafts as room, index (room.id)}
              <article class="settings-list-item settings-editor-item">
                <div class="settings-form-grid">
                  <label>
                    <span>ID</span>
                    <input bind:value={roomDrafts[index].id} />
                  </label>
                  <label>
                    <span>Name</span>
                    <input bind:value={roomDrafts[index].name} />
                  </label>
                  <label>
                    <span>Status</span>
                    <input bind:value={roomDrafts[index].status} />
                  </label>
                  <label>
                    <span>Summary</span>
                    <input bind:value={roomDrafts[index].summary} />
                  </label>
                </div>
                <div class="settings-item-actions">
                  <Button
                    size="sm"
                    variant="ghost"
                    on:click={() =>
                      (roomDrafts = roomDrafts.filter(
                        (_, itemIndex) => itemIndex !== index
                      ))}>Remove</Button
                  >
                </div>
              </article>
            {/each}
          {/if}
        </div>
        <div class="settings-actions">
          <Button variant="outline" on:click={addRoom}>Add room</Button>
          <Button on:click={saveRoomDrafts} disabled={savingRooms}
            >{savingRooms ? 'Saving' : 'Save rooms'}</Button
          >
        </div>
      {:else if activeSection === 'tiles'}
        <div class="settings-list">
          {#if tileDrafts.length === 0}
            <p class="settings-empty">No tiles configured yet.</p>
          {:else}
            {#each tileDrafts as tile, index (tile.id)}
              <article class="settings-list-item settings-editor-item">
                <div class="settings-form-grid">
                  <label>
                    <span>ID</span>
                    <input bind:value={tileDrafts[index].id} />
                  </label>
                  <label>
                    <span>Kind</span>
                    <input bind:value={tileDrafts[index].kind} />
                  </label>
                  <label>
                    <span>Label</span>
                    <input bind:value={tileDrafts[index].label} />
                  </label>
                  <label>
                    <span>Value</span>
                    <input bind:value={tileDrafts[index].value} />
                  </label>
                  <label>
                    <span>Detail</span>
                    <input bind:value={tileDrafts[index].detail} />
                  </label>
                </div>
                <div class="settings-item-actions">
                  <Button
                    size="sm"
                    variant="ghost"
                    on:click={() =>
                      (tileDrafts = tileDrafts.filter(
                        (_, itemIndex) => itemIndex !== index
                      ))}>Remove</Button
                  >
                </div>
              </article>
            {/each}
          {/if}
        </div>
        <div class="settings-actions">
          <Button variant="outline" on:click={addTile}>Add tile</Button>
          <Button on:click={saveTileDrafts} disabled={savingTiles}
            >{savingTiles ? 'Saving' : 'Save tiles'}</Button
          >
        </div>
      {:else if activeSection === 'agents'}
        <form class="settings-add-form" on:submit|preventDefault={addAgent}>
          <input
            bind:value={localAgentCardUrl}
            placeholder="http://127.0.0.1:9797/.well-known/agent-card.json"
          />
          <Button
            type="submit"
            disabled={savingAgent || !localAgentCardUrl.trim()}
            >{savingAgent ? 'Adding' : 'Add agent'}</Button
          >
        </form>
        <div class="settings-list">
          {#if agents.length === 0}
            <p class="settings-empty">No agents configured yet.</p>
          {:else}
            {#each agents as agent (agent.id)}
              <article class="settings-list-item">
                <div>
                  <strong>{agent.name}</strong>
                  <span>{agent.cardUrl}</span>
                  <div class="settings-badges">
                    <Badge tone="neutral"
                      >{availabilityLabel(getAgentAvailability(agent))}</Badge
                    >
                    <Badge tone="neutral"
                      >{agent.selectedProtocolBinding ||
                        agent.protocolBinding}</Badge
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
                      >{refreshingAgentId === agent.id
                        ? 'Refreshing'
                        : 'Refresh'}</span
                    >
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    on:click={() => onToggleAgent(agent)}
                    >{agent.enabled ? 'Disable' : 'Enable'}</Button
                  >
                  <Button
                    size="sm"
                    variant="ghost"
                    on:click={() => onRemoveAgent(agent)}>Remove</Button
                  >
                </div>
              </article>
            {/each}
          {/if}
        </div>
      {:else if activeSection === 'mcp'}
        <div class="settings-status-grid">
          <div>
            <span>Status</span><strong
              >{status?.mcp.enabled
                ? status.mcp.serviceStatus
                : 'disabled'}</strong
            >
          </div>
          <div>
            <span>Transport</span><strong
              >{status?.mcp.transport || 'streamable-http'}</strong
            >
          </div>
          <div>
            <span>Path</span><strong>{status?.mcp.path || '/mcp'}</strong>
          </div>
          <div>
            <span>Auth</span><strong
              >{status?.mcp.authMode || 'not configured'}</strong
            >
          </div>
        </div>
        <p class="settings-help">
          MCP is configured at hub startup. Edit the harness or bootstrap
          config, then restart the stack to change it.
        </p>
      {:else if activeSection === 'voice'}
        <div class="settings-status-grid">
          <div><span>Status</span><strong>{voice.serviceStatus}</strong></div>
          <div><span>State</span><strong>{voice.state}</strong></div>
          <div>
            <span>STT provider</span><strong
              >{voice.sttProviderId || 'not configured'}</strong
            >
          </div>
          <div>
            <span>TTS provider</span><strong
              >{voice.ttsProviderId || 'not configured'}</strong
            >
          </div>
        </div>
        <p class="settings-help">
          Voice provider selection is planned next. This panel currently shows
          the safe hub status only.
        </p>
      {:else if activeSection === 'appearance'}
        {#if draft && draft.display.background}
          {@const bg = draft.display.background}
          {#if backgroundError}
            <p class="settings-issue">{backgroundError}</p>
          {/if}
          <div class="settings-form-grid">
            <label>
              <span>Background</span>
              <select bind:value={draft.display.background.kind}>
                {#each BACKGROUND_KINDS as kind (kind)}
                  <option value={kind}>{kind}</option>
                {/each}
              </select>
            </label>
            {#if bg.kind === 'color'}
              <label>
                <span>Color value</span>
                <input
                  bind:value={draft.display.background.value}
                  placeholder="#101010"
                />
              </label>
            {/if}
            <label>
              <span>Fit</span>
              <select bind:value={draft.display.background.fit}>
                <option value="cover">Cover</option>
                <option value="contain">Contain</option>
                <option value="tile">Tile</option>
              </select>
            </label>
            <label>
              <span>Overlay</span>
              <select bind:value={draft.display.background.overlay}>
                <option value="none">None</option>
                <option value="dim">Dim</option>
                <option value="smoked">Smoked</option>
                <option value="frosted">Frosted</option>
              </select>
            </label>
            {#if bg.kind === 'slideshow'}
              <label>
                <span>Interval (seconds)</span>
                <input
                  type="number"
                  min="3"
                  value={bg.intervalSeconds ?? 30}
                  on:input={(event) =>
                    draft &&
                    (draft.display.background.intervalSeconds = numeric(
                      event.currentTarget.value
                    ))}
                />
              </label>
            {/if}
          </div>

          {#if bg.kind === 'file' || bg.kind === 'slideshow'}
            <div class="settings-actions">
              <label class="background-upload">
                <Upload size={15} />
                <span
                  >{uploadingBackground ? 'Uploading…' : 'Upload image'}</span
                >
                <input
                  type="file"
                  accept="image/*"
                  hidden
                  on:change={handleBackgroundUpload}
                />
              </label>
            </div>
            {#if backgroundLibrary.length === 0}
              <p class="settings-empty">No background images uploaded yet.</p>
            {:else}
              <div class="background-grid">
                {#each backgroundLibrary as image (image.name)}
                  {@const selected =
                    bg.kind === 'slideshow'
                      ? (bg.images ?? []).includes(image.name)
                      : bg.value === image.name}
                  <div
                    class="background-thumb"
                    class:background-thumb--selected={selected}
                  >
                    <button
                      type="button"
                      class="background-thumb-pick"
                      style={`background-image: url("${backgroundImageURL(
                        image.name
                      )}")`}
                      aria-label={`Use ${image.name}`}
                      on:click={() =>
                        bg.kind === 'slideshow'
                          ? toggleSlideshowImage(image.name)
                          : selectSingleBackground(image.name)}
                    >
                      {#if selected}<span class="background-thumb-check">✓</span
                        >{/if}
                    </button>
                    <button
                      type="button"
                      class="background-thumb-delete"
                      aria-label={`Delete ${image.name}`}
                      on:click={() => removeBackgroundImage(image.name)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                {/each}
              </div>
            {/if}
          {/if}

          <div class="settings-actions">
            <Button on:click={saveHousehold} disabled={saving}
              >{saving ? 'Saving' : 'Save appearance'}</Button
            >
          </div>
        {:else}
          <p class="settings-empty">Appearance settings are loading.</p>
        {/if}
      {:else}
        <div class="settings-status-grid">
          <div>
            <span>Home</span><strong
              >{settings?.home.name || 'Jute Dash'}</strong
            >
          </div>
          <div>
            <span>Hub version</span><strong>{status?.version || 'dev'}</strong>
          </div>
          <div>
            <span>Setup</span><strong
              >{status?.setup.complete ? 'complete' : 'incomplete'}</strong
            >
          </div>
          <div>
            <span>Config</span><strong
              >{status?.config.writableYaml
                ? 'writable YAML'
                : 'runtime store'}</strong
            >
          </div>
          <div>
            <span>Agents</span><strong
              >{status?.agents.enabled ??
                agents.filter((agent) => agent.enabled).length} enabled</strong
            >
          </div>
        </div>
      {/if}
    </div>
  </section>
</div>

<style>
  .background-upload {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 9px 14px;
    border-radius: 9px;
    border: 1px solid var(--border, rgba(255, 255, 255, 0.18));
    cursor: pointer;
    font-size: 0.86rem;
    min-height: 44px;
  }
  .background-upload:hover {
    background: var(--surface-strong, rgba(255, 255, 255, 0.06));
  }
  .background-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 10px;
    margin-top: 12px;
  }
  .background-thumb {
    position: relative;
    border-radius: 10px;
    overflow: hidden;
    border: 2px solid transparent;
  }
  .background-thumb--selected {
    border-color: var(--accent, #6366f1);
  }
  .background-thumb-pick {
    display: block;
    width: 100%;
    aspect-ratio: 16 / 10;
    background-size: cover;
    background-position: center;
    border: none;
    cursor: pointer;
    padding: 0;
  }
  .background-thumb-check {
    position: absolute;
    top: 6px;
    left: 6px;
    background: var(--accent, #6366f1);
    color: #fff;
    border-radius: 50%;
    width: 22px;
    height: 22px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
  }
  .background-thumb-delete {
    position: absolute;
    top: 6px;
    right: 6px;
    border: none;
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    border-radius: 6px;
    padding: 4px;
    cursor: pointer;
    display: inline-flex;
  }
</style>
