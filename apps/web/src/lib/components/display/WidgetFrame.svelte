<script lang="ts">
  import { Grip, Maximize2 } from 'lucide-svelte';
  import type { WidgetInstance } from '$lib/types';
  import { cn } from '$lib/utils';

  export let widget: WidgetInstance;
  export let editMode = false;
  export let overflow: 'clip' | 'scroll' | 'expand' = 'clip';
  export let onMoveStart: (event: PointerEvent) => void = () => {};
  export let onResizeStart: (event: PointerEvent) => void = () => {};
  let className = '';
  export { className as class };
</script>

<section
  class={cn('widget-frame', `widget-frame--${widget.size}`, `widget-frame--overflow-${overflow}`, className)}
  aria-label={widget.title}
>
  <header class="widget-frame-header">
    <div class="widget-frame-title">{widget.title}</div>
    <div class="widget-frame-actions">
      <slot name="actions" />
      {#if editMode}
        <button
          type="button"
          class="widget-frame-handle"
          aria-label={`Move ${widget.title}`}
          on:pointerdown|preventDefault={onMoveStart}
        >
          <Grip size={17} />
        </button>
      {/if}
    </div>
  </header>

  <div class="widget-frame-body">
    <slot />
  </div>

  {#if editMode}
    <button
      type="button"
      class="widget-resize-handle"
      aria-label={`Resize ${widget.title}`}
      on:pointerdown|preventDefault={onResizeStart}
    >
      <Maximize2 size={16} />
    </button>
  {/if}
</section>
