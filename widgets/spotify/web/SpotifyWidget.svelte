<script lang="ts">
  import { onMount } from "svelte";
  import { MonitorSpeaker } from "lucide-svelte";
  import { getSpotifyWebPlaybackToken } from "$lib/hubClient";
  import {
    SPOTIFY_PREVIOUS_DOUBLE_PRESS_MS,
    clampProgress,
    createPlayerStateProgressClock,
    createServerProgressClock,
    estimateProgress,
    nextRepeatState,
    normalizeRepeatState,
    nowMS,
    progressPercent as calculateProgressPercent,
    setProgressClockPlaying,
    setProgressClockPosition,
    type PlaybackProgressClock,
  } from "./playbackViewModel";
  import {
    actionForPlayableItem,
    playableItemsFromAlbums,
    playableItemsFromSearchResults,
  } from "./discovery";
  import {
    createSpotifyPlayerSession,
    type SpotifyPlayerSession,
  } from "./playerSession";
  import type {
    SpotifyPlayableItem,
    SpotifyPlayerState,
    SpotifyRepeatState,
  } from "./types";
  import NowPlaying from "./components/NowPlaying.svelte";
  import OptionsMenu from "./components/OptionsMenu.svelte";
  import QuickPicks from "./components/QuickPicks.svelte";
  import Timeline from "./components/Timeline.svelte";
  import TransportControls from "./components/TransportControls.svelte";
  import WidgetActionButton from "$lib/components/widget-content/WidgetActionButton.svelte";
  import WidgetStack from "$lib/components/widget-content/WidgetStack.svelte";

  export let connectionId = "";
  export let data: any = {};
  export let dispatch: (
    action: string,
    args?: any,
  ) => Promise<any> = async () => {};
  export let stale = false;

  let busy = false;
  let playerSession: SpotifyPlayerSession | undefined;
  let activatingPlayer = false;
  let playerReady = false;
  let playerDeviceID = "";
  let playerIssue = "";
  let mounted = false;
  let activationAttemptedFor = "";
  let optimisticPlaying: boolean | undefined;
  let optimisticVolume: number | undefined;
  let optimisticClearTimer: number | undefined;
  let previousPressTimer: number | undefined;
  let searchQuery = "";
  let searchMode: "album" | "track" | "playlist" = "album";
  let searchSuggestions: SpotifyPlayableItem[] = [];
  let searchDebounceTimer: number | undefined;
  let searchRequestID = 0;
  let searching = false;
  let optionsOpen = false;
  let spotifyStatus: "connected" | "connecting" | "disconnected" =
    "disconnected";
  let optimisticProgressMS: number | undefined;
  let optimisticShuffle: boolean | undefined;
  let optimisticRepeatState: SpotifyRepeatState | undefined;
  let progressClock: PlaybackProgressClock = createServerProgressClock({
    progressMS: 0,
    durationMS: 0,
    observedAtMS: 0,
    isPlaying: false,
  });
  let progressServerSnapshotKey = "";
  let progressTrackKey = "";
  let progressTickTimer: number | undefined;
  let localProgressNowMS = 0;

  $: serverPlaying = data?.is_playing ?? false;
  $: serverVolume = data?.volume ?? 50;
  $: serverProgressMS = data?.progress_ms ?? 0;
  $: durationMS = data?.duration_ms ?? 0;
  $: serverShuffle = data?.shuffle ?? false;
  $: serverRepeatState = normalizeRepeatState(data?.repeat_state);
  $: isPlaying = optimisticPlaying ?? serverPlaying;
  $: trackTitle = data?.track_title ?? "Not Playing";
  $: artistName = data?.artist_name ?? "Unknown";
  $: albumArtURL = data?.album_art_url ?? "";
  $: trackURI = data?.track_uri ?? "";
  $: trackIdentity = trackURI || `${trackTitle}|${artistName}|${durationMS}`;
  $: topAlbums = playableItemsFromAlbums(data?.playable_items ?? data?.top_albums);
  $: volume = optimisticVolume ?? serverVolume;
  $: shuffle = optimisticShuffle ?? serverShuffle;
  $: repeatState = optimisticRepeatState ?? serverRepeatState;
  $: syncProgressClockFromServer(
    `${trackIdentity}|${durationMS}|${serverProgressMS}`,
    trackIdentity,
    serverProgressMS,
    durationMS,
    serverPlaying,
  );
  $: syncProgressClockPlaying(isPlaying);
  $: timelineDurationMS = Math.max(progressClock.durationMS, durationMS, 0);
  $: progressMS = clampProgress(
    optimisticProgressMS ?? estimateProgress(progressClock, localProgressNowMS),
    timelineDurationMS,
  );
  $: progressPercent = calculateProgressPercent(progressMS, timelineDurationMS);
  $: if (
    optimisticPlaying !== undefined &&
    optimisticPlaying === serverPlaying
  ) {
    optimisticPlaying = undefined;
  }
  $: if (optimisticVolume !== undefined && optimisticVolume === serverVolume) {
    optimisticVolume = undefined;
  }
  $: if (optimisticShuffle !== undefined && optimisticShuffle === serverShuffle) {
    optimisticShuffle = undefined;
  }
  $: if (
    optimisticRepeatState !== undefined &&
    optimisticRepeatState === serverRepeatState
  ) {
    optimisticRepeatState = undefined;
  }
  $: spotifyStatus = !connectionId
    ? "disconnected"
    : playerReady
      ? "connected"
      : "connecting";
  $: spotifyStatusLabel =
    spotifyStatus === "connected"
      ? "Spotify connected"
      : spotifyStatus === "connecting"
        ? "Spotify getting ready"
        : "Spotify not connected";
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
  $: queueSearchSuggestions(optionsOpen, searchQuery, searchMode, connectionId);

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
      optimisticProgressMS = undefined;
      optimisticShuffle = undefined;
      optimisticRepeatState = undefined;
      optimisticClearTimer = undefined;
    }, 6000);
  }

  function syncProgressClockFromServer(
    snapshotKey: string,
    trackKey: string,
    progressMS: number,
    nextDurationMS: number,
    playing: boolean,
  ) {
    if (progressServerSnapshotKey === snapshotKey) {
      return;
    }

    const now = nowMS();
    progressServerSnapshotKey = snapshotKey;
    progressTrackKey = trackKey;
    progressClock = createServerProgressClock({
      progressMS,
      durationMS: nextDurationMS,
      observedAtMS: now,
      isPlaying: playing,
    });
    localProgressNowMS = now;
    restartProgressTicker();
  }

  function syncProgressClockFromPlayerState(state: SpotifyPlayerState | null) {
    if (!state) {
      return;
    }

    const now = nowMS();
    const sdkTrackKey = state.track_window?.current_track?.uri ?? progressTrackKey;
    progressTrackKey = sdkTrackKey;
    progressClock = createPlayerStateProgressClock(state, now);
    localProgressNowMS = now;
    restartProgressTicker();
  }

  function syncProgressClockPlaying(playing: boolean) {
    if (progressClock.isPlaying === playing) {
      return;
    }

    const now = nowMS();
    progressClock = setProgressClockPlaying(progressClock, playing, now);
    localProgressNowMS = now;
    restartProgressTicker();
  }

  function previewProgress(positionMS: number) {
    const now = nowMS();
    optimisticProgressMS = positionMS;
    progressClock = setProgressClockPosition(progressClock, positionMS, now);
    localProgressNowMS = now;
    restartProgressTicker();
  }

  function restartProgressTicker() {
    if (typeof window === "undefined") {
      return;
    }
    if (progressTickTimer) {
      window.clearInterval(progressTickTimer);
      progressTickTimer = undefined;
    }
    if (!progressClock.isPlaying || progressClock.durationMS <= 0) {
      return;
    }
    progressTickTimer = window.setInterval(() => {
      const now = nowMS();
      localProgressNowMS = now;
      if (estimateProgress(progressClock, now) >= progressClock.durationMS) {
        window.clearInterval(progressTickTimer);
        progressTickTimer = undefined;
      }
    }, 500);
  }

  async function handlePlayPause() {
    const nextPlaying = !isPlaying;
    optimisticPlaying = nextPlaying;
    clearOptimisticStateSoon();
    await playerSession?.activateElement();
    if (connectionId && !playerReady && !activatingPlayer) {
      await activatePlayer();
    }
    await runAction(nextPlaying ? "play" : "pause", withPlayerDevice());
  }

  async function handleNext() {
    clearOptimisticStateSoon();
    await runAction("next", withPlayerDevice());
  }

  async function handlePrevious() {
    if (typeof window !== "undefined" && previousPressTimer) {
      window.clearTimeout(previousPressTimer);
      previousPressTimer = undefined;
      clearOptimisticStateSoon();
      await runAction("previous", withPlayerDevice());
      return;
    }
    if (typeof window === "undefined") {
      await runAction("restart_track", withPlayerDevice());
      return;
    }

    previousPressTimer = window.setTimeout(() => {
      previousPressTimer = undefined;
      clearOptimisticStateSoon();
      void runAction("restart_track", withPlayerDevice());
    }, SPOTIFY_PREVIOUS_DOUBLE_PRESS_MS);
  }

  async function handleVolume(vol: number) {
    optimisticVolume = vol;
    clearOptimisticStateSoon();
    await runAction("set_volume", withPlayerDevice({ volume: vol }));
  }

  async function handleSeek(positionMS: number) {
    previewProgress(positionMS);
    optimisticProgressMS = undefined;
    clearOptimisticStateSoon();
    await runAction("seek", withPlayerDevice({ position_ms: positionMS }));
  }

  async function handleShuffleToggle() {
    const next = !shuffle;
    optimisticShuffle = next;
    clearOptimisticStateSoon();
    await runAction("set_shuffle", withPlayerDevice({ state: next }));
  }

  async function handleRepeatToggle() {
    const next = nextRepeatState(repeatState);
    optimisticRepeatState = next;
    clearOptimisticStateSoon();
    await runAction("set_repeat", withPlayerDevice({ state: next }));
  }

  async function handleSearchPlay() {
    const query = searchQuery.trim();
    if (!query) return;
    optimisticPlaying = true;
    clearOptimisticStateSoon();
    await playerSession?.activateElement();
    await runAction(
      searchMode === "playlist"
        ? "play_playlist"
        : searchMode === "album"
          ? "play_album"
          : "play_track",
      withPlayerDevice({ query }),
    );
    searchQuery = "";
    searchSuggestions = [];
    optionsOpen = false;
  }

  async function handleSuggestionPlay(result: SpotifyPlayableItem) {
    if (!result.uri) return;
    optimisticPlaying = true;
    clearOptimisticStateSoon();
    await playerSession?.activateElement();
    await runAction(actionForPlayableItem(result), withPlayerDevice({ uri: result.uri }));
    searchQuery = "";
    searchSuggestions = [];
    optionsOpen = false;
  }

  async function handleAlbumPlay(album: SpotifyPlayableItem) {
    if (!album.uri) return;
    optimisticPlaying = true;
    clearOptimisticStateSoon();
    await playerSession?.activateElement();
    await runAction("play_album", withPlayerDevice({ uri: album.uri }));
  }

  function queueSearchSuggestions(
    open: boolean,
    queryValue: string,
    mode: "album" | "track" | "playlist",
    activeConnectionID: string,
  ) {
    if (typeof window === "undefined") return;
    if (searchDebounceTimer) {
      window.clearTimeout(searchDebounceTimer);
      searchDebounceTimer = undefined;
    }

    const query = queryValue.trim();
    if (!open || !activeConnectionID || query.length < 2) {
      searchRequestID += 1;
      searchSuggestions = [];
      searching = false;
      return;
    }

    searchDebounceTimer = window.setTimeout(() => {
      void loadSearchSuggestions(query, mode);
    }, 260);
  }

  async function loadSearchSuggestions(
    query: string,
    mode: "album" | "track" | "playlist",
  ) {
    if (stale) return;
    const requestID = ++searchRequestID;
    searching = true;
    try {
      const result = await dispatch("search", {
        query,
        type: mode,
        limit: 5,
      });
      if (requestID !== searchRequestID) return;
      searchSuggestions = playableItemsFromSearchResults(result?.results);
    } catch {
      if (requestID === searchRequestID) {
        searchSuggestions = [];
      }
    } finally {
      if (requestID === searchRequestID) {
        searching = false;
      }
    }
  }

  function withPlayerDevice(args: Record<string, any> = {}) {
    return playerDeviceID ? { ...args, device_id: playerDeviceID } : args;
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
      playerSession?.disconnect();
      const nextSession = await createSpotifyPlayerSession({
        volume,
        getOAuthToken: async () => {
          const token = await getSpotifyWebPlaybackToken(fetch, connectionId);
          return token.accessToken;
        },
        onReady: (deviceID) => {
          playerReady = true;
          playerDeviceID = deviceID;
          void runAction("transfer_playback", {
            device_id: deviceID,
            play: isPlaying,
          });
        },
        onNotReady: () => {
          playerReady = false;
          playerDeviceID = "";
        },
        onStateChanged: syncProgressClockFromPlayerState,
        onIssue: (message) => {
          playerIssue = message;
        },
      });
      playerSession = nextSession;
      const connected = await nextSession.connect();
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
      if (previousPressTimer) {
        window.clearTimeout(previousPressTimer);
      }
      if (progressTickTimer) {
        window.clearInterval(progressTickTimer);
      }
      if (searchDebounceTimer) {
        window.clearTimeout(searchDebounceTimer);
      }
      playerDeviceID = "";
      playerSession?.disconnect();
    };
  });
</script>

<svelte:window on:pointerdown={() => (optionsOpen = false)} />

<WidgetStack {stale} class="media-widget">
  <div class="spotify-player">
    <NowPlaying
      {trackTitle}
      {artistName}
      {albumArtURL}
      {spotifyStatus}
      {spotifyStatusLabel}
      {optionsOpen}
      {busy}
      {stale}
      onToggleOptions={() => (optionsOpen = !optionsOpen)}
    >
      <OptionsMenu
        slot="options"
        {volume}
        {busy}
        {stale}
        {searchQuery}
        {searchMode}
        {searchSuggestions}
        {searching}
        onVolumePreview={(nextVolume) => (optimisticVolume = nextVolume)}
        onVolumeChange={handleVolume}
        onSearchQueryChange={(query) => (searchQuery = query)}
        onSearchModeChange={(mode) => (searchMode = mode)}
        onSearchPlay={handleSearchPlay}
        onSuggestionPlay={handleSuggestionPlay}
      />
    </NowPlaying>

    <TransportControls
      {busy}
      {stale}
      {shuffle}
      {isPlaying}
      {repeatState}
      onShuffle={handleShuffleToggle}
      onPrevious={handlePrevious}
      onPlayPause={handlePlayPause}
      onNext={handleNext}
      onRepeat={handleRepeatToggle}
    />

    <Timeline
      {progressMS}
      durationMS={timelineDurationMS}
      {progressPercent}
      {busy}
      {stale}
      onPreview={previewProgress}
      onSeek={handleSeek}
    />

    <QuickPicks
      items={topAlbums}
      {busy}
      {stale}
      onPlay={handleAlbumPlay}
    />
  </div>

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
    position: relative;
    justify-content: center;
    gap: clamp(6px, 2cqmin, 12px);
    padding: clamp(4px, 1.8cqmin, 8px);
  }

  .spotify-player {
    display: grid;
    grid-template-rows: minmax(0, 1fr) auto auto;
    gap: clamp(4px, 1.6cqmin, 10px);
    width: 100%;
    height: 100%;
    min-height: 0;
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

  @container (max-height: 190px) {
    :global(.media-widget) {
      gap: 4px;
    }
  }

  @container (max-height: 360px) {
    :global(.media-widget) {
      gap: clamp(4px, 1.4cqmin, 8px);
    }

    .spotify-player {
      align-content: center;
      grid-template-rows: auto auto auto;
      max-width: min(560px, 100%);
      justify-self: center;
    }
  }
</style>
