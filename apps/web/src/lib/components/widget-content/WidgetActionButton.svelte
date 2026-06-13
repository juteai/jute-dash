<script lang="ts">
  export let label: string;
  export let pressed = false;
  export let disabled = false;
</script>

<button
  type="button"
  class="widget-action-button"
  class:widget-action-button--pressed={pressed}
  aria-label={label}
  aria-pressed={pressed}
  {disabled}
  on:click
>
  <slot />
</button>

<style>
  .widget-action-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: clamp(30px, 10cqmin, 40px);
    height: clamp(30px, 10cqmin, 40px);
    min-width: 30px;
    min-height: 30px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: transparent;
    color: var(--foreground);
    cursor: pointer;
    transition:
      background-color 0.18s ease,
      border-color 0.18s ease,
      color 0.18s ease,
      transform 0.18s ease;
  }

  .widget-action-button:hover:not(:disabled),
  .widget-action-button--pressed {
    border-color: var(--border-strong);
    background: var(--surface-strong);
    color: var(--active);
  }

  .widget-action-button:active:not(:disabled) {
    transform: scale(0.96);
  }

  .widget-action-button:disabled {
    cursor: default;
    opacity: 0.52;
  }

  .widget-action-button:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }
</style>
