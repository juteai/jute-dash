<script lang="ts">
  import PlaybackControls from './components/PlaybackControls.svelte';
  import TrackSummary from './components/TrackSummary.svelte';

  export let data: any = {};
  export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

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
  <div class="player">
    <TrackSummary title={trackTitle} artist={artistName} />
    <PlaybackControls {isPlaying} onPlayPause={handlePlayPause} onNext={handleNext} />
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
