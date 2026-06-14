<script lang="ts">
  import {
    Cloud,
    CloudFog,
    CloudRain,
    CloudSnow,
    CloudSun,
    Moon,
    Sun,
    Wind,
    Sunrise,
    Sunset,
  } from "lucide-svelte";
  import {
    WidgetListItem,
    WidgetMeta,
    WidgetStack,
    WidgetValue,
    WidgetValueRow,
  } from "$lib/components/widget-content";
  import type { WeatherState } from "$lib/types";

  export let weather: WeatherState;
  export let stale = false;

  $: temperature =
    weather.temperature === null
      ? "—"
      : `${Math.round(weather.temperature)}${unitLabel(weather.temperatureUnit)}`;
  $: apparent =
    weather.apparentTemperature === null
      ? "—"
      : `${Math.round(weather.apparentTemperature)}${unitLabel(weather.temperatureUnit)}`;
  $: wind =
    weather.windSpeed === null
      ? "—"
      : `${Math.round(weather.windSpeed)} ${windUnitLabel(weather.windSpeedUnit)}`;

  function unitLabel(unit: string) {
    if (unit === "fahrenheit") {
      return "°F";
    }
    return "°C";
  }

  function windUnitLabel(unit: string) {
    if (unit === "mph") {
      return "mph";
    }
    if (unit === "ms") {
      return "m/s";
    }
    if (unit === "kn") {
      return "kn";
    }
    return "km/h";
  }

  function formatTime(isoStr: string) {
    if (!isoStr) return "";
    try {
      const date = new Date(isoStr);
      if (isNaN(date.getTime())) return "";
      return new Intl.DateTimeFormat(undefined, {
        hour: "numeric",
        minute: "2-digit",
        hour12: true,
      }).format(date);
    } catch {
      return "";
    }
  }
</script>

<WidgetStack {stale}>
  <div class="weather-primary">
    <div>
      <div class="weather-location">{weather.locationName}</div>
      <div class="weather-condition">{weather.condition}</div>
    </div>

    <div class="weather-icon" aria-hidden="true">
      {#if weather.icon === "sun"}
        <Sun size={42} />
      {:else if weather.icon === "moon"}
        <Moon size={42} />
      {:else if weather.icon === "rain"}
        <CloudRain size={42} />
      {:else if weather.icon === "snow"}
        <CloudSnow size={42} />
      {:else if weather.icon === "fog"}
        <CloudFog size={42} />
      {:else if weather.icon === "cloud-sun"}
        <CloudSun size={42} />
      {:else}
        <Cloud size={42} />
      {/if}
    </div>
  </div>

  <div class="weather-temp">{temperature}</div>

  <WidgetValueRow columns={3}>
    <WidgetValue label="Feels Like" value={apparent} />
    <WidgetValue
      label="Humidity"
      value={weather.humidity === null ? "—" : `${weather.humidity}%`}
    />
    <WidgetValue label="Wind" value={wind}>
      <Wind size={13} />
      {wind}
    </WidgetValue>
  </WidgetValueRow>

  {#if weather.sunrise || weather.sunset}
    <WidgetListItem>
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
    </WidgetListItem>
  {/if}

  <div class="weather-footer">
    <span class="weather-source">Provider: {weather.source}</span>
    {#if stale}
      <WidgetMeta separator={false}
        ><span class="weather-stale-text">stale</span></WidgetMeta
      >
    {/if}
  </div>
</WidgetStack>

<style>
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
</style>
