<script lang="ts">
  import { Volume1, Volume2 } from 'lucide-svelte';

  export let volume = 50;
  export let disabled = false;
  export let onVolume: (volume: number) => Promise<void> | void = () => {};

  function handleChange(event: Event) {
    onVolume(parseInt((event.currentTarget as HTMLInputElement).value, 10));
  }
</script>

<label class="volume-control">
  <Volume1 size={14} />
  <input
    type="range"
    min="0"
    max="100"
    value={volume}
    {disabled}
    aria-label="Playback volume"
    on:change={handleChange}
  />
  <Volume2 size={14} />
</label>

<style>
  .volume-control {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--muted);
    font-size: var(--widget-label-size);
  }

  input {
    min-width: 0;
    flex: 1;
    accent-color: var(--active);
  }
</style>
