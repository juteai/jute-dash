export const SUPPORTED_NOTIFICATION_SOUNDS = [
  'chime',
  'bell',
  'pulse',
  'soft',
  'none'
] as const;

export type NotificationSound = (typeof SUPPORTED_NOTIFICATION_SOUNDS)[number];

type AudioContextConstructor = new () => AudioContext;
type TimerHandle = unknown;

type AudioWindow = {
  AudioContext?: AudioContextConstructor;
  clearInterval?: (id: TimerHandle) => void;
  setInterval?: (handler: () => void, timeout?: number) => TimerHandle;
  setTimeout: (handler: () => void, timeout?: number) => unknown;
  webkitAudioContext?: AudioContextConstructor;
};

export type NotificationSoundLoop = {
  key: string;
  stop: () => void;
};

const frequencyBySound: Record<Exclude<NotificationSound, 'none'>, number> = {
  chime: 660,
  bell: 880,
  pulse: 520,
  soft: 392
};

export function normalizeNotificationSound(
  value: unknown,
  fallback: NotificationSound = 'chime'
): NotificationSound {
  const sound = typeof value === 'string' ? value.trim().toLowerCase() : '';
  if (isNotificationSound(sound)) {
    return sound;
  }
  return fallback;
}

export function isNotificationSound(value: string): value is NotificationSound {
  return SUPPORTED_NOTIFICATION_SOUNDS.includes(value as NotificationSound);
}

export function playNotificationSound(
  value: unknown,
  audioWindow: AudioWindow | undefined = defaultAudioWindow()
): boolean {
  const sound = normalizeNotificationSound(value);
  if (sound === 'none' || !audioWindow) {
    return false;
  }

  const AudioContextClass =
    audioWindow.AudioContext || audioWindow.webkitAudioContext;
  if (!AudioContextClass) {
    return false;
  }

  const ctx = new AudioContextClass();
  const gain = ctx.createGain();
  gain.gain.value = 0.035;
  gain.connect(ctx.destination);

  const base = frequencyBySound[sound];
  for (const offset of [0, 0.18, 0.36]) {
    const osc = ctx.createOscillator();
    osc.type = sound === 'pulse' ? 'square' : 'sine';
    osc.frequency.value = base;
    osc.connect(gain);
    osc.start(ctx.currentTime + offset);
    osc.stop(ctx.currentTime + offset + 0.12);
  }
  audioWindow.setTimeout(() => void ctx.close(), 900);
  return true;
}

export function startNotificationSoundLoop(
  key: string,
  value: unknown,
  audioWindow: AudioWindow | undefined = defaultAudioWindow(),
  intervalMs = 1400
): NotificationSoundLoop | undefined {
  const sound = normalizeNotificationSound(value);
  if (sound === 'none' || !audioWindow?.setInterval) {
    return undefined;
  }

  playNotificationSound(sound, audioWindow);
  const interval = audioWindow.setInterval(() => {
    playNotificationSound(sound, audioWindow);
  }, intervalMs);

  return {
    key,
    stop: () => audioWindow.clearInterval?.(interval)
  };
}

function defaultAudioWindow(): AudioWindow | undefined {
  if (typeof window === 'undefined') {
    return undefined;
  }

  const browserWindow = window as typeof window & {
    webkitAudioContext?: AudioContextConstructor;
  };

  return {
    AudioContext: browserWindow.AudioContext,
    webkitAudioContext: browserWindow.webkitAudioContext,
    setTimeout: (handler, timeout) =>
      browserWindow.setTimeout(handler, timeout),
    setInterval: (handler, timeout) =>
      browserWindow.setInterval(handler, timeout),
    clearInterval: (id) => browserWindow.clearInterval(id as number)
  };
}
