import { describe, expect, it } from 'vitest';
import {
  fallbackDashboard,
  getTTSVoices,
  getVoiceProviders,
  getVoiceSatellites,
  initialDashboard,
  submitVoiceFinalTranscript,
  updateVoiceSatellite
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

  it('posts final browser spike transcripts to the hub voice API', async () => {
    const fetcher = async (url: string | URL | Request, init?: RequestInit) =>
      new Response(
        JSON.stringify({
          conversation: { id: 'conversation-1' },
          followup: { active: true, turns: 1, maxTurns: 5 }
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        }
      );
    const calls: Array<{ url: string | URL | Request; init?: RequestInit }> =
      [];
    const recordingFetcher = (async (
      url: string | URL | Request,
      init?: RequestInit
    ) => {
      calls.push({ url, init });
      return fetcher(url, init);
    }) as typeof fetch;

    const response = await submitVoiceFinalTranscript(recordingFetcher, {
      text: 'turn the kitchen lights on',
      deviceProfileId: 'browser-spike',
      deviceId: 'browser-spike-display'
    });

    expect(response.followup.active).toBe(true);
    expect(String(calls[0].url)).toContain('/api/v1/voice/transcripts/final');
    expect(calls[0].init?.method).toBe('POST');
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({
      text: 'turn the kitchen lights on',
      deviceProfileId: 'browser-spike',
      deviceId: 'browser-spike-display'
    });
  });

  it('reads safe voice satellite projections from the hub', async () => {
    const fetcher = (async () =>
      new Response(
        JSON.stringify({
          satellites: [
            {
              id: 'sat-kitchen',
              displayName:
                'Kitchen Satellite http://provider.local?token=secret',
              roomLabel: 'Kitchen secret:room-token',
              deviceProfileId: 'kitchen-display password:profile-secret',
              enabled: true,
              status: 'stack trace with token=secret',
              version: '0.1.0 apiKey=version-secret',
              pairedAt: '2026-06-15T08:00:00Z',
              lastSeenAt: '2026-06-15T08:05:00Z',
              credentialSecretRef: 'secret-ref:JUTE_SATELLITE_TOKEN',
              rawTranscript: 'raw transcript from the kitchen',
              providerEndpointUrl: 'http://provider.local?token=secret',
              lastError: 'stack trace with token=secret',
              createdAt: '2026-06-15T08:00:00Z',
              updatedAt: '2026-06-15T08:05:00Z'
            }
          ]
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        }
      )) as typeof fetch;

    const satellites = await getVoiceSatellites(fetcher);

    expect(satellites[0].displayName).toBe(
      'Kitchen Satellite [redacted-url]'
    );
    expect(satellites[0].roomLabel).toBe('Kitchen secret=[redacted]');
    expect(satellites[0].deviceProfileId).toBe(
      'kitchen-display password=[redacted]'
    );
    expect(satellites[0].status).toBe('misconfigured');
    expect(satellites[0].version).toBe('0.1.0 api_key=[redacted]');
    expect(JSON.stringify(satellites)).not.toContain('credential');
    expect(JSON.stringify(satellites)).not.toContain('secret-ref');
    expect(JSON.stringify(satellites)).not.toContain('raw transcript');
    expect(JSON.stringify(satellites)).not.toContain('provider.local');
    expect(JSON.stringify(satellites)).not.toContain('token=secret');
    expect(JSON.stringify(satellites)).not.toContain('room-token');
    expect(JSON.stringify(satellites)).not.toContain('profile-secret');
    expect(JSON.stringify(satellites)).not.toContain('version-secret');
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
              transportType: 'wyoming',
              capabilities: {
                streaming: true,
                partialTranscripts: true,
                offline: true,
                languages: ['en-GB'],
                inputFormats: ['audio/pcm;rate=16000'],
                outputFormats: ['text/plain']
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
              styles: ['neutral', 'token=style-secret'],
              outputFormats: ['audio/wav', 'apiKey=format-secret'],
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
    expect(voices.voices[0].styles).toContain('token=[redacted]');
    const serialized = JSON.stringify(voices);
    expect(serialized).not.toContain('credentialSecretRef');
    expect(serialized).not.toContain('endpoint');
    expect(serialized).not.toContain('providerPayload');
    expect(serialized).not.toContain('JUTE_TTS_TOKEN');
    expect(serialized).not.toContain('secret-value');
    expect(serialized).not.toContain('voice-secret');
    expect(serialized).not.toContain('sk-voice-secret');
  });

  it('patches only explicit satellite settings', async () => {
    const calls: Array<{ url: string | URL | Request; init?: RequestInit }> =
      [];
    const fetcher = (async (
      url: string | URL | Request,
      init?: RequestInit
    ) => {
      calls.push({ url, init });
      return new Response(
        JSON.stringify({
          id: 'sat-kitchen',
          displayName: 'Kitchen Voice tcp://provider.local:10300',
          roomLabel: 'Kitchen',
          deviceProfileId: 'kitchen-display token=profile-secret',
          enabled: false,
          status: 'paired',
          credentialSecretRef: 'secret-ref:JUTE_SATELLITE_TOKEN',
          rawTranscript: 'raw transcript from the kitchen',
          providerEndpointUrl: 'http://provider.local?token=secret',
          pairedAt: '2026-06-15T08:00:00Z',
          createdAt: '2026-06-15T08:00:00Z',
          updatedAt: '2026-06-15T08:10:00Z'
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' }
        }
      );
    }) as typeof fetch;

    const satellite = await updateVoiceSatellite(fetcher, 'sat-kitchen', {
      displayName: 'Kitchen Voice',
      roomLabel: 'Kitchen',
      deviceProfileId: 'kitchen-display',
      enabled: false
    });

    expect(satellite.enabled).toBe(false);
    expect(satellite.displayName).toBe('Kitchen Voice [redacted-url]');
    expect(satellite.deviceProfileId).toBe(
      'kitchen-display token=[redacted]'
    );
    expect(JSON.stringify(satellite)).not.toContain('credential');
    expect(JSON.stringify(satellite)).not.toContain('secret-ref');
    expect(JSON.stringify(satellite)).not.toContain('raw transcript');
    expect(JSON.stringify(satellite)).not.toContain('provider.local');
    expect(JSON.stringify(satellite)).not.toContain('profile-secret');
    expect(String(calls[0].url)).toContain(
      '/api/v1/voice/satellites/sat-kitchen'
    );
    expect(calls[0].init?.method).toBe('PATCH');
    expect(JSON.parse(String(calls[0].init?.body))).toEqual({
      displayName: 'Kitchen Voice',
      roomLabel: 'Kitchen',
      deviceProfileId: 'kitchen-display',
      enabled: false
    });
  });
});
