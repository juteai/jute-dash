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

<style>
  .assistant-activity {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .assistant-activity span {
    width: 7px;
    height: 7px;
    border-radius: 999px;
    background: var(--muted);
  }

  .assistant-activity--thinking span,
  .assistant-activity--streaming span,
  .assistant-activity--listening span {
    animation: activity-pulse 950ms ease-in-out infinite;
    background: var(--active);
  }

  .assistant-activity span:nth-child(2) {
    animation-delay: 120ms;
  }

  .assistant-activity span:nth-child(3) {
    animation-delay: 240ms;
  }

  @keyframes activity-pulse {
    0%,
    100% {
      opacity: 0.35;
      transform: translateY(0);
    }

    50% {
      opacity: 1;
      transform: translateY(-2px);
    }
  }
</style>
