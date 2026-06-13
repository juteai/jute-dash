<script lang="ts">
  import {
    MediaControls,
    MediaSummary,
    VolumeControl,
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
  $: volume = data?.volume ?? 50;

  async function runAction(action: string, args?: any) {
    if (busy || stale) {
      return;
    }
    busy = true;
    try {
      await dispatch(action, args);
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

  async function handlePrevious() {
    await runAction("previous");
  }

  async function handleVolume(vol: number) {
    await runAction("set_volume", { volume: vol });
  }
</script>

<WidgetStack {stale} class="media-widget">
  <MediaSummary title={trackTitle} subtitle={artistName} />
  <MediaControls
    {isPlaying}
    disabled={busy || stale}
    onPrevious={handlePrevious}
    onPlayPause={handlePlayPause}
    onNext={handleNext}
  />
  <VolumeControl {volume} disabled={busy || stale} onVolume={handleVolume} />
</WidgetStack>

<style>
  :global(.media-widget) {
    justify-content: center;
    padding: clamp(4px, 2cqmin, 8px);
  }
</style>
