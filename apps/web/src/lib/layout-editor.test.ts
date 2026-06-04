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
  removeWidget
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
    expect(sizeFromDimensions(2, 1)).toBe('wide');
    expect(sizeFromDimensions(2, 2)).toBe('medium');
    expect(sizeFromDimensions(3, 2)).toBe('large');
  });

  it('clamps widget coordinates and sizes', () => {
    const widget = createWidget({
      x: 5,
      y: -5,
      w: 10,
      h: 10,
      minW: 2,
      minH: 2
    });
    clampWidget(widget);
    expect(widget.x).toBe(0); // columns - w (4 - 4 = 0)
    expect(widget.y).toBe(0);
    expect(widget.w).toBe(4); // columns (4)
    expect(widget.h).toBe(6); // 6
  });

  it('packs widgets sequentially on empty spots', () => {
    const layout = createLayout([
      createWidget({ id: 'w1', x: 0, y: 0, w: 2, h: 1 }),
      createWidget({ id: 'w2', x: 0, y: 0, w: 3, h: 1 })
    ]);
    const packed = packLayout(layout);
    expect(packed.widgets[0].x).toBe(0);
    expect(packed.widgets[0].y).toBe(0);
    expect(packed.widgets[1].x).toBe(0);
    expect(packed.widgets[1].y).toBe(1); // falls to row 1 since row 0 has only 2 slots left
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
    const layout = createLayout([createWidget({ id: 'w1', w: 1, h: 1 })]);
    const resized = resizeWidget(layout, 'w1', 2, 2);
    expect(resized.widgets[0].w).toBe(2);
    expect(resized.widgets[0].size).toBe('medium');
  });

  it('removes widgets by setting visible to false', () => {
    const layout = createLayout([createWidget({ id: 'w1' })]);
    const removed = removeWidget(layout, 'w1');
    expect(removed.widgets[0].visible).toBe(false);
  });
});
