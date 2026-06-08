<script lang="ts">
  import { cn } from '$lib/utils';

  export let label: string;
  export let variant: 'default' | 'ghost' | 'outline' | 'danger' = 'ghost';
  export let pressed = false;
  export let disabled = false;
  let className = '';
  export { className as class };
</script>

<button
  type="button"
  aria-label={label}
  title={label}
  aria-pressed={pressed}
  {disabled}
  class={cn('ui-icon-button', className)}
  class:default={variant === 'default'}
  class:ghost={variant === 'ghost'}
  class:outline={variant === 'outline'}
  class:danger={variant === 'danger'}
  on:click
>
  <slot />
</button>

<style>
  .ui-icon-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: 44px;
    width: 44px;
    padding: 0;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 720;
    transition:
      background-color 160ms ease,
      color 160ms ease,
      border-color 160ms ease,
      opacity 160ms ease;
  }

  .ui-icon-button:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .default {
    background: var(--foreground);
    color: var(--inverse);
  }

  .ghost {
    background: transparent;
    color: var(--foreground);
  }

  .outline {
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--foreground);
  }

  .danger {
    background: var(--danger);
    color: var(--inverse);
  }

  .ui-icon-button:hover:not(:disabled) {
    background: var(--surface-strong);
    color: var(--foreground);
  }

  .ui-icon-button:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }
</style>
