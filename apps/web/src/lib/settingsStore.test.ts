import { describe, expect, it, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { settingsStore } from './settingsStore';
import { hubStream } from './hubStream';
import type { HouseholdSettings } from './types';

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
    ...init
  });
}

const mockHouseholdSettings: HouseholdSettings = {
  home: {
    name: 'Jute Home'
  },
  display: {
    theme: 'system',
    colorMode: 'system',
    themeId: 'jute-mono',
    density: 'comfortable',
    motion: 'full',
    background: {
      kind: 'theme',
      value: '',
      fit: 'cover',
      position: 'center',
      overlay: 'none'
    },
    widgetChrome: {
      default: 'solid'
    },
    accentColor: 'neutral',
    idleMode: 'ambient'
  },
  setup: {
    complete: true,
    missing: []
  }
};

describe('settingsStore', () => {
  beforeEach(() => {
    // Reset store to initial state
    settingsStore.clearIssue();
  });

  it('loads settings successfully and populates state', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.load(fetcher);
    const state = get(settingsStore);

    expect(state.loading).toBe(false);
    expect(state.householdSettings).toEqual(mockHouseholdSettings);
    expect(state.roomSettings).toEqual([]);
    expect(state.tileSettings).toEqual([]);
    expect(state.issue).toBe('');
  });

  it('loads voice satellites as safe display projections', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      if (String(url).includes('/voice/providers')) {
        return jsonResponse({ providers: [] });
      }
      if (String(url).includes('/voice/satellites')) {
        return jsonResponse({
          satellites: [
            {
              id: 'sat-kitchen',
              displayName:
                'Kitchen Satellite http://provider.local?token=secret',
              roomLabel: 'Kitchen secret:room-token',
              deviceProfileId: 'kitchen-display token=profile-secret',
              enabled: true,
              status: 'auth_failed',
              version: '0.1.0 apiKey=version-secret',
              pairedAt: '2026-06-15T08:00:00Z',
              lastSeenAt: '2026-06-15T08:05:00Z',
              credentialSecretRef: 'secret-ref:JUTE_SATELLITE_TOKEN',
              rawTranscript: 'raw transcript from the kitchen',
              providerEndpointUrl: 'http://provider.local?token=secret',
              rawError: 'stack trace with token=secret',
              createdAt: '2026-06-15T08:00:00Z',
              updatedAt: '2026-06-15T08:05:00Z'
            }
          ]
        });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({
          providerId: '',
          healthStatus: 'disabled',
          setupStatus: 'disabled',
          locale: 'en',
          speed: 1,
          volume: 1,
          cloudProvider: false,
          voices: []
        });
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.load(fetcher);
    const state = get(settingsStore);

    expect(state.voiceSatellites).toHaveLength(1);
    expect(state.voiceSatellites[0].displayName).toBe(
      'Kitchen Satellite [redacted-url]'
    );
    expect(state.voiceSatellites[0].roomLabel).toBe(
      'Kitchen secret=[redacted]'
    );
    expect(state.voiceSatellites[0].deviceProfileId).toBe(
      'kitchen-display token=[redacted]'
    );
    expect(state.voiceSatellites[0].status).toBe('auth_failed');
    expect(state.voiceSatellites[0].version).toBe(
      '0.1.0 api_key=[redacted]'
    );
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('secret-ref');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('credential');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain(
      'raw transcript'
    );
    expect(JSON.stringify(state.voiceSatellites)).not.toContain(
      'provider.local'
    );
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('token=secret');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('room-token');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain(
      'profile-secret'
    );
    expect(JSON.stringify(state.voiceSatellites)).not.toContain(
      'version-secret'
    );
  });

  it('loads voice providers as safe setup projections', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      if (String(url).includes('/voice/providers')) {
        return jsonResponse({
          providers: [
            {
              id: 'local-stt',
              name: 'Local STT',
              version: '1.0.0',
              kind: 'stt',
              transportType: 'wyoming',
              capabilities: {
                streaming: true,
                partialTranscripts: true,
                offline: true,
                languages: ['en-GB'],
                inputFormats: ['audio/pcm;rate=16000'],
                outputFormats: ['text/plain']
              },
              healthStatus: 'misconfigured',
              lastError: 'provider failed token=provider-secret',
              manifestJson: '{"credentials":[{"env":"JUTE_STT_TOKEN"}]}',
              endpoint: 'tcp://127.0.0.1:10300?token=provider-secret',
              credentialSecretRef: 'secret-ref:JUTE_STT_TOKEN',
              providerPayload: { apiKey: 'sk-provider-secret' },
              updatedAt: '2026-06-15T08:05:00Z'
            }
          ]
        });
      }
      if (String(url).includes('/voice/satellites')) {
        return jsonResponse({ satellites: [] });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({
          providerId: '',
          healthStatus: 'disabled',
          setupStatus: 'disabled',
          locale: 'en',
          speed: 1,
          volume: 1,
          cloudProvider: false,
          voices: []
        });
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.load(fetcher);
    const state = get(settingsStore);

    expect(state.voiceProviders).toHaveLength(1);
    expect(state.voiceProviders[0].lastError).toBe(
      'provider failed token=[redacted]'
    );
    const serialized = JSON.stringify(state.voiceProviders);
    expect(serialized).not.toContain('credentialSecretRef');
    expect(serialized).not.toContain('manifestJson');
    expect(serialized).not.toContain('endpoint');
    expect(serialized).not.toContain('JUTE_STT_TOKEN');
    expect(serialized).not.toContain('provider-secret');
    expect(serialized).not.toContain('sk-provider-secret');
  });

  it('loads TTS voices as safe setup projections', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      if (String(url).includes('/voice/providers')) {
        return jsonResponse({ providers: [] });
      }
      if (String(url).includes('/voice/satellites')) {
        return jsonResponse({ satellites: [] });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({
          providerId: 'local-tts',
          providerName: 'Local TTS token=provider-secret',
          healthStatus: 'available',
          setupStatus: 'available',
          selectedVoiceId: 'amy',
          selectedModelId: 'model-secret=provider',
          locale: 'en-GB',
          speed: 1,
          volume: 1,
          cloudProvider: false,
          endpoint: 'tcp://127.0.0.1:10500?token=provider-secret',
          credentialSecretRef: 'secret-ref:JUTE_TTS_TOKEN',
          voices: [
            {
              id: 'amy',
              label: 'Amy token=voice-secret',
              locale: 'en-GB',
              modelId: 'tts-secret=voice',
              styles: ['neutral', 'token=style-secret'],
              outputFormats: ['audio/wav', 'apiKey=format-secret'],
              providerPayload: { apiKey: 'sk-voice-secret' }
            }
          ]
        });
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.load(fetcher);
    const state = get(settingsStore);

    expect(state.ttsVoices?.providerName).toBe('Local TTS token=[redacted]');
    expect(state.ttsVoices?.voices[0].label).toBe('Amy token=[redacted]');
    expect(state.ttsVoices?.voices[0].styles).toContain('token=[redacted]');
    const serialized = JSON.stringify(state.ttsVoices);
    expect(serialized).not.toContain('credentialSecretRef');
    expect(serialized).not.toContain('endpoint');
    expect(serialized).not.toContain('providerPayload');
    expect(serialized).not.toContain('JUTE_TTS_TOKEN');
    expect(serialized).not.toContain('provider-secret');
    expect(serialized).not.toContain('voice-secret');
    expect(serialized).not.toContain('sk-voice-secret');
  });

  it('handles load error gracefully', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockRejectedValue(new Error('Network error'));

    await expect(settingsStore.load(fetcher)).rejects.toThrow('Network error');
    const state = get(settingsStore);

    expect(state.loading).toBe(false);
    expect(state.issue).toBe(
      'Settings are unavailable. Check that the hub is running.'
    );
  });

  it('saves household settings and triggers hubStream refresh', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (
        String(url).includes('/config') ||
        String(url).includes('/home') ||
        String(url).includes('/agents') ||
        String(url).includes('/widgets/layout') ||
        String(url).includes('/voice/status') ||
        String(url).includes('/status')
      ) {
        return jsonResponse({});
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    const refreshSpy = vi.spyOn(hubStream, 'refreshAfterMutation');

    await settingsStore.saveHousehold(mockHouseholdSettings, fetcher);
    const state = get(settingsStore);

    expect(state.saving).toBe(false);
    expect(state.householdSettings).toEqual(mockHouseholdSettings);
    expect(state.issue).toBe('');
    expect(refreshSpy).toHaveBeenCalled();
  });

  it('saves voice settings through the hub API and refreshes dashboard state', async () => {
    const savedVoice = {
      enabled: true,
      muted: false,
      state: 'wake_listening',
      serviceStatus: 'ready',
      deviceProfileId: 'default-display',
      wakeWordModelId: 'hey-jute',
      wakeWordPhrase: 'Hey Jute',
      wakeSensitivity: 0.5,
      sttProviderId: 'local-stt',
      ttsProviderId: 'local-tts',
      sttModelId: '',
      ttsModelId: '',
      ttsVoiceId: 'amy',
      ttsEnabled: true,
      ttsLocale: 'en-GB',
      ttsSpeed: 1,
      ttsVolume: 1,
      preferredAgentId: 'house',
      cloudOptIn: false,
      commandProvidersEnabled: false,
      followupWindowSeconds: 8,
      microphoneProfile: '',
      updatedAt: new Date().toISOString()
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async (url, init) => {
        if (String(url).includes('/voice/settings')) {
          expect(init?.method).toBe('PATCH');
          expect(JSON.parse(String(init?.body))).toMatchObject({
            enabled: true,
            sttProviderId: 'local-stt'
          });
          return jsonResponse(savedVoice);
        }
        if (String(url).includes('/tts/voices')) {
          return jsonResponse({
            providerId: 'local-tts',
            healthStatus: 'available',
            setupStatus: 'available',
            locale: 'en-GB',
            speed: 1,
            volume: 1,
            cloudProvider: false,
            voices: []
          });
        }
        if (
          String(url).includes('/config') ||
          String(url).includes('/home') ||
          String(url).includes('/agents') ||
          String(url).includes('/widgets/layout') ||
          String(url).includes('/voice/status') ||
          String(url).includes('/status')
        ) {
          return jsonResponse({});
        }
        return jsonResponse({ error: 'Not mocked' }, { status: 400 });
      });
    const refreshSpy = vi.spyOn(hubStream, 'refreshAfterMutation');

    await settingsStore.saveVoice(
      {
        enabled: true,
        sttProviderId: 'local-stt'
      },
      fetcher
    );
    const state = get(settingsStore);

    expect(state.savingVoice).toBe(false);
    expect(state.ttsVoices?.providerId).toBe('local-tts');
    expect(state.issue).toBe('');
    expect(refreshSpy).toHaveBeenCalled();
  });

  it('normalizes voice control ranges before saving through the hub API', async () => {
    let savedBody: Record<string, unknown> | undefined;
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => {
      if (String(url).includes('/voice/settings')) {
        savedBody = JSON.parse(String(init?.body));
        return jsonResponse({
          enabled: true,
          muted: false,
          state: 'idle',
          serviceStatus: 'ready',
          deviceProfileId: 'default-display',
          wakeWordModelId: '',
          wakeWordPhrase: '',
          wakeSensitivity: savedBody?.wakeSensitivity,
          sttProviderId: '',
          ttsProviderId: '',
          sttModelId: '',
          ttsModelId: '',
          ttsVoiceId: '',
          ttsEnabled: true,
          ttsLocale: 'en-GB',
          ttsSpeed: savedBody?.ttsSpeed,
          ttsVolume: savedBody?.ttsVolume,
          preferredAgentId: '',
          cloudOptIn: false,
          commandProvidersEnabled: false,
          followupWindowSeconds: savedBody?.followupWindowSeconds,
          microphoneProfile: '',
          updatedAt: '2026-06-16T10:00:00Z'
        });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({
          providerId: '',
          healthStatus: 'disabled',
          setupStatus: 'disabled',
          locale: 'en',
          speed: 1,
          volume: 1,
          cloudProvider: false,
          voices: []
        });
      }
      if (
        String(url).includes('/config') ||
        String(url).includes('/home') ||
        String(url).includes('/agents') ||
        String(url).includes('/widgets/layout') ||
        String(url).includes('/voice/status') ||
        String(url).includes('/status')
      ) {
        return jsonResponse({});
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.saveVoice(
      {
        enabled: true,
        wakeSensitivity: 2,
        ttsSpeed: 0,
        ttsVolume: Number.NaN,
        followupWindowSeconds: 99
      },
      fetcher
    );

    expect(savedBody).toMatchObject({
      wakeSensitivity: 1,
      ttsSpeed: 0.5,
      ttsVolume: 1,
      followupWindowSeconds: 45
    });
    expect(get(settingsStore).issue).toBe('');
  });

  it('requires explicit cloud opt-in before saving cloud voice providers', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      if (String(url).includes('/voice/providers')) {
        return jsonResponse({
          providers: [
            {
              id: 'cloud-tts',
              name: 'Cloud TTS',
              version: '1.0.0',
              kind: 'tts',
              transportType: 'http-json',
              capabilities: {
                streaming: false,
                partialTranscripts: false,
                offline: false,
                languages: ['en-GB'],
                inputFormats: ['text/plain'],
                outputFormats: ['audio/pcm']
              },
              healthStatus: 'available',
              updatedAt: '2026-06-16T10:00:00Z'
            }
          ]
        });
      }
      if (String(url).includes('/voice/satellites')) {
        return jsonResponse({ satellites: [] });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({ providerId: 'cloud-tts', voices: [] });
      }
      if (String(url).includes('/voice/settings')) {
        return jsonResponse({ error: 'should not save' }, { status: 500 });
      }
      return jsonResponse({});
    });

    await settingsStore.load(fetcher);

    await expect(
      settingsStore.saveVoice(
        {
          ttsProviderId: 'cloud-tts',
          cloudOptIn: false
        },
        fetcher
      )
    ).rejects.toThrow('Cloud opt-in is required for Cloud TTS.');

    expect(
      fetcher.mock.calls.some((call) =>
        String(call[0]).includes('/voice/settings')
      )
    ).toBe(false);
    expect(get(settingsStore)).toMatchObject({
      savingVoice: false,
      issue: 'Cloud opt-in is required for Cloud TTS.'
    });
  });

  it('requires explicit command-provider enablement before saving command voice providers', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      if (String(url).includes('/voice/providers')) {
        return jsonResponse({
          providers: [
            {
              id: 'command-stt',
              name: 'Command STT',
              version: '1.0.0',
              kind: 'stt',
              transportType: 'command',
              capabilities: {
                streaming: false,
                partialTranscripts: false,
                offline: true,
                languages: ['en-GB'],
                inputFormats: ['audio/pcm'],
                outputFormats: ['text/plain']
              },
              healthStatus: 'available',
              updatedAt: '2026-06-16T10:00:00Z'
            }
          ]
        });
      }
      if (String(url).includes('/voice/satellites')) {
        return jsonResponse({ satellites: [] });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({ providerId: '', voices: [] });
      }
      if (String(url).includes('/voice/settings')) {
        return jsonResponse({ error: 'should not save' }, { status: 500 });
      }
      return jsonResponse({});
    });

    await settingsStore.load(fetcher);

    await expect(
      settingsStore.saveVoice(
        {
          sttProviderId: 'command-stt',
          commandProvidersEnabled: false
        },
        fetcher
      )
    ).rejects.toThrow(
      'Command providers must be enabled before saving Command STT.'
    );

    expect(
      fetcher.mock.calls.some((call) =>
        String(call[0]).includes('/voice/settings')
      )
    ).toBe(false);
    expect(get(settingsStore)).toMatchObject({
      savingVoice: false,
      issue: 'Command providers must be enabled before saving Command STT.'
    });
  });

  it('updates a satellite projection without secret material', async () => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
      if (String(url).includes('/settings/household')) {
        return jsonResponse(mockHouseholdSettings);
      }
      if (String(url).includes('/settings/rooms')) {
        return jsonResponse({ rooms: [] });
      }
      if (String(url).includes('/settings/tiles')) {
        return jsonResponse({ tiles: [] });
      }
      if (String(url).includes('/backgrounds')) {
        return jsonResponse({ images: [] });
      }
      if (String(url).includes('/voice/providers')) {
        return jsonResponse({ providers: [] });
      }
      if (String(url).includes('/voice/satellites/sat-kitchen')) {
        return jsonResponse({
          id: 'sat-kitchen',
          displayName: 'Kitchen Voice https://provider.local?token=secret',
          roomLabel: 'Kitchen password:room-secret',
          deviceProfileId: 'kitchen-display apiKey=profile-secret',
          enabled: false,
          status: 'unexpected raw error token=secret',
          pairedAt: '2026-06-15T08:00:00Z',
          createdAt: '2026-06-15T08:00:00Z',
          updatedAt: '2026-06-15T08:10:00Z'
        });
      }
      if (String(url).includes('/voice/satellites')) {
        return jsonResponse({
          satellites: [
            {
              id: 'sat-kitchen',
              displayName: 'Kitchen Satellite',
              roomLabel: '',
              deviceProfileId: 'default-display',
              enabled: true,
              status: 'paired',
              pairedAt: '2026-06-15T08:00:00Z',
              createdAt: '2026-06-15T08:00:00Z',
              updatedAt: '2026-06-15T08:00:00Z'
            }
          ]
        });
      }
      if (String(url).includes('/tts/voices')) {
        return jsonResponse({
          providerId: '',
          healthStatus: 'disabled',
          setupStatus: 'disabled',
          locale: 'en',
          speed: 1,
          volume: 1,
          cloudProvider: false,
          voices: []
        });
      }
      return jsonResponse({ error: 'Not mocked' }, { status: 400 });
    });

    await settingsStore.load(fetcher);
    await settingsStore.updateSatellite(
      'sat-kitchen',
      {
        displayName: 'Kitchen Voice',
        roomLabel: 'Kitchen',
        deviceProfileId: 'kitchen-display',
        enabled: false
      },
      fetcher
    );
    const state = get(settingsStore);

    expect(state.savingSatellite).toBe(false);
    expect(state.voiceSatellites[0].displayName).toBe(
      'Kitchen Voice [redacted-url]'
    );
    expect(state.voiceSatellites[0].roomLabel).toBe(
      'Kitchen password=[redacted]'
    );
    expect(state.voiceSatellites[0].deviceProfileId).toBe(
      'kitchen-display api_key=[redacted]'
    );
    expect(state.voiceSatellites[0].status).toBe('misconfigured');
    expect(state.voiceSatellites[0].enabled).toBe(false);
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('secret-ref');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('credential');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain(
      'provider.local'
    );
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('token=secret');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain('room-secret');
    expect(JSON.stringify(state.voiceSatellites)).not.toContain(
      'profile-secret'
    );
  });
});
