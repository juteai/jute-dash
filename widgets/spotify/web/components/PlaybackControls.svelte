<script lang="ts">
  export let isPlaying = false;
  export let volume = 50;
  export let onPrevious: () => Promise<void> | void = () => {};
  export let onPlayPause: () => Promise<void> | void = () => {};
  export let onNext: () => Promise<void> | void = () => {};
  export let onVolume: (volume: number) => Promise<void> | void = () => {};

  function handleVolume(e: Event) {
    const vol = parseInt((e.target as HTMLInputElement).value, 10);
    onVolume(vol);
  }
</script>

<div class="controls">
  <button on:click={onPrevious}>⏮</button>
  <button on:click={onPlayPause}>{isPlaying ? '⏸' : '▶'}</button>
  <button on:click={onNext}>⏭</button>
</div>
<div class="vol-slider">
  <span>🔈</span>
  <input type="range" min="0" max="100" value={volume} on:change={handleVolume} />
  <span>🔊</span>
</div>

<style>
  .controls {
    display: flex;
    gap: 16px;
    justify-content: center;
  }
  .controls button {
    background: none;
    border: none;
    font-size: 1.2rem;
    cursor: pointer;
    color: var(--foreground);
  }
  .vol-slider {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 0.8rem;
  }
  .vol-slider input {
    flex: 1;
  }
</style>
