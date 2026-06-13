<script lang="ts">
  export let data: any = {};
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

  $: devices = data?.devices ?? [];
</script>

<div class="widget-content">
  <div class="devices-list">
    <p class="title">Zigbee Devices</p>
    {#if devices.length === 0}
      <p class="empty">No Zigbee devices discovered.</p>
    {:else}
      {#each devices as dev}
        <div class="device-row">
          <span>{dev.name}</span>
          {#if dev.type === 'switch' || dev.type === 'light'}
            <button on:click={() => dispatch('toggle', { device_id: dev.id, state: !dev.state })}>
              {dev.state ? 'ON' : 'OFF'}
            </button>
          {:else if dev.type === 'sensor'}
            <span class="sensor-val">{dev.value}</span>
          {/if}
        </div>
      {/each}
    {/if}
  </div>
</div>

<style>
  .widget-content {
    padding: 12px;
    height: 100%;
    overflow-y: auto;
  }
  .title {
    font-weight: bold;
    margin-bottom: 4px;
  }
  .devices-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .device-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.85rem;
  }
  .device-row button {
    padding: 2px 8px;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .sensor-val {
    font-weight: bold;
    color: var(--muted);
  }
  .empty {
    color: var(--muted);
    font-size: 0.8rem;
  }
</style>
