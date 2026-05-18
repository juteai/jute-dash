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
