import { expect, test } from '@playwright/test';
import { createMockHub } from './mockHub';

test('Spotify login opens the hub OAuth URL and stores no token material', async ({
  page
}) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, '__juteOpenedURL', {
      value: '',
      writable: true
    });
    window.open = ((url: string | URL | undefined) => {
      window.__juteOpenedURL = String(url ?? '');
      return {
        closed: false,
        focus() {}
      } as Window;
    }) as typeof window.open;
  });
  const hub = await createMockHub(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'Connections' }).click();
  await page
    .getByLabel('Jute settings')
    .getByRole('button', { name: 'Login with Spotify' })
    .click();

  await hub.expectWrite('PUT', '/api/v1/settings/connections');
  await expect
    .poll(() => page.evaluate(() => window.__juteOpenedURL))
    .not.toBe('');
  const openedURL = await page.evaluate(() => window.__juteOpenedURL);
  expect(openedURL).toContain('/api/v1/integrations/spotify/auth');
  expect(openedURL).toContain('connectionId=spotify-main');
  expect(openedURL).toContain('returnUri=http%3A%2F%2F127.0.0.1%3A5173');
  await expect(
    page.getByText(/access_token|refresh_token|client_secret/i)
  ).toHaveCount(0);
  await hub.storageShouldNotContain('mock-web-playback-token');
});

test('saving a single matching connection auto-links waiting widget refs', async ({
  page
}) => {
  const hub = await createMockHub(page, { layout: 'core-widgets' });
  hub.state.connections = [];
  clearWidgetConnectionRef(hub.state.layout, 'spotify');

  await page.goto('/');
  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'Connections' }).click();
  await page.getByRole('button', { name: 'Save', exact: true }).click();

  await hub.expectWrite('PUT', '/api/v1/settings/connections');
  await hub.expectWrite('PUT', '/api/v1/widgets/layout');
  const savedLayout = latestWrite(hub.writes, '/api/v1/widgets/layout')
    ?.body as
    | {
        widgets?: Array<{
          id: string;
          connectionRefs?: Record<string, string>;
        }>;
      }
    | undefined;
  expect(
    savedLayout?.widgets?.find((widget) => widget.id === 'spotify')
  ).toMatchObject({
    connectionRefs: { account: 'spotify-main' }
  });
});

test('widget settings can choose among multiple connector instances', async ({
  page
}) => {
  const hub = await createMockHub(page, { layout: 'core-widgets' });
  hub.state.connections = [
    {
      id: 'spotify-main',
      kind: 'spotify',
      name: 'Kitchen Spotify',
      settings: {},
      enabled: true
    },
    {
      id: 'spotify-office',
      kind: 'spotify',
      name: 'Office Spotify',
      settings: {},
      enabled: true
    }
  ];
  clearWidgetConnectionRef(hub.state.layout, 'spotify');

  await page.goto('/');
  await page.getByRole('button', { name: 'Edit dashboard' }).click();
  await page
    .getByRole('button', { name: 'Widget options for Spotify' })
    .click();
  await page.getByRole('menuitem', { name: 'Configure' }).click();
  const sheet = page.getByLabel('Configure Spotify');
  await expect(sheet.getByText('Connections')).toBeVisible();
  await sheet.getByLabel('Spotify account').selectOption('spotify-office');
  await sheet.getByRole('button', { name: 'Save' }).click();
  await page.getByRole('button', { name: 'Done' }).click();

  await hub.expectWrite('PUT', '/api/v1/widgets/layout');
  const savedLayout = latestWrite(hub.writes, '/api/v1/widgets/layout')
    ?.body as
    | {
        widgets?: Array<{
          id: string;
          connectionRefs?: Record<string, string>;
        }>;
      }
    | undefined;
  expect(
    savedLayout?.widgets?.find((widget) => widget.id === 'spotify')
  ).toMatchObject({
    connectionRefs: { account: 'spotify-office' }
  });
});

test('non-Spotify connector settings keep secrets as references', async ({
  page
}) => {
  const hub = await createMockHub(page);
  await page.goto('/');

  await page.getByRole('button', { name: 'Settings' }).click();
  await page.getByRole('button', { name: 'Connections' }).click();
  const settings = page.getByLabel('Jute settings');
  await settings.getByRole('button', { name: 'New', exact: true }).click();
  await settings.getByLabel('Kind').selectOption('philips-hue');
  await settings
    .getByRole('textbox', { name: 'ID', exact: true })
    .fill('hue-kitchen');
  await settings
    .getByRole('textbox', { name: 'Name', exact: true })
    .fill('Kitchen Hue');
  await settings.getByLabel('Bridge host').fill('192.0.2.10');
  await settings
    .getByLabel('Username secret reference')
    .fill('secret:hue-kitchen-username');
  await settings.getByRole('button', { name: 'Save', exact: true }).click();

  await hub.expectWrite('PUT', '/api/v1/settings/connections');
  const saved = latestWrite(hub.writes, '/api/v1/settings/connections')
    ?.body as
    | {
        settings?: Record<string, unknown>;
        secretRefs?: Record<string, string>;
      }
    | undefined;
  expect(saved?.settings).toMatchObject({ bridgeHost: '192.0.2.10' });
  expect(saved?.secretRefs).toMatchObject({
    username: 'secret:hue-kitchen-username'
  });
  await expect(
    page.getByText(/raw-token|raw secret|access_token/i)
  ).toHaveCount(0);
});

function clearWidgetConnectionRef(
  layout: {
    widgets?: Array<{ id: string; connectionRefs?: Record<string, string> }>;
    screens?: Array<{
      widgets?: Array<{ id: string; connectionRefs?: Record<string, string> }>;
    }>;
  },
  widgetId: string
) {
  for (const widget of layout.widgets ?? []) {
    if (widget.id === widgetId) widget.connectionRefs = {};
  }
  for (const screen of layout.screens ?? []) {
    for (const widget of screen.widgets ?? []) {
      if (widget.id === widgetId) widget.connectionRefs = {};
    }
  }
}

function latestWrite(
  writes: Array<{ method: string; path: string; body: unknown }>,
  path: string
) {
  return writes.findLast(
    (write) => write.method !== 'GET' && write.path === path
  );
}

declare global {
  interface Window {
    __juteOpenedURL: string;
  }
}
