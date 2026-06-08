<script lang="ts">
  import { cn } from '$lib/utils';

  export let value = '';
  export let placeholder = '';
  export let rows = 1;
  export let disabled = false;
  let className = '';
  export { className as class };
</script>

<textarea
  bind:value
  {placeholder}
  {rows}
  {disabled}
  class={cn('ui-textarea', className)}
  on:keydown
  on:input
></textarea>

<style>
  .ui-textarea {
    width: 100%;
    min-height: 44px;
    max-height: 128px;
    resize: none;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 11px 12px;
    font: inherit;
  }

  .ui-textarea:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .ui-textarea:disabled {
    cursor: not-allowed;
    opacity: 0.48;
  }
</style>
