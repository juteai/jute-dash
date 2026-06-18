<script lang="ts">
  import { onMount } from "svelte";
  import {
    ListMusic,
    MoreVertical,
    Music2,
    Pause,
    Play,
    Repeat,
    Repeat1,
    Search,
    Shuffle,
    SkipBack,
    SkipForward,
    Volume2,
  } from "lucide-svelte";
  import Fa from "svelte-fa";
  import { faApple } from "@fortawesome/free-brands-svg-icons";
  import {
    getAppleMusicKitToken,
    saveAppleMusicUserToken,
  } from "$lib/hubClient";
  import WidgetActionButton from "$lib/components/widget-content/WidgetActionButton.svelte";
  import WidgetStack from "$lib/components/widget-content/WidgetStack.svelte";
  import {
    actionForPlayableItem,
    musicKitQueueDescriptor,
    playableItemsFromAlbums,
    playableItemsFromSearchResults,
  } from "./discovery";
  import {
    APPLE_MUSIC_PREVIOUS_DOUBLE_PRESS_MS,
    clampProgress,
    createPlayerStateProgressClock,
    createServerProgressClock,
    estimateProgress,
    formatTime,
    nextRepeatState,
    normalizeRepeatState,
    nowMS,
    progressPercent as calculateProgressPercent,
    repeatLabel,
    setProgressClockPlaying,
    setProgressClockPosition,
    shuffleLabel,
    type PlaybackProgressClock,
  } from "./playbackViewModel";
  import {
    createAppleMusicPlayerSession,
    type AppleMusicPlayerSession,
  } from "./playerSession";
  import type {
    AppleMusicPlayableItem,
    AppleMusicPlayableType,
    AppleMusicPlayerState,
    AppleMusicRepeatState,
  } from "./types";

  export let connectionId = "";
  export let data: any = {};
  export let dispatch: (
    action: string,
    args?: any,
  ) => Promise<any> = async () => {};
  export let stale = false;

  let busy = false;
  let playerSession: AppleMusicPlayerSession | undefined;
  let activatingPlayer = false;
  let playerReady = false;
  let playerIssue = "";
  let mounted = false;
  let activationAttemptedFor = "";
  let optimisticPlaying: boolean | undefined;
  let optimisticVolume: number | undefined;
  let optimisticClearTimer: number | undefined;
  let previousPressTimer: number | undefined;
  let searchQuery = "";
  let searchMode: AppleMusicPlayableType = "album";
  let searchSuggestions: AppleMusicPlayableItem[] = [];
  let searchDebounceTimer: number | undefined;
  let searchRequestID = 0;
  let searching = false;
  let optionsOpen = false;
  let optimisticProgressMS: number | undefined;
  let optimisticShuffle: boolean | undefined;
  let optimisticRepeatState: AppleMusicRepeatState | undefined;
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
  $: needsUserToken = Boolean(data?.needs_user_token);
  $: trackIdentity = trackURI || `${trackTitle}|${artistName}|${durationMS}`;
  $: quickPicks = playableItemsFromAlbums(data?.playable_items ?? data?.top_albums);
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
  $: appleStatus = !connectionId
    ? "disconnected"
    : playerReady
      ? "connected"
      : "connecting";
  $: appleStatusLabel =
    appleStatus === "connected"
      ? "Apple Music connected"
      : appleStatus === "connecting"
        ? "Apple Music ready to sign in"
        : "Apple Music not connected";
  $: if (
    mounted &&
    connectionId &&
    activationAttemptedFor !== connectionId &&
    !playerReady
  ) {
    activationAttemptedFor = connectionId;
    void activatePlayer(false);
  }
  $: if (!connectionId) {
    activationAttemptedFor = "";
  }
  $: queueSearchSuggestions(optionsOpen, searchQuery, searchMode, connectionId);

  async function runAction(action: string, args?: any) {
    if (busy || stale) return;
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
    if (progressServerSnapshotKey === snapshotKey) return;
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

  function syncProgressClockFromPlayerState(state: AppleMusicPlayerState) {
    const now = nowMS();
    const item = state.nowPlayingItem;
    const sdkTrackKey =
      item?.playParams?.id ?? item?.id ?? `${item?.title}|${item?.artistName}`;
    progressTrackKey = sdkTrackKey || progressTrackKey;
    progressClock = createPlayerStateProgressClock(state, now);
    localProgressNowMS = now;
    if (item?.title) {
      data = {
        ...data,
        track_title: item.title,
        artist_name: item.artistName ?? item.albumName ?? "Unknown",
        album_art_url: item.artworkURL ?? item.artwork?.url ?? albumArtURL,
        is_playing: Boolean(state.isPlaying),
        progress_ms: Math.round((state.currentPlaybackTime ?? 0) * 1000),
        duration_ms: Math.round((state.currentPlaybackDuration ?? 0) * 1000),
      };
    }
    restartProgressTicker();
  }

  function syncProgressClockPlaying(playing: boolean) {
    if (progressClock.isPlaying === playing) return;
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
    if (typeof window === "undefined") return;
    if (progressTickTimer) {
      window.clearInterval(progressTickTimer);
      progressTickTimer = undefined;
    }
    if (!progressClock.isPlaying || progressClock.durationMS <= 0) return;
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
    await activatePlayer(true);
    if (nextPlaying) {
      await playerSession?.play();
    } else {
      await playerSession?.pause();
    }
    await runAction(nextPlaying ? "play" : "pause");
  }

  async function handleNext() {
    clearOptimisticStateSoon();
    await playerSession?.next();
    await runAction("next");
  }

  async function handlePrevious() {
    if (typeof window !== "undefined" && previousPressTimer) {
      window.clearTimeout(previousPressTimer);
      previousPressTimer = undefined;
      clearOptimisticStateSoon();
      await playerSession?.previous();
      await runAction("previous");
      return;
    }
    if (typeof window === "undefined") {
      await handleSeek(0);
      return;
    }
    previousPressTimer = window.setTimeout(() => {
      previousPressTimer = undefined;
      clearOptimisticStateSoon();
      void handleSeek(0);
    }, APPLE_MUSIC_PREVIOUS_DOUBLE_PRESS_MS);
  }

  async function handleVolume(vol: number) {
    optimisticVolume = vol;
    clearOptimisticStateSoon();
    playerSession?.setVolume(vol);
    await runAction("set_volume", { volume: vol });
  }

  async function handleSeek(positionMS: number) {
    previewProgress(positionMS);
    optimisticProgressMS = undefined;
    clearOptimisticStateSoon();
    await playerSession?.seek(positionMS);
    await runAction("seek", { position_ms: positionMS });
  }

  async function handleShuffleToggle() {
    const next = !shuffle;
    optimisticShuffle = next;
    clearOptimisticStateSoon();
    playerSession?.setShuffle(next);
    await runAction("set_shuffle", { state: next });
  }

  async function handleRepeatToggle() {
    const next = nextRepeatState(repeatState);
    optimisticRepeatState = next;
    clearOptimisticStateSoon();
    playerSession?.setRepeat(next);
    await runAction("set_repeat", { state: next });
  }

  async function handleSearchPlay() {
    const query = searchQuery.trim();
    if (!query) return;
    optimisticPlaying = true;
    clearOptimisticStateSoon();
    await activatePlayer(true);
    await runAction(
      searchMode === "playlist"
        ? "play_playlist"
        : searchMode === "album"
          ? "play_album"
          : "play_track",
      { query },
    );
    searchQuery = "";
    searchSuggestions = [];
    optionsOpen = false;
  }

  async function handleSuggestionPlay(result: AppleMusicPlayableItem) {
    if (!result.uri) return;
    optimisticPlaying = true;
    clearOptimisticStateSoon();
    await activatePlayer(true);
    const descriptor = musicKitQueueDescriptor(result);
    if (descriptor) {
      await playerSession?.playItem(descriptor);
    }
    await runAction(actionForPlayableItem(result), { uri: result.uri });
    searchQuery = "";
    searchSuggestions = [];
    optionsOpen = false;
  }

  async function handleAlbumPlay(album: AppleMusicPlayableItem) {
    if (!album.uri) return;
    optimisticPlaying = true;
    clearOptimisticStateSoon();
    await activatePlayer(true);
    const descriptor = musicKitQueueDescriptor(album);
    if (descriptor) {
      await playerSession?.playItem(descriptor);
    }
    await runAction("play_album", { uri: album.uri });
  }

  function queueSearchSuggestions(
    open: boolean,
    queryValue: string,
    mode: AppleMusicPlayableType,
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
    mode: AppleMusicPlayableType,
  ) {
    if (stale) return;
    const requestID = ++searchRequestID;
    searching = true;
    try {
      const result = await dispatch("search", { query, type: mode, limit: 5 });
      if (requestID !== searchRequestID) return;
      searchSuggestions = playableItemsFromSearchResults(result?.results);
    } catch {
      if (requestID === searchRequestID) searchSuggestions = [];
    } finally {
      if (requestID === searchRequestID) searching = false;
    }
  }

  async function activatePlayer(authorize: boolean) {
    if (activatingPlayer || playerReady) {
      if (authorize) await playerSession?.authorize();
      return;
    }
    if (!connectionId) {
      playerIssue = "Choose an Apple Music Account connection in settings.";
      return;
    }
    activatingPlayer = true;
    playerIssue = "";
    try {
      playerSession?.disconnect();
      const token = await getAppleMusicKitToken(fetch, connectionId);
      const nextSession = await createAppleMusicPlayerSession({
        developerToken: token.developerToken,
        userToken: token.userToken,
        onAuthorized: (userToken) => {
          void saveAppleMusicUserToken(fetch, connectionId, userToken);
          data = { ...data, needs_user_token: false };
        },
        onStateChanged: syncProgressClockFromPlayerState,
        onIssue: (message) => {
          playerIssue = message;
        },
      });
      playerSession = nextSession;
      playerSession.setVolume(volume);
      playerReady = Boolean(token.userToken) && !needsUserToken;
      if (authorize || needsUserToken) {
        await nextSession.authorize();
        playerReady = true;
      }
      syncProgressClockFromPlayerState(nextSession.snapshot());
    } catch (err) {
      playerIssue =
        err instanceof Error ? err.message : "Apple Music player could not start.";
    } finally {
      activatingPlayer = false;
    }
  }

  onMount(() => {
    mounted = true;
    return () => {
      mounted = false;
      if (optimisticClearTimer) window.clearTimeout(optimisticClearTimer);
      if (previousPressTimer) window.clearTimeout(previousPressTimer);
      if (progressTickTimer) window.clearInterval(progressTickTimer);
      if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer);
      playerSession?.disconnect();
    };
  });
</script>

<svelte:window on:pointerdown={() => (optionsOpen = false)} />

<WidgetStack {stale} class="apple-music-widget">
  <div class="apple-player">
    <div class="now-playing">
      <div class:album-art--empty={!albumArtURL} class="album-art">
        {#if albumArtURL}
          <img
            src={albumArtURL}
            alt={`${trackTitle} album art`}
            loading="lazy"
            referrerpolicy="no-referrer"
          />
        {:else}
          <Music2 size={28} />
        {/if}
      </div>
      <div class="track-copy">
        <p class="track-title">{trackTitle}</p>
        <p class="track-artist">{artistName}</p>
        <span
          class="apple-status"
          class:apple-status--disconnected={appleStatus === "disconnected"}
          class:apple-status--connecting={appleStatus === "connecting"}
          class:apple-status--connected={appleStatus === "connected"}
          role="img"
          aria-label={appleStatusLabel}
          title={appleStatusLabel}
        >
          <Fa icon={faApple} />
        </span>
      </div>
      <div class="player-menu">
        <button
          type="button"
          class="menu-button"
          aria-label="Apple Music widget options"
          aria-expanded={optionsOpen}
          disabled={busy || stale}
          on:pointerdown|stopPropagation
          on:click|stopPropagation={() => (optionsOpen = !optionsOpen)}
        >
          <MoreVertical size={18} />
        </button>
        {#if optionsOpen}
          <div
            class="options-modal"
            aria-label="Apple Music options"
            role="presentation"
            on:pointerdown|stopPropagation
          >
            <section class="options-section">
              <label>
                <span><Volume2 size={14} /> Volume</span>
                <strong>{volume}%</strong>
              </label>
              <input
                type="range"
                min="0"
                max="100"
                step="1"
                value={volume}
                disabled={busy || stale}
                aria-label="Apple Music volume"
                on:input={(event) =>
                  (optimisticVolume = Number(event.currentTarget.value))}
                on:change={(event) =>
                  handleVolume(Number(event.currentTarget.value))}
              />
            </section>
            {#if needsUserToken}
              <WidgetActionButton
                label="Login with Apple Music"
                disabled={busy || stale || activatingPlayer}
                on:click={() => activatePlayer(true)}
              />
            {/if}
            <form class="options-search" on:submit|preventDefault={handleSearchPlay}>
              <label for="apple-music-search">Search Apple Music</label>
              <div class="search-controls">
                <div class="search-input-wrap">
                  <Search size={14} />
                  <input
                    id="apple-music-search"
                    value={searchQuery}
                    disabled={busy || stale}
                    placeholder={searchMode === "album"
                      ? "Find an album"
                      : searchMode === "track"
                        ? "Find a song"
                        : "Find a playlist"}
                    on:input={(event) =>
                      (searchQuery = event.currentTarget.value)}
                  />
                </div>
                <select
                  value={searchMode}
                  disabled={busy || stale}
                  aria-label="Apple Music search type"
                  on:change={(event) =>
                    (searchMode = event.currentTarget.value as AppleMusicPlayableType)}
                >
                  <option value="album">Album</option>
                  <option value="track">Song</option>
                  <option value="playlist">Playlist</option>
                </select>
                <button
                  type="submit"
                  class="search-submit"
                  disabled={busy || stale || !searchQuery.trim()}
                  aria-label="Play Apple Music search result"
                >
                  <ListMusic size={16} />
                </button>
              </div>
              {#if searchSuggestions.length > 0}
                <div class="search-suggestions" aria-label="Apple Music suggestions">
                  {#each searchSuggestions as suggestion (suggestion.id)}
                    <button
                      type="button"
                      class="search-suggestion"
                      disabled={busy || stale}
                      aria-label={`Play ${suggestion.name}`}
                      on:click={() => handleSuggestionPlay(suggestion)}
                    >
                      <span
                        class:search-suggestion-art--empty={!suggestion.album_art_url}
                        class="search-suggestion-art"
                      >
                        {#if suggestion.album_art_url}
                          <img
                            src={suggestion.album_art_url}
                            alt={`${suggestion.name} artwork`}
                            loading="lazy"
                            referrerpolicy="no-referrer"
                          />
                        {:else}
                          <Music2 size={16} />
                        {/if}
                      </span>
                      <span class="search-suggestion-copy">
                        <span>{suggestion.name}</span>
                        {#if suggestion.subtitle}
                          <small>{suggestion.subtitle}</small>
                        {/if}
                      </span>
                    </button>
                  {/each}
                </div>
              {:else if searching}
                <div class="search-suggestions search-suggestions--loading">
                  <span>Searching...</span>
                </div>
              {/if}
            </form>
          </div>
        {/if}
      </div>
    </div>

    <div class="touch-controls" aria-label="Apple Music playback controls">
      <button
        type="button"
        class="control-button control-button--mode"
        class:control-button--mode-active={shuffle}
        disabled={busy || stale}
        aria-label={shuffleLabel(shuffle)}
        aria-pressed={shuffle}
        title={shuffleLabel(shuffle)}
        on:click={handleShuffleToggle}
      >
        <Shuffle size={20} />
      </button>
      <button
        type="button"
        class="control-button"
        disabled={busy || stale}
        aria-label="Restart track. Press twice for previous track."
        on:click={handlePrevious}
      >
        <SkipBack size={22} />
      </button>
      <button
        type="button"
        class="control-button control-button--primary"
        class:control-button--active={isPlaying}
        disabled={busy || stale}
        aria-label={isPlaying ? "Pause playback" : "Start playback"}
        aria-pressed={isPlaying}
        on:click={handlePlayPause}
      >
        {#if isPlaying}
          <Pause size={25} />
        {:else}
          <Play size={25} />
        {/if}
      </button>
      <button
        type="button"
        class="control-button"
        disabled={busy || stale}
        aria-label="Next track"
        on:click={handleNext}
      >
        <SkipForward size={22} />
      </button>
      <button
        type="button"
        class="control-button control-button--mode"
        class:control-button--mode-active={repeatState !== "off"}
        disabled={busy || stale}
        aria-label={repeatLabel(repeatState)}
        aria-pressed={repeatState !== "off"}
        title={repeatLabel(repeatState)}
        on:click={handleRepeatToggle}
      >
        {#if repeatState === "track"}
          <Repeat1 size={20} />
        {:else}
          <Repeat size={20} />
        {/if}
      </button>
    </div>

    <div class="timeline-row">
      <span>{formatTime(progressMS)}</span>
      <input
        type="range"
        min="0"
        max={Math.max(timelineDurationMS, 1)}
        step="1000"
        value={progressMS}
        disabled={busy || stale || timelineDurationMS <= 0}
        aria-label="Apple Music track timeline"
        aria-valuetext={`${formatTime(progressMS)} of ${formatTime(timelineDurationMS)}`}
        style={`--timeline-progress: ${progressPercent}%`}
        on:input={(event) => previewProgress(Number(event.currentTarget.value))}
        on:change={(event) => handleSeek(Number(event.currentTarget.value))}
      />
      <span>{formatTime(timelineDurationMS)}</span>
    </div>

    {#if quickPicks.length > 0}
      <div class="album-strip" aria-label="Apple Music album suggestions">
        {#each quickPicks as item (item.id)}
          <button
            type="button"
            class="album-button"
            disabled={busy || stale}
            aria-label={`Play ${item.name}`}
            on:click={() => handleAlbumPlay(item)}
          >
            {#if item.album_art_url}
              <img
                src={item.album_art_url}
                alt={`${item.name} album art`}
                loading="lazy"
                referrerpolicy="no-referrer"
              />
            {:else}
              <Music2 size={20} />
            {/if}
            <span>{item.name}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>

  {#if playerIssue}
    <div class="player-issue-row">
      <p class="player-issue">{playerIssue}</p>
      <WidgetActionButton
        label="Retry Apple Music"
        disabled={busy || stale || activatingPlayer}
        on:click={() => activatePlayer(true)}
      />
    </div>
  {/if}
</WidgetStack>

<style>
  :global(.apple-music-widget) {
    position: relative;
    justify-content: center;
    gap: clamp(6px, 2cqmin, 12px);
    padding: clamp(4px, 1.8cqmin, 8px);
  }

  .apple-player {
    display: grid;
    grid-template-rows: minmax(0, 1fr) auto auto;
    gap: clamp(4px, 1.6cqmin, 10px);
    width: 100%;
    height: 100%;
    min-height: 0;
  }

  .now-playing {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: clamp(10px, 3cqmin, 16px);
    min-width: 0;
    min-height: 0;
    padding: clamp(10px, 3cqmin, 16px);
  }

  .album-art {
    display: grid;
    width: clamp(58px, 22cqmin, 96px);
    height: clamp(58px, 22cqmin, 96px);
    place-items: center;
    overflow: hidden;
    border-radius: 8px;
    background: color-mix(in srgb, var(--background) 82%, transparent);
    color: var(--muted);
    box-shadow: 0 8px 24px color-mix(in srgb, #000 24%, transparent);
  }

  .album-art img {
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .album-art--empty {
    border: 1px solid var(--border);
  }

  .track-copy {
    display: grid;
    min-width: 0;
    gap: 4px;
  }

  .track-title,
  .track-artist {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .track-title {
    color: var(--foreground);
    font-size: clamp(1rem, 6cqmin, 1.28rem);
    font-weight: 780;
  }

  .track-artist {
    color: color-mix(in srgb, var(--foreground) 70%, var(--muted));
    font-size: clamp(0.82rem, 4.5cqmin, 1rem);
    font-weight: 620;
  }

  .player-menu {
    position: relative;
    align-self: start;
  }

  .menu-button,
  .search-submit {
    display: grid;
    place-items: center;
    border: 0;
    border-radius: 50%;
    background: color-mix(in srgb, var(--surface-muted) 76%, transparent);
    color: var(--muted);
    cursor: pointer;
  }

  .menu-button {
    width: clamp(34px, 10cqmin, 42px);
    height: clamp(34px, 10cqmin, 42px);
  }

  .menu-button:hover:not(:disabled),
  .menu-button[aria-expanded="true"] {
    background: var(--surface-strong);
    color: var(--foreground);
  }

  .apple-status {
    display: inline-grid;
    width: 24px;
    height: 24px;
    place-items: center;
    color: var(--muted);
  }

  .apple-status :global(svg) {
    display: block;
    width: 100%;
    height: 100%;
  }

  .apple-status--disconnected {
    color: var(--danger);
  }

  .apple-status--connecting {
    color: var(--warning);
  }

  .apple-status--connected {
    color: var(--success);
  }

  .touch-controls {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: clamp(8px, 4cqmin, 18px);
    padding: clamp(6px, 2cqmin, 12px) 0;
  }

  .control-button {
    display: inline-grid;
    width: clamp(46px, 14cqmin, 62px);
    height: clamp(46px, 14cqmin, 62px);
    place-items: center;
    border: 0;
    border-radius: 50%;
    background: transparent;
    color: color-mix(in srgb, var(--foreground) 82%, var(--muted));
    cursor: pointer;
  }

  .control-button--primary {
    background: color-mix(in srgb, var(--foreground) 90%, transparent);
    color: var(--background);
  }

  .control-button--primary.control-button--active {
    background: color-mix(in srgb, var(--active) 72%, var(--foreground));
  }

  .control-button--mode {
    width: clamp(38px, 11cqmin, 50px);
    height: clamp(38px, 11cqmin, 50px);
    color: var(--muted);
  }

  .control-button--mode-active {
    background: color-mix(in srgb, var(--active) 16%, transparent);
    color: var(--foreground);
  }

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

  .timeline-row input,
  .options-section input {
    width: 100%;
    accent-color: var(--active);
  }

  .album-strip {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(52px, 72px));
    justify-content: center;
    gap: clamp(6px, 2cqmin, 10px);
    width: 100%;
    min-width: 0;
    min-height: 0;
    overflow: hidden;
  }

  .album-button {
    display: grid;
    min-width: 0;
    gap: 4px;
    border: 0;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
    font: inherit;
    font-size: clamp(0.58rem, 3cqmin, 0.72rem);
    font-weight: 700;
    text-align: left;
  }

  .album-button img,
  .album-button :global(svg) {
    width: 100%;
    aspect-ratio: 1;
    border-radius: 6px;
    object-fit: cover;
    background: var(--surface-muted);
  }

  .album-button span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .options-modal {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 3;
    display: grid;
    width: min(340px, 82cqw);
    gap: 14px;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: color-mix(in srgb, var(--background) 96%, transparent);
    box-shadow: 0 18px 40px color-mix(in srgb, #000 34%, transparent);
  }

  .options-section,
  .options-search {
    display: grid;
    gap: 10px;
  }

  .options-section label,
  .options-search > label {
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 760;
  }

  .options-section label {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .options-section label span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .search-controls {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    gap: 6px;
    width: 100%;
    min-width: 0;
  }

  .search-input-wrap {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: center;
    min-width: 0;
    gap: 6px;
    min-height: 38px;
    padding: 0 2px;
    border-bottom: 1px solid var(--border);
    color: var(--muted);
  }

  .options-search input,
  .options-search select {
    min-width: 0;
    border: 0;
    background: transparent;
    color: var(--foreground);
    font: inherit;
    font-size: var(--widget-label-size);
    font-weight: 720;
  }

  .options-search select {
    min-height: 38px;
    padding: 0 8px;
    border-bottom: 1px solid var(--border);
    border-radius: 8px;
  }

  .search-submit {
    width: 40px;
    min-height: 38px;
  }

  .search-suggestions {
    display: grid;
    gap: 4px;
    max-height: min(230px, 44cqh);
    overflow: auto;
    padding-top: 2px;
  }

  .search-suggestions--loading {
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 720;
  }

  .search-suggestion {
    display: grid;
    grid-template-columns: 34px minmax(0, 1fr);
    align-items: center;
    gap: 8px;
    min-width: 0;
    min-height: 42px;
    padding: 4px;
    border: 0;
    border-radius: 8px;
    background: transparent;
    color: var(--foreground);
    cursor: pointer;
    font: inherit;
    text-align: left;
  }

  .search-suggestion-art {
    display: grid;
    width: 34px;
    height: 34px;
    place-items: center;
    overflow: hidden;
    border-radius: 6px;
    background: var(--surface-muted);
    color: var(--muted);
  }

  .search-suggestion-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .search-suggestion-copy {
    display: grid;
    min-width: 0;
    gap: 2px;
  }

  .search-suggestion-copy span,
  .search-suggestion-copy small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-suggestion-copy small {
    color: var(--muted);
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

  button:disabled,
  input:disabled,
  select:disabled {
    cursor: default;
    opacity: 0.54;
  }

  @container (max-width: 260px) {
    .apple-status,
    .timeline-row span {
      display: none;
    }

    .now-playing {
      grid-template-columns: auto minmax(0, 1fr);
    }
  }

  @container (max-height: 190px) {
    .apple-status,
    .album-strip,
    .timeline-row span {
      display: none;
    }

    .now-playing {
      padding: 8px 10px;
    }
  }

  @container (max-height: 360px) {
    :global(.apple-music-widget) {
      gap: clamp(4px, 1.4cqmin, 8px);
    }

    .apple-player {
      align-content: center;
      grid-template-rows: auto auto auto;
      max-width: min(560px, 100%);
      justify-self: center;
    }
  }
</style>
