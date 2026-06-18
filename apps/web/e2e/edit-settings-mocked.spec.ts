import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

test('edit mode adds widgets, configures headless, saves, and resets through hub', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Edit dashboard' }).click();
  await expect(page.getByRole('status')).toContainText('Edit dashboard');

  await page.getByRole('button', { name: 'Add widget' }).click();
  await page.getByRole('button', { name: 'Headless' }).first().click();
  await expect(page.getByLabel('Headless widgets')).toContainText('Weather');

  await page.getByRole('button', { name: 'Configure' }).click();
  await expect(page.getByLabel(/Configure/)).toBeVisible();
  await page.getByLabel('Title').fill('Outdoor weather');
  await page.getByRole('button', { name: 'Save' }).click();

  await page.getByRole('button', { name: 'Done' }).click();
  await hub.expectWrite('PUT', '/api/v1/widgets/layout');

  await page.getByRole('button', { name: 'Edit dashboard' }).click();
  await page.getByRole('button', { name: 'Reset' }).click();
  await hub.expectWrite('POST', '/api/v1/widgets/layout/reset');
});

test('phone edit mode uses reorder menu and stale state disables hub writes', async ({
  page
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await createMockHub(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Edit dashboard' }).click();

  await page
    .getByRole('button', { name: /Widget options for/ })
    .first()
    .click();
  await expect(page.getByRole('menuitem', { name: 'Move down' })).toBeVisible();

  await page.evaluate(() =>
    (
      window as Window &
        typeof globalThis & { __juteMockSSE: { error(): void } }
    ).__juteMockSSE.error()
  );
  await expect(
    page.getByRole('status').filter({ hasText: 'Event stream disconnected' })
  ).toBeVisible();
});

test('settings writes household, rooms, tiles, and connections through hub', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Settings' }).click();
  await page
    .getByLabel('Jute settings')
    .getByLabel('Home name')
    .fill('Jute QA Home');
  await page.getByRole('button', { name: 'Save household' }).click();
  await hub.expectWrite('PATCH', '/api/v1/settings/household');

  await page.getByRole('button', { name: 'Rooms' }).click();
  await page.getByLabel('Name').first().fill('Kitchen Lab');
  await page.getByRole('button', { name: 'Save rooms' }).click();
  await hub.expectWrite('PUT', '/api/v1/settings/rooms');

  await page.getByRole('button', { name: 'Tiles' }).click();
  await page.getByLabel('Label').first().fill('Back Door');
  await page.getByRole('button', { name: 'Save tiles' }).click();
  await hub.expectWrite('PUT', '/api/v1/settings/tiles');

  await page.getByRole('button', { name: 'Connections' }).click();
  const settings = page.getByLabel('Jute settings');
  await settings.getByRole('button', { name: 'New', exact: true }).click();
  await settings
    .getByRole('textbox', { name: 'ID', exact: true })
    .fill('hue-test');
  await settings
    .getByRole('textbox', { name: 'Name', exact: true })
    .fill('Hue Test');
  await settings.getByRole('button', { name: 'Save', exact: true }).click();
  await hub.expectWrite('PUT', '/api/v1/settings/connections');
});

test('settings saves voice provider selections through the hub', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Settings' }).click();
  const settings = page.getByLabel('Jute settings');
  await settings.getByRole('button', { name: 'Voice', exact: true }).click();

  await settings.getByLabel('Wake provider').selectOption('local-wake');
  await settings.getByLabel('STT provider').selectOption('local-stt');
  await settings.getByLabel('STT model').fill('tiny-en');
  await settings.getByLabel('TTS provider').selectOption('local-tts');
  await settings.getByLabel('TTS voice').selectOption('amy');
  await settings.getByLabel('Command providers').check();
  await settings.getByRole('button', { name: 'Save voice' }).click();

  await expect
    .poll(
      () =>
        hub.writes.find((write) => write.path === '/api/v1/voice/settings')
          ?.body
    )
    .toMatchObject({
      wakeWordModelId: 'hey-jute',
      wakeWordPhrase: 'Hey Jute',
      sttProviderId: 'local-stt',
      sttModelId: 'tiny-en',
      ttsProviderId: 'local-tts',
      ttsVoiceId: 'amy',
      commandProvidersEnabled: true
    });
});
