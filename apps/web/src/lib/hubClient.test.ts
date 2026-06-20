import { describe, expect, it, vi } from 'vitest';
import {
  API_BASE,
  fallbackDashboard,
  getAdapterConnectionKinds,
  getSpotifyWebPlaybackToken,
  getTTSVoices,
  getVoiceProviders,
  initialDashboard,
  spotifyCallbackDisplayURL,
  spotifyCallbackParams,
  spotifyOAuthRedirectURI,
  spotifyAuthURL,
  submitVoiceAudio
} from './hubClient';

describe('fallback dashboard', () => {
  it('marks offline scaffolding as stale and agentless', () => {
    const fallback = fallbackDashboard();

    expect(fallback.connectionState).toBe('offline');
    expect(fallback.stale).toBe(true);
    expect(fallback.agents).toEqual([]);
    expect(fallback.layout.widgets.map((widget) => widget.kind)).toEqual([
      'date-time',
      'weather',
      'chat-history'
    ]);
    expect('weather' in fallback.home).toBe(false);
  });

  it('can create a neutral initial dashboard before client-side hub connection', () => {
    const initial = initialDashboard();

    expect(initial.connectionState).toBe('starting');
    expect(initial.issue).toBeUndefined();
  });

  it('posts browser microphone PCM to the hub voice audio API', async () => {
    const fetcher = async (url: string | URL | Request, init?: RequestInit) => {
      void url;
      void init;
      return new Response(
        JSON.stringify({
          conversation: { id: 'conversation-1' },
          followup: { active: true, turns: 1, maxTurns: 5 }
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        }
      );
    };
    const calls: Array<{ url: string | URL | Request; init?: RequestInit }> =
      [];
    const recordingFetcher = (async (
      url: string | URL | Request,
      init?: RequestInit
    ) => {
      calls.push({ url, init });
      return fetcher(url, init);
    }) as typeof fetch;

    const response = await submitVoiceAudio(
      recordingFetcher,
      new Blob(['pcm']),
      {
        sampleRate: 16000,
        channels: 1,
        deviceProfileId: 'browser-display',
        conversationId: 'conversation-1'
      }
    );

    expect(response.followup.active).toBe(true);
    expect(String(calls[0].url)).toContain('/api/v1/voice/audio');
    expect(calls[0].init?.method).toBe('POST');
    const headers = calls[0].init?.headers as Headers;
    expect(headers.get('X-Jute-Sample-Rate')).toBe('16000');
    expect(headers.get('X-Jute-Channels')).toBe('1');
    expect(headers.get('X-Jute-Device-Profile-Id')).toBe('browser-display');
    expect(headers.get('X-Jute-Conversation-Id')).toBe('conversation-1');
  });

  it('reads safe voice provider projections from the hub', async () => {
    const fetcher = (async () =>
      new Response(
        JSON.stringify({
          providers: [
            {
              id: 'local-stt',
              name: 'Local STT',
              version: '1.0.0',
              kind: 'stt',
              transportType: 'command',
              capabilities: {
                streaming: true,
                partialTranscripts: true,
                offline: true,
                languages: ['en-GB'],
                inputFormats: ['audio/pcm;rate=16000']
              },
              healthStatus: 'available',
              lastActivationAt: '2026-06-15T08:05:00Z',
              lastError: 'provider failed token=secret-value',
              manifestJson: '{"credentials":[{"env":"JUTE_STT_TOKEN"}]}',
              endpoint: 'tcp://127.0.0.1:10300?token=secret-value',
              credentialSecretRef: 'secret-ref:JUTE_STT_TOKEN',
              providerPayload: { apiKey: 'sk-secret' },
              updatedAt: '2026-06-15T08:05:00Z'
            }
          ]
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        }
      )) as typeof fetch;

    const providers = await getVoiceProviders(fetcher);

    expect(providers[0].id).toBe('local-stt');
    expect(providers[0].lastError).toBe('provider failed token=[redacted]');
    const serialized = JSON.stringify(providers);
    expect(serialized).not.toContain('credentialSecretRef');
    expect(serialized).not.toContain('manifestJson');
    expect(serialized).not.toContain('endpoint');
    expect(serialized).not.toContain('JUTE_STT_TOKEN');
    expect(serialized).not.toContain('secret-value');
    expect(serialized).not.toContain('sk-secret');
  });

  it('reads safe TTS voice projections from the hub', async () => {
    const fetcher = (async () =>
      new Response(
        JSON.stringify({
          providerId: 'local-tts',
          providerName: 'Local TTS token=secret-value',
          healthStatus: 'available',
          setupStatus: 'available',
          selectedVoiceId: 'amy',
          selectedModelId: 'model-token',
          locale: 'en-GB',
          speed: 1,
          volume: 1,
          cloudProvider: false,
          endpoint: 'tcp://127.0.0.1:10500?token=secret-value',
          credentialSecretRef: 'secret-ref:JUTE_TTS_TOKEN',
          voices: [
            {
              id: 'amy',
              label: 'Amy token=voice-secret',
              locale: 'en-GB',
              modelId: 'model-secret=voice',
              providerPayload: { apiKey: 'sk-voice-secret' }
            }
          ]
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        }
      )) as typeof fetch;

    const voices = await getTTSVoices(fetcher, 'local-tts');

    expect(voices.providerName).toBe('Local TTS token=[redacted]');
    expect(voices.voices[0].label).toBe('Amy token=[redacted]');
    const serialized = JSON.stringify(voices);
    expect(serialized).not.toContain('credentialSecretRef');
    expect(serialized).not.toContain('endpoint');
    expect(serialized).not.toContain('providerPayload');
    expect(serialized).not.toContain('JUTE_TTS_TOKEN');
    expect(serialized).not.toContain('secret-value');
    expect(serialized).not.toContain('voice-secret');
    expect(serialized).not.toContain('sk-voice-secret');
  });
});

describe('adapter connection kinds', () => {
  it('loads typed connection setup metadata from the hub', async () => {
    const fetcher = vi.fn(async () =>
      Response.json({
        kinds: [
          {
            kind: 'spotify',
            displayName: 'Spotify Account',
            fields: [
              {
                id: 'client_secret',
                label: 'Client secret reference',
                type: 'string',
                required: true,
                secret: true
              }
            ]
          }
        ]
      })
    ) as unknown as typeof fetch;

    const kinds = await getAdapterConnectionKinds(fetcher);

    expect(fetcher).toHaveBeenCalledWith(
      `${API_BASE}/api/v1/settings/connection-kinds`
    );
    expect(kinds[0].fields[0]).toMatchObject({
      id: 'client_secret',
      required: true,
      secret: true
    });
  });
});

describe('spotify auth URL', () => {
  it('points setup to the hub-owned OAuth route', () => {
    expect(spotifyAuthURL('spotify-main')).toBe(
      `${API_BASE}/api/v1/integrations/spotify/auth?connectionId=spotify-main`
    );
    expect(spotifyAuthURL('spotify-main', 'spotify-widget-1')).toBe(
      `${API_BASE}/api/v1/integrations/spotify/auth?connectionId=spotify-main&widgetInstanceId=spotify-widget-1`
    );
    expect(
      spotifyAuthURL('spotify-main', undefined, 'https://localhost:5173')
    ).toBe(
      `${API_BASE}/api/v1/integrations/spotify/auth?connectionId=spotify-main&returnUri=https%3A%2F%2Flocalhost%3A5173`
    );
  });

  it('uses the hub loopback callback as Spotify redirect URI', () => {
    expect(spotifyOAuthRedirectURI()).toBe(
      `${API_BASE}/api/v1/integrations/spotify/callback`
    );
  });

  it('parses and cleans display callback query strings', () => {
    expect(spotifyCallbackParams('?code=abc&state=xyz')).toEqual({
      code: 'abc',
      state: 'xyz'
    });
    expect(
      spotifyCallbackDisplayURL(
        '/',
        '?code=abc&state=xyz&theme=dark',
        '#top',
        'linked'
      )
    ).toBe('/?theme=dark&spotify=linked#top');
  });
});

describe('spotify web playback token', () => {
  it('loads a display-scoped token from the hub', async () => {
    const fetcher = vi.fn(async () =>
      Response.json({
        accessToken: 'access-token',
        expiresAt: 123,
        scope: 'streaming'
      })
    ) as unknown as typeof fetch;

    const token = await getSpotifyWebPlaybackToken(fetcher, 'spotify-main');

    expect(fetcher).toHaveBeenCalledWith(
      `${API_BASE}/api/v1/integrations/spotify/web-playback-token?connectionId=spotify-main`
    );
    expect(token).toEqual({
      accessToken: 'access-token',
      expiresAt: 123,
      scope: 'streaming'
    });
  });
});
