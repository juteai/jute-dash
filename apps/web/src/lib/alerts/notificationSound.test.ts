import { describe, expect, it, vi } from 'vitest';
import {
  SUPPORTED_NOTIFICATION_SOUNDS,
  normalizeNotificationSound,
  playNotificationSound,
  startNotificationSoundLoop
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

  it('repeats sounds until stopped', () => {
    const fake = fakeAudioWindow();

    const loop = startNotificationSoundLoop('alert-1', 'bell', fake, 1400);

    expect(loop?.key).toBe('alert-1');
    expect(fake.contexts).toHaveLength(1);
    expect(fake.setInterval).toHaveBeenCalledWith(expect.any(Function), 1400);

    fake.intervalHandlers[0]?.();
    expect(fake.contexts).toHaveLength(2);

    loop?.stop();
    expect(fake.clearInterval).toHaveBeenCalledWith(1);
  });

  it('does not start a repeat loop for none', () => {
    const fake = fakeAudioWindow();

    expect(startNotificationSoundLoop('alert-1', 'none', fake)).toBeUndefined();
    expect(fake.setInterval).not.toHaveBeenCalled();
  });
});

function fakeAudioWindow() {
  const contexts: FakeAudioContext[] = [];
  const intervalHandlers: Array<() => void> = [];
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
    clearInterval: vi.fn(),
    AudioContext: FakeAudioContext as unknown as typeof AudioContext,
    setInterval: vi.fn((handler: () => void) => {
      intervalHandlers.push(handler);
      return 1;
    }),
    setTimeout: vi.fn(),
    intervalHandlers,
    contexts
  };
}
