import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fallbackDashboard } from '$lib/hubClient';
import { hubStream } from '$lib/hubStream';
import { settingsStore } from '$lib/settingsStore';
import type { HouseholdSettings, VoiceStatus } from '$lib/types';
import VoiceSettings from './VoiceSettings.svelte';

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
    ...init
  });
}

const household: HouseholdSettings = {
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

const readyVoice: VoiceStatus = {
  enabled: true,
  muted: false,
  state: 'wake_listening',
  serviceStatus: 'ready',
  deviceProfileId: 'kitchen-display',
  wakeWordModelId: 'hey-jute',
  wakeWordPhrase: 'Hey Jute',
  wakeSensitivity: 0.5,
  sttProviderId: 'cloud-stt',
  ttsProviderId: 'command-tts',
  sttModelId: '',
  ttsModelId: '',
  ttsVoiceId: '',
  ttsEnabled: true,
  ttsLocale: 'en-GB',
  ttsSpeed: 1,
  ttsVolume: 1,
  preferredAgentId: '',
  cloudOptIn: false,
  commandProvidersEnabled: false,
  followupWindowSeconds: 8,
  microphoneProfile: '',
  updatedAt: '2026-06-17T08:00:00Z'
};

async function loadVoiceSettings() {
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url) => {
    if (String(url).includes('/settings/household')) {
      return jsonResponse(household);
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
            id: 'local-wake',
            name: 'Local Wake',
            version: '1.0.0',
            kind: 'wake-word',
            transportType: 'command',
            capabilities: {
              streaming: false,
              partialTranscripts: false,
              offline: true,
              languages: ['en-GB'],
              inputFormats: ['audio/pcm']
            },
            wakeWord: {
              defaultModelId: 'hey-jute',
              phrase: 'Hey Jute',
              sensitivity: 0.5,
              models: [
                {
                  id: 'hey-jute',
                  phrase: 'Hey Jute',
                  sensitivity: 0.5
                }
              ]
            },
            healthStatus: 'available',
            updatedAt: '2026-06-17T08:00:00Z'
          },
          {
            id: 'cloud-stt',
            name: 'Cloud STT',
            version: '1.0.0',
            kind: 'stt',
            transportType: 'command',
            capabilities: {
              streaming: false,
              partialTranscripts: false,
              offline: false,
              languages: ['en-GB'],
              inputFormats: ['audio/pcm']
            },
            healthStatus: 'available',
            updatedAt: '2026-06-17T08:00:00Z'
          },
          {
            id: 'command-tts',
            name: 'Command TTS',
            version: '1.0.0',
            kind: 'tts',
            transportType: 'command',
            capabilities: {
              streaming: false,
              partialTranscripts: false,
              offline: true,
              languages: ['en-GB'],
              inputFormats: ['text/plain']
            },
            healthStatus: 'available',
            updatedAt: '2026-06-17T08:00:00Z'
          }
        ]
      });
    }
    if (String(url).includes('/tts/voices')) {
      return jsonResponse({
        providerId: 'command-tts',
        healthStatus: 'available',
        setupStatus: 'available',
        locale: 'en-GB',
        speed: 1,
        volume: 1,
        cloudProvider: false,
        voices: []
      });
    }
    return jsonResponse({ error: 'Not mocked' }, { status: 400 });
  });

  hubStream.init({
    ...fallbackDashboard(),
    voice: readyVoice,
    agents: []
  });
  await settingsStore.load(fetcher);
}

describe('VoiceSettings', () => {
  beforeEach(() => {
    settingsStore.clearIssue();
  });

  it('renders wake settings without STT or TTS controls', async () => {
    await loadVoiceSettings();

    const { body } = render(VoiceSettings, { props: { section: 'wake' } });

    expect(body).toContain('Device profile');
    expect(body).toContain('kitchen-display');
    expect(body).toContain('Mute');
    expect(body).toContain('Cancel');
    expect(body).toContain('Save voice');
    expect(body).toContain('Provider access');
    expect(body).toContain('Wake provider');
    expect(body).toContain('Wake model');
    expect(body).toContain('Cloud providers');
    expect(body).toContain('Command providers');
    expect(body).toContain(
      '<option value="local-wake" selected="">Local Wake · local · available</option>'
    );
    expect(body).toContain(
      '<option value="hey-jute" selected="">Hey Jute</option>'
    );
    expect(body).not.toContain('STT provider');
    expect(body).not.toContain('TTS provider');
    expect(body).not.toContain('Satellites');
  });

  it('renders STT settings without Wake or TTS controls', async () => {
    await loadVoiceSettings();

    const { body } = render(VoiceSettings, { props: { section: 'stt' } });

    expect(body).toContain('Provider access');
    expect(body).toContain('STT provider');
    expect(body).toContain('STT model');
    expect(body).toContain('Cloud STT · cloud · available');
    expect(body).not.toContain('Wake provider');
    expect(body).not.toContain('TTS provider');
  });

  it('renders TTS settings without Wake or STT controls', async () => {
    await loadVoiceSettings();

    const { body } = render(VoiceSettings, { props: { section: 'tts' } });

    expect(body).toContain('Provider access');
    expect(body).toContain('TTS provider');
    expect(body).toContain('TTS voice');
    expect(body).toContain('Command TTS · local · available');
    expect(body).toContain('TTS setup');
    expect(body).toContain('available');
    expect(body).not.toContain('Wake provider');
    expect(body).not.toContain('STT provider');
  });
});
