<script lang="ts">
  import { Pause, Play, Repeat, Repeat1, Shuffle, SkipBack, SkipForward } from "lucide-svelte";
  import { repeatLabel, shuffleLabel } from "../playbackViewModel";
  import type { SpotifyRepeatState } from "../types";

  export let busy = false;
  export let stale = false;
  export let shuffle = false;
  export let isPlaying = false;
  export let repeatState: SpotifyRepeatState = "off";
  export let onShuffle: () => void = () => {};
  export let onPrevious: () => void = () => {};
  export let onPlayPause: () => void = () => {};
  export let onNext: () => void = () => {};
  export let onRepeat: () => void = () => {};
</script>

<div class="touch-controls" aria-label="Spotify playback controls">
  <button
    type="button"
    class="control-button control-button--mode"
    class:control-button--mode-active={shuffle}
    disabled={busy || stale}
    aria-label={shuffleLabel(shuffle)}
    aria-pressed={shuffle}
    title={shuffleLabel(shuffle)}
    on:click={onShuffle}
  >
    <Shuffle size={20} />
  </button>
  <button
    type="button"
    class="control-button"
    disabled={busy || stale}
    aria-label="Restart track. Press twice for previous track."
    on:click={onPrevious}
  >
    <SkipBack size={22} />
  </button>
  <button
    type="button"
    class="control-button control-button--primary"
    class:control-button--active={isPlaying}
    disabled={busy || stale}
    aria-label={isPlaying ? "Pause playback" : "Start playback"}
    aria-pressed={isPlaying}
    on:click={onPlayPause}
  >
    {#if isPlaying}
      <Pause size={25} />
    {:else}
      <Play size={25} />
    {/if}
  </button>
  <button
    type="button"
    class="control-button"
    disabled={busy || stale}
    aria-label="Next track"
    on:click={onNext}
  >
    <SkipForward size={22} />
  </button>
  <button
    type="button"
    class="control-button control-button--mode"
    class:control-button--mode-active={repeatState !== "off"}
    disabled={busy || stale}
    aria-label={repeatLabel(repeatState)}
    aria-pressed={repeatState !== "off"}
    title={repeatLabel(repeatState)}
    on:click={onRepeat}
  >
    {#if repeatState === "track"}
      <Repeat1 size={20} />
    {:else}
      <Repeat size={20} />
    {/if}
  </button>
</div>

<style>
  .touch-controls {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: clamp(8px, 4cqmin, 18px);
    padding: clamp(6px, 2cqmin, 12px) 0;
  }

  .control-button {
    display: inline-grid;
    width: clamp(46px, 14cqmin, 62px);
    height: clamp(46px, 14cqmin, 62px);
    place-items: center;
    border: 0;
    border-radius: 50%;
    background: transparent;
    color: color-mix(in srgb, var(--foreground) 82%, var(--muted));
    cursor: pointer;
    transition:
      background-color 0.18s ease,
      color 0.18s ease,
      transform 0.18s ease;
  }

  .control-button--primary {
    background: color-mix(in srgb, var(--foreground) 90%, transparent);
    color: var(--background);
  }

  .control-button--primary.control-button--active {
    background: color-mix(in srgb, var(--active) 72%, var(--foreground));
  }

  .control-button--mode {
    width: clamp(38px, 11cqmin, 50px);
    height: clamp(38px, 11cqmin, 50px);
    color: var(--muted);
  }

  .control-button--mode-active {
    background: color-mix(in srgb, var(--active) 16%, transparent);
    color: var(--foreground);
  }

  .control-button:hover:not(:disabled) {
    background: color-mix(in srgb, var(--foreground) 14%, transparent);
    color: var(--foreground);
  }

  .control-button--primary:hover:not(:disabled) {
    background: var(--foreground);
    color: var(--background);
  }

  .control-button:active:not(:disabled) {
    transform: scale(0.94);
  }

  .control-button:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .control-button:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  @container (max-height: 190px) {
    .touch-controls {
      padding: 5px 8px 8px;
    }

    .control-button {
      width: clamp(40px, 13cqmin, 54px);
      height: clamp(40px, 13cqmin, 54px);
    }
  }

  @container (max-height: 360px) {
    .touch-controls {
      padding-block: clamp(4px, 1.4cqmin, 8px);
    }
  }
</style>
