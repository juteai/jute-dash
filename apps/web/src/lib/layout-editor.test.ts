import { describe, expect, it } from 'vitest';
import {
  cloneLayout,
  uniqueWidgetId,
  nextWidgetRow,
  sizeFromDimensions,
  clampWidget,
  packLayout,
  addWidget,
  moveWidget,
  resizeWidget,
  removeWidget,
  setWidgetMode,
  reorderWidget,
  remapLayout,
  columnsForWidth,
  BASE_COLUMNS
} from './layout-editor';
import type { WidgetLayout, WidgetInstance, WidgetCatalogItem } from './types';

function createLayout(widgets: WidgetInstance[] = []): WidgetLayout {
  return {
    profileId: 'default',
    widgets
  };
}

function createWidget(overrides: Partial<WidgetInstance> = {}): WidgetInstance {
  return {
    id: 'w1',
    kind: 'weather',
    title: 'Weather',
    x: 0,
    y: 0,
    w: 1,
    h: 1,
    minW: 1,
    minH: 1,
    size: 'small',
    settings: {},
    visible: true,
    ...overrides
  };
}

describe('layout-editor', () => {
  it('clones layouts properly', () => {
    const layout = createLayout([createWidget()]);
    const cloned = cloneLayout(layout);
    expect(cloned).toEqual(layout);
    expect(cloned).not.toBe(layout);
  });

  it('generates unique widget IDs', () => {
    const layout = createLayout([
      createWidget({ id: 'weather' }),
      createWidget({ id: 'weather-2' })
    ]);
    expect(uniqueWidgetId(layout, 'weather')).toBe('weather-3');
    expect(uniqueWidgetId(layout, 'clock')).toBe('clock');
  });

  it('finds next available row for widget packing', () => {
    const layout = createLayout([
      createWidget({ id: 'w1', y: 0, h: 2, visible: true }),
      createWidget({ id: 'w2', y: 2, h: 1, visible: true }),
      createWidget({ id: 'w3', y: 3, h: 1, visible: false })
    ]);
    expect(nextWidgetRow(layout)).toBe(3);
  });

  it('resolves size description from dimensions', () => {
    expect(sizeFromDimensions(1, 1)).toBe('small');
    expect(sizeFromDimensions(6, 1)).toBe('wide');
    expect(sizeFromDimensions(6, 2)).toBe('medium');
    expect(sizeFromDimensions(9, 1)).toBe('large');
  });

  it('clamps widget coordinates and sizes to the 12-column grid', () => {
    const widget = createWidget({
      x: 5,
      y: -5,
      w: 20,
      h: 20,
      minW: 3,
      minH: 2
    });
    clampWidget(widget);
    expect(widget.x).toBe(0); // BASE_COLUMNS - w (12 - 12 = 0)
    expect(widget.y).toBe(0);
    expect(widget.w).toBe(BASE_COLUMNS); // 12
    expect(widget.h).toBe(12); // MAX_ROWS
    expect(widget.mode).toBe('ui');
  });

  it('packs two half-width widgets side by side on the 12-col grid', () => {
    const layout = createLayout([
      createWidget({ id: 'w1', x: 0, y: 0, w: 6, h: 1 }),
      createWidget({ id: 'w2', x: 0, y: 0, w: 6, h: 1 })
    ]);
    const packed = packLayout(layout);
    expect(packed.widgets[0].y).toBe(0);
    expect(packed.widgets[1].y).toBe(0); // both fit on row 0 (6 + 6 = 12)
  });

  it('adds widgets correctly', () => {
    const layout = createLayout();
    const catalogItem: WidgetCatalogItem = {
      kind: 'rss',
      name: 'RSS Feed',
      description: 'RSS',
      defaultTitle: 'News',
      defaultW: 2,
      defaultH: 2,
      minW: 1,
      minH: 1,
      defaultSize: 'medium',
      overflow: 'scroll',
      allowMultiple: true
    };
    const result = addWidget(layout, catalogItem);
    expect(result.layout.widgets.length).toBe(1);
    expect(result.widgetId).toBe('rss');
    expect(result.layout.widgets[0].w).toBe(2);
  });

  it('moves widgets and repacks layout', () => {
    const layout = createLayout([
      createWidget({ id: 'w1', x: 0, y: 0, w: 2, h: 1 }),
      createWidget({ id: 'w2', x: 2, y: 0, w: 2, h: 1 })
    ]);
    const moved = moveWidget(layout, 'w2', 0, 1);
    expect(moved.widgets[1].x).toBe(0);
    expect(moved.widgets[1].y).toBe(1);
  });

  it('resizes widgets and updates size attribute', () => {
    const layout = createLayout([createWidget({ id: 'w1', w: 3, h: 1 })]);
    const resized = resizeWidget(layout, 'w1', 6, 2);
    expect(resized.widgets[0].w).toBe(6);
    expect(resized.widgets[0].size).toBe('medium');
  });

  it('removes widgets by setting visible to false', () => {
    const layout = createLayout([createWidget({ id: 'w1' })]);
    const removed = removeWidget(layout, 'w1');
    expect(removed.widgets[0].visible).toBe(false);
  });

  it('toggles a widget to headless and back to ui', () => {
    const layout = createLayout([createWidget({ id: 'w1', w: 6, h: 1 })]);
    const headless = setWidgetMode(layout, 'w1', 'headless');
    expect(headless.widgets[0].mode).toBe('headless');
    const restored = setWidgetMode(headless, 'w1', 'ui');
    expect(restored.widgets[0].mode).toBe('ui');
  });

  it('excludes headless widgets from grid packing', () => {
    const layout = createLayout([
      createWidget({ id: 'tile', x: 0, y: 0, w: 6, h: 1 }),
      createWidget({ id: 'ctx', x: 0, y: 0, w: 6, h: 1, mode: 'headless' })
    ]);
    const packed = packLayout(layout);
    const ctx = packed.widgets.find((w) => w.id === 'ctx');
    // Headless widget retained but not placed into the occupied grid.
    expect(ctx?.mode).toBe('headless');
  });

  it('reorders tiles in reading order', () => {
    const layout = createLayout([
      createWidget({ id: 'a', x: 0, y: 0, w: 12, h: 1 }),
      createWidget({ id: 'b', x: 0, y: 1, w: 12, h: 1 })
    ]);
    const reordered = reorderWidget(layout, 'b', -1);
    const a = reordered.widgets.find((w) => w.id === 'a');
    const b = reordered.widgets.find((w) => w.id === 'b');
    expect(b!.y).toBeLessThan(a!.y);
  });

  it('chooses responsive column counts by width', () => {
    expect(columnsForWidth(1280)).toBe(BASE_COLUMNS);
    expect(columnsForWidth(800)).toBe(6);
    expect(columnsForWidth(500)).toBe(4);
    expect(columnsForWidth(360)).toBe(2);
  });

  it('proportionally remaps a 12-col layout to fewer columns', () => {
    const layout = createLayout([
      createWidget({ id: 'a', x: 0, y: 0, w: 6, h: 1 }),
      createWidget({ id: 'b', x: 6, y: 0, w: 6, h: 1 })
    ]);
    const remapped = remapLayout(layout, 6);
    const a = remapped.widgets.find((w) => w.id === 'a');
    const b = remapped.widgets.find((w) => w.id === 'b');
    // 6 of 12 -> 3 of 6; both still fit one row.
    expect(a!.w).toBe(3);
    expect(b!.w).toBe(3);
    expect(a!.y).toBe(0);
    expect(b!.y).toBe(0);
  });

  it('stacks widgets full-width when remapping to a single column', () => {
    const layout = createLayout([
      createWidget({ id: 'a', x: 0, y: 0, w: 6, h: 1 }),
      createWidget({ id: 'b', x: 6, y: 0, w: 6, h: 1 })
    ]);
    const remapped = remapLayout(layout, 1);
    const a = remapped.widgets.find((w) => w.id === 'a');
    const b = remapped.widgets.find((w) => w.id === 'b');
    expect(a!.w).toBe(1);
    expect(b!.w).toBe(1);
    expect(b!.y).toBe(1);
  });

  it('resolves overlaps recursively by pushing overlapping widgets down', () => {
    // a is moved to x=0, y=0.
    // b is at x=0, y=0, w=2, h=1. Should be pushed down to y=1.
    // c is at x=0, y=1, w=2, h=1. Should be recursively pushed down to y=2.
    // d is at x=4, y=0, w=2, h=2 (non-overlapping). Should not move.
    const layout = createLayout([
      createWidget({ id: 'a', x: 0, y: 0, w: 2, h: 1 }),
      createWidget({ id: 'b', x: 0, y: 0, w: 2, h: 1 }),
      createWidget({ id: 'c', x: 0, y: 1, w: 2, h: 1 }),
      createWidget({ id: 'd', x: 4, y: 0, w: 2, h: 2 })
    ]);
    const resolved = moveWidget(layout, 'a', 0, 0);
    const a = resolved.widgets.find((w) => w.id === 'a')!;
    const b = resolved.widgets.find((w) => w.id === 'b')!;
    const c = resolved.widgets.find((w) => w.id === 'c')!;
    const d = resolved.widgets.find((w) => w.id === 'd')!;

    expect(a.y).toBe(0);
    expect(b.y).toBe(1);
    expect(c.y).toBe(2);
    expect(d.y).toBe(0);
  });
});
