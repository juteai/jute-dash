<script lang="ts">
  import { formatTime } from "../playbackViewModel";

  export let progressMS = 0;
  export let durationMS = 0;
  export let progressPercent = 0;
  export let busy = false;
  export let stale = false;
  export let onPreview: (positionMS: number) => void = () => {};
  export let onSeek: (positionMS: number) => void = () => {};
</script>

<div class="timeline-row">
  <span>{formatTime(progressMS)}</span>
  <input
    type="range"
    min="0"
    max={Math.max(durationMS, 1)}
    step="1000"
    value={progressMS}
    disabled={busy || stale || durationMS <= 0}
    aria-label="Spotify track timeline"
    aria-valuetext={`${formatTime(progressMS)} of ${formatTime(durationMS)}`}
    style={`--timeline-progress: ${progressPercent}%`}
    on:input={(event) => onPreview(Number(event.currentTarget.value))}
    on:change={(event) => onSeek(Number(event.currentTarget.value))}
  />
  <span>{formatTime(durationMS)}</span>
</div>

<style>
  .timeline-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    padding: 0 2px;
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 720;
  }

  .timeline-row input {
    width: 100%;
    accent-color: var(--active);
  }

  .timeline-row input:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  @container (max-width: 260px) {
    .timeline-row span {
      display: none;
    }
  }

  @container (max-height: 190px) {
    .timeline-row span {
      display: none;
    }

    .timeline-row {
      padding-bottom: 8px;
    }
  }
</style>
