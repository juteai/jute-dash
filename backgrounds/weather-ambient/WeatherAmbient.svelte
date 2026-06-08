<script lang="ts">
  export let motion: string = 'full';
  export let weatherData: any = null;
  export let properties: {
    speed?: number;
    maxOpacity?: number;
    'max-opacity'?: number;
  } = {};

  $: condition =
    weatherData?.status === 'available' ? weatherData.icon || 'cloud' : 'default';
  $: isDay = weatherData?.isDay ?? true;

  // Determine colors and particles based on weather condition
  interface WeatherStyle {
    colors: [string, string, string];
    speedMultiplier: number;
    particleCount: number;
    particleType: 'snow' | 'rain' | 'float' | 'mist';
    particleColor: string;
    opacity: number;
  }

  $: style = getStyle(condition, isDay);

  function getStyle(cond: string, day: boolean): WeatherStyle {
    const baseOpacity = properties.maxOpacity ?? properties['max-opacity'] ?? 0.6;

    switch (cond) {
      case 'sun':
        return {
          colors: ['#ffe0b2', '#ffb74d', '#ffa726'],
          speedMultiplier: 0.8,
          particleCount: 25,
          particleType: 'float',
          particleColor: 'rgba(255, 183, 77, 0.7)',
          opacity: baseOpacity * 0.65
        };
      case 'moon':
        return {
          colors: ['#0d1b2a', '#1b263b', '#415a77'],
          speedMultiplier: 0.5,
          particleCount: 30,
          particleType: 'float',
          particleColor: 'rgba(224, 225, 221, 0.8)',
          opacity: baseOpacity * 0.8
        };
      case 'rain':
        return {
          colors: ['#2b3d52', '#1a2535', '#0f172a'],
          speedMultiplier: 2.2,
          particleCount: 60,
          particleType: 'rain',
          particleColor: 'rgba(155, 205, 235, 0.75)',
          opacity: baseOpacity * 0.9
        };
      case 'snow':
        return {
          colors: ['#e2e8f0', '#cbd5e1', '#94a3b8'],
          speedMultiplier: 0.6,
          particleCount: 45,
          particleType: 'snow',
          particleColor: 'rgba(255, 255, 255, 0.95)',
          opacity: baseOpacity * 0.75
        };
      case 'fog':
        return {
          colors: ['#475569', '#334155', '#1e293b'],
          speedMultiplier: 0.3,
          particleCount: 25,
          particleType: 'mist',
          particleColor: 'rgba(203, 213, 225, 0.4)',
          opacity: baseOpacity * 0.6
        };
      case 'cloud-sun':
      case 'cloud':
      default:
        // Use soft theme blending by mixing active colors
        return {
          colors: [
            'var(--surface-muted)',
            'color-mix(in srgb, var(--active) 8%, var(--surface-muted))',
            'var(--surface-strong)'
          ],
          speedMultiplier: 0.7,
          particleCount: 25,
          particleType: 'float',
          particleColor: 'color-mix(in srgb, var(--active) 50%, transparent)',
          opacity: baseOpacity * 0.7
        };
    }
  }

  // Generate particles based on style
  $: speed =
    motion === 'reduced'
      ? 0.15
      : motion === 'none'
        ? 0
        : (properties.speed ?? 0.4) * style.speedMultiplier;

  $: particles = Array.from({ length: style.particleCount }, (_, i) => {
    const baseDuration =
      style.particleType === 'rain'
        ? 0.8 + ((i * 3) % 5) * 0.2 // fast rain drops
        : style.particleType === 'snow'
          ? 8 + ((i * 7) % 6) * 1.5 // slow snow flutter
          : 15 + ((i * 13) % 15); // gentle floating float/mist

    const duration = speed > 0 ? baseDuration / speed : 0;
    const delay = speed > 0 ? -(((i * 11) % baseDuration) / speed) : 0;

    return {
      size:
        style.particleType === 'rain'
          ? 2.0 // slightly thicker rain lines
          : style.particleType === 'snow'
            ? 3 + ((i * 3) % 4) // larger fluffy snow flakes
            : 5 + ((i * 7) % 8), // larger mist/glow circles
      left: ((i * 27) % 96) + 2,
      duration,
      delay,
      driftX:
        style.particleType === 'rain'
          ? -10 + ((i * 3) % 20) // straight rain down with tiny wind drift
          : style.particleType === 'snow'
            ? -25 + ((i * 19) % 50) // snowy sway
            : -40 + ((i * 23) % 80), // normal drift
      maxOpacity:
        style.particleType === 'rain'
          ? 0.5 + ((i * 2) % 3) * 0.15
          : 0.3 + ((i * 3) % 4) * 0.15,
      scale: 0.8 + ((i * 5) % 5) * 0.15
    };
  });
</script>

{#if motion !== 'none'}
  <div
    class="weather-ambient-container"
    style="--weather-opacity: {style.opacity};"
  >
    <!-- Dynamic gradient backdrop -->
    <div
      class="weather-gradient"
      style="
        --color-1: {style.colors[0]};
        --color-2: {style.colors[1]};
        --color-3: {style.colors[2]};
        --anim-duration: {motion === 'reduced' ? '60s' : '24s'};
      "
    ></div>

    <!-- Weather particles -->
    <div
      class="weather-particles"
      style="--particle-color: {style.particleColor};"
    >
      {#each particles as p, i (i)}
        <div
          class="weather-particle weather-particle--{style.particleType}"
          style="
            --left: {p.left}%;
            --duration: {p.duration}s;
            --delay: {p.delay}s;
            --drift-x: {p.driftX}px;
            --scale: {p.scale};
          "
        >
          <div
            class="weather-particle-inner weather-particle-inner--{style.particleType}"
            style="
              --size: {p.size}px;
              --max-opacity: {p.maxOpacity};
            "
          ></div>
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .weather-ambient-container {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    overflow: hidden;
    pointer-events: none;
    z-index: 0;
    opacity: var(--weather-opacity, 0.4);
    transition: opacity 1.5s ease;
  }

  .weather-gradient {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    background: linear-gradient(
      135deg,
      var(--color-1) 0%,
      var(--color-2) 50%,
      var(--color-3) 100%
    );
    background-size: 200% 200%;
    animation: gradient-flow var(--anim-duration, 24s) ease-in-out infinite
      alternate;
  }

  @keyframes gradient-flow {
    0% {
      background-position: 0% 0%;
    }
    50% {
      background-position: 100% 100%;
    }
    100% {
      background-position: 0% 0%;
    }
  }

  .weather-particles {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
  }

  .weather-particle {
    position: absolute;
    will-change: transform;
    opacity: 0;
  }

  .weather-particle-inner {
    width: var(--size);
    height: var(--size);
    opacity: var(--max-opacity, 0.4);
    background: var(--particle-color);
  }

  /* Floating particle / star style */
  .weather-particle--float {
    bottom: -15px;
    left: var(--left);
    animation: drift-up var(--duration) linear infinite;
    animation-delay: var(--delay);
  }

  .weather-particle-inner--float {
    border-radius: 50%;
    filter: drop-shadow(0 0 7px var(--particle-color));
  }

  /* Rain drop style */
  .weather-particle--rain {
    top: -30px;
    left: var(--left);
    animation: drift-down var(--duration) linear infinite;
    animation-delay: var(--delay);
  }

  .weather-particle-inner--rain {
    height: calc(var(--size) * 10);
    border-radius: 2px;
  }

  /* Snow flake style */
  .weather-particle--snow {
    top: -20px;
    left: var(--left);
    animation: drift-down var(--duration) linear infinite;
    animation-delay: var(--delay);
  }

  .weather-particle-inner--snow {
    border-radius: 50%;
    filter: drop-shadow(0 0 4px #ffffff);
    background: #ffffff;
  }

  /* Mist drift style */
  .weather-particle--mist {
    bottom: 5%;
    left: var(--left);
    animation: drift-horizontal var(--duration) linear infinite;
    animation-delay: var(--delay);
  }

  .weather-particle-inner--mist {
    width: calc(var(--size) * 15);
    height: calc(var(--size) * 5);
    border-radius: 40%;
    filter: blur(8px);
    opacity: calc(var(--max-opacity, 0.4) * 0.6);
  }

  @keyframes drift-up {
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
      transform: translate3d(var(--drift-x), -105vh, 0) scale(var(--scale));
      opacity: 0;
    }
  }

  @keyframes drift-down {
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
      transform: translate3d(var(--drift-x), 105vh, 0) scale(var(--scale));
      opacity: 0;
    }
  }

  @keyframes drift-horizontal {
    0% {
      transform: translate3d(-10vw, 0, 0);
      opacity: 0;
    }
    20% {
      opacity: 1;
    }
    80% {
      opacity: 1;
    }
    100% {
      transform: translate3d(110vw, var(--drift-x), 0);
      opacity: 0;
    }
  }
</style>
