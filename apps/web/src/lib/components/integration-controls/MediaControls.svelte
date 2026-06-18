<script lang="ts">
  import { Pause, Play, SkipBack, SkipForward } from 'lucide-svelte';
  import WidgetActionButton from '$lib/components/widget-content/WidgetActionButton.svelte';

  export let isPlaying = false;
  export let disabled = false;
  export let showPrevious = true;
  export let onPrevious: () => Promise<void> | void = () => {};
  export let onPlayPause: () => Promise<void> | void = () => {};
  export let onNext: () => Promise<void> | void = () => {};
</script>

<div class="media-controls">
  {#if showPrevious}
    <WidgetActionButton label="Previous track" {disabled} on:click={onPrevious}>
      <SkipBack size={17} />
    </WidgetActionButton>
  {/if}
  <WidgetActionButton
    label={isPlaying ? 'Pause playback' : 'Start playback'}
    pressed={isPlaying}
    {disabled}
    on:click={onPlayPause}
  >
    {#if isPlaying}
      <Pause size={18} />
    {:else}
      <Play size={18} />
    {/if}
  </WidgetActionButton>
  <WidgetActionButton label="Next track" {disabled} on:click={onNext}>
    <SkipForward size={17} />
  </WidgetActionButton>
</div>

<style>
  .media-controls {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: clamp(8px, 3cqmin, 14px);
  }
</style>
