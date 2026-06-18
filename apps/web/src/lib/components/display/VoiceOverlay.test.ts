import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import type { VoiceStatus } from '$lib/types';
import VoiceOverlay from './VoiceOverlay.svelte';

const readyVoice: VoiceStatus = {
  enabled: true,
  muted: false,
  state: 'wake_listening',
  serviceStatus: 'ready',
  deviceProfileId: 'kitchen-display',
  wakeWordModelId: 'hey-jute',
  wakeWordPhrase: 'Hey Jute',
  wakeSensitivity: 0.5,
  sttProviderId: 'local-stt',
  ttsProviderId: 'piper-local',
  sttModelId: '',
  ttsModelId: '',
  ttsVoiceId: '',
  ttsEnabled: true,
  ttsLocale: 'en',
  ttsSpeed: 1,
  ttsVolume: 1,
  preferredAgentId: 'house',
  cloudOptIn: false,
  commandProvidersEnabled: false,
  followupWindowSeconds: 8,
  microphoneProfile: '',
  updatedAt: '2026-06-16T10:00:00Z'
};

describe('VoiceOverlay', () => {
  it('hides conversation text in the ambient overlay by default', () => {
    const { body } = render(VoiceOverlay, {
      props: {
        voice: readyVoice,
        voiceOrbState: 'speaking',
        voiceMessages: [
          {
            id: 'user-1',
            role: 'user',
            text: 'unlock the side door',
            createdAt: '2026-06-16T10:00:01Z',
            status: 'final'
          },
          {
            id: 'assistant-1',
            role: 'assistant',
            text: 'The side door is unlocked.',
            createdAt: '2026-06-16T10:00:02Z',
            status: 'speaking'
          }
        ],
        voiceTranscript: 'unlock the side door',
        assistantSpeech: 'The side door is unlocked.'
      }
    });

    expect(body).toContain('Speaking');
    expect(body).not.toContain('unlock the side door');
    expect(body).not.toContain('The side door is unlocked.');
  });

  it('renders conversation text only when explicitly enabled', () => {
    const { body } = render(VoiceOverlay, {
      props: {
        voice: readyVoice,
        voiceOrbState: 'speaking',
        showConversationText: true,
        voiceMessages: [
          {
            id: 'assistant-1',
            role: 'assistant',
            text: 'The porch light is on.',
            createdAt: '2026-06-16T10:00:02Z',
            status: 'speaking'
          }
        ]
      }
    });

    expect(body).toContain('The porch light is on.');
  });

  it('renders safe recoverable errors inline', () => {
    const { body } = render(VoiceOverlay, {
      props: {
        voice: readyVoice,
        voiceOrbState: 'error',
        voiceError: 'The selected voice provider is unavailable.'
      }
    });

    expect(body).toContain('The selected voice provider is unavailable.');
  });

  it('redacts raw provider details from inline voice errors', () => {
    const { body } = render(VoiceOverlay, {
      props: {
        voice: readyVoice,
        voiceOrbState: 'error',
        voiceError:
          'dial tcp 127.0.0.1:10500: token=secret failed for https://voice.example.test'
      }
    });

    expect(body).toContain('Voice needs attention. Check provider settings.');
    expect(body).not.toContain('127.0.0.1:10500');
    expect(body).not.toContain('token=secret');
    expect(body).not.toContain('https://voice.example.test');
    expect(body).not.toContain('dial tcp');
  });

  it('redacts credential-like provider metadata from the ambient footer', () => {
    const { body } = render(VoiceOverlay, {
      props: {
        voice: {
          ...readyVoice,
          deviceProfileId: 'kitchen-display https://voice.example.test',
          sttProviderId: 'local-stt token=secret'
        },
        voiceOrbState: 'listening'
      }
    });

    expect(body).toContain('default display');
    expect(body).toContain('No STT provider');
    expect(body).not.toContain('https://voice.example.test');
    expect(body).not.toContain('token=secret');
  });
});
