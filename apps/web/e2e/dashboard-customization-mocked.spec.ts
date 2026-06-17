import { expect, test, type Locator, type Page } from '@playwright/test';
import { createMockHub } from './mockHub';

test('dashboard geometry stays stable while switching viewport sizes in edit mode', async ({
  page
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await createMockHub(page, { layout: 'core-widgets' });
  await page.goto('/');
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();

  await expectDashboardGeometry(page);

  await page.getByRole('button', { name: 'Edit dashboard' }).click();
  await expect(page.getByLabel(/Resize both of/).first()).toBeVisible();
  await expectDashboardGeometry(page);

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();
  await expect(page.getByLabel(/Resize both of/)).toHaveCount(0);
  await page
    .getByRole('button', { name: /Widget options for/ })
    .first()
    .click();
  await expect(page.getByRole('menuitem', { name: 'Move down' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expectDashboardGeometry(page);

  await page.setViewportSize({ width: 900, height: 700 });
  await expect(page.getByLabel(/Resize both of/).first()).toBeVisible();
  await expectDashboardGeometry(page);
});

test('dragging and resizing a widget persists changed desktop placement', async ({
  page
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const hub = await createMockHub(page);
  await page.goto('/');
  await expect(page.getByLabel('Widget dashboard')).toBeVisible();

  await page.getByRole('button', { name: 'Edit dashboard' }).click();
  const before = await placement(page, 'weather');
  const step = await gridStep(page);

  await dragBy(widgetFrame(page, 'weather'), step.cellWidth, step.rowHeight);
  await dragBy(
    page
      .locator('[data-widget-id="weather"]')
      .getByLabel('Resize both of Weather'),
    step.cellWidth,
    step.rowHeight
  );
  await expectDashboardGeometry(page);

  await page.getByRole('button', { name: 'Done' }).click();
  await hub.expectWrite('PUT', '/api/v1/widgets/layout');

  const saved = hub.writes.findLast(
    (write) => write.method === 'PUT' && write.path === '/api/v1/widgets/layout'
  )?.body as
    | {
        screens?: Array<{
          variants?: Array<{
            id: string;
            placements?: Record<
              string,
              { x: number; y: number; w: number; h: number }
            >;
          }>;
        }>;
      }
    | undefined;
  const weather = saved?.screens?.[0]?.variants?.find(
    (variant) => variant.id === 'desktop'
  )?.placements?.weather;

  expect(weather).toBeTruthy();
  expect(weather).not.toEqual(before);
  expect(weather?.x).toBeGreaterThanOrEqual(0);
  expect(weather?.y).toBeGreaterThanOrEqual(0);
  expect(weather?.w).toBeGreaterThanOrEqual(1);
  expect(weather?.h).toBeGreaterThanOrEqual(1);
});

function widgetFrame(page: Page, id: string) {
  return page.locator(`[data-widget-id="${id}"] .widget-frame`);
}

async function dragBy(locator: Locator, dx: number, dy: number) {
  const box = await locator.boundingBox();
  expect(box).toBeTruthy();
  const x = box!.x + box!.width / 2;
  const y = box!.y + box!.height / 2;
  await locator.page().mouse.move(x, y);
  await locator.page().mouse.down();
  await locator.page().mouse.move(x + dx, y + dy, { steps: 8 });
  await locator.page().mouse.up();
}

async function gridStep(page: Page) {
  return page.getByLabel('Widget dashboard').evaluate((canvas) => {
    const styles = window.getComputedStyle(canvas);
    const rect = canvas.getBoundingClientRect();
    const columns = styles.gridTemplateColumns
      .split(' ')
      .filter(Boolean).length;
    const rows = styles.gridTemplateRows.split(' ').filter(Boolean).length;
    const columnGap = Number.parseFloat(styles.columnGap || '0') || 0;
    const rowGap = Number.parseFloat(styles.rowGap || '0') || 0;
    return {
      cellWidth:
        (rect.width - columnGap * Math.max(0, columns - 1)) / columns +
        columnGap,
      rowHeight: (rect.height - rowGap * Math.max(0, rows - 1)) / rows + rowGap
    };
  });
}

async function placement(page: Page, widgetId: string) {
  return page.evaluate((id) => {
    const slot = document.querySelector<HTMLElement>(
      `[data-widget-id="${id}"]`
    );
    if (!slot) throw new Error(`Missing widget ${id}`);
    const styles = window.getComputedStyle(slot);
    return {
      x: Number.parseInt(styles.gridColumnStart, 10) - 1,
      y: Number.parseInt(styles.gridRowStart, 10) - 1,
      w: Number.parseInt(styles.gridColumnEnd.replace('span ', ''), 10),
      h: Number.parseInt(styles.gridRowEnd.replace('span ', ''), 10)
    };
  }, widgetId);
}

async function expectDashboardGeometry(page: Page) {
  const result = await page.evaluate(() => {
    const overflow =
      document.documentElement.scrollWidth > window.innerWidth + 1;
    const slots = Array.from(
      document.querySelectorAll<HTMLElement>('.dashboard-widget-slot')
    ).filter((slot) => {
      const rect = slot.getBoundingClientRect();
      return rect.width > 0 && rect.height > 0;
    });
    const overlaps: string[] = [];
    for (let i = 0; i < slots.length; i += 1) {
      const a = slots[i].getBoundingClientRect();
      for (let j = i + 1; j < slots.length; j += 1) {
        const b = slots[j].getBoundingClientRect();
        const separated =
          a.right <= b.left + 1 ||
          b.right <= a.left + 1 ||
          a.bottom <= b.top + 1 ||
          b.bottom <= a.top + 1;
        if (!separated) {
          overlaps.push(
            `${slots[i].dataset.widgetId ?? i}:${slots[j].dataset.widgetId ?? j}`
          );
        }
      }
    }
    return { overflow, overlaps };
  });

  expect(result.overflow).toBe(false);
  expect(result.overlaps).toEqual([]);
}
