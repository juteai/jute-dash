import {
  createPlaybackProgressClock,
  estimatePlaybackProgress,
  setPlaybackProgressClockPlaying,
  setPlaybackProgressClockPosition,
  type PlaybackProgressClock,
} from "$lib/spotifyTimeline";
import type { AppleMusicPlayerState, AppleMusicRepeatState } from "./types";

export const APPLE_MUSIC_PREVIOUS_DOUBLE_PRESS_MS = 520;

export function normalizeRepeatState(value: unknown): AppleMusicRepeatState {
  return value === "context" || value === "track" ? value : "off";
}

export function nextRepeatState(
  value: AppleMusicRepeatState,
): AppleMusicRepeatState {
  if (value === "off") return "context";
  if (value === "context") return "track";
  return "off";
}

export function repeatLabel(value: AppleMusicRepeatState) {
  if (value === "track") return "Repeat one";
  if (value === "context") return "Repeat list";
  return "Repeat off";
}

export function shuffleLabel(value: boolean) {
  return value ? "Shuffle on" : "Shuffle off";
}

export function formatTime(ms: number) {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000));
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = String(totalSeconds % 60).padStart(2, "0");
  return `${minutes}:${seconds}`;
}

export function nowMS() {
  if (typeof performance !== "undefined") {
    return performance.now();
  }
  return Date.now();
}

export function progressPercent(progressMS: number, durationMS: number) {
  return durationMS > 0 ? Math.round((progressMS / durationMS) * 100) : 0;
}

export function clampProgress(progressMS: number, durationMS: number) {
  return Math.min(Math.max(progressMS, 0), Math.max(durationMS, 0));
}

export function createServerProgressClock(input: {
  progressMS: number;
  durationMS: number;
  isPlaying: boolean;
  observedAtMS: number;
}) {
  return createPlaybackProgressClock(input);
}

export function createPlayerStateProgressClock(
  state: AppleMusicPlayerState,
  observedAtMS: number,
) {
  return createPlaybackProgressClock({
    progressMS: Math.round((state.currentPlaybackTime ?? 0) * 1000),
    durationMS: Math.round((state.currentPlaybackDuration ?? 0) * 1000),
    observedAtMS,
    isPlaying: Boolean(state.isPlaying),
  });
}

export function setProgressClockPlaying(
  clock: PlaybackProgressClock,
  playing: boolean,
  observedAtMS: number,
) {
  return setPlaybackProgressClockPlaying(clock, playing, observedAtMS);
}

export function setProgressClockPosition(
  clock: PlaybackProgressClock,
  positionMS: number,
  observedAtMS: number,
) {
  return setPlaybackProgressClockPosition(clock, positionMS, observedAtMS);
}

export function estimateProgress(
  clock: PlaybackProgressClock,
  observedAtMS: number,
) {
  return estimatePlaybackProgress(clock, observedAtMS);
}

export type { PlaybackProgressClock };
