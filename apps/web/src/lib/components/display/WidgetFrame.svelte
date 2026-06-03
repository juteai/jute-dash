<script lang="ts">
  import {
    Grip,
    Maximize2,
    Loader2,
    AlertCircle,
    EyeOff,
    ShieldAlert,
    Inbox
  } from 'lucide-svelte';
  import type { WidgetInstance } from '$lib/types';
  import { cn } from '$lib/utils';

  export let widget: WidgetInstance;
  export let editMode = false;
  export let focused = false;
  export let chrome = 'solid';
  export let overflow: 'clip' | 'scroll' | 'expand' = 'clip';
  export let state:
    | 'ok'
    | 'loading'
    | 'empty'
    | 'unavailable'
    | 'error'
    | 'permission_required'
    | 'stale' = 'ok';
  export let onMoveStart: (event: PointerEvent) => void = () => {};
  export let onResizeStart: (event: PointerEvent) => void = () => {};
  let className = '';
  export { className as class };

  $: stateDetails =
    {
      loading: { title: 'Loading', message: 'Checking for updates...' },
      empty: { title: 'No Data', message: 'Nothing to display yet.' },
      unavailable: {
        title: 'Unavailable',
        message: 'Dependency is offline or disabled.'
      },
      error: {
        title: 'Widget Error',
        message: 'Failed to load required data.'
      },
      permission_required: {
        title: 'Access Blocked',
        message: 'Grant permission to display.'
      }
    }[
      state as
        | 'loading'
        | 'empty'
        | 'unavailable'
        | 'error'
        | 'permission_required'
    ] || null;
</script>

<section
  class={cn(
    'widget-frame',
    `widget-frame--${widget.size}`,
    `widget-frame--chrome-${chrome}`,
    `widget-frame--overflow-${overflow}`,
    className
  )}
  class:widget-frame--focused={focused}
  class:widget-frame--stale={state === 'stale'}
  aria-label={widget.title}
>
  <header class="widget-frame-header">
    <div class="widget-frame-title">
      {widget.title}
      {#if state === 'stale'}
        <span class="widget-stale-badge">· Stale</span>
      {/if}
    </div>
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

  <div class="widget-frame-body" class:widget-body-stale={state === 'stale'}>
    {#if stateDetails}
      <div class="widget-state-overlay">
        {#if state === 'loading'}
          <Loader2 class="widget-state-icon animate-spin" size={24} />
        {:else if state === 'error'}
          <AlertCircle class="widget-state-icon text-danger" size={24} />
        {:else if state === 'unavailable'}
          <EyeOff class="widget-state-icon text-muted" size={24} />
        {:else if state === 'permission_required'}
          <ShieldAlert class="widget-state-icon text-warning" size={24} />
        {:else}
          <Inbox class="widget-state-icon text-muted" size={24} />
        {/if}
        <div class="widget-state-title">{stateDetails.title}</div>
        <div class="widget-state-message">{stateDetails.message}</div>
      </div>
    {:else}
      <slot />
    {/if}
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

<style>
  .widget-frame--stale {
    border-style: dashed;
  }
  .widget-stale-badge {
    color: var(--warning);
    font-size: 0.75rem;
    font-weight: 700;
    margin-left: 4px;
    text-transform: none;
  }
  .widget-body-stale {
    opacity: 0.64;
  }
  .widget-state-overlay {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    min-height: 90px;
    gap: 8px;
    padding: 12px;
    text-align: center;
  }
  .widget-state-icon {
    margin-bottom: 2px;
  }
  .widget-state-title {
    font-weight: 750;
    font-size: 0.88rem;
    color: var(--foreground);
  }
  .widget-state-message {
    font-size: 0.78rem;
    color: var(--muted);
    line-height: 1.35;
    max-width: 200px;
  }
  :global(.animate-spin) {
    animation: spin 1.2s linear infinite;
  }
  @keyframes spin {
    from {
      transform: rotate(0deg);
    }
    to {
      transform: rotate(360deg);
    }
  }
</style>
