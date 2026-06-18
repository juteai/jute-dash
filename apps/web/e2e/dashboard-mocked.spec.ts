import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

test('startup offline shows retry, then dashboard loads without overflow', async ({
  page
}) => {
  let online = false;
  const hub = await createMockHub(page);
  await page.route('**/api/v1/**', async (route) => {
    if (!online) {
      await route.abort('failed');
      return;
    }
    await route.fallback();
  });

  await page.goto('/');
  await expect(
    page.getByRole('heading', { name: 'Hub not reachable' })
  ).toBeVisible();
  await expect(page.getByText(/Jute Dash cannot connect/)).toBeVisible();

  online = true;
  await page.getByRole('button', { name: /Retry/ }).click();
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();
  await expect(page.getByText('Kitchen')).toBeVisible();

  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth > window.innerWidth + 1
  );
  expect(overflow).toBe(false);
  await hub.storageShouldNotContain('AGENT_TOKEN');
});

test('SSE events drive degraded, notification, focus, and voice states', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();

  await hub.emit('display.notification', {
    id: 'note-1',
    severity: 'warning',
    message: 'Garage door is still open',
    createdAt: new Date().toISOString(),
    expiresAt: new Date(Date.now() + 60_000).toISOString()
  });
  await expect(page.getByText('Garage door is still open')).toBeVisible();

  await hub.emit('display.focus_widget', {
    id: 'focus-1',
    widgetInstanceId: 'weather',
    createdAt: new Date().toISOString()
  });
  await expect(
    page.locator('[data-widget-id="weather"] .widget-frame--focused')
  ).toBeVisible();

  await hub.emit('voice.wake_detected', {});
  await hub.emit('voice.transcript.partial', {
    payload: { text: 'turn on the kitchen lights' }
  });
  await expect(page.getByText('turn on the kitchen lights')).toBeVisible();
  await hub.emit('voice.transcript.final', {
    id: 'transcript-1',
    conversationId: 'conversation-1',
    payload: { text: 'turn on the kitchen lights' }
  });
  await hub.emit('conversation.turn_completed', {
    id: 'turn-1',
    conversationId: 'conversation-1',
    payload: { text: 'The kitchen lights are on.' }
  });
  await expect(page.getByText('The kitchen lights are on.')).toBeVisible();

  await hub.emit('tts.failed', {
    id: 'tts-1',
    conversationId: 'conversation-1',
    payload: { reason: 'tts_failure' }
  });
  await expect(
    page.getByText(
      'Speech playback is unavailable. The visual response is still available.'
    )
  ).toBeVisible();

  await hub.eventStreamError();
  await expect(page.getByRole('status')).toContainText(
    'Event stream disconnected'
  );
});

for (const widgetState of [
  'empty',
  'loading',
  'unavailable',
  'error',
  'permission_required',
  'issue'
] as const) {
  test(`widget frame renders ${widgetState} state safely`, async ({ page }) => {
    await createMockHub(page, { widgetState });
    await page.goto('/');
    await expect(page.getByLabel('Widget dashboard')).toBeVisible();
    await expect(
      page.getByText(
        /No Data|Loading|Unavailable|Widget Error|Access Blocked|Connection needed/
      )
    ).toBeVisible();
    await expect(
      page.getByText(/env:|token|stack trace|127\.0\.0\.1:9797\/invoke\?/i)
    ).toHaveCount(0);
  });
}
