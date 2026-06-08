<script lang="ts">
  import { backgroundRegistry } from './background-registry';
  import type { DisplayBackground } from '$lib/types';

  export let backgroundConfig: DisplayBackground | undefined;
  export let motion: string = 'full';
  export let weatherData: unknown = null;

  $: activeBackgroundId =
    backgroundConfig?.kind === 'dynamic' ? backgroundConfig.value : null;
  $: backgroundComponent = activeBackgroundId
    ? backgroundRegistry[activeBackgroundId]
    : null;
  $: properties = backgroundConfig?.properties ?? {};

  $: {
    console.log('BackgroundRenderer config:', backgroundConfig);
    console.log('BackgroundRenderer activeBackgroundId:', activeBackgroundId);
    console.log('BackgroundRenderer component:', backgroundComponent);
  }
</script>

{#if backgroundComponent}
  <div class="dynamic-background-container" aria-hidden="true">
    <svelte:component
      this={backgroundComponent}
      {motion}
      {weatherData}
      {properties}
    />
  </div>
{/if}

<style>
  .dynamic-background-container {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    z-index: 0;
    pointer-events: none;
    overflow: hidden;
  }
</style>
