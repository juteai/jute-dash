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
  import {
    BASE_COLUMNS,
    columnsForWidth,
    remapLayout,
    rendersTile
  } from '$lib/layout-editor';
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
  // In fine-edit the user edits the canonical 12-column base layout; otherwise
  // render a proportional remap for the current viewport.
  $: activeColumns = fineEdit ? BASE_COLUMNS : columnsForWidth(viewportWidth);
  $: displayLayout =
    activeColumns >= BASE_COLUMNS
      ? data.layout
      : remapLayout(data.layout, activeColumns);
  $: widgets = displayLayout.widgets
    .filter(rendersTile)
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));

  let drag:
    | {
        id: string;
        mode: 'move' | 'resize';
        startClientX: number;
        startClientY: number;
        startX: number;
        startY: number;
        startW: number;
        startH: number;
        cellWidth: number;
        rowHeight: number;
      }
    | undefined;

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
    return `grid-column: span ${columns}; min-height: ${rows * 124}px;`;
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
    mode: 'move' | 'resize',
    event: PointerEvent
  ) {
    if (!fineEdit || !canvasEl) {
      return;
    }
    event.stopPropagation();
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
      rowHeight: metrics.rowHeight
    };
    window.addEventListener('pointermove', handleDragMove);
    window.addEventListener('pointerup', endDrag, { once: true });
    window.addEventListener('pointercancel', endDrag, { once: true });
  }

  function handleDragMove(event: PointerEvent) {
    if (!drag) {
      return;
    }
    const dx = Math.round((event.clientX - drag.startClientX) / drag.cellWidth);
    const dy = Math.round((event.clientY - drag.startClientY) / drag.rowHeight);
    if (drag.mode === 'move') {
      onMoveWidget(drag.id, drag.startX + dx, drag.startY + dy);
    } else {
      onResizeWidget(drag.id, drag.startW + dx, drag.startH + dy);
    }
  }

  function endDrag() {
    drag = undefined;
    if (typeof window !== 'undefined') {
      window.removeEventListener('pointermove', handleDragMove);
    }
  }

  function gridMetrics() {
    const styles = window.getComputedStyle(canvasEl);
    const columns =
      styles.gridTemplateColumns.split(' ').filter(Boolean).length ||
      activeColumns;
    const gap = Number.parseFloat(styles.columnGap || '12') || 12;
    const rect = canvasEl.getBoundingClientRect();
    return {
      cellWidth: Math.max(
        1,
        (rect.width - gap * Math.max(0, columns - 1)) / columns + gap
      ),
      rowHeight: 124
    };
  }

  onDestroy(endDrag);
</script>

<svelte:window on:click={closeMenu} />

<section
  bind:this={canvasEl}
  class:dashboard-grid-edit={editMode}
  class="dashboard-canvas"
  style={`grid-template-columns: repeat(${activeColumns}, minmax(0, 1fr));`}
  aria-label="Widget dashboard"
>
  {#each widgets as widget (widget.id)}
    <div
      class="dashboard-widget-slot"
      style={gridStyle(widget)}
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
        onResizeStart={(event) => startDrag(widget, 'resize', event)}
      >
        <svelte:fragment slot="actions">
          {#if editMode}
            <div class="widget-menu">
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
    background: var(--strong-surface, rgba(255, 255, 255, 0.08));
  }
  .widget-menu-danger {
    color: var(--danger, #ef4444);
  }
</style>
