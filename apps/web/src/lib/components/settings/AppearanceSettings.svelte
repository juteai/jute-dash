<script lang="ts">
  /* eslint-disable no-useless-assignment */
  import { onDestroy } from 'svelte';
  import { Trash2, Upload } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import { themeOptions } from '$lib/themes';
  import { settingsStore } from '$lib/settingsStore';
  import { hubStream } from '$lib/hubStream';
  import { backgroundImageURL } from '$lib/hubClient';
  import type { HouseholdSettings } from '$lib/types';
  import { numeric } from '$lib/utils';

  let draft: HouseholdSettings | undefined;
  let lastJSON = '';
  let savedNotice = '';
  let savedNoticeTimer: number | undefined;

  $: draftJSON = draft ? JSON.stringify(draft) : '';
  $: hasChanges = !!draft && draftJSON !== lastJSON;
  $: if (hasChanges && savedNotice) {
    savedNotice = '';
  }

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
      if (!draft.display.widgetChrome) {
        draft.display.widgetChrome = {
          default: 'solid',
          smokedOpacity: 0.6,
          frostedOpacity: 0.3
        };
      }
      lastJSON = currentJSON;
    }
  }

  const BACKGROUND_KINDS = ['theme', 'color', 'file', 'slideshow', 'dynamic'];

  $: if (
    draft &&
    draft.display.background?.kind === 'dynamic' &&
    !draft.display.background.value
  ) {
    draft.display.background.value = 'stardust';
    draft = draft;
  }

  onDestroy(() => {
    if (savedNoticeTimer) {
      window.clearTimeout(savedNoticeTimer);
    }
  });

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
    savedNotice = '';
    if (savedNoticeTimer) {
      window.clearTimeout(savedNoticeTimer);
    }
    try {
      await settingsStore.saveHousehold(draft);
      savedNotice = $hubStream.dashboard.status?.config.writableYaml
        ? 'Saved to hub and YAML config.'
        : 'Saved to hub.';
      savedNoticeTimer = window.setTimeout(() => {
        savedNotice = '';
      }, 4000);
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
    {#if draft.display.widgetChrome.default === 'smoked' || draft.display.widgetChrome.default === 'auto'}
      <label>
        <span
          >Smoked opacity ({Math.round(
            (draft.display.widgetChrome.smokedOpacity ?? 0.6) * 100
          )}%)</span
        >
        <input
          type="range"
          min="0.1"
          max="0.95"
          step="0.05"
          value={draft.display.widgetChrome.smokedOpacity ?? 0.6}
          on:input={(event) => {
            if (draft) {
              draft.display.widgetChrome.smokedOpacity = parseFloat(
                event.currentTarget.value
              );
              draft = draft;
            }
          }}
        />
      </label>
    {/if}
    {#if draft.display.widgetChrome.default === 'frosted' || draft.display.widgetChrome.default === 'auto'}
      <label>
        <span
          >Frosted opacity ({Math.round(
            (draft.display.widgetChrome.frostedOpacity ?? 0.3) * 100
          )}%)</span
        >
        <input
          type="range"
          min="0.1"
          max="0.95"
          step="0.05"
          value={draft.display.widgetChrome.frostedOpacity ?? 0.3}
          on:input={(event) => {
            if (draft) {
              draft.display.widgetChrome.frostedOpacity = parseFloat(
                event.currentTarget.value
              );
              draft = draft;
            }
          }}
        />
      </label>
    {/if}
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
    {#if bg.kind === 'dynamic'}
      <label>
        <span>Dynamic background</span>
        <select bind:value={draft.display.background.value}>
          <option value="stardust">Stardust</option>
          <option value="weather-ambient">Weather Ambient</option>
        </select>
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
          on:input={(event) => {
            if (draft) {
              draft.display.background.intervalSeconds = numeric(
                event.currentTarget.value
              );
              draft = draft;
            }
          }}
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
    <span class="settings-save-state" aria-live="polite">
      {#if $settingsStore.saving}
        Saving appearance…
      {:else if hasChanges}
        Unsaved changes
      {:else if savedNotice}
        {savedNotice}
      {:else}
        All changes saved
      {/if}
    </span>
    <Button on:click={save} disabled={$settingsStore.saving || !hasChanges}
      >{$settingsStore.saving
        ? 'Saving'
        : hasChanges
          ? 'Save appearance'
          : 'Saved'}</Button
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

  .settings-form-grid input:not([type='range']),
  .settings-form-grid select {
    min-width: 0;
    min-height: 42px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 10px;
  }

  .settings-form-grid input[type='range'] {
    min-height: 42px;
    border: none;
    background: transparent;
    padding: 0;
    cursor: pointer;
    accent-color: var(--active);
  }

  .settings-actions {
    display: flex;
    align-items: center;
    gap: 10px;
    justify-content: flex-end;
    margin-top: 12px;
  }

  .settings-save-state {
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 720;
  }

  .settings-empty {
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
    margin: 12px 0 0;
    line-height: 1.4;
  }

  @media (max-width: 640px) {
    .settings-form-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
