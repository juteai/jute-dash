import { describe, expect, it } from 'vitest';
import {
  SPOTIFY_PREVIOUS_DOUBLE_PRESS_MS,
  clampProgress,
  createPlayerStateProgressClock,
  createServerProgressClock,
  estimateProgress,
  formatTime,
  nextRepeatState,
  normalizeRepeatState,
  progressPercent,
  setProgressClockPlaying,
  setProgressClockPosition
} from '../../../../widgets/spotify/web/playbackViewModel';

describe('spotify playback view model', () => {
  it('normalizes and cycles repeat state', () => {
    expect(normalizeRepeatState('anything')).toBe('off');
    expect(nextRepeatState('off')).toBe('context');
    expect(nextRepeatState('context')).toBe('track');
    expect(nextRepeatState('track')).toBe('off');
  });

  it('formats and clamps timeline state', () => {
    expect(formatTime(125_000)).toBe('2:05');
    expect(clampProgress(-20, 1000)).toBe(0);
    expect(clampProgress(1200, 1000)).toBe(1000);
    expect(progressPercent(500, 1000)).toBe(50);
  });

  it('creates and updates local progress clocks', () => {
    const serverClock = createServerProgressClock({
      progressMS: 10_000,
      durationMS: 180_000,
      observedAtMS: 1_000,
      isPlaying: true
    });
    expect(estimateProgress(serverClock, 3_000)).toBe(12_000);

    const paused = setProgressClockPlaying(serverClock, false, 4_000);
    expect(estimateProgress(paused, 8_000)).toBe(13_000);

    const seeked = setProgressClockPosition(paused, 60_000, 9_000);
    expect(estimateProgress(seeked, 10_000)).toBe(60_000);
  });

  it('creates a clock from Spotify SDK player state', () => {
    const clock = createPlayerStateProgressClock(
      {
        paused: false,
        position: 30_000,
        duration: 200_000,
        track_window: {
          current_track: {
            uri: 'spotify:track:glue',
            duration_ms: 200_000
          }
        }
      },
      1_000
    );

    expect(estimateProgress(clock, 2_500)).toBe(31_500);
  });

  it('keeps the previous button timing decision explicit', () => {
    expect(SPOTIFY_PREVIOUS_DOUBLE_PRESS_MS).toBe(520);
  });
});
