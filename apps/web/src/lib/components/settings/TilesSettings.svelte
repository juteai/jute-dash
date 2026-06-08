<script lang="ts">
  /* eslint-disable no-useless-assignment */
  import Button from '$lib/components/ui/Button.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import type { Tile } from '$lib/types';
  import { uniqueId } from '$lib/utils';

  let tileDrafts: Tile[] = [];
  let lastJSON = '';

  $: {
    const next = JSON.stringify($settingsStore.tileSettings);
    if (next !== lastJSON) {
      tileDrafts = structuredClone($settingsStore.tileSettings);
      lastJSON = next;
    }
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

  async function save() {
    if ($settingsStore.savingTiles) {
      return;
    }
    try {
      await settingsStore.saveTiles(tileDrafts);
    } catch {
      // Error is set in settingsStore.issue
    }
  }
</script>

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
  <Button on:click={save} disabled={$settingsStore.savingTiles}
    >{$settingsStore.savingTiles ? 'Saving' : 'Save tiles'}</Button
  >
</div>

<style>
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

  .settings-editor-item {
    align-items: flex-start;
  }

  .settings-editor-item .settings-form-grid {
    flex: 1;
  }

  .settings-form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .settings-form-grid label {
    display: grid;
    gap: 6px;
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .settings-form-grid label span {
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 760;
  }

  .settings-form-grid input {
    min-width: 0;
    min-height: 42px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 10px;
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

  .settings-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    justify-content: flex-end;
    margin-top: 12px;
  }

  @media (max-width: 640px) {
    .settings-form-grid {
      grid-template-columns: 1fr;
    }

    .settings-list-item {
      align-items: stretch;
      flex-direction: column;
    }
  }
</style>
