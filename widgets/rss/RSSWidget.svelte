<script lang="ts">
  import { Rss } from 'lucide-svelte';

  export let data: any = null;
  export let stale = false;

  $: feeds = Array.isArray(data) ? data : [];
</script>

<div class="rss-widget flex flex-col h-full w-full" class:stale>
  {#if feeds.length === 0}
    <div class="flex-1 flex flex-col items-center justify-center text-muted-foreground p-4">
      <Rss class="w-8 h-8 mb-2 opacity-30" />
      <span class="text-sm">No feeds configured</span>
    </div>
  {:else}
    <div class="flex-1 overflow-y-auto space-y-4 pr-1 select-none">
      {#each feeds as feed}
        <div class="feed-section space-y-2">
          <div class="flex items-center space-x-2 border-b border-neutral-200 dark:border-neutral-800 pb-1">
            <Rss class="w-3.5 h-3.5 text-neutral-400" />
            <h4 class="text-xs font-semibold uppercase tracking-wider text-neutral-500 dark:text-neutral-400">
              {feed.feedName}
            </h4>
          </div>
          <ul class="space-y-2">
            {#each feed.items || [] as item}
              <li class="group">
                <a
                  href={item.link}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="block text-sm font-medium leading-snug text-neutral-800 dark:text-neutral-200 group-hover:text-teal-600 dark:group-hover:text-teal-400 transition-colors"
                >
                  {item.title}
                </a>
              </li>
            {:else}
              <li class="text-xs text-muted-foreground italic">No articles found</li>
            {/each}
          </ul>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .rss-widget {
    height: 100%;
    overflow: hidden;
  }
  .stale {
    opacity: 0.6;
    pointer-events: none;
  }
</style>
