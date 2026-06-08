<script lang="ts">
  import type { ChatState } from '$lib/types';

  export let state: ChatState = 'idle';

  interface Particle {
    size: number;
    left: number;
    duration: number;
    delay: number;
    driftX: number;
    maxOpacity: number;
    scale: number;
  }

  const particles: Particle[] = Array.from({ length: 24 }, (_, i) => {
    const duration = 12 + ((i * 13) % 16);
    return {
      size: 3 + ((i * 7) % 6),
      left: ((i * 17) % 95) + 2.5,
      duration,
      delay: -((i * 9) % duration),
      driftX: -40 + ((i * 29) % 80),
      maxOpacity: 0.2 + ((i * 3) % 4) * 0.15,
      scale: 0.8 + ((i * 5) % 5) * 0.15
    };
  });
</script>

<div class="stardust-canvas stardust-canvas--{state}">
  {#each particles as p, i (i)}
    <div
      class="stardust-particle"
      style="
        --size: {p.size}px;
        --left: {p.left}%;
        --duration: {p.duration}s;
        --delay: {p.delay}s;
        --drift-x: {p.driftX}px;
        --max-opacity: {p.maxOpacity};
        --scale: {p.scale};
      "
    ></div>
  {/each}
</div>
