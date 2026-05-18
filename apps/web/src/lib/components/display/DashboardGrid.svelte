<script lang="ts">
  import WidgetFrame from '$lib/components/display/WidgetFrame.svelte';
  import ChatHistoryWidget from '$lib/components/widgets/ChatHistoryWidget.svelte';
  import DateTimeWidget from '$lib/components/widgets/DateTimeWidget.svelte';
  import WeatherWidget from '$lib/components/widgets/WeatherWidget.svelte';
  import type { Agent, AgentAvailability, ChatMessage, DashboardData, WidgetInstance } from '$lib/types';

  export let data: DashboardData;
  export let editMode = false;
  export let messages: ChatMessage[] = [];
  export let stale = false;
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let onOpenChat: () => void = () => {};

  $: widgets = [...data.layout.widgets]
    .filter((widget) => widget.visible)
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));

  function gridStyle(widget: WidgetInstance) {
    const columns = Math.min(Math.max(widget.w, widget.minW, 1), 4);
    const rows = Math.max(widget.h, widget.minH, 1);
    return `grid-column: span ${columns}; min-height: ${rows * 132}px;`;
  }
</script>

<section class:dashboard-grid-edit={editMode} class="dashboard-canvas" aria-label="Widget dashboard">
  {#each widgets as widget}
    <div class="dashboard-widget-slot" style={gridStyle(widget)}>
      <WidgetFrame {widget} {editMode} overflow={widget.kind === 'chat-history' ? 'scroll' : 'clip'}>
        {#if widget.kind === 'date-time'}
          <DateTimeWidget home={data.config.home} {stale} />
        {:else if widget.kind === 'weather'}
          <WeatherWidget weather={data.home.weather} {stale} />
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
