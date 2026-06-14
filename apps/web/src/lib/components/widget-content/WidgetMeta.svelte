<script lang="ts">
  export let separator = true;
</script>

<span class="widget-meta" class:widget-meta--separator={separator}>
  <slot />
</span>

<style>
  .widget-meta {
    display: inline-flex;
    min-width: 0;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    line-height: 1.25;
  }

  .widget-meta--separator :global(span + span)::before {
    margin-right: 6px;
    color: var(--muted);
    content: '·';
    opacity: 0.55;
  }
</style>
