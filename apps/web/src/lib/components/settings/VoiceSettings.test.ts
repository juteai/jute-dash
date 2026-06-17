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

async function loadVoiceSettings(status = 'auth_failed') {
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
            id: 'cloud-stt',
            name: 'Cloud STT',
            version: '1.0.0',
            kind: 'stt',
            transportType: 'http-json',
            capabilities: {
              streaming: false,
              partialTranscripts: false,
              offline: false,
              languages: ['en-GB'],
              inputFormats: ['audio/pcm'],
              outputFormats: ['text/plain']
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
              inputFormats: ['text/plain'],
              outputFormats: ['audio/wav']
            },
            healthStatus: 'available',
            updatedAt: '2026-06-17T08:00:00Z'
          }
        ]
      });
    }
    if (String(url).includes('/voice/satellites')) {
      return jsonResponse({
        satellites: [
          {
            id: 'sat-kitchen',
            displayName: 'Kitchen Satellite https://hub.local?token=secret',
            roomLabel: 'Kitchen password:room-secret',
            deviceProfileId: 'kitchen-display apiKey=profile-secret',
            enabled: status !== 'revoked',
            status,
            version: '0.2.0 token=version-secret',
            pairedAt: '2026-06-17T08:00:00Z',
            revokedAt:
              status === 'revoked' ? '2026-06-17T09:00:00Z' : undefined,
            lastSeenAt: '2026-06-17T08:05:00Z',
            credentialSecretRef: 'secret-ref:JUTE_SATELLITE_TOKEN',
            rawTranscript: 'unlock the side door',
            rawError: 'dial tcp 127.0.0.1:10500 token=secret',
            createdAt: '2026-06-17T08:00:00Z',
            updatedAt: '2026-06-17T08:05:00Z'
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

  it('renders safe satellite projections separately from display profile controls', async () => {
    await loadVoiceSettings();

    const { body } = render(VoiceSettings);

    expect(body).toContain('Device profile');
    expect(body).toContain('kitchen-display');
    expect(body).toContain('Mute');
    expect(body).toContain('Cancel');
    expect(body).toContain('Save voice');
    expect(body).toContain('Cloud providers');
    expect(body).toContain('Command providers');
    expect(body).toContain('Cloud STT · available · cloud');
    expect(body).toContain('Command TTS · available · local');
    expect(body).toContain('TTS setup');
    expect(body).toContain('available');
    expect(body).toContain('Satellites');
    expect(body).toContain('sat-kitchen');
    expect(body).toContain('auth_failed');
    expect(body).toContain('Kitchen Satellite [redacted-url]');
    expect(body).toContain('Kitchen password=[redacted]');
    expect(body).toContain('kitchen-display api_key=[redacted]');
    expect(body).not.toContain('JUTE_SATELLITE_TOKEN');
    expect(body).not.toContain('hub.local');
    expect(body).not.toContain('token=secret');
    expect(body).not.toContain('room-secret');
    expect(body).not.toContain('profile-secret');
    expect(body).not.toContain('unlock the side door');
  });

  it('renders revoked satellites as non-editable from the old credential path', async () => {
    await loadVoiceSettings('revoked');

    const { body } = render(VoiceSettings);

    expect(body).toContain('revoked');
    expect(body).toContain('disabled');
  });
});
