<script lang="ts">
  export let tone: 'neutral' | 'active' | 'success' | 'warning' | 'danger' =
    'neutral';
  export let pulse = false;
</script>

<span class={`widget-badge widget-badge--${tone}`}>
  {#if pulse}
    <span class="widget-badge-pulse"></span>
  {/if}
  <slot />
</span>

<style>
  .widget-badge {
    --widget-badge-color: var(--muted);
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 1px 5px;
    border-radius: 3px;
    background: color-mix(in srgb, var(--widget-badge-color) 12%, transparent);
    color: var(--widget-badge-color);
    font-size: 0.6rem;
    font-weight: 740;
    line-height: 1;
    text-transform: uppercase;
  }

  .widget-badge--active {
    --widget-badge-color: var(--active);
  }

  .widget-badge--success {
    --widget-badge-color: var(--success);
  }

  .widget-badge--warning {
    --widget-badge-color: var(--warning);
  }

  .widget-badge--danger {
    --widget-badge-color: var(--danger);
  }

  .widget-badge-pulse {
    width: 5px;
    height: 5px;
    border-radius: 999px;
    background: currentColor;
    animation: widget-badge-pulse 1.8s infinite ease-in-out;
  }

  @keyframes widget-badge-pulse {
    0%,
    100% {
      opacity: 0.5;
      transform: scale(0.85);
    }

    50% {
      opacity: 1;
      transform: scale(1.15);
    }
  }
</style>
