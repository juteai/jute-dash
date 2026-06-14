<script lang="ts">
  import { ListMusic, Music2, Search, Volume2 } from "lucide-svelte";
  import type { SpotifyPlayableItem, SpotifyPlayableType } from "../types";

  export let volume = 50;
  export let busy = false;
  export let stale = false;
  export let searchQuery = "";
  export let searchMode: SpotifyPlayableType = "album";
  export let searchSuggestions: SpotifyPlayableItem[] = [];
  export let searching = false;
  export let onVolumePreview: (volume: number) => void = () => {};
  export let onVolumeChange: (volume: number) => void = () => {};
  export let onSearchPlay: () => void = () => {};
  export let onSuggestionPlay: (item: SpotifyPlayableItem) => void = () => {};
</script>

<div
  class="options-modal"
  aria-label="Spotify options"
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
      aria-label="Spotify volume"
      on:input={(event) => onVolumePreview(Number(event.currentTarget.value))}
      on:change={(event) => onVolumeChange(Number(event.currentTarget.value))}
    />
  </section>

  <form class="options-search" on:submit|preventDefault={onSearchPlay}>
    <label for="spotify-search">Search Spotify</label>
    <div class="search-controls">
      <div class="search-input-wrap">
        <Search size={14} />
        <input
          id="spotify-search"
          bind:value={searchQuery}
          disabled={busy || stale}
          placeholder={searchMode === "album"
            ? "Find an album"
            : searchMode === "track"
              ? "Find a song"
              : "Find a playlist"}
        />
      </div>
      <select
        bind:value={searchMode}
        disabled={busy || stale}
        aria-label="Spotify search type"
      >
        <option value="album">Album</option>
        <option value="track">Song</option>
        <option value="playlist">Playlist</option>
      </select>
      <button
        type="submit"
        class="search-submit"
        disabled={busy || stale || !searchQuery.trim()}
        aria-label="Play Spotify search result"
      >
        <ListMusic size={16} />
      </button>
    </div>
    {#if searchSuggestions.length > 0}
      <div class="search-suggestions" aria-label="Spotify suggestions">
        {#each searchSuggestions as suggestion (suggestion.id)}
          <button
            type="button"
            class="search-suggestion"
            disabled={busy || stale}
            aria-label={`Play ${suggestion.name}`}
            on:click={() => onSuggestionPlay(suggestion)}
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

<style>
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

  .options-section label {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 760;
  }

  .options-section label span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .options-section strong {
    color: var(--foreground);
  }

  .options-section input {
    width: 100%;
    accent-color: var(--active);
  }

  .options-search > label {
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 760;
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
    border: 0;
    border-bottom: 1px solid var(--border);
    border-radius: 8px;
    background: transparent;
  }

  .search-submit {
    display: inline-grid;
    width: 40px;
    min-height: 38px;
    place-items: center;
    border: 0;
    border-radius: 50%;
    background: color-mix(in srgb, var(--surface-muted) 72%, transparent);
    color: var(--foreground);
    cursor: pointer;
  }

  .search-submit:disabled,
  .options-search input:disabled,
  .options-search select:disabled,
  .search-suggestion:disabled {
    cursor: default;
    opacity: 0.54;
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

  .search-suggestion:hover:not(:disabled) {
    background: var(--surface-muted);
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

  .search-suggestion-art--empty {
    border: 1px solid var(--border);
  }

  .search-suggestion-copy {
    display: grid;
    min-width: 0;
    gap: 1px;
  }

  .search-suggestion-copy span,
  .search-suggestion-copy small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-suggestion-copy span {
    font-size: var(--widget-label-size);
    font-weight: 780;
  }

  .search-suggestion-copy small {
    color: var(--muted);
    font-size: clamp(0.58rem, 2.8cqmin, 0.72rem);
    font-weight: 680;
  }

  .search-submit:focus-visible,
  .options-search input:focus-visible,
  .options-search select:focus-visible,
  .search-suggestion:focus-visible,
  .options-section input:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  @container (max-width: 260px) {
    .options-search select {
      display: none;
    }

    .search-controls {
      grid-template-columns: minmax(0, 1fr) auto;
    }
  }
</style>
