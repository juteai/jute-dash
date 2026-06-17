<script lang="ts">
  import {
    Loader2,
    AlertCircle,
    EyeOff,
    ShieldAlert,
    Inbox
  } from 'lucide-svelte';
  import type { UserFacingIssue, WidgetInstance } from '$lib/types';
  import { cn } from '$lib/utils';

  export let widget: WidgetInstance;
  export let editMode = false;
  export let resizable = editMode;
  export let focused = false;
  export let chrome = 'solid';
  export let overflow: 'clip' | 'scroll' | 'expand' = 'clip';
  export let issue: UserFacingIssue | undefined;
  export let state:
    | 'ok'
    | 'loading'
    | 'empty'
    | 'unavailable'
    | 'error'
    | 'permission_required'
    | 'stale' = 'ok';
  export let onMoveStart: (event: PointerEvent) => void = () => {};
  export let onResizeStart: (
    event: PointerEvent,
    resizeMode: 'both' | 'w' | 'h'
  ) => void = () => {};
  export let onIssueAction: (issue: UserFacingIssue) => void = () => {};
  let className = '';
  export { className as class };

  $: stateDetails =
    issue ??
    ({
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
    ] ||
      null);
</script>

<section
  class={cn(
    'widget-frame',
    `widget-frame--${widget.size}`,
    `widget-frame--chrome-${chrome}`,
    `widget-frame--overflow-${overflow}`,
    editMode && 'widget-frame--edit-mode',
    className
  )}
  class:widget-frame--focused={focused}
  class:widget-frame--stale={state === 'stale'}
  aria-label={widget.title}
  on:pointerdown={editMode ? onMoveStart : undefined}
>
  {#if editMode}
    <header class="widget-frame-header">
      <div class="widget-frame-title">
        {widget.title}
        {#if state === 'stale'}
          <span class="widget-stale-badge">· Stale</span>
        {/if}
      </div>
      <div
        class="widget-frame-actions"
        role="none"
        on:pointerdown|stopPropagation
      >
        <slot name="actions" />
      </div>
    </header>
  {/if}

  <div
    class="widget-frame-body"
    class:widget-body-stale={state === 'stale'}
    style={editMode ? 'pointer-events: none; user-select: none;' : ''}
  >
    {#if stateDetails}
      <div class="widget-state-overlay">
        {#if state === 'loading'}
          <Loader2
            class="widget-state-icon animate-spin text-active"
            size={24}
          />
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
        {#if issue?.action}
          <button
            type="button"
            class="widget-state-action"
            on:click|stopPropagation={() => onIssueAction(issue)}
          >
            {issue.action.label}
          </button>
        {/if}
      </div>
    {:else}
      <slot />
    {/if}
  </div>

  {#if resizable}
    <button
      type="button"
      class="widget-resize-dot widget-resize-dot--right"
      aria-label={`Resize width of ${widget.title}`}
      on:pointerdown|preventDefault|stopPropagation={(e) =>
        onResizeStart(e, 'w')}
    ></button>
    <button
      type="button"
      class="widget-resize-dot widget-resize-dot--bottom"
      aria-label={`Resize height of ${widget.title}`}
      on:pointerdown|preventDefault|stopPropagation={(e) =>
        onResizeStart(e, 'h')}
    ></button>
    <button
      type="button"
      class="widget-resize-dot widget-resize-dot--bottom-right"
      aria-label={`Resize both of ${widget.title}`}
      on:pointerdown|preventDefault|stopPropagation={(e) =>
        onResizeStart(e, 'both')}
    ></button>
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
    gap: 8px;
    padding: 8px;
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
  .widget-state-action {
    min-height: 30px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    color: var(--foreground);
    padding: 0 10px;
    cursor: pointer;
    font: inherit;
    font-size: 0.74rem;
    font-weight: 760;
  }
  .widget-state-action:hover {
    border-color: var(--foreground);
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
