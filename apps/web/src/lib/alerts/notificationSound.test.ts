import { describe, expect, it, vi } from 'vitest';
import {
  SUPPORTED_NOTIFICATION_SOUNDS,
  normalizeNotificationSound,
  playNotificationSound
} from './notificationSound';

describe('notificationSound', () => {
  it('declares supported sound names and fallback behavior', () => {
    expect(SUPPORTED_NOTIFICATION_SOUNDS).toEqual([
      'chime',
      'bell',
      'pulse',
      'soft',
      'none'
    ]);
    expect(normalizeNotificationSound('BELL')).toBe('bell');
    expect(normalizeNotificationSound('gong', 'soft')).toBe('soft');
  });

  it('does not play the none sound', () => {
    expect(playNotificationSound('none', fakeAudioWindow())).toBe(false);
  });

  it('maps supported sounds to oscillator settings', () => {
    const fake = fakeAudioWindow();

    expect(playNotificationSound('pulse', fake)).toBe(true);

    const ctx = fake.contexts[0];
    expect(ctx.oscillators).toHaveLength(3);
    expect(ctx.oscillators[0].type).toBe('square');
    expect(ctx.oscillators[0].frequency.value).toBe(520);
    expect(fake.setTimeout).toHaveBeenCalledWith(expect.any(Function), 900);
  });
});

function fakeAudioWindow() {
  const contexts: FakeAudioContext[] = [];
  class FakeAudioContext {
    currentTime = 10;
    destination = {};
    oscillators: FakeOscillator[] = [];
    close = vi.fn();

    constructor() {
      contexts.push(this);
    }

    createGain() {
      return {
        gain: { value: 0 },
        connect: vi.fn()
      };
    }

    createOscillator() {
      const oscillator = new FakeOscillator();
      this.oscillators.push(oscillator);
      return oscillator;
    }
  }

  class FakeOscillator {
    type = 'sine';
    frequency = { value: 0 };
    connect = vi.fn();
    start = vi.fn();
    stop = vi.fn();
  }

  return {
    AudioContext: FakeAudioContext as unknown as typeof AudioContext,
    setTimeout: vi.fn(),
    contexts
  };
}
