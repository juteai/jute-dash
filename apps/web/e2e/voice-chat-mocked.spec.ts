import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

test('chat mic button submits browser speech through hub voice transcript route', async ({
  page
}) => {
  await page.addInitScript(() => {
    class MockSpeechRecognition {
      continuous = false;
      interimResults = false;
      lang = '';
      onresult:
        | ((event: { resultIndex: number; results: ArrayLike<any> }) => void)
        | null = null;
      onerror: ((event: { error?: string }) => void) | null = null;
      onend: (() => void) | null = null;

      start() {
        const partial = Object.assign([{ transcript: 'turn on' }], {
          isFinal: false
        });
        const final = Object.assign([{ transcript: 'turn on the lights' }], {
          isFinal: true
        });
        setTimeout(() => {
          this.onresult?.({ resultIndex: 0, results: [partial] });
          this.onresult?.({ resultIndex: 0, results: [final] });
          this.onend?.();
        }, 0);
      }

      stop() {}
    }

    (
      window as Window &
        typeof globalThis & {
          SpeechRecognition: typeof MockSpeechRecognition;
          webkitSpeechRecognition: typeof MockSpeechRecognition;
        }
    ).SpeechRecognition = MockSpeechRecognition;
    (
      window as Window &
        typeof globalThis & {
          webkitSpeechRecognition: typeof MockSpeechRecognition;
        }
    ).webkitSpeechRecognition = MockSpeechRecognition;
  });
  const hub = await createMockHub(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Open chat' }).click();

  const mic = page.getByRole('button', { name: 'Wake listening' }).last();
  await expect(mic).toHaveAttribute('aria-pressed', 'false');
  await mic.click();
  await hub.expectWrite('POST', '/api/v1/voice/transcripts/final');
  await expect(
    page
      .getByRole('region', { name: 'Agent conversation' })
      .getByText('turn on the lights')
      .first()
  ).toBeVisible();
  await expect
    .poll(
      () =>
        hub.writes.find(
          (write) => write.path === '/api/v1/voice/transcripts/final'
        )?.body
    )
    .toMatchObject({ text: 'turn on the lights' });
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
