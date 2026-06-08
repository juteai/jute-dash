/* eslint-disable @typescript-eslint/no-explicit-any */
import StardustBackground from '$backgrounds/stardust/Stardust.svelte';
import WeatherAmbientBackground from '$backgrounds/weather-ambient/WeatherAmbient.svelte';

export const backgroundRegistry: Record<string, any> = {
  stardust: StardustBackground,
  'weather-ambient': WeatherAmbientBackground
};
