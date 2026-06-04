import type { WidgetLayout, WidgetInstance, WidgetCatalogItem } from './types';

export function cloneLayout(layout: WidgetLayout): WidgetLayout {
  return JSON.parse(JSON.stringify(layout)) as WidgetLayout;
}

export function uniqueWidgetId(layout: WidgetLayout, kind: string): string {
  const base = kind.replace(/[^a-z0-9-]/gi, '-').toLowerCase();
  if (!layout.widgets.some((widget) => widget.id === base)) {
    return base;
  }
  let counter = 2;
  while (layout.widgets.some((widget) => widget.id === `${base}-${counter}`)) {
    counter += 1;
  }
  return `${base}-${counter}`;
}

export function nextWidgetRow(layout: WidgetLayout): number {
  return layout.widgets.reduce(
    (row, widget) =>
      widget.visible ? Math.max(row, widget.y + widget.h) : row,
    0
  );
}

export function sizeFromDimensions(
  w: number,
  h: number
): WidgetInstance['size'] {
  if (w >= 3 || h >= 3) {
    return 'large';
  }
  if (w >= 2 && h >= 2) {
    return 'medium';
  }
  if (w >= 2) {
    return 'wide';
  }
  return 'small';
}

export function clampWidget(widget: WidgetInstance): void {
  const columns = 4;
  widget.minW = Math.min(Math.max(widget.minW || 1, 1), columns);
  widget.minH = Math.min(Math.max(widget.minH || 1, 1), 6);
  widget.w = Math.min(Math.max(widget.w || widget.minW, widget.minW), columns);
  widget.h = Math.min(Math.max(widget.h || widget.minH, widget.minH), 6);
  widget.x = Math.min(Math.max(widget.x, 0), columns - widget.w);
  widget.y = Math.min(Math.max(widget.y, 0), 99 - widget.h + 1);
  widget.size = sizeFromDimensions(widget.w, widget.h);
  widget.settings = widget.settings ?? {};
}

export function packLayout(layout: WidgetLayout, activeId = ''): WidgetLayout {
  const next = cloneLayout(layout);
  const visible = next.widgets.filter((widget) => widget.visible);
  const ordered = visible.sort((a, b) => {
    if (a.id === activeId) {
      return -1;
    }
    if (b.id === activeId) {
      return 1;
    }
    return a.y - b.y || a.x - b.x || a.id.localeCompare(b.id);
  });
  const occupied: boolean[][] = [];

  for (const widget of ordered) {
    clampWidget(widget);
    if (widget.id === activeId) {
      occupy(occupied, widget);
      continue;
    }
    const spot = firstOpenSpot(occupied, widget.w, widget.h);
    widget.x = spot.x;
    widget.y = spot.y;
    occupy(occupied, widget);
  }
  return next;
}

function firstOpenSpot(occupied: boolean[][], w: number, h: number) {
  for (let y = 0; y < 100; y += 1) {
    for (let x = 0; x <= 4 - w; x += 1) {
      if (canPlace(occupied, x, y, w, h)) {
        return { x, y };
      }
    }
  }
  return { x: 0, y: 99 - h + 1 };
}

function canPlace(
  occupied: boolean[][],
  x: number,
  y: number,
  w: number,
  h: number
) {
  for (let row = y; row < y + h; row += 1) {
    for (let column = x; column < x + w; column += 1) {
      if (occupied[row]?.[column]) {
        return false;
      }
    }
  }
  return true;
}

function occupy(occupied: boolean[][], widget: WidgetInstance) {
  for (let row = widget.y; row < widget.y + widget.h; row += 1) {
    occupied[row] = occupied[row] ?? [];
    for (let column = widget.x; column < widget.x + widget.w; column += 1) {
      occupied[row][column] = true;
    }
  }
}

export function addWidget(
  layout: WidgetLayout,
  item: WidgetCatalogItem
): { layout: WidgetLayout; error?: string; widgetId?: string } {
  const next = cloneLayout(layout);
  const targetRow = nextWidgetRow(next);
  let widget = next.widgets.find((candidate) => candidate.kind === item.kind);
  if (widget && !item.allowMultiple) {
    widget.visible = true;
    widget.title = widget.title || item.defaultTitle;
    widget.w = item.defaultW;
    widget.h = item.defaultH;
    widget.minW = item.minW;
    widget.minH = item.minH;
    widget.size = item.defaultSize;
  } else {
    widget = {
      id: uniqueWidgetId(next, item.kind),
      kind: item.kind,
      title: item.defaultTitle,
      x: 0,
      y: targetRow,
      w: item.defaultW,
      h: item.defaultH,
      minW: item.minW,
      minH: item.minH,
      size: item.defaultSize,
      settings: {},
      visible: true
    };
    next.widgets = [...next.widgets, widget];
  }
  widget.x = 0;
  widget.y = targetRow;
  return {
    layout: packLayout(next, widget.id),
    widgetId: widget.id
  };
}

export function moveWidget(
  layout: WidgetLayout,
  widgetId: string,
  x: number,
  y: number
): WidgetLayout {
  const next = cloneLayout(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  widget.x = x;
  widget.y = y;
  return packLayout(next, widgetId);
}

export function resizeWidget(
  layout: WidgetLayout,
  widgetId: string,
  w: number,
  h: number
): WidgetLayout {
  const next = cloneLayout(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  widget.w = w;
  widget.h = h;
  widget.size = sizeFromDimensions(w, h);
  return packLayout(next, widgetId);
}

export function removeWidget(
  layout: WidgetLayout,
  widgetId: string
): WidgetLayout {
  const next = cloneLayout(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  widget.visible = false;
  return packLayout(next);
}
