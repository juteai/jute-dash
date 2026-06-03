<script lang="ts">
  import { RefreshCw, Wifi, WifiOff } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import type { AppConnectionState, UserFacingIssue } from '$lib/types';

  export let state: AppConnectionState = 'connected';
  export let stale = false;
  export let issue: UserFacingIssue | undefined;
  export let retrying = false;
  export let onRetry: () => Promise<void> | void = () => {};

  $: visible = state !== 'connected' || stale || Boolean(issue);
  $: title = issue?.title ?? defaultTitle(state, stale);
  $: message = issue?.message ?? defaultMessage(state, stale);
  $: tone = issue?.severity ?? (state === 'offline' ? 'error' : 'warning');

  function defaultTitle(connectionState: AppConnectionState, isStale: boolean) {
    if (connectionState === 'reconnecting') {
      return 'Reconnecting to hub';
    }
    if (connectionState === 'offline') {
      return 'Hub not reachable';
    }
    if (connectionState === 'degraded') {
      return 'Jute is degraded';
    }
    if (isStale) {
      return 'Showing stale dashboard';
    }
    return 'Connected';
  }

  function defaultMessage(
    connectionState: AppConnectionState,
    isStale: boolean
  ) {
    if (connectionState === 'reconnecting') {
      return 'Showing the last dashboard state while Jute reconnects.';
    }
    if (connectionState === 'offline') {
      return 'Check that the local hub is running, then retry.';
    }
    if (connectionState === 'degraded') {
      return 'One or more local services need attention.';
    }
    if (isStale) {
      return 'Some hub-backed data may be out of date.';
    }
    return 'Jute is connected.';
  }
</script>

{#if visible}
  <div
    class={`status-ribbon status-ribbon--${tone}`}
    role="status"
    aria-live="polite"
  >
    <div class="status-ribbon-icon" aria-hidden="true">
      {#if state === 'offline' || state === 'reconnecting'}
        <WifiOff size={18} />
      {:else}
        <Wifi size={18} />
      {/if}
    </div>
    <div class="status-ribbon-copy">
      <strong>{title}</strong>
      <span>{message}</span>
    </div>
    <Button size="sm" variant="outline" on:click={onRetry} disabled={retrying}>
      <RefreshCw size={15} />
      <span>{retrying ? 'Retrying' : 'Retry'}</span>
    </Button>
  </div>
{/if}
