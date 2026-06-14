import {
  createPlaybackProgressClock,
  estimatePlaybackProgress,
  setPlaybackProgressClockPlaying,
  setPlaybackProgressClockPosition,
  type PlaybackProgressClock,
} from "$lib/spotifyTimeline";
import type { SpotifyPlayerState, SpotifyRepeatState } from "./types";

export const SPOTIFY_PREVIOUS_DOUBLE_PRESS_MS = 520;

export function normalizeRepeatState(value: unknown): SpotifyRepeatState {
  return value === "context" || value === "track" ? value : "off";
}

export function nextRepeatState(
  value: SpotifyRepeatState,
): SpotifyRepeatState {
  if (value === "off") return "context";
  if (value === "context") return "track";
  return "off";
}

export function repeatLabel(value: SpotifyRepeatState) {
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
  state: SpotifyPlayerState,
  observedAtMS: number,
) {
  return createPlaybackProgressClock({
    progressMS: state.position ?? 0,
    durationMS:
      state.duration ?? state.track_window?.current_track?.duration_ms ?? 0,
    observedAtMS,
    isPlaying: !state.paused,
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
