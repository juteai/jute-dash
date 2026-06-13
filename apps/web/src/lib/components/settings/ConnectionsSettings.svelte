<script lang="ts">
  import { onMount } from 'svelte';
  import { Plug, Plus, Save } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import {
    getAdapterConnectionKinds,
    getAdapterConnections,
    saveAdapterConnection
  } from '$lib/hubClient';
  import { hubStream } from '$lib/hubStream';
  import type {
    AdapterConnection,
    AdapterConnectionKind,
    ConnectionField
  } from '$lib/types';

  let connections: AdapterConnection[] = [];
  let connectionKinds: AdapterConnectionKind[] = [];
  let selectedId = '';
  let draft = blankConnection();
  let loading = false;
  let saving = false;
  let issue = '';

  $: selected = connections.find((connection) => connection.id === selectedId);
  $: selectedKind = connectionKinds.find((kind) => kind.kind === draft.kind);

  onMount(() => {
    void load();
  });

  function defaultKind(): string {
    return connectionKinds[0]?.kind ?? 'philips-hue';
  }

  function blankConnection(kind = defaultKind()): AdapterConnection {
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
      [connectionKinds, connections] = await Promise.all([
        getAdapterConnectionKinds(fetch),
        getAdapterConnections(fetch)
      ]);
      if (connections.length > 0) {
        selectConnection(connections[0]);
      } else {
        newConnection(defaultKind());
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
  }

  function newConnection(kind = defaultKind()) {
    selectedId = '';
    draft = blankConnection(kind);
    issue = '';
  }

  function changeKind(kind: string) {
    draft = {
      ...draft,
      kind,
      settings: {},
      secretRefs: {}
    };
  }

  function fieldValue(field: ConnectionField): unknown {
    const source = field.secret ? draft.secretRefs : draft.settings;
    const current = source?.[field.id];
    if (current !== undefined) return current;
    if (field.default !== undefined) return field.default;
    if (field.type === 'boolean') return false;
    return '';
  }

  function setFieldValue(field: ConnectionField, value: unknown) {
    const key = field.secret ? 'secretRefs' : 'settings';
    const next = { ...(draft[key] ?? {}) };
    if (value === '' || value === undefined) {
      delete next[field.id];
    } else {
      next[field.id] = value;
    }
    draft = { ...draft, [key]: next };
  }

  function inputValue(event: Event, field: ConnectionField): unknown {
    const input = event.currentTarget as HTMLInputElement | HTMLSelectElement;
    if (field.type === 'number') {
      return input.value === '' ? '' : Number(input.value);
    }
    if (field.type === 'boolean') {
      return (input as HTMLInputElement).checked;
    }
    return input.value;
  }

  async function save() {
    saving = true;
    issue = '';
    try {
      const saved = await saveAdapterConnection(fetch, {
        ...draft,
        id: draft.id.trim(),
        name: draft.name.trim()
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
      <select
        class="text-input"
        value={draft.kind}
        on:change={(event) =>
          changeKind((event.currentTarget as HTMLSelectElement).value)}
      >
        {#each connectionKinds as kind (kind.kind)}
          <option value={kind.kind}>{kind.displayName || kind.kind}</option>
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

    {#if selectedKind}
      <div class="field-group">
        <span class="field-label">{selectedKind.displayName}</span>
        {#if selectedKind.description}
          <span class="field-help">{selectedKind.description}</span>
        {/if}
        {#each selectedKind.fields as field (field.id)}
          {#if field.type === 'boolean'}
            <label class="field field-inline">
              <input
                type="checkbox"
                checked={Boolean(fieldValue(field))}
                on:change={(event) =>
                  setFieldValue(field, inputValue(event, field))}
              />
              <span class="field-label">{field.label}</span>
            </label>
          {:else if field.type === 'enum'}
            <label class="field">
              <span class="field-label"
                >{field.label}{field.required ? ' *' : ''}</span
              >
              <select
                class="text-input"
                value={fieldValue(field) as string}
                required={field.required}
                on:change={(event) =>
                  setFieldValue(field, inputValue(event, field))}
              >
                <option value=""></option>
                {#each field.options ?? [] as option (option)}
                  <option value={option}>{option}</option>
                {/each}
              </select>
              {#if field.help}<span class="field-help">{field.help}</span>{/if}
            </label>
          {:else}
            <label class="field">
              <span class="field-label"
                >{field.label}{field.required ? ' *' : ''}</span
              >
              <input
                class="text-input"
                type={field.type === 'number' ? 'number' : 'text'}
                value={fieldValue(field) as string | number}
                required={field.required}
                placeholder={field.secret ? 'env:SECRET_NAME' : ''}
                on:input={(event) =>
                  setFieldValue(field, inputValue(event, field))}
              />
              {#if field.help}<span class="field-help">{field.help}</span>{/if}
            </label>
          {/if}
        {/each}
      </div>
    {:else}
      <p class="settings-issue">
        This Adapter Connection kind is not registered by a built-in Widget.
      </p>
    {/if}

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

  .field-group {
    display: grid;
    gap: 10px;
    padding-top: 4px;
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
