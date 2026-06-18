<script lang="ts">
  export let columns = 3;
</script>

<div class="widget-value-row" style={`--widget-value-row-columns: ${columns};`}>
  <slot />
</div>

<style>
  .widget-value-row {
    display: grid;
    grid-template-columns: repeat(
      var(--widget-value-row-columns),
      minmax(0, 1fr)
    );
    gap: clamp(6px, 1.5cqmin, 12px);
    padding: clamp(6px, 2cqmin, 10px) 0;
    border-top: 1px dashed var(--border);
    border-bottom: 1px dashed var(--border);
  }
</style>
