<script lang="ts">
  export let message: string;
</script>

<div class="widget-empty">
  <slot name="icon" />
  <span>{message}</span>
</div>

<style>
  .widget-empty {
    display: flex;
    flex: 1;
    min-height: 100%;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: clamp(8px, 4cqmin, 16px);
    color: var(--muted);
    text-align: center;
  }

  .widget-empty :global(svg) {
    opacity: 0.34;
  }

  span {
    font-size: var(--widget-body-size);
    line-height: 1.35;
  }
</style>
