import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

test('no-agent chat state is calm and composer is disabled', async ({
  page
}) => {
  await createMockHub(page, { agents: 'none' });
  await page.goto('/');

  await page.getByRole('button', { name: 'Open chat' }).click();
  await expect(page.getByText('No agent connected')).toBeVisible();
  await expect(page.getByPlaceholder('Ask your home assistant')).toBeDisabled();
});

test('agent settings add, refresh, disable, and remove use hub APIs', async ({
  page
}) => {
  const hub = await createMockHub(page, { agents: 'none' });
  await page.goto('/');

  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'Agents' }).click();
  await page
    .getByPlaceholder(/agent-card/)
    .fill('http://127.0.0.1:9797/.well-known/agent-card.json');
  await page.getByRole('button', { name: 'Add agent' }).click();
  await hub.expectWrite('POST', '/api/v1/agents');
  const settings = page.getByLabel('Jute settings');
  await expect(settings.getByText('House Agent')).toBeVisible();

  await settings.getByRole('button', { name: 'Refresh' }).click();
  await hub.expectWrite('POST', '/api/v1/agents/house/refresh-card');
  await settings.getByRole('button', { name: 'Disable' }).click();
  await hub.expectWrite('PATCH', '/api/v1/agents/house');
  await settings.getByRole('button', { name: 'Remove' }).click();
  await hub.expectWrite('DELETE', '/api/v1/agents/house');
});

test('chat success and failure stay inside the hub proxy boundary', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');
  await page.getByRole('button', { name: 'Open chat' }).click();
  await page.getByPlaceholder('Ask your home assistant').fill('hello');
  await page.getByRole('button', { name: 'Send' }).click();
  await expect(
    page.getByText('Mock A2A reply from the local hub.')
  ).toBeVisible();
  await hub.expectWrite('POST', '/api/v1/proxy/agents/house');
});

test('chat failure shows safe copy and no raw credentials', async ({
  page
}) => {
  await createMockHub(page, { chatFailure: true });
  await page.goto('/');
  await page.getByRole('button', { name: 'Open chat' }).click();
  await page.getByPlaceholder('Ask your home assistant').fill('fail please');
  await page.getByRole('button', { name: 'Send' }).click();
  await expect(
    page.getByText('Message not sent', { exact: true })
  ).toBeVisible();
  await expect(
    page.getByText(/env:AGENT_TOKEN|credential failed/i)
  ).toHaveCount(0);
});

test('Spotify callback is deterministic and does not store raw token material', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/?code=oauth-code&state=oauth-state');
  await expect(page).toHaveURL(/spotify=linked/);
  await hub.storageShouldNotContain('oauth-code');
  await hub.storageShouldNotContain('refresh_token');
});
