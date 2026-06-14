import { describe, expect, it } from 'vitest';
import {
  createPlaybackProgressClock,
  estimatePlaybackProgress,
  setPlaybackProgressClockPlaying,
  setPlaybackProgressClockPosition
} from './spotifyTimeline';

describe('spotify timeline clock', () => {
  it('advances while playback is active', () => {
    const clock = createPlaybackProgressClock({
      progressMS: 10_000,
      durationMS: 180_000,
      observedAtMS: 1_000,
      isPlaying: true
    });

    expect(estimatePlaybackProgress(clock, 4_250)).toBe(13_250);
  });

  it('stays fixed while playback is paused', () => {
    const clock = createPlaybackProgressClock({
      progressMS: 10_000,
      durationMS: 180_000,
      observedAtMS: 1_000,
      isPlaying: false
    });

    expect(estimatePlaybackProgress(clock, 9_000)).toBe(10_000);
  });

  it('materialises progress when playback pauses', () => {
    const playingClock = createPlaybackProgressClock({
      progressMS: 10_000,
      durationMS: 180_000,
      observedAtMS: 1_000,
      isPlaying: true
    });

    const pausedClock = setPlaybackProgressClockPlaying(
      playingClock,
      false,
      3_500
    );

    expect(estimatePlaybackProgress(pausedClock, 9_000)).toBe(12_500);
  });

  it('resets the local position after a seek', () => {
    const clock = createPlaybackProgressClock({
      progressMS: 10_000,
      durationMS: 180_000,
      observedAtMS: 1_000,
      isPlaying: true
    });

    const seekedClock = setPlaybackProgressClockPosition(clock, 60_000, 2_000);

    expect(estimatePlaybackProgress(seekedClock, 4_000)).toBe(62_000);
  });

  it('clamps progress to the track duration', () => {
    const clock = createPlaybackProgressClock({
      progressMS: 178_000,
      durationMS: 180_000,
      observedAtMS: 1_000,
      isPlaying: true
    });

    expect(estimatePlaybackProgress(clock, 10_000)).toBe(180_000);
  });
});
