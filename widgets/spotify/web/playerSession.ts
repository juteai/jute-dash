import type { SpotifyPlayerState } from "./types";

type SpotifyPlayer = {
  addListener: (event: string, callback: (payload: any) => void) => boolean;
  activateElement?: () => Promise<void>;
  connect: () => Promise<boolean>;
  disconnect: () => void;
};

type SpotifyNamespace = {
  Player: new (options: {
    name: string;
    getOAuthToken: (callback: (token: string) => void) => void;
    volume?: number;
  }) => SpotifyPlayer;
};

type SpotifyWindow = Window &
  typeof globalThis & {
    Spotify?: SpotifyNamespace;
    onSpotifyWebPlaybackSDKReady?: () => void;
  };

export type SpotifyPlayerSession = {
  activateElement: () => Promise<void>;
  connect: () => Promise<boolean>;
  disconnect: () => void;
};

export type SpotifyPlayerSessionOptions = {
  volume: number;
  getOAuthToken: () => Promise<string>;
  onReady: (deviceID: string) => void;
  onNotReady: () => void;
  onStateChanged: (state: SpotifyPlayerState | null) => void;
  onIssue: (message: string) => void;
};

export async function createSpotifyPlayerSession(
  options: SpotifyPlayerSessionOptions,
): Promise<SpotifyPlayerSession> {
  await loadSpotifySDK();
  const Spotify = (window as SpotifyWindow).Spotify;
  if (!Spotify?.Player) {
    throw new Error("Spotify player is unavailable.");
  }

  const player = new Spotify.Player({
    name: "Jute Dash",
    volume: Math.max(0, Math.min(1, options.volume / 100)),
    getOAuthToken: async (callback: (token: string) => void) => {
      const token = await options.getOAuthToken();
      callback(token);
    },
  });

  player.addListener("ready", ({ device_id }) => {
    options.onReady(device_id);
  });
  player.addListener("not_ready", () => {
    options.onNotReady();
  });
  player.addListener("player_state_changed", (state) => {
    options.onStateChanged(state as SpotifyPlayerState | null);
  });
  player.addListener("initialization_error", ({ message }) => {
    options.onIssue(message || "Spotify player could not start.");
  });
  player.addListener("authentication_error", ({ message }) => {
    options.onIssue(message || "Spotify login needs to be refreshed.");
  });
  player.addListener("account_error", ({ message }) => {
    options.onIssue(message || "Spotify Premium is required for browser playback.");
  });
  player.addListener("autoplay_failed", () => {
    options.onIssue(
      "Browser autoplay rules blocked playback. Press play once to continue.",
    );
  });

  return {
    activateElement: async () => {
      await player.activateElement?.();
    },
    connect: () => player.connect(),
    disconnect: () => player.disconnect(),
  };
}

function loadSpotifySDK(): Promise<void> {
  if (typeof window === "undefined") {
    return Promise.reject(new Error("Spotify playback needs a browser."));
  }
  const spotifyWindow = window as SpotifyWindow;
  if (spotifyWindow.Spotify?.Player) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const previousReady = spotifyWindow.onSpotifyWebPlaybackSDKReady;
    spotifyWindow.onSpotifyWebPlaybackSDKReady = () => {
      previousReady?.();
      resolve();
    };
    if (document.querySelector("script[data-jute-spotify-sdk]")) {
      return;
    }
    const script = document.createElement("script");
    script.src = "https://sdk.scdn.co/spotify-player.js";
    script.async = true;
    script.dataset.juteSpotifySdk = "true";
    script.onerror = () =>
      reject(new Error("Spotify player could not be loaded."));
    document.head.appendChild(script);
  });
}
