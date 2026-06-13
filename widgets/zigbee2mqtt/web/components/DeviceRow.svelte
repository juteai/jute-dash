<script lang="ts">
  export let device: any;
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};
</script>

<div class="device-row">
  <span>{device.name}</span>
  {#if device.type === 'switch' || device.type === 'light'}
    <button on:click={() => dispatch('toggle', { device_id: device.id, state: !device.state })}>
      {device.state ? 'ON' : 'OFF'}
    </button>
  {:else if device.type === 'sensor'}
    <span class="sensor-val">{device.value}</span>
  {/if}
</div>

<style>
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
</style>
