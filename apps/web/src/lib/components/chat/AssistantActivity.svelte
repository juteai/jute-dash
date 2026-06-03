<script lang="ts">
  import type { ChatState } from '$lib/types';

  export let state: ChatState = 'idle';
</script>

<div
  class={`assistant-activity assistant-activity--${state}`}
  aria-live="polite"
>
  <span></span>
  <span></span>
  <span></span>
</div>
