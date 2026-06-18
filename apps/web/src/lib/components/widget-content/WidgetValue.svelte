<script lang="ts">
  export let label: string;
  export let value: string;
</script>

<span class="widget-value">
  <span class="widget-value-label">{label}</span>
  <span class="widget-value-body">
    <slot>{value}</slot>
  </span>
</span>

<style>
  .widget-value {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 2px;
  }

  .widget-value-label {
    overflow: hidden;
    color: var(--muted);
    font-size: 0.65rem;
    font-weight: 740;
    line-height: 1.1;
    text-overflow: ellipsis;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .widget-value-body {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 4px;
    color: var(--foreground);
    font-size: 0.82rem;
    font-weight: 650;
  }

  .widget-value-body :global(svg) {
    flex: 0 0 auto;
    color: var(--muted);
  }
</style>
