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
