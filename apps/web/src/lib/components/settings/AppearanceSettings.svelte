<script lang="ts">
  /* eslint-disable no-useless-assignment */
  import { Trash2, Upload } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { themeOptions } from '$lib/themes';
  import { settingsStore } from '$lib/settingsStore';
  import { backgroundImageURL } from '$lib/hubClient';
  import type { HouseholdSettings } from '$lib/types';
  import { numeric } from '$lib/utils';

  let draft: HouseholdSettings | undefined;
  let lastJSON = '';

  $: if ($settingsStore.householdSettings) {
    const currentJSON = JSON.stringify($settingsStore.householdSettings);
    if (currentJSON !== lastJSON) {
      draft = structuredClone($settingsStore.householdSettings);
      if (!draft.display.background) {
        draft.display.background = {
          kind: 'theme',
          value: '',
          fit: 'cover',
          position: 'center',
          overlay: 'none'
        };
      }
      lastJSON = currentJSON;
    }
  }

  const BACKGROUND_KINDS = ['theme', 'color', 'file', 'slideshow'];

  function ensureBackground() {
    if (!draft) {
      return;
    }
    if (!draft.display.background) {
      draft.display.background = {
        kind: 'theme',
        value: '',
        fit: 'cover',
        position: 'center',
        overlay: 'none'
      };
    }
  }

  async function handleBackgroundUpload(event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) {
      return;
    }
    try {
      await settingsStore.uploadBackground(file);
    } catch {
      // Error is set in settingsStore.issue
    } finally {
      input.value = '';
    }
  }

  async function removeBackgroundImage(name: string) {
    try {
      await settingsStore.deleteBackground(name);
    } catch {
      // Error is set in settingsStore.issue
    }
  }

  function selectSingleBackground(name: string) {
    if (!draft) {
      return;
    }
    ensureBackground();
    draft.display.background.kind = 'file';
    draft.display.background.value = name;
    draft = draft;
  }

  function toggleSlideshowImage(name: string) {
    if (!draft) {
      return;
    }
    ensureBackground();
    draft.display.background.kind = 'slideshow';
    const images = draft.display.background.images ?? [];
    draft.display.background.images = images.includes(name)
      ? images.filter((image) => image !== name)
      : [...images, name];
    draft = draft;
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

{#if draft && draft.display.background}
  {@const bg = draft.display.background}
  <div class="settings-form-grid">
    <label>
      <span>Theme pack</span>
      <select bind:value={draft.display.themeId}>
        {#each themeOptions as option (option.id)}
          <option value={option.id}>{option.name}</option>
        {/each}
      </select>
    </label>
    <label>
      <span>Color mode</span>
      <select
        bind:value={draft.display.colorMode}
        on:change={() =>
          draft && (draft.display.theme = draft.display.colorMode)}
      >
        <option value="system">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
    <label>
      <span>Widget chrome</span>
      <select bind:value={draft.display.widgetChrome.default}>
        <option value="solid">Solid</option>
        <option value="clear">Clear</option>
        <option value="smoked">Smoked</option>
        <option value="frosted">Frosted</option>
        <option value="auto">Auto</option>
      </select>
    </label>
    <label>
      <span>Background</span>
      <select bind:value={draft.display.background.kind}>
        {#each BACKGROUND_KINDS as kind (kind)}
          <option value={kind}>{kind}</option>
        {/each}
      </select>
    </label>
    {#if bg.kind === 'color'}
      <label>
        <span>Color value</span>
        <input
          bind:value={draft.display.background.value}
          placeholder="#101010"
        />
      </label>
    {/if}
    <label>
      <span>Fit</span>
      <select bind:value={draft.display.background.fit}>
        <option value="cover">Cover</option>
        <option value="contain">Contain</option>
        <option value="tile">Tile</option>
      </select>
    </label>
    <label>
      <span>Overlay</span>
      <select bind:value={draft.display.background.overlay}>
        <option value="none">None</option>
        <option value="dim">Dim</option>
        <option value="smoked">Smoked</option>
        <option value="frosted">Frosted</option>
      </select>
    </label>
    {#if bg.kind === 'slideshow'}
      <label>
        <span>Interval (seconds)</span>
        <input
          type="number"
          min="3"
          value={bg.intervalSeconds ?? 30}
          on:input={(event) =>
            draft &&
            (draft.display.background.intervalSeconds = numeric(
              event.currentTarget.value
            ))}
        />
      </label>
    {/if}
  </div>

  {#if bg.kind === 'file' || bg.kind === 'slideshow'}
    <div class="settings-actions">
      <label class="background-upload">
        <Upload size={15} />
        <span
          >{$settingsStore.uploadingBackground
            ? 'Uploading…'
            : 'Upload image'}</span
        >
        <input
          type="file"
          accept="image/*"
          hidden
          on:change={handleBackgroundUpload}
        />
      </label>
    </div>
    {#if $settingsStore.backgroundLibrary.length === 0}
      <p class="settings-empty">No background images uploaded yet.</p>
    {:else}
      <div class="background-grid">
        {#each $settingsStore.backgroundLibrary as image (image.name)}
          {@const selected =
            bg.kind === 'slideshow'
              ? (bg.images ?? []).includes(image.name)
              : bg.value === image.name}
          <div
            class="background-thumb"
            class:background-thumb--selected={selected}
          >
            <button
              type="button"
              class="background-thumb-pick"
              style={`background-image: url("${backgroundImageURL(
                image.name
              )}")`}
              aria-label={`Use ${image.name}`}
              on:click={() =>
                bg.kind === 'slideshow'
                  ? toggleSlideshowImage(image.name)
                  : selectSingleBackground(image.name)}
            >
              {#if selected}<span class="background-thumb-check">✓</span>{/if}
            </button>
            <button
              type="button"
              class="background-thumb-delete"
              aria-label={`Delete ${image.name}`}
              on:click={() => removeBackgroundImage(image.name)}
            >
              <Trash2 size={14} />
            </button>
          </div>
        {/each}
      </div>
    {/if}
  {/if}

  <div class="settings-actions">
    <Button on:click={save} disabled={$settingsStore.saving}
      >{$settingsStore.saving ? 'Saving' : 'Save appearance'}</Button
    >
  </div>
{:else}
  <p class="settings-empty">Appearance settings are loading.</p>
{/if}

<style>
  .background-upload {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 9px 14px;
    border-radius: 9px;
    border: 1px solid var(--border, rgba(255, 255, 255, 0.18));
    cursor: pointer;
    font-size: 0.86rem;
    min-height: 44px;
  }
  .background-upload:hover {
    background: var(--surface-strong, rgba(255, 255, 255, 0.06));
  }
  .background-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 10px;
    margin-top: 12px;
  }
  .background-thumb {
    position: relative;
    border-radius: 10px;
    overflow: hidden;
    border: 2px solid transparent;
  }
  .background-thumb--selected {
    border-color: var(--accent, #6366f1);
  }
  .background-thumb-pick {
    display: block;
    width: 100%;
    aspect-ratio: 16 / 10;
    background-size: cover;
    background-position: center;
    border: none;
    cursor: pointer;
    padding: 0;
  }
  .background-thumb-check {
    position: absolute;
    top: 6px;
    left: 6px;
    background: var(--accent, #6366f1);
    color: #fff;
    border-radius: 50%;
    width: 22px;
    height: 22px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
  }
  .background-thumb-delete {
    position: absolute;
    top: 6px;
    right: 6px;
    border: none;
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    border-radius: 6px;
    padding: 4px;
    cursor: pointer;
    display: inline-flex;
  }
</style>
