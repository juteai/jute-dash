export type PlaybackProgressClock = {
  baseProgressMS: number;
  durationMS: number;
  observedAtMS: number;
  isPlaying: boolean;
};

type PlaybackProgressClockInput = {
  progressMS: number;
  durationMS: number;
  observedAtMS: number;
  isPlaying: boolean;
};

export function createPlaybackProgressClock(
  input: PlaybackProgressClockInput
): PlaybackProgressClock {
  return {
    baseProgressMS: clampProgress(input.progressMS, input.durationMS),
    durationMS: Math.max(0, input.durationMS),
    observedAtMS: input.observedAtMS,
    isPlaying: input.isPlaying
  };
}

export function estimatePlaybackProgress(
  clock: PlaybackProgressClock,
  nowMS: number
): number {
  const elapsedMS = clock.isPlaying
    ? Math.max(0, nowMS - clock.observedAtMS)
    : 0;
  return clampProgress(clock.baseProgressMS + elapsedMS, clock.durationMS);
}

export function setPlaybackProgressClockPlaying(
  clock: PlaybackProgressClock,
  isPlaying: boolean,
  nowMS: number
): PlaybackProgressClock {
  if (clock.isPlaying === isPlaying) {
    return clock;
  }
  return {
    ...clock,
    baseProgressMS: estimatePlaybackProgress(clock, nowMS),
    observedAtMS: nowMS,
    isPlaying
  };
}

export function setPlaybackProgressClockPosition(
  clock: PlaybackProgressClock,
  progressMS: number,
  nowMS: number
): PlaybackProgressClock {
  return {
    ...clock,
    baseProgressMS: clampProgress(progressMS, clock.durationMS),
    observedAtMS: nowMS
  };
}

function clampProgress(progressMS: number, durationMS: number): number {
  return Math.min(Math.max(0, progressMS), Math.max(0, durationMS));
}
