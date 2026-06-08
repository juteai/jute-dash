<script lang="ts">
  import { Cloud, CloudFog, CloudRain, CloudSnow, CloudSun, Moon, Sun, Wind } from 'lucide-svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import type { WeatherState } from '$lib/types';

  export let weather: WeatherState;
  export let stale = false;

  $: temperature = weather.temperature === null ? '—' : `${Math.round(weather.temperature)}${unitLabel(weather.temperatureUnit)}`;
  $: apparent = weather.apparentTemperature === null
    ? 'Feels unavailable'
    : `Feels ${Math.round(weather.apparentTemperature)}${unitLabel(weather.temperatureUnit)}`;
  $: wind = weather.windSpeed === null
    ? 'Wind unavailable'
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
</script>

<div class:widget-stale={stale} class={`weather-widget weather-widget--${weather.status}`}>
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
    <span>{apparent}</span>
    <span>{weather.humidity === null ? 'Humidity unavailable' : `${weather.humidity}% humidity`}</span>
    <span><Wind size={15} /> {wind}</span>
  </div>

  <div class="weather-footer">
    <Badge tone={weather.status === 'available' ? 'active' : 'warning'}>{weather.status}</Badge>
    <span>{stale ? 'stale' : weather.source}</span>
  </div>
</div>

<style>
  .weather-widget {
    display: flex;
    flex-direction: column;
    min-height: 100%;
    gap: 10px;
  }

  .weather-primary,
  .weather-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .weather-location {
    color: var(--foreground);
    font-size: var(--widget-value-size);
    font-weight: 760;
  }

  .weather-condition {
    margin-top: 4px;
    color: var(--muted);
    font-size: var(--widget-body-size);
  }

  .weather-icon {
    flex: 0 0 auto;
  }

  .weather-icon :global(svg) {
    width: clamp(20px, 16cqmin, 64px);
    height: auto;
  }

  .weather-meta :global(svg) {
    width: clamp(10px, 4cqmin, 20px);
    height: auto;
  }

  .weather-temp {
    font-size: var(--widget-display-size);
    font-weight: 780;
    line-height: 1;
  }

  .weather-meta {
    align-items: stretch;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 640;
  }

  .weather-meta span {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .weather-footer {
    margin-top: auto;
    color: var(--muted);
    font-size: var(--widget-label-size);
  }

  :global(.widget-frame--wide) .weather-widget {
    justify-content: center;
  }

  :global(.widget-frame--wide) .weather-primary {
    min-height: 54px;
  }

  :global(.widget-frame--wide) .weather-temp,
  :global(.widget-frame--wide) .weather-meta,
  :global(.widget-frame--wide) .weather-footer {
    display: none;
  }

  .weather-widget--unavailable,
  .weather-widget--disabled {
    color: var(--muted-strong);
  }

  .widget-stale {
    opacity: 0.72;
  }

  @media (max-width: 640px) {
    .weather-icon :global(svg) {
      width: 42px;
    }

    .weather-meta :global(svg) {
      width: 15px;
    }

    .weather-meta {
      grid-template-columns: 1fr;
    }
  }
</style>
