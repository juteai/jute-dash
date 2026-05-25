<script lang="ts">
  import { TrendingUp, TrendingDown, DollarSign } from 'lucide-svelte';

  export let data: any = null;
  export let stale = false;

  $: tickers = Array.isArray(data) ? data : [];

  function formatPrice(price: number, currency: string) {
    if (price == null) return '—';
    const formatter = new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: currency || 'USD',
      minimumFractionDigits: price < 10 ? 4 : 2,
      maximumFractionDigits: price < 10 ? 4 : 2
    });
    return formatter.format(price);
  }

  function formatPercent(pct: number) {
    if (pct == null) return '0.00%';
    const prefix = pct >= 0 ? '+' : '';
    return `${prefix}${pct.toFixed(2)}%`;
  }
</script>

<div class="markets-widget flex flex-col h-full w-full" class:stale>
  {#if tickers.length === 0}
    <div class="flex-1 flex flex-col items-center justify-center text-muted-foreground p-4">
      <DollarSign class="w-8 h-8 mb-2 opacity-30" />
      <span class="text-sm">No tickers configured</span>
    </div>
  {:else}
    <div class="flex-1 overflow-y-auto space-y-3 pr-1 select-none">
      {#each tickers as ticker}
        <div class="ticker-item flex items-center justify-between p-2 rounded-lg bg-neutral-50 dark:bg-neutral-900 border border-neutral-100 dark:border-neutral-800 transition-all hover:scale-[1.01]">
          <div class="flex flex-col min-w-0">
            <span class="text-sm font-bold tracking-tight text-neutral-800 dark:text-neutral-200">
              {ticker.symbol}
            </span>
            <span class="text-xs text-neutral-400 dark:text-neutral-500 truncate">
              {ticker.name}
            </span>
          </div>
          <div class="flex items-center space-x-3 text-right">
            <div class="flex flex-col">
              <span class="text-sm font-semibold text-neutral-800 dark:text-neutral-200">
                {formatPrice(ticker.price, ticker.currency)}
              </span>
              <span class="text-xs font-medium" class:text-green-600={ticker.change >= 0} class:text-red-500={ticker.change < 0}>
                {formatPercent(ticker.changePercent)}
              </span>
            </div>
            <div class="flex items-center justify-center w-7 h-7 rounded-full" class:bg-green-50={ticker.change >= 0} class:dark:bg-green-950={ticker.change >= 0} class:text-green-600={ticker.change >= 0} class:bg-red-50={ticker.change < 0} class:dark:bg-red-950={ticker.change < 0} class:text-red-500={ticker.change < 0}>
              {#if ticker.change >= 0}
                <TrendingUp class="w-4 h-4" />
              {:else}
                <TrendingDown class="w-4 h-4" />
              {/if}
            </div>
          </div>
        </div>
      {:else}
        <div class="text-xs text-muted-foreground italic">No tickers loaded</div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .markets-widget {
    height: 100%;
    overflow: hidden;
  }
  .stale {
    opacity: 0.6;
    pointer-events: none;
  }
</style>
