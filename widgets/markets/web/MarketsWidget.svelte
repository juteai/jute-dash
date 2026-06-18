<script lang="ts">
  import { TrendingUp, TrendingDown, DollarSign } from "lucide-svelte";
  import {
    WidgetEmptyState,
    WidgetList,
    WidgetListItem,
    WidgetStack,
  } from "$lib/components/widget-content";

  export let data: any = null;
  export let stale = false;
  export let onQueryAgent: ((symbol: string) => void) | undefined = undefined;

  $: tickers = Array.isArray(data) ? data : [];

  function formatPrice(price: number, currency: string) {
    if (price == null) return "—";
    const formatter = new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: currency || "USD",
      minimumFractionDigits: price < 10 ? 4 : 2,
      maximumFractionDigits: price < 10 ? 4 : 2,
    });
    return formatter.format(price);
  }

  function formatPercent(pct: number) {
    if (pct == null) return "0.00%";
    const prefix = pct >= 0 ? "+" : "";
    return `${prefix}${pct.toFixed(2)}%`;
  }
</script>

<WidgetStack {stale}>
  {#if tickers.length === 0}
    <WidgetEmptyState message="No tickers configured">
      <DollarSign slot="icon" size={32} />
    </WidgetEmptyState>
  {:else}
    <WidgetList>
      {#each tickers as ticker}
        <WidgetListItem
          class="ticker-item"
          clickable={!!onQueryAgent}
          on:click={() => onQueryAgent?.(ticker.symbol)}
          disabled={stale}
        >
          <div class="ticker-info">
            <span class="ticker-symbol">{ticker.symbol}</span>
            <span class="ticker-name">{ticker.name}</span>
          </div>
          <div class="ticker-pricing">
            <div class="ticker-values">
              <span class="ticker-price">
                {formatPrice(ticker.price, ticker.currency)}
              </span>
              <span
                class="ticker-change"
                class:ticker-change--up={ticker.change >= 0}
                class:ticker-change--down={ticker.change < 0}
              >
                {formatPercent(ticker.changePercent)}
              </span>
            </div>
            <div
              class="ticker-trend-badge"
              class:ticker-trend-badge--up={ticker.change >= 0}
              class:ticker-trend-badge--down={ticker.change < 0}
            >
              {#if ticker.change >= 0}
                <TrendingUp size={16} />
              {:else}
                <TrendingDown size={16} />
              {/if}
            </div>
          </div>
        </WidgetListItem>
      {:else}
        <div class="markets-no-data">No tickers loaded</div>
      {/each}
    </WidgetList>
  {/if}
</WidgetStack>

<style>
  :global(.ticker-item) {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .ticker-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .ticker-symbol {
    font-size: var(--widget-label-size);
    font-weight: 700;
    letter-spacing: -0.01em;
    color: var(--foreground);
  }

  .ticker-name {
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    color: var(--muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .ticker-pricing {
    display: flex;
    align-items: center;
    gap: clamp(8px, 3cqmin, 12px);
    text-align: right;
  }

  .ticker-values {
    display: flex;
    flex-direction: column;
  }

  .ticker-price {
    font-size: var(--widget-label-size);
    font-weight: 600;
    color: var(--foreground);
  }

  .ticker-change {
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    font-weight: 500;
  }

  .ticker-change--up {
    color: var(--success);
  }

  .ticker-change--down {
    color: var(--danger);
  }

  .ticker-trend-badge {
    display: flex;
    align-items: center;
    justify-content: center;
    width: clamp(24px, 8cqmin, 30px);
    height: clamp(24px, 8cqmin, 30px);
    border-radius: 50%;
  }

  .ticker-trend-badge--up {
    background: color-mix(in srgb, var(--success) 10%, transparent);
    color: var(--success);
  }

  .ticker-trend-badge--down {
    background: color-mix(in srgb, var(--danger) 10%, transparent);
    color: var(--danger);
  }

  .markets-no-data {
    font-size: clamp(0.6rem, 4cqmin, 0.75rem);
    color: var(--muted);
    font-style: italic;
  }
</style>
