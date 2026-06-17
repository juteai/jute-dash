import { expect, test } from '@playwright/test';

test('real stack smoke loads dashboard and chat entry point', async ({
  page
}) => {
  await page.goto('/');
  await expect(page.getByLabel('Widget dashboard')).toBeVisible({
    timeout: 30_000
  });
  await page.getByRole('button', { name: 'Open chat' }).click();
  await expect(page.getByLabel('Agent conversation')).toBeVisible();
  await expect(
    page.getByText(/Jute Mock A2A Agent|No agent connected|Ask Jute anything/)
  ).toBeVisible();
});
