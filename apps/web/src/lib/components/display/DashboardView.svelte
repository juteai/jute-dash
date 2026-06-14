<script lang="ts">
  import { onMount } from 'svelte';
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
  import {
    ensureLayoutVariants,
    selectLayoutVariant
  } from '$lib/layout-editor';
  import { layoutStore } from '$lib/layoutStore';
  import type {
    Agent,
    AgentAvailability,
    ChatMessage,
    DashboardData,
    UserFacingIssue,
    VoiceStatus,
    WidgetCatalogItem,
    WidgetInstance
  } from '$lib/types';

  export let data: DashboardData;
  export let editMode = false;
  export let messages: ChatMessage[] = [];
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
  export let onEnterEdit: (activeVariantId: string) => void = () => {};
  export let onSaveEdit: () => void = () => {};
  export let onCancelEdit: () => void = () => {};
  export let onResetLayout: () => void = () => {};
  export let onIssueAction: (
    issue: UserFacingIssue,
    widget: WidgetInstance
  ) => void = () => {};

  let showCatalog = false;
  let viewportWidth = 1280;
  let viewportHeight = 800;

  $: headlessWidgets = data.layout.widgets.filter(
    (widget) => widget.visible && widget.mode === 'headless'
  );

  $: saveDisabled = savingLayout || stale;
  $: voiceReady = voice?.serviceStatus === 'ready';
  $: voiceLabel = voiceReady
    ? voice.muted
      ? 'Voice muted'
      : 'Wake listening'
    : 'Voice not configured';
  $: layoutWithVariants = ensureLayoutVariants(data.layout);
  $: activeVariant = selectLayoutVariant(
    layoutWithVariants,
    viewportWidth,
    viewportHeight,
    $layoutStore.activeVariantId
  );
  $: if (
    editMode &&
    activeVariant &&
    $layoutStore.activeVariantId !== activeVariant.id
  ) {
    layoutStore.setActiveVariant(activeVariant.id);
  }

  onMount(() => {
    updateViewport();
    window.addEventListener('resize', updateViewport);
    return () => window.removeEventListener('resize', updateViewport);
  });

  function updateViewport() {
    viewportWidth = window.innerWidth;
    viewportHeight = window.innerHeight;
  }

  function canAddWidget(item: WidgetCatalogItem) {
    return (
      item.allowMultiple ||
      !data.layout.widgets.some(
        (widget) => widget.kind === item.kind && widget.visible
      )
    );
  }

  function addWidget(kind: string, mode: 'ui' | 'headless' = 'ui') {
    const item = $layoutStore.widgetCatalog.find((c) => c.kind === kind);
    if (item) {
      layoutStore.addWidget(item, mode);
    }
    showCatalog = false;
  }
</script>

<section class="dashboard-view" aria-label="Jute dashboard">
  <header class="dashboard-header dashboard-header--minimal">
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
          on:click={() => onEnterEdit(activeVariant.id)}
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
      {#if activeVariant}
        <div class="layout-variant-controls" aria-label="Layout size">
          <div
            class="layout-variant-tabs"
            role="tablist"
            aria-label="Layout variants"
          >
            {#each layoutWithVariants.variants ?? [] as variant (variant.id)}
              <button
                type="button"
                class:layout-variant-tab--active={variant.id ===
                  activeVariant.id}
                role="tab"
                aria-selected={variant.id === activeVariant.id}
                on:click={() => layoutStore.setActiveVariant(variant.id)}
              >
                {variant.label}
              </button>
            {/each}
          </div>
          <label>
            <span>Columns</span>
            <input
              type="number"
              min="1"
              max="24"
              value={activeVariant.columns}
              on:change={(event) =>
                layoutStore.setVariantGridSize(
                  Number((event.currentTarget as HTMLInputElement).value),
                  activeVariant.rows
                )}
            />
          </label>
          <label>
            <span>Rows</span>
            <input
              type="number"
              min="1"
              max="24"
              value={activeVariant.rows}
              on:change={(event) =>
                layoutStore.setVariantGridSize(
                  activeVariant.columns,
                  Number((event.currentTarget as HTMLInputElement).value)
                )}
            />
          </label>
        </div>
      {/if}
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
              <div class="widget-catalog-actions">
                <Button
                  size="sm"
                  disabled={!canAddWidget(item)}
                  on:click={() => addWidget(item.kind)}
                >
                  {canAddWidget(item) ? 'Add' : 'Added'}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!canAddWidget(item)}
                  on:click={() => addWidget(item.kind, 'headless')}
                >
                  Headless
                </Button>
              </div>
            </article>
          {/each}
          {#if widgetCatalog.length === 0}
            <p class="widget-catalog-empty">Widget catalog is unavailable.</p>
          {/if}
        </div>
      </div>
    {/if}

    {#if headlessWidgets.length > 0}
      <div class="headless-tray" aria-label="Headless widgets">
        <span class="headless-tray-label">Headless (context-only)</span>
        <div class="headless-tray-chips">
          {#each headlessWidgets as widget (widget.id)}
            <div class="headless-chip">
              <span class="headless-chip-title">{widget.title}</span>
              <button
                type="button"
                class="headless-chip-action"
                on:click={() => layoutStore.openWidgetConfig(widget.id)}
              >
                Configure
              </button>
              <button
                type="button"
                class="headless-chip-action"
                on:click={() => layoutStore.restoreWidget(widget.id)}
              >
                Show
              </button>
              <button
                type="button"
                class="headless-chip-action headless-chip-danger"
                on:click={() => layoutStore.removeWidget(widget.id)}
                aria-label={`Remove ${widget.title}`}
              >
                <X size={14} />
              </button>
            </div>
          {/each}
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
    activeVariantId={$layoutStore.activeVariantId}
    {onOpenChat}
    {onIssueAction}
  />
</section>

<style>
  .layout-variant-controls {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }
  .layout-variant-tabs {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px;
    border: 1px solid var(--border, rgba(255, 255, 255, 0.16));
    border-radius: 8px;
    background: var(--surface, rgba(255, 255, 255, 0.04));
  }
  .layout-variant-tabs button {
    min-height: 32px;
    border: none;
    border-radius: 6px;
    padding: 0 9px;
    background: transparent;
    color: var(--foreground, inherit);
    font-size: 0.78rem;
    cursor: pointer;
  }
  .layout-variant-tabs button.layout-variant-tab--active {
    background: var(--foreground, #fff);
    color: var(--background, #000);
  }
  .layout-variant-controls label {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 0.78rem;
    opacity: 0.88;
  }
  .layout-variant-controls input {
    width: 56px;
    min-height: 34px;
    border: 1px solid var(--border, rgba(255, 255, 255, 0.16));
    border-radius: 7px;
    background: var(--surface, rgba(255, 255, 255, 0.04));
    color: var(--foreground, inherit);
    padding: 0 8px;
  }
  .widget-catalog-actions {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  .headless-tray {
    margin: 0 0 12px;
    padding: 12px 14px;
    border: 1px dashed var(--border, rgba(255, 255, 255, 0.2));
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .headless-tray-label {
    font-size: 0.72rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    opacity: 0.6;
  }
  .headless-tray-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
  .headless-chip {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    border-radius: 999px;
    background: var(--surface-strong, rgba(255, 255, 255, 0.08));
    border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
  }
  .headless-chip-title {
    font-size: 0.84rem;
    font-weight: 600;
  }
  .headless-chip-action {
    border: none;
    background: transparent;
    color: var(--foreground, inherit);
    font-size: 0.78rem;
    cursor: pointer;
    padding: 4px 6px;
    border-radius: 6px;
    display: inline-flex;
    align-items: center;
  }
  .headless-chip-action:hover {
    background: var(--surface, rgba(255, 255, 255, 0.06));
  }
  .headless-chip-danger {
    color: var(--danger, #ef4444);
  }
  .dashboard-header--minimal {
    justify-content: flex-end;
  }
</style>
