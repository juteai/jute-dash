<script lang="ts">
  import { RefreshCw } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import type { UserFacingIssue } from '$lib/types';

  export let theme: 'light' | 'dark' = 'light';
  export let hubUrl = '';
  export let issue: UserFacingIssue | undefined;
  export let retrying = false;
  export let onRetry: () => Promise<void> | void = () => {};

  $: title = issue?.title ?? 'Hub not reachable';
  $: message =
    issue?.message ?? `Jute Dash cannot connect to the local hub at ${hubUrl}.`;
</script>

<section class="offline-state" aria-live="polite">
  <img
    src={theme === 'dark' ? '/brand/logo_light.svg' : '/brand/logo_dark.svg'}
    alt="Jute"
    class="offline-logo"
  />
  <div class="offline-copy">
    <h1>{title}</h1>
    <p>{message}</p>
    <p class="offline-hub">Hub: {hubUrl}</p>
  </div>
  <Button size="lg" on:click={onRetry} disabled={retrying}>
    <RefreshCw size={18} />
    <span>{retrying ? 'Retrying' : 'Retry'}</span>
  </Button>
</section>
