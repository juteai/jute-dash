<script lang="ts">
  export let title = 'Not Playing';
  export let subtitle = 'Unknown';
  export let imageUrl = '';
  export let imageAlt = '';
</script>

<div class:media-summary--with-art={imageUrl} class="media-summary">
  {#if imageUrl}
    <img
      class="media-summary-art"
      src={imageUrl}
      alt={imageAlt || `${title} artwork`}
      loading="lazy"
      referrerpolicy="no-referrer"
    />
  {/if}
  <div class="media-summary-copy">
    <p class="media-summary-title">{title}</p>
    <p class="media-summary-subtitle">{subtitle}</p>
  </div>
</div>

<style>
  .media-summary {
    display: flex;
    align-items: center;
    justify-content: center;
    min-width: 0;
    gap: clamp(8px, 3cqmin, 12px);
    text-align: center;
  }

  .media-summary--with-art {
    text-align: left;
  }

  .media-summary-art {
    width: clamp(44px, 18cqmin, 76px);
    height: clamp(44px, 18cqmin, 76px);
    flex: 0 0 auto;
    border: 1px solid var(--border);
    border-radius: 8px;
    object-fit: cover;
    background: var(--surface-muted);
  }

  .media-summary-copy {
    display: grid;
    min-width: 0;
    gap: 3px;
  }

  p {
    margin: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .media-summary-title {
    color: var(--foreground);
    font-size: var(--widget-body-size);
    font-weight: 740;
  }

  .media-summary-subtitle {
    color: var(--muted);
    font-size: var(--widget-label-size);
  }
</style>
