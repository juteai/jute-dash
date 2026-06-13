<script lang="ts">
  import { onMount } from "svelte";
  import { MonitorSpeaker } from "lucide-svelte";
  import { getSpotifyWebPlaybackToken } from "$lib/hubClient";
  import {
    MediaControls,
    MediaSummary,
    VolumeControl,
  } from "$lib/components/integration-controls";
  import WidgetActionButton from "$lib/components/widget-content/WidgetActionButton.svelte";
  import WidgetStack from "$lib/components/widget-content/WidgetStack.svelte";

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

  export let connectionId = "";
  export let data: any = {};
  export let dispatch: (
    action: string,
    args?: any,
  ) => Promise<any> = async () => {};
  export let stale = false;

  let busy = false;
  let player: SpotifyPlayer | undefined;
  let activatingPlayer = false;
  let playerReady = false;
  let playerIssue = "";
  let mounted = false;
  let activationAttemptedFor = "";
  let optimisticPlaying: boolean | undefined;
  let optimisticVolume: number | undefined;
  let optimisticClearTimer: number | undefined;

  $: serverPlaying = data?.is_playing ?? false;
  $: serverVolume = data?.volume ?? 50;
  $: isPlaying = optimisticPlaying ?? serverPlaying;
  $: trackTitle = data?.track_title ?? "Not Playing";
  $: artistName = data?.artist_name ?? "Unknown";
  $: albumArtURL = data?.album_art_url ?? "";
  $: volume = optimisticVolume ?? serverVolume;
  $: if (
    optimisticPlaying !== undefined &&
    optimisticPlaying === serverPlaying
  ) {
    optimisticPlaying = undefined;
  }
  $: if (optimisticVolume !== undefined && optimisticVolume === serverVolume) {
    optimisticVolume = undefined;
  }
  $: playerStatus = playerReady
    ? "Jute player ready"
    : activatingPlayer
      ? "Starting Jute player"
      : "Jute player connecting";
  $: if (
    mounted &&
    connectionId &&
    activationAttemptedFor !== connectionId &&
    !playerReady
  ) {
    activationAttemptedFor = connectionId;
    void activatePlayer();
  }
  $: if (!connectionId) {
    activationAttemptedFor = "";
  }

  async function runAction(action: string, args?: any) {
    if (busy || stale) {
      return;
    }
    busy = true;
    try {
      await dispatch(action, args);
    } finally {
      busy = false;
    }
  }

  function clearOptimisticStateSoon() {
    if (typeof window === "undefined") return;
    if (optimisticClearTimer) {
      window.clearTimeout(optimisticClearTimer);
    }
    optimisticClearTimer = window.setTimeout(() => {
      optimisticPlaying = undefined;
      optimisticVolume = undefined;
      optimisticClearTimer = undefined;
    }, 6000);
  }

  async function handlePlayPause() {
    const nextPlaying = !isPlaying;
    optimisticPlaying = nextPlaying;
    clearOptimisticStateSoon();
    await player?.activateElement?.();
    if (connectionId && !playerReady && !activatingPlayer) {
      await activatePlayer();
    }
    await runAction(nextPlaying ? "play" : "pause");
  }

  async function handleNext() {
    clearOptimisticStateSoon();
    await runAction("next");
  }

  async function handlePrevious() {
    clearOptimisticStateSoon();
    await runAction("previous");
  }

  async function handleVolume(vol: number) {
    optimisticVolume = vol;
    clearOptimisticStateSoon();
    await runAction("set_volume", { volume: vol });
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

  async function activatePlayer() {
    if (activatingPlayer || playerReady) return;
    if (!connectionId) {
      playerIssue = "Choose a Spotify Account connection in settings.";
      return;
    }
    activatingPlayer = true;
    playerIssue = "";
    try {
      await loadSpotifySDK();
      const Spotify = (window as SpotifyWindow).Spotify;
      if (!Spotify?.Player) {
        throw new Error("Spotify player is unavailable.");
      }
      player?.disconnect();
      const nextPlayer = new Spotify.Player({
        name: "Jute Dash",
        volume: Math.max(0, Math.min(1, volume / 100)),
        getOAuthToken: async (callback: (token: string) => void) => {
          const token = await getSpotifyWebPlaybackToken(fetch, connectionId);
          callback(token.accessToken);
        },
      });
      nextPlayer.addListener("ready", ({ device_id }) => {
        playerReady = true;
        void runAction("transfer_playback", { device_id, play: isPlaying });
      });
      nextPlayer.addListener("not_ready", () => {
        playerReady = false;
      });
      nextPlayer.addListener("initialization_error", ({ message }) => {
        playerIssue = message || "Spotify player could not start.";
      });
      nextPlayer.addListener("authentication_error", ({ message }) => {
        playerIssue = message || "Spotify login needs to be refreshed.";
      });
      nextPlayer.addListener("account_error", ({ message }) => {
        playerIssue =
          message || "Spotify Premium is required for browser playback.";
      });
      nextPlayer.addListener("autoplay_failed", () => {
        playerIssue =
          "Browser autoplay rules blocked playback. Press play once to continue.";
      });
      player = nextPlayer;
      const connected = await nextPlayer.connect();
      if (!connected) {
        playerIssue = "Spotify player could not connect in this browser.";
      }
    } catch (err) {
      playerIssue =
        err instanceof Error ? err.message : "Spotify player could not start.";
    } finally {
      activatingPlayer = false;
    }
  }

  function handleSpotifyLinked(event: MessageEvent) {
    if (event.origin !== window.location.origin) return;
    if (event.data?.type !== "jute.spotify.linked") return;
    activationAttemptedFor = "";
    if (connectionId) {
      window.setTimeout(() => {
        void activatePlayer();
      }, 900);
    }
  }

  onMount(() => {
    mounted = true;
    window.addEventListener("message", handleSpotifyLinked);
    return () => {
      mounted = false;
      window.removeEventListener("message", handleSpotifyLinked);
      if (optimisticClearTimer) {
        window.clearTimeout(optimisticClearTimer);
      }
      player?.disconnect();
    };
  });
</script>

<WidgetStack {stale} class="media-widget">
  <MediaSummary
    title={trackTitle}
    subtitle={artistName}
    imageUrl={albumArtURL}
    imageAlt={`${trackTitle} album art`}
  />
  {#if connectionId}
    <div class="player-status">
      <span class:player-status--ready={playerReady} class="player-status-icon">
        <MonitorSpeaker size={16} />
      </span>
      <span>{playerStatus}</span>
    </div>
  {/if}
  <MediaControls
    {isPlaying}
    disabled={busy || stale}
    onPrevious={handlePrevious}
    onPlayPause={handlePlayPause}
    onNext={handleNext}
  />
  <VolumeControl {volume} disabled={busy || stale} onVolume={handleVolume} />
  {#if playerIssue}
    <div class="player-issue-row">
      <p class="player-issue">{playerIssue}</p>
      <WidgetActionButton
        label="Retry Jute player"
        disabled={busy || stale || activatingPlayer}
        on:click={activatePlayer}
      >
        <MonitorSpeaker size={16} />
      </WidgetActionButton>
    </div>
  {/if}
</WidgetStack>

<style>
  :global(.media-widget) {
    justify-content: center;
    padding: clamp(4px, 2cqmin, 8px);
  }

  .player-status {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: 30px;
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 700;
  }

  .player-status-icon {
    display: inline-grid;
    width: 30px;
    height: 30px;
    place-items: center;
    border: 1px solid var(--border);
    border-radius: 8px;
    color: var(--muted);
  }

  .player-status--ready {
    border-color: color-mix(in srgb, var(--success) 45%, var(--border));
    color: var(--success);
  }

  .player-issue-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-width: 0;
  }

  .player-issue {
    margin: 0;
    overflow: hidden;
    min-width: 0;
    color: var(--warning);
    font-size: var(--widget-label-size);
    font-weight: 700;
    text-align: center;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
