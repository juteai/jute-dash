<script lang="ts">
  import { Cloud, CloudFog, CloudRain, CloudSnow, CloudSun, Moon, Sun, Wind, Sunrise, Sunset } from 'lucide-svelte';
  import type { WeatherState } from '$lib/types';

  export let weather: WeatherState;
  export let stale = false;

  $: temperature = weather.temperature === null ? '—' : `${Math.round(weather.temperature)}${unitLabel(weather.temperatureUnit)}`;
  $: apparent = weather.apparentTemperature === null
    ? '—'
    : `${Math.round(weather.apparentTemperature)}${unitLabel(weather.temperatureUnit)}`;
  $: wind = weather.windSpeed === null
    ? '—'
    : `${Math.round(weather.windSpeed)} ${windUnitLabel(weather.windSpeedUnit)}`;

  function unitLabel(unit: string) {
    if (unit === 'fahrenheit') {
      return '°F';
    }
    return '°C';
  }

  function windUnitLabel(unit: string) {
    if (unit === 'mph') {
      return 'mph';
    }
    if (unit === 'ms') {
      return 'm/s';
    }
    if (unit === 'kn') {
      return 'kn';
    }
    return 'km/h';
  }

  function formatTime(isoStr: string) {
    if (!isoStr) return '';
    try {
      const date = new Date(isoStr);
      if (isNaN(date.getTime())) return '';
      return new Intl.DateTimeFormat(undefined, {
        hour: 'numeric',
        minute: '2-digit',
        hour12: true
      }).format(date);
    } catch {
      return '';
    }
  }
</script>

<div class:widget-stale={stale} class="weather-widget">
  <div class="weather-primary">
    <div>
      <div class="weather-location">{weather.locationName}</div>
      <div class="weather-condition">{weather.condition}</div>
    </div>

    <div class="weather-icon" aria-hidden="true">
      {#if weather.icon === 'sun'}
        <Sun size={42} />
      {:else if weather.icon === 'moon'}
        <Moon size={42} />
      {:else if weather.icon === 'rain'}
        <CloudRain size={42} />
      {:else if weather.icon === 'snow'}
        <CloudSnow size={42} />
      {:else if weather.icon === 'fog'}
        <CloudFog size={42} />
      {:else if weather.icon === 'cloud-sun'}
        <CloudSun size={42} />
      {:else}
        <Cloud size={42} />
      {/if}
    </div>
  </div>

  <div class="weather-temp">{temperature}</div>

  <div class="weather-meta">
    <span class="meta-item">
      <span class="meta-label">Feels Like</span>
      <span class="meta-val">{apparent}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">Humidity</span>
      <span class="meta-val">{weather.humidity === null ? '—' : `${weather.humidity}%`}</span>
    </span>
    <span class="meta-item">
      <span class="meta-label">Wind</span>
      <span class="meta-val"><Wind size={13} /> {wind}</span>
    </span>
  </div>

  {#if weather.sunrise || weather.sunset}
    <div class="weather-sun-info">
      {#if weather.sunrise}
        <span class="sun-time-item">
          <Sunrise size={15} class="sun-icon-up" />
          <span class="sun-label">Sunrise</span>
          <span class="sun-val">{formatTime(weather.sunrise)}</span>
        </span>
      {/if}
      {#if weather.sunset}
        <span class="sun-time-item">
          <Sunset size={15} class="sun-icon-down" />
          <span class="sun-label">Sunset</span>
          <span class="sun-val">{formatTime(weather.sunset)}</span>
        </span>
      {/if}
    </div>
  {/if}

  <div class="weather-footer">
    <span class="weather-source">Provider: {weather.source}</span>
    {#if stale}
      <span class="weather-stale-text">stale</span>
    {/if}
  </div>
</div>

<style>
  .weather-widget {
    display: flex;
    flex-direction: column;
    height: 100%;
    width: 100%;
    gap: clamp(8px, 2.5cqmin, 14px);
    user-select: none;
  }

  .weather-primary {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
  }

  .weather-location {
    color: var(--foreground);
    font-size: var(--widget-value-size, 1.15rem);
    font-weight: 765;
    line-height: 1.2;
  }

  .weather-condition {
    margin-top: 2px;
    color: var(--muted);
    font-size: var(--widget-body-size, 0.88rem);
    font-weight: 500;
  }

  .weather-icon {
    flex: 0 0 auto;
    color: var(--foreground);
  }

  .weather-icon :global(svg) {
    width: clamp(24px, 14cqmin, 48px);
    height: auto;
  }

  .weather-temp {
    font-size: var(--widget-display-size, 3rem);
    font-weight: 800;
    line-height: 1;
    color: var(--foreground);
    letter-spacing: -0.02em;
  }

  .weather-meta {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: clamp(6px, 1.5cqmin, 12px);
    border-top: 1px dashed var(--border);
    border-bottom: 1px dashed var(--border);
    padding: clamp(6px, 2cqmin, 10px) 0;
  }

  .meta-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .meta-label {
    font-size: 0.65rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--muted);
  }

  .meta-val {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--foreground);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .meta-val :global(svg) {
    color: var(--muted);
    flex-shrink: 0;
  }

  .weather-sun-info {
    display: flex;
    align-items: center;
    gap: 20px;
    background: var(--surface-muted, rgba(255, 255, 255, 0.01));
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: clamp(6px, 1.8cqmin, 10px) clamp(8px, 2.2cqmin, 12px);
  }

  .sun-time-item {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.78rem;
  }

  :global(.weather-sun-info .sun-icon-up) {
    color: var(--active);
  }

  :global(.weather-sun-info .sun-icon-down) {
    color: var(--danger);
  }

  .sun-label {
    font-weight: 500;
    color: var(--muted);
  }

  .sun-val {
    font-weight: 650;
    color: var(--foreground);
  }

  .weather-footer {
    margin-top: auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.65rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--muted);
  }

  .weather-stale-text {
    color: var(--warning);
  }

  .widget-stale {
    opacity: 0.64;
  }
</style>
