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
