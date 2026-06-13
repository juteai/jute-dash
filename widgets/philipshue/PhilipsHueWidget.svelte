<script lang="ts">
  export let instanceId: string = '';
  export let data: any = {};
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

  $: isConfigured = data?.is_configured ?? false;
  $: devices = data?.devices ?? [];
  $: bridgeIP = data?.bridge_ip ?? '';

  let loading = false;
  let errorMsg = '';
  let successMsg = '';

  async function handleLinkBridge() {
    if (!bridgeIP) {
      errorMsg = 'Please configure Bridge IP in widget settings first.';
      return;
    }
    loading = true;
    errorMsg = '';
    successMsg = '';
    try {
      const resp = await fetch('/api/widgets/philips-hue/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          instance_id: instanceId,
          bridge_ip: bridgeIP
        })
      });

      if (!resp.ok) {
        let errMsg = 'Failed to register bridge';
        try {
          const errData = await resp.json();
          if (errData.error) errMsg = errData.error;
        } catch {
          // ignore
        }
        errorMsg = errMsg;
      } else {
        successMsg = 'Bridge linked successfully!';
        const { hubStream } = await import('$lib/hubStream');
        await hubStream.refreshAfterMutation();
      }
    } catch (err: any) {
      errorMsg = err.message || 'Failed to connect to Jute Hub';
    } finally {
      loading = false;
    }
  }
</script>

<div class="widget-content">
  {#if !isConfigured}
    <div class="unconfigured">
      <p class="title">Philips Hue</p>
      {#if !bridgeIP}
        <p class="help">Configure your Bridge IP in the settings sheet first.</p>
      {:else}
        <p class="help">Bridge IP: {bridgeIP}</p>
        <p class="prompt">Press the link button on your physical Hue Bridge, then click Link below.</p>
        <button class="connect-btn" on:click={handleLinkBridge} disabled={loading}>
          {loading ? 'Linking...' : 'Link Bridge'}
        </button>
      {/if}
      {#if errorMsg}
        <p class="error">{errorMsg}</p>
      {/if}
      {#if successMsg}
        <p class="success">{successMsg}</p>
      {/if}
    </div>
  {:else}
    <div class="devices-list">
      <p class="title">Philips Hue Lights</p>
      {#if devices.length === 0}
        <p class="empty">No devices discovered yet.</p>
      {:else}
        {#each devices as dev}
          <div class="device-row">
            <span>{dev.name}</span>
            <button on:click={() => dispatch('toggle_light', { device_id: dev.id, state: !dev.state })}>
              {dev.state ? 'ON' : 'OFF'}
            </button>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .widget-content {
    padding: 12px;
    height: 100%;
    overflow-y: auto;
  }
  .unconfigured {
    text-align: center;
    padding-top: 16px;
  }
  .title {
    font-weight: bold;
    margin-bottom: 8px;
  }
  .connect-btn {
    padding: 6px 12px;
    background: var(--foreground);
    color: var(--inverse);
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  .connect-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
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
  .empty {
    color: var(--muted);
    font-size: 0.8rem;
  }
  .help {
    color: var(--muted);
    font-size: 0.8rem;
    margin-bottom: 4px;
  }
  .prompt {
    color: var(--muted);
    font-size: 0.8rem;
    margin-bottom: 12px;
  }
  .error {
    color: #ef4444;
    font-size: 0.8rem;
    margin-top: 8px;
  }
  .success {
    color: #22c55e;
    font-size: 0.8rem;
    margin-top: 8px;
  }
</style>
