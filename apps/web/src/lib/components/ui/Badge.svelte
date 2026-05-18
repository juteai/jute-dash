<script lang="ts">
  import { cn } from '$lib/utils';

  export let tone: 'neutral' | 'active' | 'warning' | 'danger' = 'neutral';
  let className = '';
  export { className as class };
</script>

<span class={cn('ui-badge', `ui-badge--${tone}`, className)}>
  <slot />
</span>
