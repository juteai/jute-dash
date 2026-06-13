<script lang="ts">
  import WidgetBadge from '$lib/components/widget-content/WidgetBadge.svelte';

  export let name: string;
  export let type = '';
  export let active = false;
  export let value: string | number | boolean | undefined;
  export let disabled = false;
  export let onToggle: (() => Promise<void> | void) | undefined;
</script>

<div class="device-row">
  <div class="device-copy">
    <span class="device-name">{name}</span>
    {#if type}
      <span class="device-type">{type}</span>
    {/if}
  </div>
  {#if onToggle}
    <button
      type="button"
      class="device-toggle"
      class:device-toggle--active={active}
      {disabled}
      on:click={onToggle}
    >
      {active ? 'On' : 'Off'}
    </button>
  {:else if value !== undefined}
    <WidgetBadge tone="neutral">{String(value)}</WidgetBadge>
  {/if}
</div>

<style>
  .device-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    min-width: 0;
    font-size: var(--widget-body-size);
  }

  .device-copy {
    display: grid;
    min-width: 0;
    gap: 2px;
  }

  .device-name {
    overflow: hidden;
    color: var(--foreground);
    font-weight: 650;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .device-type {
    color: var(--muted);
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    text-transform: capitalize;
  }

  .device-toggle {
    min-width: 48px;
    min-height: 30px;
    padding: 0 10px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    color: var(--muted);
    cursor: pointer;
    font-size: 0.72rem;
    font-weight: 760;
    text-transform: uppercase;
  }

  .device-toggle--active {
    border-color: color-mix(in srgb, var(--success) 55%, var(--border));
    background: color-mix(in srgb, var(--success) 14%, transparent);
    color: var(--success);
  }

  .device-toggle:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }

  .device-toggle:disabled {
    cursor: default;
    opacity: 0.55;
  }
</style>
