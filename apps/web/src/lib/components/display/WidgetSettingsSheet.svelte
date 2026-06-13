<script lang="ts">
  import { onMount } from 'svelte';
  import { X, Plus, Trash2 } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import { getAdapterConnections } from '$lib/hubClient';
  import type {
    AdapterConnection,
    SettingField,
    WidgetCatalogItem,
    WidgetInstance,
    WidgetMode
  } from '$lib/types';

  export let widget: WidgetInstance;
  export let catalogItem: WidgetCatalogItem | undefined;
  export let onClose: () => void = () => {};
  export let onSave: (patch: {
    title: string;
    settings: Record<string, unknown>;
    connectionRefs: Record<string, string>;
    mode: WidgetMode;
  }) => void = () => {};

  const CHROME_OPTIONS = ['auto', 'solid', 'clear', 'smoked', 'frosted'];

  let title = widget.title;
  let mode: WidgetMode = widget.mode === 'headless' ? 'headless' : 'ui';
  // Working copy of settings so edits are committed only on save.
  let settings: Record<string, unknown> = structuredClone(
    widget.settings ?? {}
  );
  let connectionRefs: Record<string, string> = structuredClone(
    widget.connectionRefs ?? {}
  );
  let connections: AdapterConnection[] = [];
  let connectionIssue = '';
  $: chrome = (settings.chrome as string) || 'auto';

  $: schema = catalogItem?.settingsSchema ?? [];
  $: connectionRequirements = catalogItem?.connectionRequirements ?? [];

  onMount(async () => {
    if (connectionRequirements.length === 0) return;
    try {
      connections = await getAdapterConnections(fetch);
      connectionIssue = '';
    } catch {
      connectionIssue = 'Connections are unavailable.';
    }
  });

  function defaultForField(field: SettingField): unknown {
    if (field.type === 'string-list') return [];
    if (field.type === 'object-list') return [];
    if (field.type === 'boolean') return false;
    if (field.type === 'number') return field.default ?? 0;
    return field.default ?? '';
  }

  function valueFor(field: SettingField): unknown {
    const current = settings[field.id];
    return current === undefined ? defaultForField(field) : current;
  }

  function setValue(id: string, value: unknown) {
    settings = { ...settings, [id]: value };
  }

  function setChrome(value: string) {
    if (value === 'auto') {
      const next = { ...settings };
      delete next.chrome;
      settings = next;
    } else {
      setValue('chrome', value);
    }
  }

  function connectionsForKind(kind: string): AdapterConnection[] {
    return connections.filter((connection) => connection.kind === kind);
  }

  function setConnectionRef(slot: string, id: string) {
    const next = { ...connectionRefs };
    if (id) {
      next[slot] = id;
    } else {
      delete next[slot];
    }
    connectionRefs = next;
  }

  // string-list helpers
  function listValue(field: SettingField): string[] {
    const v = valueFor(field);
    return Array.isArray(v) ? (v as string[]) : [];
  }
  function addListItem(field: SettingField) {
    setValue(field.id, [...listValue(field), '']);
  }
  function updateListItem(field: SettingField, index: number, value: string) {
    const next = [...listValue(field)];
    next[index] = value;
    setValue(field.id, next);
  }
  function removeListItem(field: SettingField, index: number) {
    const next = listValue(field).filter((_, i) => i !== index);
    setValue(field.id, next);
  }

  // object-list helpers
  function objectListValue(field: SettingField): Record<string, unknown>[] {
    const v = valueFor(field);
    return Array.isArray(v) ? (v as Record<string, unknown>[]) : [];
  }
  function addObjectItem(field: SettingField) {
    const blank: Record<string, unknown> = {};
    for (const sub of field.fields ?? []) {
      blank[sub.id] = defaultForField(sub);
    }
    setValue(field.id, [...objectListValue(field), blank]);
  }
  function updateObjectItem(
    field: SettingField,
    index: number,
    key: string,
    value: unknown
  ) {
    const next = objectListValue(field).map((item, i) =>
      i === index ? { ...item, [key]: value } : item
    );
    setValue(field.id, next);
  }
  function removeObjectItem(field: SettingField, index: number) {
    setValue(
      field.id,
      objectListValue(field).filter((_, i) => i !== index)
    );
  }

  function commit() {
    onSave({
      title: title.trim() || widget.title,
      settings,
      connectionRefs,
      mode
    });
    onClose();
  }
</script>

<div
  class="sheet-backdrop"
  role="button"
  tabindex="-1"
  on:click={onClose}
  on:keydown={(e) => e.key === 'Escape' && onClose()}
></div>
<aside class="settings-sheet" aria-label={`Configure ${widget.title}`}>
  <header class="sheet-header">
    <div>
      <div class="sheet-eyebrow">{catalogItem?.name ?? widget.kind}</div>
      <h2>Configure widget</h2>
    </div>
    <IconButton label="Close" variant="ghost" on:click={onClose}>
      <X size={20} />
    </IconButton>
  </header>

  <div class="sheet-body">
    <!-- Frame settings (common to all widgets) -->
    <section class="field-group">
      <label class="field">
        <span class="field-label">Title</span>
        <input class="text-input" type="text" bind:value={title} />
      </label>

      <label class="field">
        <span class="field-label">Appearance (chrome)</span>
        <select
          class="text-input"
          value={chrome}
          on:change={(e) => setChrome((e.target as HTMLSelectElement).value)}
        >
          {#each CHROME_OPTIONS as option (option)}
            <option value={option}>{option}</option>
          {/each}
        </select>
      </label>

      <label class="field field-inline">
        <input
          type="checkbox"
          checked={mode === 'headless'}
          on:change={(e) =>
            (mode = (e.target as HTMLInputElement).checked ? 'headless' : 'ui')}
        />
        <span class="field-label">
          Headless (feeds the assistant, hides the tile)
        </span>
      </label>
    </section>

    {#if schema.length > 0}
      <section class="field-group">
        <div class="group-title">Widget settings</div>
        {#each schema as field (field.id)}
          {#if field.type === 'string' || field.type === 'number'}
            <label class="field">
              <span class="field-label">{field.label}</span>
              <input
                class="text-input"
                type={field.type === 'number' ? 'number' : 'text'}
                value={valueFor(field) as string | number}
                on:input={(e) =>
                  setValue(
                    field.id,
                    field.type === 'number'
                      ? Number((e.target as HTMLInputElement).value)
                      : (e.target as HTMLInputElement).value
                  )}
              />
              {#if field.help}<span class="field-help">{field.help}</span>{/if}
            </label>
          {:else if field.type === 'boolean'}
            <label class="field field-inline">
              <input
                type="checkbox"
                checked={Boolean(valueFor(field))}
                on:change={(e) =>
                  setValue(field.id, (e.target as HTMLInputElement).checked)}
              />
              <span class="field-label">{field.label}</span>
            </label>
          {:else if field.type === 'enum'}
            <label class="field">
              <span class="field-label">{field.label}</span>
              <select
                class="text-input"
                value={valueFor(field) as string}
                on:change={(e) =>
                  setValue(field.id, (e.target as HTMLSelectElement).value)}
              >
                {#each field.options ?? [] as option (option)}
                  <option value={option}>{option}</option>
                {/each}
              </select>
            </label>
          {:else if field.type === 'string-list'}
            <div class="field">
              <span class="field-label">{field.label}</span>
              {#if field.help}<span class="field-help">{field.help}</span>{/if}
              {#each listValue(field) as item, index (index)}
                <div class="list-row">
                  <input
                    class="text-input"
                    type="text"
                    value={item}
                    on:input={(e) =>
                      updateListItem(
                        field,
                        index,
                        (e.target as HTMLInputElement).value
                      )}
                  />
                  <IconButton
                    label="Remove"
                    variant="ghost"
                    on:click={() => removeListItem(field, index)}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </div>
              {/each}
              <Button
                size="sm"
                variant="outline"
                on:click={() => addListItem(field)}
              >
                <Plus size={15} /><span>Add</span>
              </Button>
            </div>
          {:else if field.type === 'object-list'}
            <div class="field">
              <span class="field-label">{field.label}</span>
              {#each objectListValue(field) as item, index (index)}
                <div class="object-card">
                  <div class="object-card-header">
                    <span>#{index + 1}</span>
                    <IconButton
                      label="Remove"
                      variant="ghost"
                      on:click={() => removeObjectItem(field, index)}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </div>
                  {#each field.fields ?? [] as sub (sub.id)}
                    <label class="field">
                      <span class="field-label">{sub.label}</span>
                      <input
                        class="text-input"
                        type={sub.type === 'number' ? 'number' : 'text'}
                        value={(item[sub.id] ?? '') as string | number}
                        on:input={(e) =>
                          updateObjectItem(
                            field,
                            index,
                            sub.id,
                            sub.type === 'number'
                              ? Number((e.target as HTMLInputElement).value)
                              : (e.target as HTMLInputElement).value
                          )}
                      />
                    </label>
                  {/each}
                </div>
              {/each}
              <Button
                size="sm"
                variant="outline"
                on:click={() => addObjectItem(field)}
              >
                <Plus size={15} /><span>Add</span>
              </Button>
            </div>
          {/if}
        {/each}
      </section>
    {/if}

    {#if connectionRequirements.length > 0}
      <section class="field-group">
        <div class="group-title">Connections</div>
        {#if connectionIssue}
          <p class="sheet-issue">{connectionIssue}</p>
        {/if}
        {#each connectionRequirements as requirement (requirement.slot)}
          <label class="field">
            <span class="field-label">{requirement.displayName}</span>
            <select
              class="text-input"
              value={connectionRefs[requirement.slot] ?? ''}
              on:change={(e) =>
                setConnectionRef(
                  requirement.slot,
                  (e.target as HTMLSelectElement).value
                )}
            >
              <option value="">
                {requirement.required ? 'Choose connection' : 'No connection'}
              </option>
              {#each connectionsForKind(requirement.kind) as connection (connection.id)}
                <option value={connection.id}
                  >{connection.name || connection.id}</option
                >
              {/each}
            </select>
            <span class="field-help">
              {requirement.description ||
                `Uses a shared ${requirement.kind} Adapter Connection from Settings.`}
            </span>
          </label>
        {/each}
      </section>
    {/if}
  </div>

  <footer class="sheet-footer">
    <Button variant="ghost" on:click={onClose}>Cancel</Button>
    <Button on:click={commit}>Save</Button>
  </footer>
</aside>

<style>
  .sheet-backdrop {
    position: fixed;
    inset: 0;
    z-index: 60;
    background: rgba(0, 0, 0, 0.5);
  }
  .settings-sheet {
    position: fixed;
    z-index: 61;
    display: flex;
    flex-direction: column;
    background: var(--surface, #161616);
    color: var(--foreground, inherit);
    border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
    /* Mobile-first: full-width bottom sheet */
    left: 0;
    right: 0;
    bottom: 0;
    max-height: 88vh;
    border-radius: 16px 16px 0 0;
  }
  @media (min-width: 768px) {
    .settings-sheet {
      left: auto;
      right: 24px;
      bottom: 24px;
      top: 24px;
      width: 420px;
      border-radius: 14px;
      max-height: none;
    }
  }
  .sheet-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border, rgba(255, 255, 255, 0.1));
  }
  .sheet-header h2 {
    margin: 2px 0 0;
    font-size: 1.05rem;
  }
  .sheet-eyebrow {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.6;
  }
  .sheet-body {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 22px;
  }
  .field-group {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .group-title {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.6;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .field-inline {
    flex-direction: row;
    align-items: center;
    gap: 10px;
  }
  .field-label {
    font-size: 0.82rem;
    font-weight: 600;
  }
  .field-help {
    font-size: 0.74rem;
    opacity: 0.6;
  }
  .text-input {
    width: 100%;
    min-height: 44px;
    padding: 9px 12px;
    border-radius: 9px;
    border: 1px solid var(--border, rgba(255, 255, 255, 0.18));
    background: var(--background, rgba(0, 0, 0, 0.2));
    color: inherit;
    font-size: 0.9rem;
  }
  .list-row {
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .object-card {
    border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
    border-radius: 10px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .object-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.78rem;
    opacity: 0.7;
  }
  .sheet-footer {
    display: flex;
    justify-content: flex-end;
    gap: 10px;
    padding: 14px 20px;
    border-top: 1px solid var(--border, rgba(255, 255, 255, 0.1));
  }

  .sheet-issue {
    margin: 0;
    color: var(--danger, #ef4444);
    font-size: 0.82rem;
    font-weight: 700;
  }
</style>
