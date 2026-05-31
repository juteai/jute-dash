<script lang="ts">
  import { onDestroy } from 'svelte';
  import { ArrowDown, ArrowLeft, ArrowRight, ArrowUp, Minus, Plus, Trash2 } from 'lucide-svelte';
  import WidgetFrame from '$lib/components/display/WidgetFrame.svelte';
  import ChatHistoryWidget from '$widgets/chathistory/ChatHistoryWidget.svelte';
  import DateTimeWidget from '$widgets/datetime/DateTimeWidget.svelte';
  import WeatherWidget from '$widgets/weather/WeatherWidget.svelte';
  import RSSWidget from '$widgets/rss/RSSWidget.svelte';
  import MarketsWidget from '$widgets/markets/MarketsWidget.svelte';
  import { resolveWidgetChrome } from '$lib/themes';
  import type { Agent, AgentAvailability, ChatMessage, DashboardData, WidgetInstance } from '$lib/types';

  export let data: DashboardData;
  export let editMode = false;
  export let messages: ChatMessage[] = [];
  export let stale = false;
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let focusedWidgetId = '';
  export let onOpenChat: () => void = () => {};
  export let onMoveWidget: (widgetId: string, x: number, y: number) => void = () => {};
  export let onResizeWidget: (widgetId: string, w: number, h: number) => void = () => {};
  export let onRemoveWidget: (widgetId: string) => void = () => {};

  let canvasEl: HTMLElement;
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

  $: widgets = [...data.layout.widgets]
    .filter((widget) => widget.visible)
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));

  function gridStyle(widget: WidgetInstance) {
    const columns = Math.min(Math.max(widget.w, widget.minW, 1), 4);
    const rows = Math.max(widget.h, widget.minH, 1);
    return `grid-column: span ${columns}; min-height: ${rows * 132}px;`;
  }

  function startDrag(widget: WidgetInstance, mode: 'move' | 'resize', event: PointerEvent) {
    if (!editMode || !canvasEl) {
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
    const columns = styles.gridTemplateColumns.split(' ').filter(Boolean).length || 4;
    const gap = Number.parseFloat(styles.columnGap || '12') || 12;
    const rect = canvasEl.getBoundingClientRect();
    return {
      cellWidth: Math.max(1, (rect.width - gap * Math.max(0, columns - 1)) / columns + gap),
      rowHeight: 144
    };
  }

  onDestroy(endDrag);
</script>

<section bind:this={canvasEl} class:dashboard-grid-edit={editMode} class="dashboard-canvas" aria-label="Widget dashboard">
  {#each widgets as widget}
    <div class="dashboard-widget-slot" style={gridStyle(widget)} data-widget-id={widget.id}>
      <WidgetFrame
        {widget}
        {editMode}
        focused={focusedWidgetId === widget.id}
        chrome={resolveWidgetChrome(widget, data.config.display)}
        overflow={widget.kind === 'chat-history' ? 'scroll' : 'clip'}
        onMoveStart={(event) => startDrag(widget, 'move', event)}
        onResizeStart={(event) => startDrag(widget, 'resize', event)}
      >
        <svelte:fragment slot="actions">
          {#if editMode}
            <button type="button" class="widget-frame-handle" aria-label={`Move ${widget.title} left`} on:click={() => onMoveWidget(widget.id, widget.x - 1, widget.y)}>
              <ArrowLeft size={15} />
            </button>
            <button type="button" class="widget-frame-handle" aria-label={`Move ${widget.title} right`} on:click={() => onMoveWidget(widget.id, widget.x + 1, widget.y)}>
              <ArrowRight size={15} />
            </button>
            <button type="button" class="widget-frame-handle" aria-label={`Move ${widget.title} up`} on:click={() => onMoveWidget(widget.id, widget.x, widget.y - 1)}>
              <ArrowUp size={15} />
            </button>
            <button type="button" class="widget-frame-handle" aria-label={`Move ${widget.title} down`} on:click={() => onMoveWidget(widget.id, widget.x, widget.y + 1)}>
              <ArrowDown size={15} />
            </button>
            <button type="button" class="widget-frame-handle" aria-label={`Make ${widget.title} smaller`} on:click={() => onResizeWidget(widget.id, widget.w - 1, widget.h)}>
              <Minus size={15} />
            </button>
            <button type="button" class="widget-frame-handle" aria-label={`Make ${widget.title} wider`} on:click={() => onResizeWidget(widget.id, widget.w + 1, widget.h)}>
              <Plus size={15} />
            </button>
            <button type="button" class="widget-frame-handle widget-frame-handle--danger" aria-label={`Remove ${widget.title}`} on:click={() => onRemoveWidget(widget.id)}>
              <Trash2 size={15} />
            </button>
          {/if}
        </svelte:fragment>
        {#if widget.kind === 'date-time'}
          <DateTimeWidget home={data.config.home} {stale} />
        {:else if widget.kind === 'weather'}
          <WeatherWidget weather={widget.data || data.home.weather} {stale} />
        {:else if widget.kind === 'rss'}
          <RSSWidget data={widget.data} {stale} />
        {:else if widget.kind === 'markets'}
          <MarketsWidget data={widget.data} {stale} />
        {:else if widget.kind === 'chat-history'}
          <ChatHistoryWidget
            agents={data.agents}
            {messages}
            {selectedAgent}
            {selectedAvailability}
            {onOpenChat}
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
