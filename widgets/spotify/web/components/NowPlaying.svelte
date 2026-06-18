<script lang="ts">
  import { MoreVertical, Music2 } from "lucide-svelte";
  import Fa from "svelte-fa";
  import { faSpotify } from "@fortawesome/free-brands-svg-icons";

  export let trackTitle = "Not Playing";
  export let artistName = "Unknown";
  export let albumArtURL = "";
  export let spotifyStatus: "connected" | "connecting" | "disconnected" =
    "disconnected";
  export let spotifyStatusLabel = "Spotify not connected";
  export let optionsOpen = false;
  export let busy = false;
  export let stale = false;
  export let onToggleOptions: () => void = () => {};
</script>

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
      class="spotify-status"
      class:spotify-status--disconnected={spotifyStatus === "disconnected"}
      class:spotify-status--connecting={spotifyStatus === "connecting"}
      class:spotify-status--connected={spotifyStatus === "connected"}
      role="img"
      aria-label={spotifyStatusLabel}
      title={spotifyStatusLabel}
    >
      <Fa icon={faSpotify} />
    </span>
  </div>

  <div class="player-menu">
    <button
      type="button"
      class="menu-button"
      aria-label="Spotify widget options"
      aria-expanded={optionsOpen}
      disabled={busy || stale}
      on:pointerdown|stopPropagation
      on:click|stopPropagation={onToggleOptions}
    >
      <MoreVertical size={18} />
    </button>
    {#if optionsOpen}
      <slot name="options" />
    {/if}
  </div>
</div>

<style>
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

  .menu-button {
    display: grid;
    width: clamp(34px, 10cqmin, 42px);
    height: clamp(34px, 10cqmin, 42px);
    place-items: center;
    border: 0;
    border-radius: 50%;
    background: color-mix(in srgb, var(--surface-muted) 76%, transparent);
    color: var(--muted);
    cursor: pointer;
  }

  .menu-button:hover:not(:disabled),
  .menu-button[aria-expanded="true"] {
    background: var(--surface-strong);
    color: var(--foreground);
  }

  .menu-button:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .menu-button:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  .spotify-status {
    display: inline-grid;
    width: 24px;
    height: 24px;
    place-items: center;
    color: var(--muted);
  }

  .spotify-status :global(svg) {
    display: block;
    width: 100%;
    height: 100%;
  }

  .spotify-status--disconnected {
    color: var(--danger);
  }

  .spotify-status--connecting {
    color: var(--warning);
  }

  .spotify-status--connected {
    color: var(--success);
  }

  @container (max-width: 260px) {
    .spotify-status {
      display: none;
    }

    .now-playing {
      grid-template-columns: auto minmax(0, 1fr);
    }
  }

  @container (max-height: 190px) {
    .spotify-status {
      display: none;
    }

    .now-playing {
      padding: 8px 10px;
    }

    .album-art {
      width: clamp(48px, 20cqmin, 72px);
      height: clamp(48px, 20cqmin, 72px);
    }
  }

  @container (max-height: 360px) {
    .now-playing {
      padding-block: clamp(8px, 2cqmin, 12px);
    }

    .album-art {
      width: clamp(50px, 18cqmin, 78px);
      height: clamp(50px, 18cqmin, 78px);
    }
  }
</style>
