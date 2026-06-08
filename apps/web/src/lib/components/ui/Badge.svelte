<script lang="ts">
  import { cn } from '$lib/utils';

  export let tone: 'neutral' | 'active' | 'warning' | 'danger' = 'neutral';
  let className = '';
  export { className as class };
</script>

<span
  class={cn('ui-badge', className)}
  class:active={tone === 'active'}
  class:warning={tone === 'warning'}
  class:danger={tone === 'danger'}
>
  <slot />
</span>

<style>
  .ui-badge {
    display: inline-flex;
    align-items: center;
    min-height: 24px;
    border: 1px solid var(--border);
    border-radius: 999px;
    padding: 0 8px;
    font-size: 0.72rem;
    font-weight: 760;
    text-transform: uppercase;
  }

  .active {
    border-color: var(--active);
    color: var(--active);
  }

  .warning {
    border-color: var(--warning);
    color: var(--warning);
  }

  .danger {
    border-color: var(--danger);
    color: var(--danger);
  }
</style>
