<script lang="ts">
  export let isPlaying = false;
  export let onPlayPause: () => Promise<void> | void = () => {};
  export let onNext: () => Promise<void> | void = () => {};
</script>

<div class="controls">
  <button on:click={onPlayPause}>{isPlaying ? '⏸' : '▶'}</button>
  <button on:click={onNext}>⏭</button>
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
</style>
