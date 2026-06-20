type SpeechRecognitionResultLike = {
  readonly isFinal: boolean;
  readonly 0?: { readonly transcript?: string };
};

type SpeechRecognitionEventLike = {
  readonly resultIndex: number;
  readonly results: ArrayLike<SpeechRecognitionResultLike>;
};

type SpeechRecognitionLike = {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  onresult: ((event: SpeechRecognitionEventLike) => void) | null;
  onerror: ((event: { error?: string }) => void) | null;
  onend: (() => void) | null;
  start(): void;
  stop(): void;
};

type SpeechRecognitionConstructor = new () => SpeechRecognitionLike;

type SpeechWindow = Window &
  typeof globalThis & {
    SpeechRecognition?: SpeechRecognitionConstructor;
    webkitSpeechRecognition?: SpeechRecognitionConstructor;
  };

function speechRecognitionConstructor(
  win: SpeechWindow
): SpeechRecognitionConstructor | undefined {
  return win.SpeechRecognition ?? win.webkitSpeechRecognition;
}

export function browserSpeechSupported(win: SpeechWindow = window): boolean {
  return Boolean(speechRecognitionConstructor(win));
}

export function listenForBrowserSpeech({
  win = window,
  lang = 'en-GB',
  onPartial = () => {}
}: {
  win?: SpeechWindow;
  lang?: string;
  onPartial?: (text: string) => void;
} = {}): Promise<string> {
  const Constructor = speechRecognitionConstructor(win);
  if (!Constructor) {
    return Promise.reject(new Error('browser speech recognition unavailable'));
  }

  const recognition = new Constructor();
  recognition.continuous = false;
  recognition.interimResults = true;
  recognition.lang = lang;

  return new Promise((resolve, reject) => {
    let finalTranscript = '';
    let settled = false;

    function settle(callback: () => void) {
      if (settled) return;
      settled = true;
      callback();
    }

    recognition.onresult = (event) => {
      for (let i = event.resultIndex; i < event.results.length; i += 1) {
        const result = event.results[i];
        const text = result?.[0]?.transcript?.trim() ?? '';
        if (!text) continue;
        if (result.isFinal) {
          finalTranscript = `${finalTranscript} ${text}`.trim();
        } else {
          onPartial(text);
        }
      }
    };
    recognition.onerror = (event) => {
      settle(() =>
        reject(new Error(event.error || 'browser speech recognition failed'))
      );
    };
    recognition.onend = () => {
      settle(() => {
        if (finalTranscript) {
          resolve(finalTranscript);
        } else {
          reject(new Error('no speech detected'));
        }
      });
    };

    recognition.start();
  });
}
