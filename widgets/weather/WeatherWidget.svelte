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
