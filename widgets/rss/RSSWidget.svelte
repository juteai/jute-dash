<script lang="ts">
  import { Rss } from 'lucide-svelte';

  export let data: any = null;
  export let stale = false;

  $: feeds = Array.isArray(data) ? data : [];
</script>

<div class="rss-widget" class:stale>
  {#if feeds.length === 0}
    <div class="rss-empty">
      <Rss class="empty-icon" size={32} />
      <span>No feeds configured</span>
    </div>
  {:else}
    <div class="feeds-list">
      {#each feeds as feed}
        <div class="feed-section">
          <div class="feed-header">
            <Rss class="feed-icon" size={14} />
            <h4 class="feed-title">{feed.feedName}</h4>
          </div>
          <ul class="feed-articles">
            {#each feed.items || [] as item}
              <li class="article-item">
                <a
                  href={item.link}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="article-link"
                >
                  {item.title}
                </a>
              </li>
            {:else}
              <li class="rss-no-data">No articles found</li>
            {/each}
          </ul>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .rss-widget {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 100%;
    overflow: hidden;
  }

  .stale {
    opacity: 0.6;
    pointer-events: none;
  }

  .rss-empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--muted);
    padding: clamp(8px, 4cqmin, 16px);
  }

  :global(.rss-widget .empty-icon) {
    margin-bottom: 8px;
    opacity: 0.3;
  }

  .rss-empty span {
    font-size: var(--widget-body-size);
  }

  .feeds-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: clamp(12px, 4cqmin, 20px);
    padding-right: 4px;
    user-select: none;
  }

  .feed-section {
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2cqmin, 10px);
  }

  .feed-header {
    display: flex;
    align-items: center;
    gap: 8px;
    border-bottom: 1px solid var(--border);
    padding-bottom: 4px;
  }

  :global(.rss-widget .feed-icon) {
    color: var(--muted);
    opacity: 0.8;
  }

  .feed-title {
    margin: 0;
    font-size: clamp(0.65rem, 4cqmin, 0.8rem);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .feed-articles {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2.5cqmin, 12px);
  }

  .article-item {
    display: block;
  }

  .article-link {
    display: block;
    font-size: var(--widget-body-size);
    font-weight: 500;
    line-height: 1.4;
    color: var(--foreground);
    text-decoration: none;
    transition: color 0.2s ease;
  }

  .article-link:hover {
    color: var(--active);
  }

  .rss-no-data {
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    color: var(--muted);
    font-style: italic;
  }

  :global(.widget-frame--medium) .rss-widget {
    font-size: var(--widget-body-size);
  }

  :global(.widget-frame--medium) .rss-widget h4 {
    font-size: 0.72rem;
  }

  :global(.widget-frame--medium) .rss-widget a {
    font-size: 0.85rem;
  }
</style>
