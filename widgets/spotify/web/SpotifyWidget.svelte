<script lang="ts">
  import PlaybackControls from './components/PlaybackControls.svelte';
  import TrackSummary from './components/TrackSummary.svelte';

  export let data: any = {};
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

  $: isPlaying = data?.is_playing ?? false;
  $: trackTitle = data?.track_title ?? 'Not Playing';
  $: artistName = data?.artist_name ?? 'Unknown';
  $: volume = data?.volume ?? 50;

  async function handlePlayPause() {
    await dispatch(isPlaying ? 'pause' : 'play');
  }

  async function handleNext() {
    await dispatch('next');
  }

  async function handlePrevious() {
    await dispatch('previous');
  }

  async function handleVolume(vol: number) {
    await dispatch('set_volume', { volume: vol });
  }
</script>

<div class="widget-content">
  <div class="player">
    <TrackSummary title={trackTitle} artist={artistName} />
    <PlaybackControls
      {isPlaying}
      {volume}
      onPrevious={handlePrevious}
      onPlayPause={handlePlayPause}
      onNext={handleNext}
      onVolume={handleVolume}
    />
  </div>
</div>

<style>
  .widget-content {
    padding: 12px;
    height: 100%;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
  .player {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
</style>
