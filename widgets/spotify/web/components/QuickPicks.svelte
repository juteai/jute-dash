<script lang="ts">
  import { Music2 } from "lucide-svelte";
  import type { SpotifyPlayableItem } from "../types";

  export let items: SpotifyPlayableItem[] = [];
  export let busy = false;
  export let stale = false;
  export let onPlay: (item: SpotifyPlayableItem) => void = () => {};
</script>

{#if items.length > 0}
  <div class="album-strip" aria-label="Spotify top albums">
    {#each items as item (item.id)}
      <button
        type="button"
        class="album-button"
        disabled={busy || stale}
        aria-label={`Play ${item.name}`}
        on:click={() => onPlay(item)}
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

<style>
  .album-strip {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(52px, 1fr));
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

  .album-button:disabled {
    cursor: default;
    opacity: 0.54;
  }

  .album-button:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 2px;
  }

  @container (max-height: 190px) {
    .album-strip {
      display: none;
    }
  }

  @container (max-height: 360px) {
    .album-strip {
      display: none;
    }
  }

  @container (min-height: 400px) {
    .album-strip {
      grid-template-columns: repeat(auto-fit, minmax(58px, 1fr));
    }
  }
</style>
