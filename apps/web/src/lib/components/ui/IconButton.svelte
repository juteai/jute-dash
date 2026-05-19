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
  class={cn('ui-icon-button', `ui-icon-button--${variant}`, className)}
  on:click
>
  <slot />
</button>
