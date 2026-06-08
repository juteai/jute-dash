<script lang="ts">
  import { Rss } from 'lucide-svelte';

  export let data: any = null;
  export let stale = false;
  export let onQueryAgent: ((title: string, link: string) => void) | undefined = undefined;

  $: feeds = Array.isArray(data) ? data : [];

  function getDomain(urlStr: string): string {
    if (!urlStr) return '';
    try {
      return new URL(urlStr).hostname.replace(/^www\./, '');
    } catch {
      return '';
    }
  }

  function formatRelativeTime(dateStr: string): string {
    if (!dateStr) return '';
    try {
      const parsed = Date.parse(dateStr);
      if (isNaN(parsed)) return '';
      const now = Date.now();
      const diffMs = now - parsed;
      const diffSec = Math.floor(diffMs / 1000);
      const diffMin = Math.floor(diffSec / 60);
      const diffHour = Math.floor(diffMin / 60);
      const diffDay = Math.floor(diffHour / 24);

      if (diffSec < 60) {
        return 'just now';
      } else if (diffMin < 60) {
        return `${diffMin}m ago`;
      } else if (diffHour < 24) {
        return `${diffHour}h ago`;
      } else {
        return `${diffDay}d ago`;
      }
    } catch {
      return '';
    }
  }
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
          <div class="feed-articles">
            {#each feed.items || [] as item}
              <button
                type="button"
                class="article-item"
                class:article-item--clickable={!!onQueryAgent}
                on:click={() => onQueryAgent?.(item.title, item.link)}
                disabled={stale}
              >
                <span class="article-title">{item.title}</span>
                <span class="article-meta">
                  {#if getDomain(item.link)}
                    <span class="article-domain">{getDomain(item.link)}</span>
                  {/if}
                  {#if getDomain(item.link) && formatRelativeTime(item.pubDate)}
                    <span class="meta-separator">&middot;</span>
                  {/if}
                  {#if formatRelativeTime(item.pubDate)}
                    <span class="article-time">{formatRelativeTime(item.pubDate)}</span>
                  {/if}
                </span>
              </button>
            {:else}
              <div class="rss-no-data">No articles found</div>
            {/each}
          </div>
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
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2.5cqmin, 12px);
  }

  .article-item {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    padding: clamp(8px, 3cqmin, 12px);
    border-radius: 8px;
    background: var(--surface-muted);
    border: 1px solid var(--border);
    transition: all 0.2s ease;
    text-align: left;
    width: 100%;
    color: inherit;
    font: inherit;
  }

  .article-item--clickable {
    cursor: pointer;
  }

  .article-item--clickable:hover:not(:disabled) {
    transform: scale(1.01);
    border-color: var(--border-strong);
    background: var(--surface-strong);
  }

  .article-item:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: -1px;
  }

  .article-title {
    font-size: var(--widget-body-size);
    font-weight: 500;
    line-height: 1.4;
    color: var(--foreground);
    transition: color 0.2s ease;
  }

  .article-item--clickable:hover .article-title {
    color: var(--active);
  }

  .article-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    color: var(--muted);
  }

  .meta-separator {
    opacity: 0.5;
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

  :global(.widget-frame--medium) .rss-widget .article-title {
    font-size: 0.85rem;
  }
</style>
