import { describe, expect, it } from 'vitest';
import {
  fallbackDashboard,
  getTTSVoices,
  getVoiceProviders,
  initialDashboard,
  submitVoiceFinalTranscript
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
});
