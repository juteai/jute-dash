<script lang="ts">
  export let motion: string = 'full';
  export let properties: {
    particleCount?: number;
    'particle-count'?: number;
    speed?: number;
    maxOpacity?: number;
    'max-opacity'?: number;
    color?: string;
  } = {};

  $: particleCount =
    properties.particleCount ?? properties['particle-count'] ?? 45;
  $: speed =
    motion === 'reduced'
      ? 0.2
      : motion === 'none'
        ? 0
        : (properties.speed ?? 0.5);
  $: baseMaxOpacity = properties.maxOpacity ?? properties['max-opacity'] ?? 0.6;
  $: particleColor = properties.color
    ? `var(${properties.color})`
    : 'var(--active)';

  interface Particle {
    size: number;
    left: number;
    duration: number;
    delay: number;
    driftX: number;
    maxOpacity: number;
    scale: number;
  }

  // Generate particles deterministically based on count
  $: particles = Array.from({ length: particleCount }, (_, i) => {
    const baseDuration = 15 + ((i * 17) % 20); // slower default drift for dashboard
    const duration = speed > 0 ? baseDuration / speed : 0;
    const delay = speed > 0 ? -(((i * 11) % baseDuration) / speed) : 0;
    return {
      size: 3 + ((i * 7) % 6), // slightly larger particles
      left: ((i * 23) % 96) + 2,
      duration,
      delay,
      driftX: -30 + ((i * 31) % 60),
      maxOpacity: (0.2 + ((i * 2) % 4) * 0.15) * baseMaxOpacity * 1.5, // higher opacity range
      scale: 0.7 + ((i * 7) % 5) * 0.15
    };
  });
</script>

{#if motion !== 'none'}
  <div
    class="stardust-canvas-bg"
    style="--stardust-color: {particleColor};"
  >
    {#each particles as p, i (i)}
      <div
        class="stardust-particle-bg"
        style="
          --left: {p.left}%;
          --duration: {p.duration}s;
          --delay: {p.delay}s;
          --drift-x: {p.driftX}px;
          --scale: {p.scale};
        "
      >
        <div
          class="stardust-particle-inner"
          style="
            width: {p.size}px;
            height: {p.size}px;
            opacity: {p.maxOpacity};
          "
        ></div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .stardust-canvas-bg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    overflow: hidden;
    z-index: 0;
    opacity: 0.9;
  }

  .stardust-particle-bg {
    position: absolute;
    bottom: -15px;
    left: var(--left);
    opacity: 0;
    animation: stardust-drift-bg var(--duration) linear infinite;
    animation-delay: var(--delay);
    will-change: transform;
  }

  .stardust-particle-inner {
    border-radius: 50%;
    background: var(--stardust-color, var(--active));
    filter: drop-shadow(0 0 6px var(--stardust-color, var(--active)));
  }

  @keyframes stardust-drift-bg {
    0% {
      transform: translate3d(0, 0, 0) scale(0.5);
      opacity: 0;
    }
    10% {
      opacity: 1;
    }
    90% {
      opacity: 1;
    }
    100% {
      transform: translate3d(var(--drift-x, 20px), -105vh, 0)
        scale(var(--scale, 1));
      opacity: 0;
    }
  }
</style>
