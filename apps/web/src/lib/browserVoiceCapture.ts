export type BrowserVoiceRecording = {
  audio: Blob;
  sampleRate: number;
  channels: number;
};

const TARGET_RATE = 16000;
const SILENCE_MS = 1200;
const MAX_MS = 15000;
const MIN_RMS = 0.015;

type WebKitWindow = Window &
  typeof globalThis & {
    webkitAudioContext?: typeof AudioContext;
  };

export async function captureBrowserVoicePCM(): Promise<BrowserVoiceRecording> {
  if (!navigator.mediaDevices?.getUserMedia) {
    throw new Error('Browser microphone access is unavailable.');
  }
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  const AudioContextCtor =
    window.AudioContext ?? (window as WebKitWindow).webkitAudioContext;
  if (!AudioContextCtor) {
    stopStream(stream);
    throw new Error('Browser audio capture is unavailable.');
  }

  const audioContext = new AudioContextCtor();
  const source = audioContext.createMediaStreamSource(stream);
  const processor = audioContext.createScriptProcessor(4096, 1, 1);
  const sink = audioContext.createGain();
  sink.gain.value = 0;

  return new Promise((resolve, reject) => {
    const chunks: ArrayBuffer[] = [];
    const startedAt = performance.now();
    let heardSpeech = false;
    let silenceStartedAt = 0;
    let done = false;

    function cleanup() {
      processor.disconnect();
      source.disconnect();
      sink.disconnect();
      void audioContext.close();
      stopStream(stream);
    }

    function finish() {
      if (done) return;
      done = true;
      cleanup();
      if (!heardSpeech || chunks.length === 0) {
        reject(new Error('No speech detected.'));
        return;
      }
      resolve({
        audio: new Blob(chunks, { type: 'application/octet-stream' }),
        sampleRate: TARGET_RATE,
        channels: 1
      });
    }

    processor.onaudioprocess = (event) => {
      const input = event.inputBuffer.getChannelData(0);
      const pcm = floatToPCM16(resample(input, audioContext.sampleRate));
      chunks.push(
        pcm.buffer.slice(pcm.byteOffset, pcm.byteOffset + pcm.byteLength)
      );

      const level = rms(input);
      const now = performance.now();
      if (level >= MIN_RMS) {
        heardSpeech = true;
        silenceStartedAt = 0;
      } else if (heardSpeech) {
        silenceStartedAt ||= now;
        if (now - silenceStartedAt >= SILENCE_MS) finish();
      }
      if (now - startedAt >= MAX_MS) finish();
    };

    source.connect(processor);
    processor.connect(sink);
    sink.connect(audioContext.destination);
  });
}

function stopStream(stream: MediaStream) {
  for (const track of stream.getTracks()) {
    track.stop();
  }
}

function rms(samples: Float32Array) {
  let sum = 0;
  for (const sample of samples) {
    sum += sample * sample;
  }
  return Math.sqrt(sum / samples.length);
}

function resample(samples: Float32Array, sourceRate: number) {
  if (sourceRate === TARGET_RATE) return samples;
  const ratio = sourceRate / TARGET_RATE;
  const length = Math.floor(samples.length / ratio);
  const out = new Float32Array(length);
  for (let i = 0; i < length; i += 1) {
    out[i] = samples[Math.min(samples.length - 1, Math.floor(i * ratio))];
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
