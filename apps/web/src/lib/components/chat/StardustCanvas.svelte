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

<style>
  .stardust-canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    overflow: hidden;
    z-index: 1; /* Behind chat content, in front of background gradient */
    opacity: 0.35;
    transition: opacity 0.8s ease;
  }

  .stardust-particle {
    position: absolute;
    bottom: 0;
    width: var(--size, 4px);
    height: var(--size, 4px);
    border-radius: 50%;
    background: var(--stardust-color, var(--active));
    filter: drop-shadow(0 0 6px var(--stardust-color, var(--active)));
    left: var(--left);
    opacity: 0;
    animation: stardust-drift calc(var(--duration) / var(--stardust-speed, 1))
      linear infinite;
    animation-delay: var(--delay);
    will-change: transform;
  }

  /* State Modifiers for Stardust Canvas */
  .stardust-canvas--idle {
    --stardust-speed: 0.55;
    --stardust-color: var(--muted);
    opacity: 0.12;
  }

  .stardust-canvas--listening {
    --stardust-speed: 1;
    --stardust-color: var(--active);
    opacity: 0.45;
  }

  .stardust-canvas--thinking,
  .stardust-canvas--streaming {
    --stardust-speed: 2.3;
    --stardust-color: var(--focus);
    opacity: 0.6;
  }

  .stardust-canvas--error {
    --stardust-speed: 1.2;
    --stardust-color: var(--danger);
    opacity: 0.4;
  }

  @keyframes stardust-drift {
    0% {
      transform: translate3d(0, 15px, 0) scale(0.5);
      opacity: 0;
    }
    12% {
      opacity: var(--max-opacity, 0.5);
    }
    88% {
      opacity: var(--max-opacity, 0.5);
    }
    100% {
      transform: translate3d(var(--drift-x, 25px), -105vh, 0)
        scale(var(--scale, 1.1));
      opacity: 0;
    }
  }

  /* Reduced Motion preference */
  @media (prefers-reduced-motion: reduce) {
    .stardust-particle {
      animation: none !important;
      opacity: var(--max-opacity, 0.3) !important;
      transform: translate3d(0, calc(-50vh + var(--left) * 1px), 0)
        scale(var(--scale, 1)) !important;
    }
  }
</style>
