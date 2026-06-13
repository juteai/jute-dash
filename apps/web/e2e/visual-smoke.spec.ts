import { expect, test } from '@playwright/test';

const cases = [
  { mode: 'light', chrome: 'solid', state: 'ok' },
  { mode: 'dark', chrome: 'smoked', state: 'ok' },
  { mode: 'light', chrome: 'solid', state: 'empty' },
  { mode: 'dark', chrome: 'smoked', state: 'unavailable' },
  { mode: 'light', chrome: 'solid', state: 'stale' }
];

for (const visualCase of cases) {
  test(`dashboard visual smoke ${visualCase.mode} ${visualCase.chrome} ${visualCase.state}`, async ({
    page
  }) => {
    await page.goto(
      `/__visual__?mode=${visualCase.mode}&chrome=${visualCase.chrome}&state=${visualCase.state}`
    );

    await expect(page.getByLabel('Widget dashboard')).toBeVisible();
    await expect(page.locator('.widget-frame').first()).toBeVisible();
    await expect(
      page.getByText(/Kitchen|Saved Chats|No Data|Unavailable/).first()
    ).toBeVisible();

    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1
    );
    expect(overflow).toBe(false);

    const visibleTextCount = await page
      .locator('.widget-frame :text-is("Kitchen")')
      .count();
    if (visualCase.state === 'ok') {
      expect(visibleTextCount).toBeGreaterThan(0);
    }
  });
}
