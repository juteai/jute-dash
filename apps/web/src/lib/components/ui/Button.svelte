<script lang="ts">
  import { cn } from '$lib/utils';

  export let variant: 'default' | 'secondary' | 'outline' | 'ghost' | 'danger' =
    'default';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
  let className = '';
  export { className as class };
</script>

<button
  {type}
  {disabled}
  class={cn('ui-button', className)}
  class:default={variant === 'default'}
  class:secondary={variant === 'secondary'}
  class:ui-outline={variant === 'outline'}
  class:ghost={variant === 'ghost'}
  class:danger={variant === 'danger'}
  class:sm={size === 'sm'}
  class:md={size === 'md'}
  class:lg={size === 'lg'}
  on:click
>
  <slot />
</button>

<style>
  .ui-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: 44px;
    border-radius: 8px;
    cursor: pointer;
    font-weight: 720;
    padding: 0 14px;
    transition:
      background-color 160ms ease,
      color 160ms ease,
      border-color 160ms ease,
      opacity 160ms ease;
  }

  .ui-button:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .sm {
    min-height: 36px;
    padding: 0 10px;
    font-size: 0.86rem;
  }

  .lg {
    min-height: 52px;
    padding: 0 18px;
  }

  .default {
    background: var(--foreground);
    color: var(--inverse);
  }

  .secondary {
    background: var(--surface-strong);
    color: var(--foreground);
  }

  .ui-outline {
    border: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
    background: transparent;
    color: var(--foreground);
  }

  .ghost {
    background: transparent;
    color: var(--foreground);
  }

  .danger {
    background: var(--danger);
    color: var(--inverse);
  }

  .ui-button:hover:not(:disabled) {
    background: var(--surface-strong);
    color: var(--foreground);
  }

  .default:hover:not(:disabled) {
    opacity: 0.86;
  }

  .ui-button:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }
</style>
