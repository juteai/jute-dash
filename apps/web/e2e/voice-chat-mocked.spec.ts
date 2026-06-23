import { expect, test, type Page } from '@playwright/test';
import { createMockHub } from './mockHub';

async function installReusableMicStub(page: Page) {
  await page.addInitScript(() => {
    type MicWindow = Window &
      typeof globalThis & {
        __juteMicRequestCount?: number;
        __juteMicStopCount?: number;
      };
    const w = window as MicWindow;
    w.__juteMicRequestCount = 0;
    w.__juteMicStopCount = 0;
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        getUserMedia: async () => {
          w.__juteMicRequestCount = (w.__juteMicRequestCount ?? 0) + 1;
          return {
            getTracks: () => [
              {
                stop: () => {
                  w.__juteMicStopCount = (w.__juteMicStopCount ?? 0) + 1;
                }
              }
            ]
          };
        }
      }
    });
    class FakeAudioContext {
      sampleRate = 16000;
      state = 'running';
      destination = {};
      createMediaStreamSource() {
        return { connect() {}, disconnect() {} };
      }
      createScriptProcessor() {
        return { connect() {}, disconnect() {}, onaudioprocess: null };
      }
      createGain() {
        return { gain: { value: 1 }, connect() {}, disconnect() {} };
      }
      close() {}
      resume() {}
    }
    w.AudioContext = FakeAudioContext as unknown as typeof AudioContext;
  });
}

function micRequestCount(page: Page) {
  return page.evaluate(
    () =>
      (
        window as Window &
          typeof globalThis & { __juteMicRequestCount?: number }
      ).__juteMicRequestCount ?? 0
  );
}

function micStopCount(page: Page) {
  return page.evaluate(
    () =>
      (window as Window & typeof globalThis & { __juteMicStopCount?: number })
        .__juteMicStopCount ?? 0
  );
}

async function installAudioPlaybackStub(page: Page) {
  await page.addInitScript(() => {
    type AudioWindow = Window &
      typeof globalThis & {
        __jutePlayedAudioSrcs?: string[];
      };
    const w = window as AudioWindow;
    w.__jutePlayedAudioSrcs = [];
    class FakeAudio {
      constructor(public src: string) {}
      addEventListener() {}
      play() {
        w.__jutePlayedAudioSrcs?.push(this.src);
        return Promise.resolve();
      }
    }
    w.Audio = FakeAudio as unknown as typeof Audio;
  });
}

function playedAudioCount(page: Page) {
  return page.evaluate(
    () =>
      (
        window as Window &
          typeof globalThis & { __jutePlayedAudioSrcs?: string[] }
      ).__jutePlayedAudioSrcs?.length ?? 0
  );
}

test('chat renders hub-owned voice transcript events as speech bubbles', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();
  await hub.emit('voice.wake_detected', {
    id: 'wake-1',
    conversationId: 'conversation-1',
    payload: {}
  });
  await expect(
    page.getByRole('region', { name: 'Agent conversation' })
  ).toBeVisible();

  await hub.emit('voice.transcript.partial', {
    id: 'partial-1',
    conversationId: 'conversation-1',
    createdAt: '2026-06-15T10:00:00Z',
    payload: { text: 'turn on' }
  });
  await hub.emit('voice.transcript.final', {
    id: 'final-1',
    conversationId: 'conversation-1',
    createdAt: '2026-06-15T10:00:01Z',
    payload: { text: 'turn on the lights' }
  });

  await expect(
    page
      .getByRole('region', { name: 'Agent conversation' })
      .getByText('turn on the lights')
      .first()
  ).toBeVisible();
});

test('voice mute button is plain when listening and colored when muted', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');

  const listening = page.getByRole('button', { name: 'Wake listening' });
  await expect(listening).toHaveAttribute('aria-pressed', 'false');

  await listening.click();
  await hub.expectWrite('POST', '/api/v1/voice/mute');
  await expect(
    page.getByRole('button', { name: 'Voice muted' })
  ).toHaveAttribute('aria-pressed', 'true');
});

test('dashboard wake opens chat before transcription completes', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();

  await hub.emit('voice.wake_detected', {
    id: 'wake-before-stt',
    conversationId: 'conversation-before-stt',
    payload: {}
  });

  await expect(
    page.getByRole('region', { name: 'Agent conversation' })
  ).toBeVisible();
});

test('dismissed voice chat does not reopen for the same conversation', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();

  await hub.emit('voice.wake_detected', {
    id: 'wake-dismissed-chat',
    conversationId: 'conversation-dismissed-chat',
    payload: {}
  });

  const chat = page.getByRole('region', { name: 'Agent conversation' });
  await expect(chat).toBeVisible();
  await page.getByRole('button', { name: 'Close chat' }).click();
  await expect(chat).toHaveCount(0);

  await hub.emit('voice.transcript.partial', {
    id: 'partial-dismissed-chat',
    conversationId: 'conversation-dismissed-chat',
    payload: { text: 'still speaking' }
  });

  await expect(chat).toHaveCount(0);
});

test('wake listening without a conversation does not open chat', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();

  await hub.emit('voice.wake_detected', {
    id: 'wake-without-conversation',
    payload: {}
  });

  await expect(
    page.getByRole('region', { name: 'Agent conversation' })
  ).toHaveCount(0);
});

test('manual chat open does not create a conversation bubble', async ({
  page
}) => {
  await createMockHub(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Open chat' }).click();

  await expect(
    page.getByRole('region', { name: 'Agent conversation' })
  ).toBeVisible();
  await expect(page.getByText('Ready to start with')).toBeVisible();
  await expect(page.getByText('New Conversation')).toHaveCount(0);
});

test('dashboard wake reuses the active browser microphone in chat', async ({
  page
}) => {
  await installReusableMicStub(page);
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();
  await expect.poll(() => micRequestCount(page)).toBe(1);

  await hub.emit('voice.wake_detected', {
    id: 'wake-command-capture',
    conversationId: 'conversation-wake-command',
    payload: {}
  });

  await expect.poll(() => micRequestCount(page)).toBe(1);
  await expect(
    page.getByRole('region', { name: 'Agent conversation' })
  ).toBeVisible();
});

test('chat mic asks the browser for microphone access', async ({ page }) => {
  await page.addInitScript(() => {
    (
      window as Window & typeof globalThis & { __juteMicRequested?: boolean }
    ).__juteMicRequested = false;
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        getUserMedia: async () => {
          (
            window as Window &
              typeof globalThis & { __juteMicRequested?: boolean }
          ).__juteMicRequested = true;
          throw new Error('Microphone permission denied.');
        }
      }
    });
  });
  await createMockHub(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Open chat' }).click();
  await page.evaluate(() => {
    (
      window as Window & typeof globalThis & { __juteMicRequested?: boolean }
    ).__juteMicRequested = false;
  });
  await page
    .getByRole('button', { name: 'Start voice input' })
    .click({ force: true });

  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (
            window as Window &
              typeof globalThis & { __juteMicRequested?: boolean }
          ).__juteMicRequested
      )
    )
    .toBe(true);
  await expect(page.getByText('Microphone permission denied.')).toBeVisible();
});

test('dashboard wake listening asks the browser for microphone access', async ({
  page
}) => {
  await page.addInitScript(() => {
    (
      window as Window & typeof globalThis & { __juteMicRequested?: boolean }
    ).__juteMicRequested = false;
    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        getUserMedia: async () => {
          (
            window as Window &
              typeof globalThis & { __juteMicRequested?: boolean }
          ).__juteMicRequested = true;
          throw new Error('Microphone permission denied.');
        }
      }
    });
  });
  await createMockHub(page);
  await page.goto('/');

  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (
            window as Window &
              typeof globalThis & { __juteMicRequested?: boolean }
          ).__juteMicRequested
      )
    )
    .toBe(true);
});

test('mute stops the browser microphone and unmute starts one session', async ({
  page
}) => {
  await installReusableMicStub(page);
  await createMockHub(page);
  await page.goto('/');
  await expect.poll(() => micRequestCount(page)).toBe(1);

  await page.getByRole('button', { name: 'Wake listening' }).click();
  await expect.poll(() => micStopCount(page)).toBe(1);

  await page.getByRole('button', { name: 'Voice muted' }).click();
  await expect.poll(() => micRequestCount(page)).toBe(2);
});

test('tts completed event plays browser audio from the hub', async ({
  page
}) => {
  await installAudioPlaybackStub(page);
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();

  await hub.emit('tts.completed', {
    id: 'tts-browser-audio',
    conversationId: 'conversation-browser-audio',
    payload: { audioUrl: '/api/v1/tts/audio/tts-browser-audio' }
  });

  await expect.poll(() => playedAudioCount(page)).toBe(1);
});
