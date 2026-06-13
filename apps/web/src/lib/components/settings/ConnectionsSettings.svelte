<script lang="ts">
  import { onMount } from 'svelte';
  import { Plug, Plus, Save } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { getAdapterConnections, saveAdapterConnection } from '$lib/hubClient';
  import { hubStream } from '$lib/hubStream';
  import type { AdapterConnection } from '$lib/types';

  const CONNECTION_KINDS = [
    'philips-hue',
    'zigbee2mqtt',
    'spotify',
    'apple-music'
  ];

  let connections: AdapterConnection[] = [];
  let selectedId = '';
  let draft = blankConnection();
  let settingsJSON = '{}';
  let secretRefsJSON = '{}';
  let loading = false;
  let saving = false;
  let issue = '';

  $: selected = connections.find((connection) => connection.id === selectedId);

  onMount(() => {
    void load();
  });

  function blankConnection(kind = 'philips-hue'): AdapterConnection {
    return {
      id: '',
      kind,
      name: '',
      settings: {},
      secretRefs: {},
      enabled: true
    };
  }

  async function load() {
    loading = true;
    issue = '';
    try {
      connections = await getAdapterConnections(fetch);
      if (connections.length > 0) {
        selectConnection(connections[0]);
      } else {
        newConnection();
      }
    } catch {
      issue = 'Connections could not be loaded.';
    } finally {
      loading = false;
    }
  }

  function selectConnection(connection: AdapterConnection) {
    selectedId = connection.id;
    draft = structuredClone(connection);
    settingsJSON = JSON.stringify(draft.settings ?? {}, null, 2);
    secretRefsJSON = JSON.stringify(draft.secretRefs ?? {}, null, 2);
  }

  function newConnection(kind = 'philips-hue') {
    selectedId = '';
    draft = blankConnection(kind);
    settingsJSON = '{}';
    secretRefsJSON = '{}';
    issue = '';
  }

  function parseRecord(value: string, label: string): Record<string, unknown> {
    const parsed = JSON.parse(value || '{}');
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error(`${label} must be a JSON object.`);
    }
    return parsed as Record<string, unknown>;
  }

  async function save() {
    saving = true;
    issue = '';
    try {
      const settings = parseRecord(settingsJSON, 'Settings');
      const secretRefs = parseRecord(
        secretRefsJSON,
        'Secret references'
      ) as Record<string, string>;
      const saved = await saveAdapterConnection(fetch, {
        ...draft,
        id: draft.id.trim(),
        name: draft.name.trim(),
        settings,
        secretRefs
      });
      const others = connections.filter(
        (connection) => connection.id !== saved.id
      );
      connections = [...others, saved].sort((a, b) =>
        a.name.localeCompare(b.name)
      );
      selectConnection(saved);
      await hubStream.refreshAfterMutation(fetch);
    } catch (err) {
      issue = err instanceof Error ? err.message : 'Connection was not saved.';
    } finally {
      saving = false;
    }
  }
</script>

<div class="connections-settings">
  <div class="connections-list" aria-label="Adapter connections">
    <div class="section-heading">
      <div>
        <strong>Connections</strong>
        <span>{connections.length} shared Adapter Connections</span>
      </div>
      <Button size="sm" variant="outline" on:click={() => newConnection()}>
        <Plus size={15} /><span>New</span>
      </Button>
    </div>

    {#if loading}
      <p class="muted">Loading connections...</p>
    {:else if connections.length === 0}
      <p class="muted">No shared connections yet.</p>
    {:else}
      {#each connections as connection (connection.id)}
        <button
          type="button"
          class="connection-row"
          class:connection-row--active={selected?.id === connection.id}
          on:click={() => selectConnection(connection)}
        >
          <Plug size={16} />
          <span>
            <strong>{connection.name || connection.id}</strong>
            <small>{connection.kind}</small>
          </span>
        </button>
      {/each}
    {/if}
  </div>

  <form class="connection-editor" on:submit|preventDefault={save}>
    <div class="section-heading">
      <div>
        <strong>{selectedId ? 'Edit connection' : 'New connection'}</strong>
        <span>Widget instances link to these by ID.</span>
      </div>
    </div>

    {#if issue}
      <p class="settings-issue">{issue}</p>
    {/if}

    <label class="field">
      <span class="field-label">ID</span>
      <input class="text-input" type="text" bind:value={draft.id} required />
    </label>

    <label class="field">
      <span class="field-label">Kind</span>
      <select class="text-input" bind:value={draft.kind}>
        {#each CONNECTION_KINDS as kind (kind)}
          <option value={kind}>{kind}</option>
        {/each}
      </select>
    </label>

    <label class="field">
      <span class="field-label">Name</span>
      <input class="text-input" type="text" bind:value={draft.name} required />
    </label>

    <label class="field field-inline">
      <input type="checkbox" bind:checked={draft.enabled} />
      <span class="field-label">Enabled</span>
    </label>

    <label class="field">
      <span class="field-label">Settings JSON</span>
      <textarea class="text-input json-input" bind:value={settingsJSON}
      ></textarea>
    </label>

    <label class="field">
      <span class="field-label">Secret references JSON</span>
      <textarea class="text-input json-input" bind:value={secretRefsJSON}
      ></textarea>
      <span class="field-help"
        >Use secret references such as {'{"username":"env:HUE_USERNAME"}'}; raw
        secret values are not returned to widgets.</span
      >
    </label>

    <div class="actions">
      <Button type="submit" disabled={saving}>
        <Save size={15} /><span>{saving ? 'Saving' : 'Save'}</span>
      </Button>
    </div>
  </form>
</div>

<style>
  .connections-settings {
    display: grid;
    grid-template-columns: minmax(180px, 260px) minmax(0, 1fr);
    gap: 14px;
  }

  .connections-list,
  .connection-editor {
    display: grid;
    align-content: start;
    gap: 10px;
  }

  .section-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .section-heading strong {
    display: block;
    color: var(--foreground);
  }

  .section-heading span,
  .muted,
  .field-help {
    color: var(--muted);
    font-size: 0.8rem;
    font-weight: 650;
  }

  .connection-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    gap: 8px;
    min-height: 46px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 8px;
    text-align: left;
    cursor: pointer;
  }

  .connection-row--active {
    border-color: var(--foreground);
    background: var(--surface-muted);
  }

  .connection-row strong,
  .connection-row small {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .connection-row small {
    color: var(--muted);
    font-size: 0.74rem;
    font-weight: 700;
  }

  .field {
    display: grid;
    gap: 6px;
  }

  .field-inline {
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
  }

  .field-label {
    color: var(--foreground);
    font-size: 0.82rem;
    font-weight: 750;
  }

  .text-input {
    min-height: 36px;
    width: 100%;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 8px 10px;
    font: inherit;
  }

  .json-input {
    min-height: 92px;
    resize: vertical;
    font-family:
      ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono',
      'Courier New', monospace;
    font-size: 0.8rem;
  }

  .settings-issue {
    margin: 0;
    color: var(--danger, #ef4444);
    font-size: 0.82rem;
    font-weight: 700;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
  }

  @media (max-width: 720px) {
    .connections-settings {
      grid-template-columns: 1fr;
    }
  }
</style>
