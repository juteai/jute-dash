import { describe, expect, it, vi } from 'vitest';
import {
  browserSpeechSupported,
  listenForBrowserSpeech
} from './browserSpeech';

class FakeSpeechRecognition {
  static instance: FakeSpeechRecognition;

  continuous = false;
  interimResults = false;
  lang = '';
  onresult:
    | ((event: { resultIndex: number; results: ArrayLike<any> }) => void)
    | null = null;
  onerror: ((event: { error?: string }) => void) | null = null;
  onend: (() => void) | null = null;

  constructor() {
    FakeSpeechRecognition.instance = this;
  }

  start() {}
  stop() {}
}

function speechResult(transcript: string, isFinal: boolean) {
  return Object.assign([{ transcript }], { isFinal });
}

describe('browserSpeech', () => {
  it('returns the final browser transcript and reports partial text', async () => {
    const win = {
      webkitSpeechRecognition: FakeSpeechRecognition
    } as unknown as Window & typeof globalThis;
    const onPartial = vi.fn();

    expect(browserSpeechSupported(win)).toBe(true);

    const transcript = listenForBrowserSpeech({ win, onPartial });
    FakeSpeechRecognition.instance.onresult?.({
      resultIndex: 0,
      results: [speechResult('turn on', false)]
    });
    FakeSpeechRecognition.instance.onresult?.({
      resultIndex: 0,
      results: [speechResult('turn on the lights', true)]
    });
    FakeSpeechRecognition.instance.onend?.();

    await expect(transcript).resolves.toBe('turn on the lights');
    expect(onPartial).toHaveBeenCalledWith('turn on');
  });
});
