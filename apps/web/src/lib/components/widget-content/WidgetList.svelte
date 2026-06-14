<script lang="ts">
  export let gap: 'tight' | 'normal' | 'loose' = 'normal';
</script>

<div class={`widget-list widget-list--${gap}`}>
  <slot />
</div>

<style>
  .widget-list {
    display: flex;
    flex: 1;
    min-height: 0;
    flex-direction: column;
    overflow-y: auto;
    padding-right: 4px;
  }

  .widget-list--tight {
    gap: clamp(5px, 2cqmin, 8px);
  }

  .widget-list--normal {
    gap: clamp(6px, 2.5cqmin, 12px);
  }

  .widget-list--loose {
    gap: clamp(10px, 4cqmin, 18px);
  }
</style>
