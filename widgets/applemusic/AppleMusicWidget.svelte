<script lang="ts">
  export let data: any = {};
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

  $: isConfigured = data?.is_configured ?? false;
  $: isPlaying = data?.is_playing ?? false;
  $: trackTitle = data?.track_title ?? 'Not Playing';
  $: artistName = data?.artist_name ?? 'Unknown';

  async function handlePlayPause() {
    await dispatch(isPlaying ? 'pause' : 'play');
  }

  async function handleNext() {
    await dispatch('next');
  }
</script>

<div class="widget-content">
  {#if !isConfigured}
    <div class="unconfigured">
      <p class="title">Apple Music</p>
      <button class="connect-btn" on:click={() => dispatch('connect')}>Connect Apple Music</button>
    </div>
  {:else}
    <div class="player">
      <div class="info">
        <p class="track">{trackTitle}</p>
        <p class="artist">{artistName}</p>
      </div>
      <div class="controls">
        <button on:click={handlePlayPause}>{isPlaying ? '⏸' : '▶'}</button>
        <button on:click={handleNext}>⏭</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .widget-content {
    padding: 12px;
    height: 100%;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  .unconfigured {
    text-align: center;
  }
  .title {
    font-weight: bold;
    margin-bottom: 8px;
  }
  .connect-btn {
    padding: 6px 12px;
    background: var(--foreground);
    color: var(--inverse);
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  .player {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .track {
    font-weight: bold;
    font-size: 0.9rem;
  }
  .artist {
    color: var(--muted);
    font-size: 0.8rem;
  }
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
