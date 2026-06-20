import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

test('chat mic button toggles hub-owned voice activation', async ({ page }) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Open chat' }).click();

  await page.getByRole('button', { name: 'Voice muted' }).last().click();
  await hub.expectWrite('POST', '/api/v1/voice/unmute');
  await expect(
    page.getByRole('button', { name: 'Wake listening' }).last()
  ).toBeVisible();

  await page.getByRole('button', { name: 'Wake listening' }).last().click();
  await hub.expectWrite('POST', '/api/v1/voice/mute');
  await expect(
    page.getByRole('button', { name: 'Voice muted' }).last()
  ).toBeVisible();
});
