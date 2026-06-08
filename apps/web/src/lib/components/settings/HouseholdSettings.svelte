<script lang="ts">
  /* eslint-disable no-useless-assignment */
  import Button from '$lib/components/ui/Button.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import type { HouseholdSettings } from '$lib/types';

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
  </div>
  <div class="settings-actions">
    <Button on:click={save} disabled={$settingsStore.saving}
      >{$settingsStore.saving ? 'Saving' : 'Save household'}</Button
    >
  </div>
{:else}
  <p class="settings-empty">Household settings are loading.</p>
{/if}
