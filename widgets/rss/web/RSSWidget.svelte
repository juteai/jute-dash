<script lang="ts">
  import { Rss } from "lucide-svelte";
  import {
    WidgetEmptyState,
    WidgetList,
    WidgetListItem,
    WidgetMeta,
    WidgetSectionHeader,
    WidgetStack,
  } from "$lib/components/widget-content";

  export let data: any = null;
  export let stale = false;
  export let onQueryAgent: ((title: string, link: string) => void) | undefined =
    undefined;

  $: feeds = Array.isArray(data) ? data : [];

  function getDomain(urlStr: string): string {
    if (!urlStr) return "";
    try {
      return new URL(urlStr).hostname.replace(/^www\./, "");
    } catch {
      return "";
    }
  }

  function formatRelativeTime(dateStr: string): string {
    if (!dateStr) return "";
    try {
      const parsed = Date.parse(dateStr);
      if (isNaN(parsed)) return "";
      const now = Date.now();
      const diffMs = now - parsed;
      const diffSec = Math.floor(diffMs / 1000);
      const diffMin = Math.floor(diffSec / 60);
      const diffHour = Math.floor(diffMin / 60);
      const diffDay = Math.floor(diffHour / 24);

      if (diffSec < 60) {
        return "just now";
      } else if (diffMin < 60) {
        return `${diffMin}m ago`;
      } else if (diffHour < 24) {
        return `${diffHour}h ago`;
      } else {
        return `${diffDay}d ago`;
      }
    } catch {
      return "";
    }
  }
</script>

<WidgetStack {stale} gap="loose">
  {#if feeds.length === 0}
    <WidgetEmptyState message="No feeds configured">
      <Rss slot="icon" size={32} />
    </WidgetEmptyState>
  {:else}
    <WidgetList gap="loose">
      {#each feeds as feed}
        <section class="feed-section">
          <WidgetSectionHeader title={feed.feedName}>
            <Rss slot="icon" size={14} />
          </WidgetSectionHeader>
          <div class="feed-items">
            {#each feed.items || [] as item}
              <WidgetListItem
                direction="column"
                clickable={!!onQueryAgent}
                on:click={() => onQueryAgent?.(item.title, item.link)}
                disabled={stale}
              >
                <span class="article-title">{item.title}</span>
                <WidgetMeta>
                  {#if getDomain(item.link)}
                    <span>{getDomain(item.link)}</span>
                  {/if}
                  {#if formatRelativeTime(item.pubDate)}
                    <span>{formatRelativeTime(item.pubDate)}</span>
                  {/if}
                </WidgetMeta>
              </WidgetListItem>
            {:else}
              <div class="rss-no-data">No articles found</div>
            {/each}
          </div>
        </section>
      {/each}
    </WidgetList>
  {/if}
</WidgetStack>

<style>
  .feed-section {
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2cqmin, 10px);
  }

  .feed-items {
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 2.5cqmin, 12px);
  }

  .article-title {
    display: block;
    width: 100%;
    font-size: var(--widget-body-size);
    font-weight: 500;
    line-height: 1.4;
    color: var(--foreground);
  }

  .rss-no-data {
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    color: var(--muted);
    font-style: italic;
  }

  :global(.widget-list-item--clickable:hover) .article-title {
    color: var(--active);
  }
</style>
