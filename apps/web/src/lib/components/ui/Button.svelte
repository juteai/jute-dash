<script lang="ts">
  import { cn } from '$lib/utils';

  export let variant: 'default' | 'secondary' | 'outline' | 'ghost' | 'danger' = 'default';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
  let className = '';
  export { className as class };
</script>

<button
  {type}
  {disabled}
  class={cn('ui-button', `ui-button--${variant}`, `ui-button--${size}`, className)}
  on:click
>
  <slot />
</button>
