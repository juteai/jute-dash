<script lang="ts">
  /* eslint-disable no-useless-assignment */
  import Button from '$lib/components/ui/Button.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import type { HouseholdSettings } from '$lib/types';
  import { numeric } from '$lib/utils';

  let draft: HouseholdSettings | undefined;
  let lastJSON = '';

  $: if ($settingsStore.householdSettings) {
    const currentJSON = JSON.stringify($settingsStore.householdSettings);
    if (currentJSON !== lastJSON) {
      draft = structuredClone($settingsStore.householdSettings);
      lastJSON = currentJSON;
    }
  }

  async function save() {
    if (!draft || $settingsStore.saving) {
      return;
    }
    try {
      await settingsStore.saveHousehold(draft);
    } catch {
      // Error is set in settingsStore.issue
    }
  }
</script>

{#if draft}
  <div class="settings-form-grid">
    <label>
      <span>Home name</span>
      <input bind:value={draft.home.name} />
    </label>
    <label>
      <span>Timezone</span>
      <input bind:value={draft.home.timezone} placeholder="Europe/London" />
    </label>
    <label>
      <span>Locale</span>
      <input bind:value={draft.home.locale} placeholder="en-GB" />
    </label>
    <label class="settings-checkbox">
      <span>Weather</span>
      <input type="checkbox" bind:checked={draft.weather.enabled} />
    </label>
    <label>
      <span>Weather location</span>
      <input bind:value={draft.weather.locationName} />
    </label>
    <label>
      <span>Latitude</span>
      <input
        type="number"
        step="0.0001"
        value={draft.weather.latitude}
        on:input={(event) =>
          draft &&
          (draft.weather.latitude = numeric(event.currentTarget.value))}
      />
    </label>
    <label>
      <span>Longitude</span>
      <input
        type="number"
        step="0.0001"
        value={draft.weather.longitude}
        on:input={(event) =>
          draft &&
          (draft.weather.longitude = numeric(event.currentTarget.value))}
      />
    </label>
    <label>
      <span>Temperature unit</span>
      <select bind:value={draft.weather.temperatureUnit}>
        <option value="celsius">Celsius</option>
        <option value="fahrenheit">Fahrenheit</option>
      </select>
    </label>
    <label>
      <span>Wind speed unit</span>
      <select bind:value={draft.weather.windSpeedUnit}>
        <option value="kmh">km/h</option>
        <option value="mph">mph</option>
        <option value="ms">m/s</option>
        <option value="kn">knots</option>
      </select>
    </label>
  </div>
  <div class="settings-actions">
    <Button on:click={save} disabled={$settingsStore.saving}
      >{$settingsStore.saving ? 'Saving' : 'Save household'}</Button
    >
  </div>
{:else}
  <p class="settings-empty">Household settings are loading.</p>
{/if}
