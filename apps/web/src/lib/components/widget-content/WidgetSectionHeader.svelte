<script lang="ts">
  export let title: string;
  export let count: string | number | undefined = undefined;
</script>

<div class="widget-section-header">
  <div class="widget-section-title">
    <slot name="icon" />
    <h3>{title}</h3>
    {#if count !== undefined}
      <span class="widget-section-count">{count}</span>
    {/if}
  </div>
  <slot name="action" />
</div>

<style>
  .widget-section-header,
  .widget-section-title {
    display: flex;
    align-items: center;
  }

  .widget-section-header {
    justify-content: space-between;
    gap: 8px;
    padding-bottom: 2px;
  }

  .widget-section-title {
    min-width: 0;
    gap: 8px;
  }

  .widget-section-title :global(svg) {
    flex: 0 0 auto;
    color: var(--muted);
  }

  h3 {
    margin: 0;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--widget-label-size, 0.75rem);
    font-weight: 740;
    line-height: 1.1;
    text-overflow: ellipsis;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .widget-section-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1.4em;
    padding: 1px 6px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--surface-strong);
    color: var(--foreground);
    font-size: 0.65rem;
    font-weight: 740;
    line-height: 1;
  }
</style>
