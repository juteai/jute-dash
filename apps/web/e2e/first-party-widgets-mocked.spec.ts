import { expect, test, type Page } from '@playwright/test';
import { createMockHub } from './mockHub';

const secretPattern =
  /secret:|refresh_token|user_token|mock-web-playback-token|mock-music-kit-token|mock-apple-music-user-token/i;

test('first-party widgets render their hub-provided content', async ({
  page
}) => {
  const hub = await createMockHub(page, { layout: 'core-widgets' });
  await page.goto('/');
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();

  await expect(widget(page, 'date-time')).toContainText('UTC');
  await expect(widget(page, 'weather')).toContainText('Clear');
  await expect(widget(page, 'weather')).toContainText('Open-Meteo');
  await expect(widget(page, 'rss')).toContainText(
    'Household automations are calm today'
  );
  await expect(widget(page, 'markets')).toContainText('AAPL');
  await expect(widget(page, 'spotify')).toContainText('Home Mode');
  await expect(widget(page, 'apple-music')).toContainText('Home Mode');
  await expect(widget(page, 'philips-hue')).toContainText('Philips Hue Lights');
  await expect(widget(page, 'philips-hue')).toContainText('Kitchen light');
  await expect(widget(page, 'zigbee2mqtt')).toContainText('Zigbee Devices');
  await expect(widget(page, 'zigbee2mqtt')).toContainText('Entry temperature');
  await expect(widget(page, 'chat-history')).toContainText('House Agent');
  await expect(widget(page, 'timers-alarms')).toContainText('Tea');
  await expect(widget(page, 'timers-alarms')).toContainText('School run');
  await expect(widget(page, 'calendar')).toContainText('School assembly');
  await expect(widget(page, 'calendar')).toContainText('Hall');

  await expect(page.getByText(secretPattern)).toHaveCount(0);
  await hub.storageShouldNotContain('mock-web-playback-token');
  await hub.storageShouldNotContain('mock-music-kit-token');
});

test('first-party widget controls dispatch through hub widget actions', async ({
  page
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('phone'),
    'Phone project covers render; desktop clicks stable control affordances.'
  );

  const hub = await createMockHub(page, { layout: 'core-widgets' });
  await page.goto('/');
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();

  await widget(page, 'philips-hue')
    .getByRole('button', { name: 'On' })
    .first()
    .evaluate((button: HTMLButtonElement) => button.click());
  await hub.expectWrite('POST', '/api/v1/widgets/philips-hue/actions/toggle');

  await widget(page, 'zigbee2mqtt')
    .getByRole('button', { name: 'On' })
    .first()
    .evaluate((button: HTMLButtonElement) => button.click());
  await hub.expectWrite('POST', '/api/v1/widgets/zigbee2mqtt/actions/toggle');

  await widget(page, 'timers-alarms')
    .getByRole('button', { name: 'Cancel Tea' })
    .evaluate((button: HTMLButtonElement) => button.click());
  await hub.expectWrite('POST', '/api/v1/widgets/timers-alarms/actions/cancel');
});

function widget(page: Page, id: string) {
  return page.locator(`[data-widget-id="${id}"]`);
}
