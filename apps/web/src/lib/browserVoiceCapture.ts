export type BrowserVoiceRecording = {
  audio: Blob;
  sampleRate: number;
  channels: number;
};

const TARGET_RATE = 16000;
const SILENCE_MS = 1200;
const MAX_MS = 15000;
const MIN_RMS = 0.015;
const PRE_ROLL_MS = 750;
const SAMPLE_BYTES = 2;
const PRE_ROLL_BYTES =
  Math.ceil((TARGET_RATE * PRE_ROLL_MS) / 1000) * SAMPLE_BYTES;

type WebKitWindow = Window &
  typeof globalThis & {
    webkitAudioContext?: typeof AudioContext;
  };

type CaptureOptions = {
  signal?: AbortSignal;
};

type ActiveCapture = {
  chunks: ArrayBuffer[];
  startedAt: number;
  heardSpeech: boolean;
  silenceStartedAt?: number;
  resolve: (recording: BrowserVoiceRecording) => void;
  reject: (err: Error) => void;
  signal?: AbortSignal;
  abort: () => void;
};

type PreRollChunk = {
  buffer: ArrayBuffer;
  level: number;
};

export class BrowserVoiceCaptureSession {
  private stream?: MediaStream;
  private audioContext?: AudioContext;
  private source?: MediaStreamAudioSourceNode;
  private processor?: ScriptProcessorNode;
  private sink?: GainNode;
  private starting?: Promise<void>;
  private active?: ActiveCapture;
  private preRoll: PreRollChunk[] = [];
  private preRollBytes = 0;
  private stopped = true;

  async start(): Promise<void> {
    if (this.stream) return;
    if (this.starting) return this.starting;

    this.stopped = false;
    this.starting = this.open();
    try {
      await this.starting;
    } finally {
      this.starting = undefined;
    }
  }

  async captureUtterance(
    options: CaptureOptions = {}
  ): Promise<BrowserVoiceRecording> {
    await this.start();
    if (this.active) {
      throw new Error('Browser microphone is already capturing.');
    }
    if (options.signal?.aborted) {
      throw abortError();
    }

    return new Promise((resolve, reject) => {
      const preRoll = this.preRoll.map((chunk) => chunk.buffer.slice(0));
      const heardPreRollSpeech = this.preRoll.some(
        (chunk) => chunk.level >= MIN_RMS
      );
      const now = performance.now();
      const capture: ActiveCapture = {
        chunks: preRoll,
        startedAt: now,
        heardSpeech: heardPreRollSpeech,
        silenceStartedAt: heardPreRollSpeech ? now : undefined,
        resolve,
        reject,
        signal: options.signal,
        abort: () => this.rejectCapture(capture, abortError())
      };
      this.active = capture;
      options.signal?.addEventListener('abort', capture.abort, { once: true });
    });
  }

  cancelUtterance(): void {
    if (this.active) {
      this.rejectCapture(this.active, abortError());
    }
  }

  stop(): void {
    this.stopped = true;
    this.cancelUtterance();
    this.processor?.disconnect();
    this.source?.disconnect();
    this.sink?.disconnect();
    void this.audioContext?.close();
    if (this.stream) {
      stopStream(this.stream);
    }
    this.stream = undefined;
    this.audioContext = undefined;
    this.source = undefined;
    this.processor = undefined;
    this.sink = undefined;
    this.starting = undefined;
    this.preRoll = [];
    this.preRollBytes = 0;
  }

  private async open(): Promise<void> {
    if (!navigator.mediaDevices?.getUserMedia) {
      throw new Error('Browser microphone access is unavailable.');
    }
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    if (this.stopped) {
      stopStream(stream);
      throw abortError();
    }
    const AudioContextCtor =
      window.AudioContext ?? (window as WebKitWindow).webkitAudioContext;
    if (!AudioContextCtor) {
      stopStream(stream);
      throw new Error('Browser audio capture is unavailable.');
    }

    const audioContext = new AudioContextCtor({ sampleRate: TARGET_RATE });
    const source = audioContext.createMediaStreamSource(stream);
    const processor = audioContext.createScriptProcessor(4096, 1, 1);
    const sink = audioContext.createGain();
    sink.gain.value = 0;
    processor.onaudioprocess = (event) => this.processAudio(event);
    source.connect(processor);
    processor.connect(sink);
    sink.connect(audioContext.destination);
    if (this.stopped) {
      processor.disconnect();
      source.disconnect();
      sink.disconnect();
      void audioContext.close();
      stopStream(stream);
      throw abortError();
    }

    this.stream = stream;
    this.audioContext = audioContext;
    this.source = source;
    this.processor = processor;
    this.sink = sink;
    if (audioContext.state === 'suspended') {
      await audioContext.resume();
    }
  }

  private processAudio(event: AudioProcessingEvent): void {
    const audioContext = this.audioContext;
    if (!audioContext) return;

    const input = event.inputBuffer.getChannelData(0);
    const level = rms(input);
    const pcm = floatToPCM16(resample(input, audioContext.sampleRate));
    const buffer = pcm.buffer.slice(
      pcm.byteOffset,
      pcm.byteOffset + pcm.byteLength
    );
    this.pushPreRoll(buffer.slice(0), level);

    const capture = this.active;
    if (!capture) return;
    capture.chunks.push(buffer);

    const now = performance.now();
    if (level >= MIN_RMS) {
      capture.heardSpeech = true;
      capture.silenceStartedAt = undefined;
    } else if (capture.heardSpeech) {
      capture.silenceStartedAt ??= now;
      if (now - capture.silenceStartedAt >= SILENCE_MS) {
        this.resolveCapture(capture);
      }
    }
    if (now - capture.startedAt >= MAX_MS) {
      this.resolveCapture(capture);
    }
  }

  private pushPreRoll(buffer: ArrayBuffer, level: number): void {
    if (buffer.byteLength === 0) return;
    this.preRoll.push({ buffer, level });
    this.preRollBytes += buffer.byteLength;
    while (this.preRollBytes > PRE_ROLL_BYTES) {
      const removed = this.preRoll.shift();
      if (!removed) break;
      this.preRollBytes -= removed.buffer.byteLength;
    }
  }

  private resolveCapture(capture: ActiveCapture): void {
    if (this.active !== capture) return;
    this.active = undefined;
    capture.signal?.removeEventListener('abort', capture.abort);
    if (!capture.heardSpeech || capture.chunks.length === 0) {
      capture.reject(new Error('No speech detected.'));
      return;
    }
    capture.resolve({
      audio: new Blob(capture.chunks, { type: 'application/octet-stream' }),
      sampleRate: TARGET_RATE,
      channels: 1
    });
  }

  private rejectCapture(capture: ActiveCapture, err: Error): void {
    if (this.active !== capture) return;
    this.active = undefined;
    capture.signal?.removeEventListener('abort', capture.abort);
    capture.reject(err);
  }
}

export function createBrowserVoiceCaptureSession(): BrowserVoiceCaptureSession {
  return new BrowserVoiceCaptureSession();
}

function abortError(): Error {
  if (typeof DOMException !== 'undefined') {
    return new DOMException('Voice capture canceled.', 'AbortError');
  }
  const err = new Error('Voice capture canceled.');
  err.name = 'AbortError';
  return err;
}

function stopStream(stream: MediaStream) {
  for (const track of stream.getTracks()) {
    track.stop();
  }
}

function rms(samples: Float32Array) {
  if (samples.length === 0) return 0;
  let sum = 0;
  for (const sample of samples) {
    sum += sample * sample;
  }
  return Math.sqrt(sum / samples.length);
}

function resample(samples: Float32Array, sourceRate: number) {
  if (sourceRate === TARGET_RATE) return samples;
  if (sourceRate <= 0 || samples.length === 0) return new Float32Array();
  const length = Math.max(
    1,
    Math.round((samples.length * TARGET_RATE) / sourceRate)
  );
  const out = new Float32Array(length);
  if (length === 1) {
    out[0] = samples[0] ?? 0;
    return out;
  }
  const scale = (samples.length - 1) / (length - 1);
  for (let i = 0; i < length; i += 1) {
    const position = i * scale;
    const left = Math.floor(position);
    const right = Math.min(samples.length - 1, left + 1);
    const fraction = position - left;
    out[i] = samples[left] + (samples[right] - samples[left]) * fraction;
  }
  return out;
}

function floatToPCM16(samples: Float32Array) {
  const pcm = new Int16Array(samples.length);
  for (let i = 0; i < samples.length; i += 1) {
    const sample = Math.max(-1, Math.min(1, samples[i]));
    pcm[i] = sample < 0 ? sample * 0x8000 : sample * 0x7fff;
  }
  return pcm;
}
