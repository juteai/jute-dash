import { get } from 'svelte/store';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/environment', () => ({
  browser: true
}));

vi.mock('$lib/hubClient', () => ({
  eventsURL: () => 'http://127.0.0.1:8787/api/v1/events',
  getDashboard: vi.fn(),
  muteVoice: vi.fn(),
  unmuteVoice: vi.fn(),
  cancelVoice: vi.fn()
}));

vi.mock('$lib/logger', () => ({
  logger: {
    sse: vi.fn(),
    sseError: vi.fn()
  }
}));

type Listener = (event: { type: string; data: string }) => void;

class FakeEventSource {
  static instances: FakeEventSource[] = [];

  listeners = new Map<string, Listener[]>();

  constructor(public url: string) {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: Listener) {
    const existing = this.listeners.get(type) ?? [];
    existing.push(listener);
    this.listeners.set(type, existing);
  }

  emit(type: string, payload: unknown) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener({ type, data: JSON.stringify(payload) });
    }
  }

  close() {}
}

describe('hubStream voice events', () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    FakeEventSource.instances = [];
    vi.stubGlobal('EventSource', FakeEventSource);
    vi.stubGlobal('window', {
      setInterval: vi.fn(() => 1),
      clearInterval: vi.fn(),
      setTimeout: vi.fn(() => 1),
      clearTimeout: vi.fn(),
      fetch: vi.fn()
    });
  });

  it('builds voice conversation sheet state from hub events without logging transcript text', async () => {
    const { hubStream } = await import('./hubStream');
    const { logger } = await import('$lib/logger');

    hubStream.connect(vi.fn() as unknown as typeof fetch);
    const source = FakeEventSource.instances[0];
    expect(source.url).toContain('/api/v1/events');

    source.emit('voice.wake_detected', {
      id: 'wake-1',
      conversationId: 'conversation-1',
      payload: {}
    });
    source.emit('voice.transcript.partial', {
      id: 'partial-1',
      conversationId: 'conversation-1',
      createdAt: '2026-06-15T10:00:00Z',
      payload: { text: 'turn on the kitchen' }
    });
    source.emit('voice.transcript.final', {
      id: 'transcript-1',
      conversationId: 'conversation-1',
      createdAt: '2026-06-15T10:00:00Z',
      payload: { text: 'turn on the kitchen lights' }
    });
    source.emit('conversation.turn_started', {
      id: 'turn-1',
      conversationId: 'conversation-1',
      payload: { status: 'working' }
    });
    source.emit('conversation.turn_completed', {
      id: 'turn-1',
      conversationId: 'conversation-1',
      createdAt: '2026-06-15T10:00:01Z',
      payload: { text: 'The kitchen lights are on.' }
    });
    source.emit('conversation.followup_started', {
      id: 'followup-1',
      conversationId: 'conversation-1',
      payload: { expiresAt: '2026-06-15T10:00:09Z' }
    });

    const state = get(hubStream);
    expect(state.showVoiceOverlay).toBe(true);
    expect(state.voiceConversationId).toBe('conversation-1');
    expect(state.voiceOrbState).toBe('followup');
    expect(state.voiceFollowupExpiresAt).toBe('2026-06-15T10:00:09Z');
    expect(state.voiceMessages).toMatchObject([
      {
        role: 'user',
        text: 'turn on the kitchen lights',
        status: 'final'
      },
      {
        role: 'assistant',
        text: 'The kitchen lights are on.',
        status: 'speaking'
      }
    ]);

    const logCalls = vi.mocked(logger.sse).mock.calls.flat().join(' ');
    expect(logCalls).toContain('chars=19');
    expect(logCalls).toContain('chars=26');
    expect(logCalls).not.toContain('turn on the kitchen ');
    expect(logCalls).not.toContain('turn on the kitchen lights');
    expect(logCalls).not.toContain('The kitchen lights are on.');
  });

  it('maps provider and playback failures to safe recoverable overlay errors', async () => {
    const { hubStream } = await import('./hubStream');

    hubStream.connect(vi.fn() as unknown as typeof fetch);
    const source = FakeEventSource.instances[0];

    source.emit('tts.failed', {
      id: 'tts-1',
      conversationId: 'conversation-1',
      payload: { reason: 'tts_failure' }
    });

    expect(get(hubStream)).toMatchObject({
      showVoiceOverlay: true,
      voiceOrbState: 'error',
      voiceError:
        'Speech playback is unavailable. The visual response is still available.'
    });

    source.emit('conversation.ended', {
      id: 'ended-1',
      conversationId: 'conversation-1',
      payload: { reason: 'agent_failure' }
    });

    expect(get(hubStream)).toMatchObject({
      showVoiceOverlay: true,
      voiceOrbState: 'error',
      voiceError: 'The agent could not complete that voice turn.'
    });
  });

  it('maps sensitive TTS policy stops to visual-only overlay status without transcript text', async () => {
    const { hubStream } = await import('./hubStream');

    hubStream.connect(vi.fn() as unknown as typeof fetch);
    const source = FakeEventSource.instances[0];

    source.emit('tts.stopped', {
      id: 'tts-visual-only-1',
      conversationId: 'conversation-1',
      payload: {
        state: 'visual_only',
        reason: 'sensitive_output_visual_only',
        text: 'the door code is 1234'
      }
    });

    const state = get(hubStream);
    expect(state).toMatchObject({
      showVoiceOverlay: true,
      voiceConversationId: 'conversation-1',
      voiceOrbState: 'followup',
      voiceError: 'Sensitive response is visual-only.'
    });
    expect(JSON.stringify(state)).not.toContain('door code');
    expect(JSON.stringify(state)).not.toContain('1234');
  });

  it('treats follow-up limit end as a normal voice session close', async () => {
    const { hubStream } = await import('./hubStream');

    hubStream.connect(vi.fn() as unknown as typeof fetch);
    const source = FakeEventSource.instances[0];

    source.emit('conversation.followup_started', {
      id: 'followup-1',
      conversationId: 'conversation-1',
      payload: { expiresAt: '2026-06-15T10:00:09Z', turns: 4, maxTurns: 5 }
    });
    source.emit('conversation.ended', {
      id: 'ended-1',
      conversationId: 'conversation-1',
      payload: { reason: 'followup_limit_reached', turns: 5, maxTurns: 5 }
    });

    expect(get(hubStream)).toMatchObject({
      showVoiceOverlay: true,
      voiceConversationId: 'conversation-1',
      voiceOrbState: 'idle',
      voiceError: ''
    });
  });
});
