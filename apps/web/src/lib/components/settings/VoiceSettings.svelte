<script lang="ts">
  import { Ban, Mic, MicOff, Save, Square } from 'lucide-svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import { settingsStore } from '$lib/settingsStore';
  import { hubStream } from '$lib/hubStream';
  import type { VoiceProvider, VoiceSatellite, VoiceStatus } from '$lib/types';

  let draft: VoiceStatus | undefined;
  let lastJSON = '';

  $: voice = $hubStream.dashboard.voice;
  $: providers = $settingsStore.voiceProviders;
  $: satellites = $settingsStore.voiceSatellites;
  $: wakeProviders = providers.filter(
    (provider) => provider.kind === 'wake-word'
  );
  $: sttProviders = providers.filter((provider) =>
    ['stt', 'stt-tts'].includes(provider.kind)
  );
  $: ttsProviders = providers.filter((provider) =>
    ['tts', 'stt-tts'].includes(provider.kind)
  );
  $: selectedWakeProvider = findProvider(providers, draft?.wakeWordModelId, [
    'wake-word'
  ]);
  $: wakeModels = selectedWakeProvider?.wakeWord?.models ?? [];
  $: selectedTTSProvider = providers.find(
    (provider) => provider.id === draft?.ttsProviderId
  );
  $: selectedTTSVoiceMetadata =
    $settingsStore.ttsVoices?.providerId === draft?.ttsProviderId
      ? $settingsStore.ttsVoices
      : undefined;
  $: ttsVoices = selectedTTSVoiceMetadata?.voices ?? [];
  $: ttsSetupStatus = selectedTTSVoiceMetadata?.setupStatus ?? 'not_configured';
  $: selectedTTSCloud =
    selectedTTSProvider && selectedTTSProvider.capabilities
      ? !selectedTTSProvider.capabilities.offline
      : Boolean($settingsStore.ttsVoices?.cloudProvider);

  $: if (voice) syncVoiceDraft(voice);

  function syncVoiceDraft(nextVoice: VoiceStatus) {
    const currentJSON = JSON.stringify(nextVoice);
    if (currentJSON === lastJSON) {
      return;
    }
    draft = structuredClone(nextVoice);
    lastJSON = currentJSON;
  }

  function findProvider(
    allProviders: VoiceProvider[],
    selectedModelId: string | undefined,
    kinds: string[]
  ) {
    if (!selectedModelId) {
      return undefined;
    }
    return allProviders.find(
      (provider) =>
        kinds.includes(provider.kind) &&
        provider.wakeWord?.models?.some((model) => model.id === selectedModelId)
    );
  }

  function providerLabel(provider: VoiceProvider) {
    const locality =
      provider.capabilities?.offline === false ? 'cloud' : 'local';
    return `${provider.name} · ${provider.healthStatus} · ${locality}`;
  }

  function badgeTone(status: string) {
    if (status === 'available' || status === 'ready' || status === 'paired') {
      return 'active';
    }
    if (
      status === 'offline' ||
      status === 'misconfigured' ||
      status === 'auth_failed' ||
      status === 'update_required' ||
      status === 'revoked'
    ) {
      return 'warning';
    }
    return 'neutral';
  }

  async function save() {
    if (!draft || $settingsStore.savingVoice) {
      return;
    }
    try {
      await settingsStore.saveVoice({
        deviceProfileId: draft.deviceProfileId,
        enabled: draft.enabled,
        wakeWordModelId: draft.wakeWordModelId,
        wakeWordPhrase: draft.wakeWordPhrase,
        wakeSensitivity: Number(draft.wakeSensitivity),
        sttProviderId: draft.sttProviderId,
        ttsProviderId: draft.ttsProviderId,
        sttModelId: draft.sttModelId,
        ttsModelId: draft.ttsModelId,
        ttsVoiceId: draft.ttsVoiceId,
        ttsEnabled: draft.ttsEnabled,
        ttsLocale: draft.ttsLocale,
        ttsSpeed: Number(draft.ttsSpeed),
        ttsVolume: Number(draft.ttsVolume),
        preferredAgentId: draft.preferredAgentId,
        cloudOptIn: draft.cloudOptIn,
        commandProvidersEnabled: draft.commandProvidersEnabled,
        followupWindowSeconds: Number(draft.followupWindowSeconds),
        microphoneProfile: draft.microphoneProfile
      });
    } catch {
      // Error is set in settingsStore.issue
    }
  }

  async function refreshTTSVoices() {
    if (!draft) {
      return;
    }
    draft.ttsVoiceId = '';
    await settingsStore.refreshTTSVoices(draft.ttsProviderId);
  }

  function selectWakeProvider(providerId: string) {
    if (!draft) {
      return;
    }
    const provider = providers.find((item) => item.id === providerId);
    draft.wakeWordModelId = provider?.wakeWord?.defaultModelId ?? '';
    draft.wakeWordPhrase = provider?.wakeWord?.phrase ?? draft.wakeWordPhrase;
    draft.wakeSensitivity =
      provider?.wakeWord?.sensitivity ?? draft.wakeSensitivity;
  }

  async function toggleMute() {
    await hubStream.toggleVoiceMute(fetch);
  }

  async function cancelVoice() {
    await hubStream.cancelVoiceSession(fetch);
  }

  async function saveSatellite(satellite: VoiceSatellite) {
    await settingsStore.updateSatellite(satellite.id, {
      displayName: satellite.displayName,
      roomLabel: satellite.roomLabel ?? '',
      deviceProfileId: satellite.deviceProfileId,
      enabled: satellite.enabled
    });
  }

  async function revokeSatellite(satellite: VoiceSatellite) {
    await settingsStore.updateSatellite(satellite.id, { revoke: true });
  }
</script>

{#if draft}
  <div class="voice-settings">
    <div class="voice-status-row">
      <div>
        <span>Service</span>
        <Badge tone={badgeTone(draft.serviceStatus)}
          >{draft.serviceStatus}</Badge
        >
      </div>
      <div>
        <span>State</span>
        <Badge tone={draft.muted ? 'warning' : badgeTone(draft.state)}
          >{draft.state}</Badge
        >
      </div>
      <div>
        <span>Device profile</span>
        <strong>{draft.deviceProfileId}</strong>
      </div>
      <div>
        <span>TTS setup</span>
        <Badge tone={badgeTone(ttsSetupStatus)}>{ttsSetupStatus}</Badge>
      </div>
    </div>

    <div class="voice-actions">
      <Button variant="outline" size="sm" on:click={toggleMute}>
        {#if draft.muted}<Mic size={16} />Unmute{:else}<MicOff
            size={16}
          />Mute{/if}
      </Button>
      <Button variant="outline" size="sm" on:click={cancelVoice}>
        <Square size={16} />Cancel
      </Button>
      <Button on:click={save} disabled={$settingsStore.savingVoice}>
        <Save size={16} />{$settingsStore.savingVoice ? 'Saving' : 'Save voice'}
      </Button>
    </div>

    <div class="settings-form-grid">
      <label class="switch-field">
        <span>Voice</span>
        <input type="checkbox" bind:checked={draft.enabled} />
      </label>

      <label>
        <span>Preferred agent</span>
        <select bind:value={draft.preferredAgentId}>
          <option value="">First enabled agent</option>
          {#each $hubStream.dashboard.agents as agent (agent.id)}
            <option value={agent.id}>{agent.name}</option>
          {/each}
        </select>
      </label>

      <label>
        <span>Wake provider</span>
        <select
          value={selectedWakeProvider?.id ?? ''}
          on:change={(event) =>
            selectWakeProvider(
              (event.currentTarget as HTMLSelectElement).value
            )}
        >
          <option value="">Not configured</option>
          {#each wakeProviders as provider (provider.id)}
            <option value={provider.id}>{providerLabel(provider)}</option>
          {/each}
        </select>
      </label>

      <label>
        <span>Wake model</span>
        <select bind:value={draft.wakeWordModelId}>
          <option value="">Not configured</option>
          {#each wakeModels as model (model.id)}
            <option value={model.id}>{model.phrase || model.id}</option>
          {/each}
        </select>
      </label>

      <label>
        <span>Wake phrase</span>
        <input bind:value={draft.wakeWordPhrase} />
      </label>

      <label>
        <span>Wake sensitivity {draft.wakeSensitivity.toFixed(2)}</span>
        <input
          type="range"
          min="0"
          max="1"
          step="0.05"
          bind:value={draft.wakeSensitivity}
        />
      </label>

      <label>
        <span>STT provider</span>
        <select bind:value={draft.sttProviderId}>
          <option value="">Not configured</option>
          {#each sttProviders as provider (provider.id)}
            <option value={provider.id}>{providerLabel(provider)}</option>
          {/each}
        </select>
      </label>

      <label>
        <span>STT model</span>
        <input bind:value={draft.sttModelId} />
      </label>

      <label class="switch-field">
        <span>Spoken responses</span>
        <input type="checkbox" bind:checked={draft.ttsEnabled} />
      </label>

      <label>
        <span>TTS provider</span>
        <select bind:value={draft.ttsProviderId} on:change={refreshTTSVoices}>
          <option value="">Not configured</option>
          {#each ttsProviders as provider (provider.id)}
            <option value={provider.id}>{providerLabel(provider)}</option>
          {/each}
        </select>
      </label>

      <label>
        <span>TTS voice</span>
        <select bind:value={draft.ttsVoiceId}>
          <option value="">Provider default</option>
          {#each ttsVoices as voiceOption (voiceOption.id)}
            <option value={voiceOption.id}>
              {voiceOption.label} · {voiceOption.locale}
            </option>
          {/each}
        </select>
      </label>

      <label>
        <span>TTS locale</span>
        <input bind:value={draft.ttsLocale} />
      </label>

      <label>
        <span>TTS speed {draft.ttsSpeed.toFixed(2)}</span>
        <input
          type="range"
          min="0.5"
          max="2"
          step="0.05"
          bind:value={draft.ttsSpeed}
        />
      </label>

      <label>
        <span>TTS volume {draft.ttsVolume.toFixed(2)}</span>
        <input
          type="range"
          min="0"
          max="1"
          step="0.05"
          bind:value={draft.ttsVolume}
        />
      </label>

      <label>
        <span>Follow-up window</span>
        <input
          type="number"
          min="1"
          max="45"
          step="1"
          bind:value={draft.followupWindowSeconds}
        />
      </label>

      <label>
        <span>Microphone profile</span>
        <input bind:value={draft.microphoneProfile} />
      </label>

      <label class="switch-field">
        <span>Cloud providers</span>
        <input type="checkbox" bind:checked={draft.cloudOptIn} />
      </label>

      <label class="switch-field">
        <span>Command providers</span>
        <input type="checkbox" bind:checked={draft.commandProvidersEnabled} />
      </label>
    </div>

    {#if selectedTTSCloud && !draft.cloudOptIn}
      <p class="settings-note">
        Cloud opt-in is required for the selected TTS provider.
      </p>
    {/if}
    {#if selectedTTSProvider?.lastError}
      <p class="settings-note">{selectedTTSProvider.lastError}</p>
    {/if}

    {#if satellites.length > 0}
      <div class="satellite-section">
        <div class="section-heading">
          <strong>Satellites</strong>
          <span>{satellites.length}</span>
        </div>
        <div class="satellite-list">
          {#each satellites as satellite (satellite.id)}
            <div class="satellite-item">
              <div class="satellite-summary">
                <div>
                  <strong>{satellite.displayName}</strong>
                  <span>{satellite.id}</span>
                </div>
                <Badge tone={badgeTone(satellite.status)}>
                  {satellite.status}
                </Badge>
              </div>

              <div class="satellite-grid">
                <label>
                  <span>Name</span>
                  <input bind:value={satellite.displayName} />
                </label>
                <label>
                  <span>Room</span>
                  <input bind:value={satellite.roomLabel} />
                </label>
                <label>
                  <span>Device profile</span>
                  <input bind:value={satellite.deviceProfileId} />
                </label>
                <label>
                  <span>Version</span>
                  <input value={satellite.version ?? ''} disabled />
                </label>
                <label class="switch-field">
                  <span>Enabled</span>
                  <input
                    type="checkbox"
                    bind:checked={satellite.enabled}
                    disabled={satellite.status === 'revoked'}
                  />
                </label>
                <label>
                  <span>Last seen</span>
                  <input value={satellite.lastSeenAt ?? ''} disabled />
                </label>
              </div>

              <div class="satellite-actions">
                <Button
                  variant="outline"
                  size="sm"
                  on:click={() => saveSatellite(satellite)}
                  disabled={$settingsStore.savingSatellite ||
                    satellite.status === 'revoked'}
                >
                  <Save size={16} />Save
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  on:click={() => revokeSatellite(satellite)}
                  disabled={$settingsStore.savingSatellite ||
                    satellite.status === 'revoked'}
                >
                  <Ban size={16} />Revoke
                </Button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
{:else}
  <p class="settings-empty">Voice settings are loading.</p>
{/if}

<style>
  .voice-settings {
    display: grid;
    gap: 12px;
  }

  .voice-status-row,
  .settings-form-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .voice-status-row > div,
  .settings-form-grid label {
    display: grid;
    gap: 6px;
    min-width: 0;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .voice-status-row span,
  .settings-form-grid label span {
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 760;
  }

  .voice-status-row strong {
    min-width: 0;
    overflow: hidden;
    color: var(--foreground);
    font-size: 0.84rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .settings-form-grid input:not([type='range']):not([type='checkbox']),
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

  .switch-field {
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
  }

  .switch-field input {
    width: 22px;
    height: 22px;
    accent-color: var(--active);
  }

  .voice-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    justify-content: flex-end;
  }

  .satellite-section {
    display: grid;
    gap: 10px;
  }

  .section-heading,
  .satellite-summary,
  .satellite-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    justify-content: space-between;
  }

  .section-heading strong {
    color: var(--foreground);
    font-size: 0.9rem;
  }

  .section-heading span,
  .satellite-summary span {
    color: var(--muted);
    font-size: 0.78rem;
    font-weight: 700;
  }

  .satellite-list {
    display: grid;
    gap: 10px;
  }

  .satellite-item {
    display: grid;
    gap: 10px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    padding: 10px;
  }

  .satellite-summary > div {
    display: grid;
    gap: 3px;
    min-width: 0;
  }

  .satellite-summary strong {
    min-width: 0;
    overflow: hidden;
    color: var(--foreground);
    font-size: 0.86rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .satellite-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .satellite-grid label {
    display: grid;
    gap: 6px;
    min-width: 0;
  }

  .satellite-grid span {
    color: var(--muted);
    font-size: 0.76rem;
    font-weight: 760;
  }

  .satellite-grid input:not([type='checkbox']) {
    min-width: 0;
    min-height: 40px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface);
    color: var(--foreground);
    padding: 0 10px;
  }

  .satellite-grid input:disabled {
    opacity: 0.72;
  }

  .settings-note,
  .settings-empty {
    margin: 0;
    line-height: 1.4;
    color: var(--muted);
    font-size: 0.82rem;
    font-weight: 650;
  }

  .settings-note {
    border: 1px solid var(--warning);
    border-radius: 8px;
    padding: 10px;
    color: var(--warning);
  }

  @media (max-width: 640px) {
    .voice-status-row,
    .settings-form-grid,
    .satellite-grid {
      grid-template-columns: 1fr;
    }

    .voice-actions {
      justify-content: stretch;
    }
  }
</style>
