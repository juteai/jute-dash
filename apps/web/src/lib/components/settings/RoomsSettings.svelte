<script lang="ts">
  /* eslint-disable no-useless-assignment */
  import Button from '$lib/components/ui/Button.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import type { Room } from '$lib/types';
  import { uniqueId } from '$lib/utils';

  let roomDrafts: Room[] = [];
  let lastJSON = '';

  $: {
    const next = JSON.stringify($settingsStore.roomSettings);
    if (next !== lastJSON) {
      roomDrafts = structuredClone($settingsStore.roomSettings);
      lastJSON = next;
    }
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

  async function save() {
    if ($settingsStore.savingRooms) {
      return;
    }
    try {
      await settingsStore.saveRooms(roomDrafts);
    } catch {
      // Error is set in settingsStore.issue
    }
  }
</script>

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
  <Button on:click={save} disabled={$settingsStore.savingRooms}
    >{$settingsStore.savingRooms ? 'Saving' : 'Save rooms'}</Button
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
