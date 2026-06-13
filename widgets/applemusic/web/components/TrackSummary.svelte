<script lang="ts">
  export let title = 'Not Playing';
  export let artist = 'Unknown';
</script>

<div class="info">
  <p class="track">{title}</p>
  <p class="artist">{artist}</p>
</div>

<style>
  .track {
    font-weight: bold;
    font-size: 0.9rem;
  }
  .artist {
    color: var(--muted);
    font-size: 0.8rem;
  }
</style>
