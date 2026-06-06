<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import {
    MoreVertical,
    Settings2,
    EyeOff,
    Trash2,
    ArrowUp,
    ArrowDown
  } from 'lucide-svelte';
  import WidgetFrame from '$lib/components/display/WidgetFrame.svelte';
  import { widgetRegistry } from '$lib/components/display/widget-registry';
  import { resolveWidgetChrome } from '$lib/themes';
  import { BASE_COLUMNS, GRID_GAP, rendersTile } from '$lib/layout-editor';
  import type {
    Agent,
    AgentAvailability,
    ChatMessage,
    DashboardData,
    WidgetInstance
  } from '$lib/types';

  export let data: DashboardData;
  export let editMode = false;
  export let messages: ChatMessage[] = [];
  export let stale = false;
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let focusedWidgetId = '';
  export let onOpenChat: () => void = () => {};
  export let onMoveWidget: (
    widgetId: string,
    x: number,
    y: number
  ) => void = () => {};
  export let onResizeWidget: (
    widgetId: string,
    w: number,
    h: number
  ) => void = () => {};
  export let onRemoveWidget: (widgetId: string) => void = () => {};
  export let onConfigureWidget: (widgetId: string) => void = () => {};
  export let onSetHeadless: (widgetId: string) => void = () => {};
  export let onReorderWidget: (
    widgetId: string,
    direction: -1 | 1
  ) => void = () => {};

  let canvasEl: HTMLElement;
  let viewportWidth = 1280;
  let openMenuId = '';

  // Fine drag/resize placement is available only on tablet and larger; phones
  // use reorder-only editing through the per-tile menu.
  $: fineEdit = editMode && viewportWidth >= 768;
  // The configured 12-column layout renders identically on every real screen and
  // is scaled to fit (see proportional rows below). Only the <=640px phone
  // fallback collapses to a single scrolling column, handled purely in CSS.
  const activeColumns = BASE_COLUMNS;
  $: widgets = data.layout.widgets
    .filter(rendersTile)
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));

  let drag:
    | {
        id: string;
        mode: 'move' | 'resize' | 'resize-w' | 'resize-h';
        startClientX: number;
        startClientY: number;
        startX: number;
        startY: number;
        startW: number;
        startH: number;
        cellWidth: number;
        rowHeight: number;
        pointerId: number;
      }
    | undefined;

  let dragDX = 0;
  let dragDY = 0;
  let ghostX = 0;
  let ghostY = 0;
  let ghostW = 0;
  let ghostH = 0;

  // The canvas fills its height with exactly as many proportional (1fr) rows as
  // the configured layout occupies, so the same layout scales to any screen.
  // Not capped at MAX_ROWS: stored layouts may legitimately run deeper and we
  // render them faithfully rather than clipping.
  $: highestY = widgets.reduce((max, w) => Math.max(max, w.y + w.h), 0);
  $: rowCount = Math.max(1, highestY);

  function updateViewport() {
    if (typeof window !== 'undefined') {
      viewportWidth = window.innerWidth;
    }
  }

  onMount(() => {
    updateViewport();
    window.addEventListener('resize', updateViewport);
    return () => window.removeEventListener('resize', updateViewport);
  });

  function gridStyle(widget: WidgetInstance) {
    const columns = Math.min(Math.max(widget.w, widget.minW, 1), activeColumns);
    const rows = Math.max(widget.h, widget.minH, 1);
    // Cell height comes from the proportional 1fr row tracks; no fixed px height.
    return `grid-column-start: ${widget.x + 1}; grid-column-end: span ${columns}; grid-row-start: ${widget.y + 1}; grid-row-end: span ${rows};`;
  }

  function dragStyle(widget: WidgetInstance) {
    if (!drag || drag.id !== widget.id) {
      return '';
    }
    if (drag.mode === 'move') {
      return `transform: translate3d(${dragDX}px, ${dragDY}px, 0) scale(1.02);`;
    }
    let extra = '';
    if (drag.mode === 'resize-w' || drag.mode === 'resize') {
      const targetWidth = Math.max(
        drag.cellWidth - GRID_GAP,
        drag.startW * drag.cellWidth + dragDX - GRID_GAP
      );
      extra += `width: ${targetWidth}px; max-width: none;`;
    }
    if (drag.mode === 'resize-h' || drag.mode === 'resize') {
      const targetHeight = Math.max(
        drag.rowHeight - GRID_GAP,
        drag.startH * drag.rowHeight + dragDY - GRID_GAP
      );
      extra += `height: ${targetHeight}px; max-height: none;`;
    }
    return extra;
  }

  function toggleMenu(id: string) {
    openMenuId = openMenuId === id ? '' : id;
  }

  function closeMenu() {
    openMenuId = '';
  }

  function determineWidgetState(
    widget: WidgetInstance,
    appStale: boolean
  ):
    | 'ok'
    | 'loading'
    | 'empty'
    | 'unavailable'
    | 'error'
    | 'permission_required'
    | 'stale' {
    if (appStale) {
      return 'stale';
    }
    const data = widget.data as { status?: string } | undefined;
    if (!data) {
      return 'empty';
    }
    if (data.status === 'unavailable' || data.status === 'offline') {
      return 'unavailable';
    }
    if (data.status === 'error' || data.status === 'failed') {
      return 'error';
    }
    if (data.status === 'loading') {
      return 'loading';
    }
    if (data.status === 'permission_required') {
      return 'permission_required';
    }
    return 'ok';
  }

  function startDrag(
    widget: WidgetInstance,
    mode: 'move' | 'resize' | 'resize-w' | 'resize-h',
    event: PointerEvent
  ) {
    if (!fineEdit || !canvasEl) {
      return;
    }
    event.stopPropagation();
    event.preventDefault();

    try {
      canvasEl.setPointerCapture(event.pointerId);
    } catch {
      // ignore
    }

    const metrics = gridMetrics();
    drag = {
      id: widget.id,
      mode,
      startClientX: event.clientX,
      startClientY: event.clientY,
      startX: widget.x,
      startY: widget.y,
      startW: widget.w,
      startH: widget.h,
      cellWidth: metrics.cellWidth,
      rowHeight: metrics.rowHeight,
      pointerId: event.pointerId
    };
    dragDX = 0;
    dragDY = 0;
    ghostX = widget.x;
    ghostY = widget.y;
    ghostW = widget.w;
    ghostH = widget.h;

    window.addEventListener('pointermove', handleDragMove);
    window.addEventListener('pointerup', endDrag, { once: true });
    window.addEventListener('pointercancel', endDrag, { once: true });
  }

  function handleDragMove(event: PointerEvent) {
    if (!drag) {
      return;
    }
    dragDX = event.clientX - drag.startClientX;
    dragDY = event.clientY - drag.startClientY;

    const gridDX = Math.round(dragDX / drag.cellWidth);
    const gridDY = Math.round(dragDY / drag.rowHeight);

    const widget = widgets.find((w) => w.id === drag!.id);
    const minW = widget ? widget.minW || 1 : 1;
    const minH = widget ? widget.minH || 1 : 1;

    if (drag.mode === 'move') {
      ghostX = Math.min(
        Math.max(drag.startX + gridDX, 0),
        activeColumns - drag.startW
      );
      ghostY = Math.max(drag.startY + gridDY, 0);
      ghostW = drag.startW;
      ghostH = drag.startH;
    } else {
      ghostX = drag.startX;
      ghostY = drag.startY;
      if (drag.mode === 'resize-w' || drag.mode === 'resize') {
        ghostW = Math.min(
          Math.max(drag.startW + gridDX, minW),
          activeColumns - drag.startX
        );
      } else {
        ghostW = drag.startW;
      }
      if (drag.mode === 'resize-h' || drag.mode === 'resize') {
        ghostH = Math.max(drag.startH + gridDY, minH);
      } else {
        ghostH = drag.startH;
      }
    }
  }

  function endDrag(event?: PointerEvent) {
    if (drag) {
      const pId = event?.pointerId ?? drag.pointerId;
      try {
        canvasEl.releasePointerCapture(pId);
      } catch {
        // ignore
      }

      if (drag.mode === 'move') {
        if (ghostX !== drag.startX || ghostY !== drag.startY) {
          onMoveWidget(drag.id, ghostX, ghostY);
        }
      } else {
        if (ghostW !== drag.startW || ghostH !== drag.startH) {
          onResizeWidget(drag.id, ghostW, ghostH);
        }
      }
    }
    drag = undefined;
    if (typeof window !== 'undefined') {
      window.removeEventListener('pointermove', handleDragMove);
    }
  }

  function gridMetrics() {
    const styles = window.getComputedStyle(canvasEl);
    const rect = canvasEl.getBoundingClientRect();

    const columns =
      styles.gridTemplateColumns.split(' ').filter(Boolean).length ||
      activeColumns;
    const columnGap =
      Number.parseFloat(styles.columnGap || `${GRID_GAP}`) || GRID_GAP;

    // Rows are proportional (1fr) so the rendered row step must be measured from
    // the DOM rather than assumed from a fixed pixel height.
    const rows =
      styles.gridTemplateRows.split(' ').filter(Boolean).length || rowCount || 1;
    const rowGap = Number.parseFloat(styles.rowGap || `${GRID_GAP}`) || GRID_GAP;

    return {
      cellWidth: Math.max(
        1,
        (rect.width - columnGap * Math.max(0, columns - 1)) / columns + columnGap
      ),
      rowHeight: Math.max(
        1,
        (rect.height - rowGap * Math.max(0, rows - 1)) / rows + rowGap
      )
    };
  }

  onDestroy(endDrag);
</script>

<svelte:window on:click={closeMenu} />

<section
  bind:this={canvasEl}
  class:dashboard-grid-edit={editMode}
  class="dashboard-canvas"
  style={`--dashboard-grid-gap: ${GRID_GAP}px; grid-template-columns: repeat(${activeColumns}, minmax(0, 1fr)); grid-template-rows: repeat(${rowCount}, minmax(0, 1fr));`}
  aria-label="Widget dashboard"
>
  {#if editMode && fineEdit}
    <div
      class="dashboard-grid-background-grid"
      style="grid-template-columns: repeat({activeColumns}, minmax(0, 1fr)); grid-template-rows: repeat({rowCount}, minmax(0, 1fr));"
    >
      {#each Array.from({ length: rowCount * activeColumns }, (_, idx) => idx) as idx (idx)}
        <div class="dashboard-grid-cell-guide"></div>
      {/each}
    </div>
  {/if}

  {#if drag}
    <div
      class="dashboard-widget-placeholder"
      style="grid-column-start: {ghostX +
        1}; grid-column-end: span {ghostW}; grid-row-start: {ghostY +
        1}; grid-row-end: span {ghostH};"
    ></div>
  {/if}

  {#each widgets as widget (widget.id)}
    <div
      class="dashboard-widget-slot"
      class:dashboard-widget-slot--dragging={drag && drag.id === widget.id}
      style="{gridStyle(widget)} {drag && drag.id === widget.id
        ? dragStyle(widget)
        : ''}"
      data-widget-id={widget.id}
    >
      <WidgetFrame
        {widget}
        editMode={fineEdit}
        focused={focusedWidgetId === widget.id}
        chrome={resolveWidgetChrome(widget, data.config.display)}
        overflow={(widget.overflow ?? 'clip') as 'clip' | 'scroll' | 'expand'}
        state={determineWidgetState(widget, stale)}
        onMoveStart={(event) => startDrag(widget, 'move', event)}
        onResizeStart={(event, resizeMode) =>
          startDrag(
            widget,
            resizeMode === 'w'
              ? 'resize-w'
              : resizeMode === 'h'
                ? 'resize-h'
                : 'resize',
            event
          )}
      >
        <svelte:fragment slot="actions">
          {#if editMode}
            <div class="widget-menu" role="none" on:pointerdown|stopPropagation>
              <button
                type="button"
                class="widget-frame-handle"
                aria-label={`Widget options for ${widget.title}`}
                aria-haspopup="menu"
                aria-expanded={openMenuId === widget.id}
                on:click|stopPropagation={() => toggleMenu(widget.id)}
              >
                <MoreVertical size={17} />
              </button>
              {#if openMenuId === widget.id}
                <div class="widget-menu-dropdown" role="menu">
                  <button
                    type="button"
                    role="menuitem"
                    on:click|stopPropagation={() => {
                      onConfigureWidget(widget.id);
                      closeMenu();
                    }}
                  >
                    <Settings2 size={15} />
                    <span>Configure</span>
                  </button>
                  {#if !fineEdit}
                    <button
                      type="button"
                      role="menuitem"
                      on:click|stopPropagation={() => {
                        onReorderWidget(widget.id, -1);
                        closeMenu();
                      }}
                    >
                      <ArrowUp size={15} />
                      <span>Move up</span>
                    </button>
                    <button
                      type="button"
                      role="menuitem"
                      on:click|stopPropagation={() => {
                        onReorderWidget(widget.id, 1);
                        closeMenu();
                      }}
                    >
                      <ArrowDown size={15} />
                      <span>Move down</span>
                    </button>
                  {/if}
                  <button
                    type="button"
                    role="menuitem"
                    on:click|stopPropagation={() => {
                      onSetHeadless(widget.id);
                      closeMenu();
                    }}
                  >
                    <EyeOff size={15} />
                    <span>Make headless</span>
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    class="widget-menu-danger"
                    on:click|stopPropagation={() => {
                      onRemoveWidget(widget.id);
                      closeMenu();
                    }}
                  >
                    <Trash2 size={15} />
                    <span>Remove</span>
                  </button>
                </div>
              {/if}
            </div>
          {/if}
        </svelte:fragment>
        {#if widgetRegistry[widget.kind]}
          <svelte:component
            this={widgetRegistry[widget.kind].component}
            {...widgetRegistry[widget.kind].props({
              widget,
              data,
              stale,
              messages,
              selectedAgent,
              selectedAvailability,
              onOpenChat
            })}
          />
        {:else}
          <div class="widget-empty-state">
            <p>{widget.kind} is not available in this display build.</p>
          </div>
        {/if}
      </WidgetFrame>
    </div>
  {/each}
</section>

<style>
  .widget-menu {
    position: relative;
    display: inline-flex;
  }
  .widget-menu-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    z-index: 30;
    min-width: 180px;
    display: flex;
    flex-direction: column;
    padding: 4px;
    border-radius: 10px;
    background: var(--surface, #161616);
    border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.45);
  }
  .widget-menu-dropdown button {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    border: none;
    background: transparent;
    color: var(--foreground, inherit);
    font-size: 0.86rem;
    text-align: left;
    border-radius: 7px;
    cursor: pointer;
    min-height: 40px;
  }
  .widget-menu-dropdown button:hover {
    background: var(--surface-strong, rgba(255, 255, 255, 0.08));
  }
  .widget-menu-danger {
    color: var(--danger, #ef4444);
  }
</style>
