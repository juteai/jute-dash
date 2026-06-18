<script lang="ts">
  export let gap: 'tight' | 'normal' | 'loose' = 'normal';
  export let stale = false;
  let className = '';
  export { className as class };
</script>

<div
  class={`widget-content-stack widget-content-stack--${gap} ${className}`}
  class:widget-content-stack--stale={stale}
>
  <slot />
</div>

<style>
  .widget-content-stack {
    display: flex;
    flex-direction: column;
    width: 100%;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    user-select: none;
  }

  .widget-content-stack--tight {
    gap: clamp(6px, 2cqmin, 10px);
  }

  .widget-content-stack--normal {
    gap: clamp(8px, 2.5cqmin, 14px);
  }

  .widget-content-stack--loose {
    gap: clamp(12px, 4cqmin, 20px);
  }

  .widget-content-stack--stale {
    opacity: 0.64;
    pointer-events: none;
  }
</style>
