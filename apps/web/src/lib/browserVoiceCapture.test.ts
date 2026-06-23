import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createBrowserVoiceCaptureSession } from './browserVoiceCapture';

class FakeProcessor {
  onaudioprocess: ((event: AudioProcessingEvent) => void) | null = null;

  connect = vi.fn();
  disconnect = vi.fn();

  emit(samples: Float32Array) {
    this.onaudioprocess?.({
      inputBuffer: {
        getChannelData: () => samples
      }
    } as unknown as AudioProcessingEvent);
  }
}

class FakeAudioContext {
  static processors: FakeProcessor[] = [];
  static instances: FakeAudioContext[] = [];
  static options: AudioContextOptions[] = [];

  sampleRate = 16000;
  state: AudioContextState = 'running';
  destination = {};
  close = vi.fn();
  resume = vi.fn();

  constructor(options: AudioContextOptions = {}) {
    FakeAudioContext.options.push(options);
    FakeAudioContext.instances.push(this);
  }

  createMediaStreamSource() {
    return {
      connect: vi.fn(),
      disconnect: vi.fn()
    };
  }

  createScriptProcessor() {
    const processor = new FakeProcessor();
    FakeAudioContext.processors.push(processor);
    return processor;
  }

  createGain() {
    return {
      gain: { value: 1 },
      connect: vi.fn(),
      disconnect: vi.fn()
    };
  }
}

describe('BrowserVoiceCaptureSession', () => {
  let now = 0;
  let getUserMedia: ReturnType<typeof vi.fn>;
  let stopTrack: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.restoreAllMocks();
    FakeAudioContext.processors = [];
    FakeAudioContext.instances = [];
    FakeAudioContext.options = [];
    now = 0;
    vi.spyOn(performance, 'now').mockImplementation(() => now);
    stopTrack = vi.fn();
    getUserMedia = vi.fn(async () => ({
      getTracks: () => [{ stop: stopTrack }]
    }));
    vi.stubGlobal('navigator', {
      mediaDevices: { getUserMedia }
    });
    vi.stubGlobal('window', {
      AudioContext: FakeAudioContext
    });
  });

  it('reuses one microphone stream across utterances and stops tracks only on stop', async () => {
    const session = createBrowserVoiceCaptureSession();
    await session.start();

    const first = session.captureUtterance();
    await Promise.resolve();
    emitSpeechThenSilence();
    await expect(first).resolves.toMatchObject({
      sampleRate: 16000,
      channels: 1
    });

    const second = session.captureUtterance();
    await Promise.resolve();
    emitSpeechThenSilence();
    await expect(second).resolves.toMatchObject({
      sampleRate: 16000,
      channels: 1
    });

    expect(getUserMedia).toHaveBeenCalledTimes(1);
    expect(FakeAudioContext.options[0]).toEqual({ sampleRate: 16000 });
    expect(stopTrack).not.toHaveBeenCalled();
    expect(FakeAudioContext.instances[0].close).not.toHaveBeenCalled();

    session.stop();

    expect(stopTrack).toHaveBeenCalledTimes(1);
    expect(FakeAudioContext.instances[0].close).toHaveBeenCalledTimes(1);
  });

  it('rejects silent captures without closing the microphone session', async () => {
    const session = createBrowserVoiceCaptureSession();
    await session.start();
    const capture = session.captureUtterance();
    await Promise.resolve();

    now = 16_000;
    FakeAudioContext.processors[0].emit(new Float32Array([0, 0, 0]));

    await expect(capture).rejects.toThrow('No speech detected.');
    expect(getUserMedia).toHaveBeenCalledTimes(1);
    expect(stopTrack).not.toHaveBeenCalled();
    expect(FakeAudioContext.instances[0].close).not.toHaveBeenCalled();

    session.stop();
  });

  function emitSpeechThenSilence() {
    now += 10;
    FakeAudioContext.processors[0].emit(new Float32Array([0.25, 0.2, 0.15]));
    now += 10;
    FakeAudioContext.processors[0].emit(new Float32Array([0, 0, 0]));
    now += 1300;
    FakeAudioContext.processors[0].emit(new Float32Array([0, 0, 0]));
  }
});
