<script lang="ts">
  import {
    MediaControls,
    MediaSummary,
  } from "$lib/components/integration-controls";
  import WidgetStack from "$lib/components/widget-content/WidgetStack.svelte";

  export let data: any = {};
  export let dispatch: (
    action: string,
    args?: any,
  ) => Promise<any> = async () => {};
  export let stale = false;

  let busy = false;

  $: isPlaying = data?.is_playing ?? false;
  $: trackTitle = data?.track_title ?? "Not Playing";
  $: artistName = data?.artist_name ?? "Unknown";

  async function runAction(action: string) {
    if (busy || stale) {
      return;
    }
    busy = true;
    try {
      await dispatch(action);
    } finally {
      busy = false;
    }
  }

  async function handlePlayPause() {
    await runAction(isPlaying ? "pause" : "play");
  }

  async function handleNext() {
    await runAction("next");
  }
</script>

<WidgetStack {stale} class="media-widget">
  <MediaSummary title={trackTitle} subtitle={artistName} />
  <MediaControls
    {isPlaying}
    showPrevious={false}
    disabled={busy || stale}
    onPlayPause={handlePlayPause}
    onNext={handleNext}
  />
</WidgetStack>

<style>
  :global(.media-widget) {
    justify-content: center;
    padding: clamp(4px, 2cqmin, 8px);
  }
</style>
