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

<style>
  .settings-form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .settings-form-grid label {
    display: grid;
    gap: 6px;
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .settings-form-grid label span {
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 760;
  }

  .settings-form-grid input {
    min-width: 0;
    min-height: 42px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 10px;
  }

  .settings-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    justify-content: flex-end;
    margin-top: 12px;
  }

  .settings-empty {
    margin: 12px 0 0;
    line-height: 1.4;
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
  }

  @media (max-width: 640px) {
    .settings-form-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
