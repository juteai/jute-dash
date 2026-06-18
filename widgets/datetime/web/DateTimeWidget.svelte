<script lang="ts">
  import { onMount } from 'svelte';

  export let settings: { timezone: string; locale: string; style?: 'digital' | 'analog' } = {
    timezone: 'UTC',
    locale: 'en',
    style: 'digital'
  };
  export let stale = false;

  let now = new Date();
  let hrAngle = 0;
  let minAngle = 0;
  let secAngle = 0;

  function updateClock() {
    now = new Date();
    const timeZone = settings.timezone || 'UTC';
    try {
      const timeString = now.toLocaleTimeString('en-US', {
        timeZone,
        hour12: false
      });
      const parts = timeString.split(':').map((s) => parseInt(s, 10) || 0);
      const hourVal = parts[0] || 0;
      const minuteVal = parts[1] || 0;
      const secondVal = parts[2] || 0;

      hrAngle = (hourVal % 12) * 30 + minuteVal * 0.5;
      minAngle = minuteVal * 6 + secondVal * 0.1;
      secAngle = secondVal * 6;
    } catch (e) {
      console.error('Failed to parse timezone clock:', e);
      // Failsafe fallback to UTC
      const hourVal = now.getUTCHours();
      const minuteVal = now.getUTCMinutes();
      const secondVal = now.getUTCSeconds();
      hrAngle = (hourVal % 12) * 30 + minuteVal * 0.5;
      minAngle = minuteVal * 6 + secondVal * 0.1;
      secAngle = secondVal * 6;
    }
  }

  $: timezoneLabel = settings.timezone || 'UTC';
  $: activeStyle = settings.style || 'digital';

  $: if (settings) {
    updateClock();
  }

  onMount(() => {
    updateClock();
    const timer = window.setInterval(updateClock, 1000);
    return () => window.clearInterval(timer);
  });

  $: time = new Intl.DateTimeFormat(settings.locale || 'en', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: settings.timezone || 'UTC'
  }).format(now);

  $: date = new Intl.DateTimeFormat(settings.locale || 'en', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    timeZone: settings.timezone || 'UTC'
  }).format(now);
</script>

<div class:widget-stale={stale} class="date-time-widget {activeStyle}-style">
  {#if activeStyle === 'analog'}
    <div class="analog-clock-container">
      <svg viewBox="0 0 100 100" class="analog-clock">
        <circle cx="50" cy="50" r="48" class="clock-face" />

        <!-- Hour Ticks -->
        {#each Array(12) as _, i}
          {@const angle = i * 30}
          {@const isMajor = i % 3 === 0}
          <line
            x1="50"
            y1={isMajor ? 6 : 8}
            x2="50"
            y2={isMajor ? 11 : 11}
            class="tick-line"
            class:major-tick={isMajor}
            transform="rotate({angle} 50 50)"
          />
        {/each}

        <!-- Clock Hands -->
        <line
          x1="50"
          y1="50"
          x2="50"
          y2="24"
          class="hand hour-hand"
          stroke="var(--foreground)"
          stroke-width="3"
          stroke-linecap="round"
          transform="rotate({hrAngle || 0} 50 50)"
        />
        <line
          x1="50"
          y1="50"
          x2="50"
          y2="14"
          class="hand minute-hand"
          stroke="var(--foreground)"
          stroke-width="2"
          stroke-linecap="round"
          opacity="0.9"
          transform="rotate({minAngle || 0} 50 50)"
        />
        <line
          x1="50"
          y1="50"
          x2="50"
          y2="10"
          class="hand second-hand"
          stroke="var(--danger)"
          stroke-width="1"
          stroke-linecap="round"
          transform="rotate({secAngle || 0} 50 50)"
        />
        <circle cx="50" cy="50" r="2.5" class="center-pin" />
      </svg>
    </div>

    <div class="analog-metadata">
      <div class="date-time-date">{date}</div>
      <div class="date-time-zone">{timezoneLabel}</div>
    </div>
  {:else}
    <div class="digital-container">
      <div class="date-time-zone">{timezoneLabel}</div>
      <div class="date-time-clock">{time}</div>
      <div class="date-time-date">{date}</div>
    </div>
  {/if}

  {#if stale}
    <div class="widget-state-note">Showing last hub state</div>
  {/if}
</div>

<style>
  .date-time-widget {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    gap: clamp(6px, 2cqmin, 12px);
    text-align: center;
    user-select: none;
  }

  /* Digital Layout */
  .digital-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: clamp(4px, 1.5cqmin, 8px);
    width: 100%;
  }

  .digital-style .date-time-clock {
    font-size: var(--widget-display-size, 3rem);
    font-weight: 800;
    line-height: 1;
    color: var(--foreground);
    letter-spacing: -0.02em;
  }

  .digital-style .date-time-date {
    font-size: var(--widget-value-size, 1.1rem);
    font-weight: 600;
    color: var(--foreground);
    opacity: 0.85;
  }

  .digital-style .date-time-zone {
    font-size: var(--widget-label-size, 0.72rem);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }

  /* Analog Layout */
  .analog-style.date-time-widget {
    justify-content: center;
    gap: clamp(8px, 4cqmin, 18px);
    padding: clamp(8px, 4cqmin, 16px);
  }

  .analog-clock-container {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 0;
  }

  .analog-clock {
    width: clamp(90px, 56cqmin, 220px);
    height: clamp(90px, 56cqmin, 220px);
    aspect-ratio: 1;
  }

  .clock-face {
    fill: var(--surface-muted, rgba(255, 255, 255, 0.02));
    stroke: var(--border);
    stroke-width: 1.5;
  }

  .tick-line {
    stroke: var(--muted);
    stroke-width: 1;
    stroke-linecap: round;
  }

  .major-tick {
    stroke: var(--foreground);
    stroke-width: 1.5;
    opacity: 0.8;
  }

  /* CSS handles standard centering; hands geometry and colors are specified inline on the SVG lines to prevent rendering bugs */

  .center-pin {
    fill: var(--danger);
    stroke: var(--surface-muted);
    stroke-width: 0.5;
  }

  .analog-metadata {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }

  .analog-metadata .date-time-date {
    font-size: clamp(0.75rem, 4cqmin, 1.05rem);
    font-weight: 600;
    color: var(--foreground);
    line-height: 1.2;
  }

  .analog-metadata .date-time-zone {
    font-size: clamp(0.6rem, 3.2cqmin, 0.78rem);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }

  /* Stale display */
  .widget-stale {
    opacity: 0.64;
  }

  .widget-state-note {
    color: var(--warning);
    font-size: 0.7rem;
    font-weight: 720;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
</style>
