<script lang="ts">
  import {
    MessageCircle,
    Mic,
    MicOff,
    Pencil,
    RotateCcw,
    Settings,
    X
  } from 'lucide-svelte';
  import DashboardGrid from '$lib/components/display/DashboardGrid.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import IconButton from '$lib/components/ui/IconButton.svelte';
  import type {
    Agent,
    AgentAvailability,
    ChatMessage,
    DashboardData,
    VoiceStatus,
    WidgetCatalogItem
  } from '$lib/types';

  export let data: DashboardData;
  export let editMode = false;
  export let messages: ChatMessage[] = [];
  export let theme: 'light' | 'dark' = 'light';
  export let stale = false;
  export let selectedAgent: Agent | undefined;
  export let selectedAvailability: AgentAvailability = 'unknown';
  export let focusedWidgetId = '';
  export let voice: VoiceStatus;
  export let widgetCatalog: WidgetCatalogItem[] = [];
  export let editIssue = '';
  export let savingLayout = false;
  export let onOpenChat: () => void = () => {};
  export let onManageAgents: () => void = () => {};
  export let onToggleVoiceMute: () => Promise<void> | void = () => {};
  export let onEnterEdit: () => void = () => {};
  export let onSaveEdit: () => void = () => {};
  export let onCancelEdit: () => void = () => {};
  export let onResetLayout: () => void = () => {};
  export let onAddWidget: (kind: string) => void = () => {};
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

  let showCatalog = false;

  $: saveDisabled = savingLayout || stale;
  $: voiceReady = voice?.serviceStatus === 'ready';
  $: voiceLabel = voiceReady
    ? voice.muted
      ? 'Voice muted'
      : 'Wake listening'
    : 'Voice not configured';

  function canAddWidget(item: WidgetCatalogItem) {
    return (
      item.allowMultiple ||
      !data.layout.widgets.some(
        (widget) => widget.kind === item.kind && widget.visible
      )
    );
  }

  function addWidget(kind: string) {
    onAddWidget(kind);
    showCatalog = false;
  }
</script>

<section class="dashboard-view" aria-label="Jute dashboard">
  <header class="dashboard-header">
    <div class="brand-lockup">
      <img
        src={theme === 'dark'
          ? '/brand/logo_light.svg'
          : '/brand/logo_dark.svg'}
        alt="Jute"
        class="brand-logo"
      />
      <div>
        <div class="home-name">{data.config.home.name}</div>
        <div class="layout-name">{data.layout.profileId}</div>
      </div>
    </div>

    <div class="dashboard-actions">
      {#if editMode}
        <Button variant="outline" disabled={saveDisabled} on:click={onSaveEdit}>
          <span>{savingLayout ? 'Saving' : 'Done'}</span>
        </Button>
        <Button variant="ghost" disabled={savingLayout} on:click={onCancelEdit}>
          <X size={17} />
          <span>Cancel</span>
        </Button>
      {:else}
        <IconButton label="Open chat" variant="outline" on:click={onOpenChat}>
          <MessageCircle size={20} />
        </IconButton>
        <IconButton
          label={voiceLabel}
          variant="outline"
          pressed={voiceReady && !voice.muted}
          disabled={!voiceReady}
          on:click={onToggleVoiceMute}
        >
          {#if voiceReady && !voice.muted}
            <Mic size={20} />
          {:else}
            <MicOff size={20} />
          {/if}
        </IconButton>
        <IconButton
          label="Edit dashboard"
          variant="outline"
          on:click={onEnterEdit}
        >
          <Pencil size={20} />
        </IconButton>
        <IconButton
          label="Settings"
          variant="outline"
          on:click={onManageAgents}
        >
          <Settings size={20} />
        </IconButton>
      {/if}
    </div>
  </header>

  {#if editMode}
    <div class="edit-toolbar" role="status">
      <div>
        <strong>Edit dashboard</strong>
        <span>
          {#if editIssue}
            {editIssue}
          {:else if stale}
            Reconnect to the hub before saving layout changes.
          {:else}
            Drag handles, use keyboard buttons, add widgets, then save.
          {/if}
        </span>
      </div>
      <div class="edit-toolbar-actions">
        <Button
          size="sm"
          variant="secondary"
          disabled={savingLayout || stale}
          on:click={() => (showCatalog = !showCatalog)}>Add widget</Button
        >
        <Button
          size="sm"
          variant="outline"
          disabled={savingLayout || stale}
          on:click={onResetLayout}
        >
          <RotateCcw size={16} />
          <span>Reset</span>
        </Button>
      </div>
    </div>

    {#if showCatalog}
      <div class="widget-catalog-sheet" aria-label="Widget catalog">
        <div class="widget-catalog-header">
          <strong>Add widget</strong>
          <IconButton
            label="Close widget catalog"
            variant="ghost"
            on:click={() => (showCatalog = false)}
          >
            <X size={18} />
          </IconButton>
        </div>
        <div class="widget-catalog-grid">
          {#each widgetCatalog as item (item.kind)}
            <article class="widget-catalog-item">
              <div>
                <strong>{item.name}</strong>
                <p>{item.description}</p>
                <span>{item.defaultW}x{item.defaultH} · {item.defaultSize}</span
                >
              </div>
              <Button
                size="sm"
                disabled={!canAddWidget(item)}
                on:click={() => addWidget(item.kind)}
              >
                {canAddWidget(item) ? 'Add' : 'Added'}
              </Button>
            </article>
          {/each}
          {#if widgetCatalog.length === 0}
            <p class="widget-catalog-empty">Widget catalog is unavailable.</p>
          {/if}
        </div>
      </div>
    {/if}
  {/if}

  <DashboardGrid
    {data}
    {editMode}
    {messages}
    {stale}
    {selectedAgent}
    {selectedAvailability}
    {focusedWidgetId}
    {onOpenChat}
    {onMoveWidget}
    {onResizeWidget}
    {onRemoveWidget}
  />
</section>
