<script lang="ts">
  export let clickable = false;
  export let disabled = false;
  export let href = '';
  export let direction: 'row' | 'column' = 'row';
  let className = '';
  export { className as class };
</script>

{#if clickable || href}
  <button
    type="button"
    class={`widget-list-item widget-list-item--${direction} widget-list-item--clickable ${className}`}
    {disabled}
    on:click
  >
    <slot />
  </button>
{:else}
  <div class={`widget-list-item widget-list-item--${direction} ${className}`}>
    <slot />
  </div>
{/if}

<style>
  .widget-list-item {
    display: flex;
    width: 100%;
    min-width: 0;
    gap: clamp(6px, 2cqmin, 10px);
    padding: clamp(7px, 3cqmin, 12px);
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    color: inherit;
    font: inherit;
    text-align: left;
    transition:
      background-color 0.18s ease,
      border-color 0.18s ease,
      color 0.18s ease,
      transform 0.18s ease;
  }

  .widget-list-item--row {
    flex-direction: row;
    align-items: center;
  }

  .widget-list-item--column {
    flex-direction: column;
    align-items: flex-start;
  }

  button.widget-list-item {
    cursor: pointer;
  }

  button.widget-list-item:disabled {
    cursor: default;
    opacity: 0.64;
  }

  .widget-list-item--clickable:hover:not(:disabled) {
    border-color: var(--border-strong);
    background: var(--surface-strong);
    transform: scale(1.01);
  }

  .widget-list-item:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: -1px;
  }
</style>
