import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

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
    payload: {}
  });

  await expect(
    page.getByRole('region', { name: 'Agent conversation' })
  ).toBeVisible();
});

test('dashboard wake starts command capture in chat', async ({ page }) => {
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
  const hub = await createMockHub(page);
  await page.goto('/');
  await hub.waitForEventStream();
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
  await page.evaluate(() => {
    (
      window as Window & typeof globalThis & { __juteMicRequested?: boolean }
    ).__juteMicRequested = false;
  });

  await hub.emit('voice.wake_detected', {
    id: 'wake-command-capture',
    conversationId: 'conversation-wake-command',
    payload: {}
  });

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

test('chat keeps wake listening active', async ({ page }) => {
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
