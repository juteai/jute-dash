<script lang="ts">
  import { TrendingUp, TrendingDown, DollarSign } from 'lucide-svelte';

  export let data: any = null;
  export let stale = false;
  export let onQueryAgent: ((symbol: string) => void) | undefined = undefined;

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

<div class="markets-widget" class:stale>
  {#if tickers.length === 0}
    <div class="markets-empty">
      <DollarSign class="empty-icon" size={32} />
      <span>No tickers configured</span>
    </div>
  {:else}
    <div class="tickers-list">
      {#each tickers as ticker}
        <button
          type="button"
          class="ticker-item"
          class:ticker-item--clickable={!!onQueryAgent}
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
        </button>
      {:else}
        <div class="markets-no-data">No tickers loaded</div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .markets-widget {
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

  .markets-empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--muted);
    padding: clamp(8px, 4cqmin, 16px);
  }

  :global(.markets-widget .empty-icon) {
    margin-bottom: 8px;
    opacity: 0.3;
  }

  .markets-empty span {
    font-size: var(--widget-body-size);
  }

  .tickers-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: clamp(6px, 3cqmin, 12px);
    padding-right: 4px;
    user-select: none;
  }

  .ticker-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: clamp(6px, 3cqmin, 10px);
    border-radius: 8px;
    background: var(--surface-muted);
    border: 1px solid var(--border);
    transition: all 0.2s ease;
    text-align: left;
    width: 100%;
    color: inherit;
    font: inherit;
  }

  .ticker-item--clickable {
    cursor: pointer;
  }

  .ticker-item--clickable:hover:not(:disabled) {
    transform: scale(1.01);
    border-color: var(--border-strong);
    background: var(--surface-strong);
  }

  .ticker-item:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: -1px;
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
