<script lang="ts">
  import DeviceRow from './DeviceRow.svelte';

  export let devices: any[] = [];
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};
</script>

<div class="devices-list">
  <p class="title">Philips Hue Lights</p>
  {#if devices.length === 0}
    <p class="empty">No devices discovered yet.</p>
  {:else}
    {#each devices as dev}
      <DeviceRow device={dev} {dispatch} />
    {/each}
  {/if}
</div>

<style>
  .title {
    font-weight: bold;
    margin-bottom: 8px;
  }
  .devices-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .empty {
    color: var(--muted);
    font-size: 0.8rem;
  }
</style>
