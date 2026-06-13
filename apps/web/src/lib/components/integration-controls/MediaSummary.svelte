<script lang="ts">
  export let title = 'Not Playing';
  export let subtitle = 'Unknown';
</script>

<div class="media-summary">
  <p class="media-summary-title">{title}</p>
  <p class="media-summary-subtitle">{subtitle}</p>
</div>

<style>
  .media-summary {
    display: grid;
    min-width: 0;
    gap: 3px;
    text-align: center;
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
