import type { AppleMusicPlayerState } from "./types";

type MusicKitInstance = {
  authorize: () => Promise<string>;
  play: () => Promise<void>;
  pause: () => Promise<void>;
  stop?: () => Promise<void>;
  skipToNextItem?: () => Promise<void>;
  skipToPreviousItem?: () => Promise<void>;
  seekToTime?: (time: number) => Promise<void>;
  setQueue?: (descriptor: Record<string, unknown>) => Promise<void>;
  addEventListener?: (event: string, callback: (event: any) => void) => void;
  removeEventListener?: (event: string, callback: (event: any) => void) => void;
  isAuthorized?: boolean;
  musicUserToken?: string;
  nowPlayingItem?: unknown;
  currentPlaybackTime?: number;
  currentPlaybackDuration?: number;
  playbackState?: number | string;
  volume?: number;
  shuffleMode?: number | string;
  repeatMode?: number | string;
};

type MusicKitNamespace = {
  configure: (configuration: {
    developerToken: string;
    app: { name: string; build: string };
  }) => MusicKitInstance;
  getInstance: () => MusicKitInstance;
  Events?: Record<string, string>;
};

type MusicKitWindow = Window &
  typeof globalThis & {
    MusicKit?: MusicKitNamespace;
  };

export type AppleMusicPlayerSession = {
  authorize: () => Promise<string>;
  play: () => Promise<void>;
  pause: () => Promise<void>;
  next: () => Promise<void>;
  previous: () => Promise<void>;
  seek: (positionMS: number) => Promise<void>;
  setVolume: (volume: number) => void;
  setShuffle: (state: boolean) => void;
  setRepeat: (state: "off" | "context" | "track") => void;
  playItem: (descriptor: Record<string, unknown>) => Promise<void>;
  snapshot: () => AppleMusicPlayerState;
  disconnect: () => void;
};

export type AppleMusicPlayerSessionOptions = {
  developerToken: string;
  userToken?: string;
  onAuthorized: (userToken: string) => void;
  onStateChanged: (state: AppleMusicPlayerState) => void;
  onIssue: (message: string) => void;
};

export async function createAppleMusicPlayerSession(
  options: AppleMusicPlayerSessionOptions,
): Promise<AppleMusicPlayerSession> {
  await loadMusicKitSDK();
  const MusicKit = (window as MusicKitWindow).MusicKit;
  if (!MusicKit?.configure) {
    throw new Error("Apple Music player is unavailable.");
  }
  const music = MusicKit.configure({
    developerToken: options.developerToken,
    app: { name: "Jute Dash", build: "local" },
  });
  if (options.userToken) {
    music.musicUserToken = options.userToken;
  }

  const emitState = () => options.onStateChanged(readState(music));
  const events = MusicKit.Events ?? {};
  const eventNames = [
    events.playbackStateDidChange,
    events.nowPlayingItemDidChange,
    events.playbackTimeDidChange,
    events.mediaItemStateDidChange,
  ].filter(Boolean);
  for (const eventName of eventNames) {
    music.addEventListener?.(eventName, emitState);
  }

  return {
    authorize: async () => {
      const token = await music.authorize();
      if (token) {
        options.onAuthorized(token);
      }
      return token;
    },
    play: async () => {
      await ensureAuthorized(music, options);
      await music.play();
      emitState();
    },
    pause: async () => {
      await music.pause();
      emitState();
    },
    next: async () => {
      await music.skipToNextItem?.();
      emitState();
    },
    previous: async () => {
      await music.skipToPreviousItem?.();
      emitState();
    },
    seek: async (positionMS: number) => {
      await music.seekToTime?.(Math.max(0, positionMS / 1000));
      emitState();
    },
    setVolume: (volume: number) => {
      music.volume = Math.max(0, Math.min(1, volume / 100));
    },
    setShuffle: (state: boolean) => {
      music.shuffleMode = state ? 1 : 0;
      emitState();
    },
    setRepeat: (state: "off" | "context" | "track") => {
      music.repeatMode = state === "track" ? 2 : state === "context" ? 1 : 0;
      emitState();
    },
    playItem: async (descriptor: Record<string, unknown>) => {
      await ensureAuthorized(music, options);
      await music.setQueue?.(descriptor);
      await music.play();
      emitState();
    },
    snapshot: () => readState(music),
    disconnect: () => {
      for (const eventName of eventNames) {
        music.removeEventListener?.(eventName, emitState);
      }
    },
  };
}

async function ensureAuthorized(
  music: MusicKitInstance,
  options: AppleMusicPlayerSessionOptions,
) {
  if (music.isAuthorized || music.musicUserToken) {
    return;
  }
  const token = await music.authorize();
  if (token) {
    options.onAuthorized(token);
  }
}

function readState(music: MusicKitInstance): AppleMusicPlayerState {
  const nowPlaying = music.nowPlayingItem as
    | AppleMusicPlayerState["nowPlayingItem"]
    | undefined;
  const playbackState = music.playbackState;
  return {
    isPlaying: playbackState === 2 || playbackState === "playing",
    currentPlaybackTime: music.currentPlaybackTime ?? 0,
    currentPlaybackDuration: music.currentPlaybackDuration ?? 0,
    nowPlayingItem: nowPlaying,
  };
}

function loadMusicKitSDK(): Promise<void> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("Apple Music playback needs a browser."));
  }
  const musicWindow = window as MusicKitWindow;
  if (musicWindow.MusicKit?.configure) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const existing = document.querySelector("script[data-jute-apple-music-sdk]");
    if (existing) {
      existing.addEventListener("load", () => resolve(), { once: true });
      existing.addEventListener(
        "error",
        () => reject(new Error("Apple Music player could not be loaded.")),
        { once: true },
      );
      return;
    }
    const script = document.createElement("script");
    script.src = "https://js-cdn.music.apple.com/musickit/v1/musickit.js";
    script.async = true;
    script.dataset.juteAppleMusicSdk = "true";
    script.onload = () => resolve();
    script.onerror = () =>
      reject(new Error("Apple Music player could not be loaded."));
    document.head.appendChild(script);
  });
}
