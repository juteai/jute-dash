import type {
  LayoutVariant,
  WidgetLayout,
  WidgetInstance,
  WidgetCatalogItem,
  WidgetMode,
  WidgetPlacement
} from './types';

// BASE_COLUMNS is the authored base grid resolution. Layouts are stored at this
// resolution and render identically on every real screen, scaled to fit (the
// dashboard grid uses proportional 1fr columns and rows). columnsForWidth() and
// remapLayout() below are retained for a potential phone reflow but are not used
// in the current render path.
export const BASE_COLUMNS = 12;

// MAX_ROWS bounds the editor's per-tile height clamp (matches the hub
// validation). It does NOT cap the rendered row count, which follows the
// configured layout's actual extent.
export const MAX_ROWS = 12;

// Gap between grid cells; shared by dashboard rendering and edit-mode math.
// Row/column sizes themselves are proportional (1fr) and measured from the DOM
// during drag/resize rather than derived from a fixed pixel height.
export const GRID_GAP = 12;
export const LAYOUT_SCHEMA_VERSION = 2;
export const DEFAULT_LAYOUT_VARIANT = 'tablet-landscape';
export const MIN_GRID_COLUMNS = 1;
export const MAX_GRID_COLUMNS = 24;
export const MIN_GRID_ROWS = 1;
export const MAX_GRID_ROWS = 24;

export const DEFAULT_LAYOUT_VARIANTS: Array<Omit<LayoutVariant, 'placements'>> =
  [
    {
      id: 'phone',
      label: 'Phone',
      minWidth: 0,
      orientation: 'any',
      columns: 1,
      rows: 8,
      gap: GRID_GAP
    },
    {
      id: 'tablet-portrait',
      label: 'Tablet',
      minWidth: 641,
      orientation: 'portrait',
      columns: 6,
      rows: 10,
      gap: GRID_GAP
    },
    {
      id: 'tablet-landscape',
      label: 'Tablet wide',
      minWidth: 768,
      orientation: 'landscape',
      columns: 10,
      rows: 6,
      gap: GRID_GAP
    },
    {
      id: 'desktop',
      label: 'Desktop',
      minWidth: 1024,
      orientation: 'any',
      columns: 12,
      rows: 8,
      gap: GRID_GAP
    },
    {
      id: 'wall',
      label: 'Wall',
      minWidth: 1600,
      orientation: 'landscape',
      columns: 16,
      rows: 9,
      gap: GRID_GAP
    }
  ];

export function cloneLayout(layout: WidgetLayout): WidgetLayout {
  return JSON.parse(JSON.stringify(layout)) as WidgetLayout;
}

export function ensureLayoutVariants(layout: WidgetLayout): WidgetLayout {
  const next = cloneLayout(layout);
  next.schemaVersion = Math.max(
    Number(next.schemaVersion || 0),
    LAYOUT_SCHEMA_VERSION
  );
  next.defaultVariant = next.defaultVariant || DEFAULT_LAYOUT_VARIANT;

  const existing = new Map(
    (next.variants ?? []).map((variant) => [variant.id, variant])
  );
  next.variants = DEFAULT_LAYOUT_VARIANTS.map((preset) => {
    const variant = existing.get(preset.id);
    return normalizeVariant(
      {
        ...preset,
        ...variant,
        placements:
          variant?.placements ??
          defaultPlacementsForVariant(next.widgets, preset.columns, preset.id)
      },
      next.widgets
    );
  });

  for (const variant of existing.values()) {
    if (!next.variants.some((candidate) => candidate.id === variant.id)) {
      next.variants.push(normalizeVariant(variant, next.widgets));
    }
  }

  if (!next.variants.some((variant) => variant.id === next.defaultVariant)) {
    next.defaultVariant = next.variants[0]?.id || DEFAULT_LAYOUT_VARIANT;
  }
  return next;
}

function normalizeVariant(
  variant: LayoutVariant,
  widgets: WidgetInstance[]
): LayoutVariant {
  const columns = clampNumber(
    variant.columns,
    MIN_GRID_COLUMNS,
    MAX_GRID_COLUMNS
  );
  const rows = clampNumber(variant.rows, MIN_GRID_ROWS, MAX_GRID_ROWS);
  const placements: Record<string, WidgetPlacement> = {};
  for (const widget of widgets) {
    const placement = variant.placements?.[widget.id] ?? {
      x: widget.x,
      y: widget.y,
      w: widget.w,
      h: widget.h
    };
    placements[widget.id] = clampPlacement(placement, widget, columns, rows);
  }
  return {
    ...variant,
    id: variant.id.trim(),
    label: variant.label.trim() || variant.id,
    minWidth: Math.max(0, Math.floor(variant.minWidth || 0)),
    orientation: variant.orientation || 'any',
    columns,
    rows,
    gap: variant.gap ?? GRID_GAP,
    placements
  };
}

function defaultPlacementsForVariant(
  widgets: WidgetInstance[],
  columns: number,
  variantId: string
): Record<string, WidgetPlacement> {
  if (columns === 1 || variantId === 'phone') {
    const placements: Record<string, WidgetPlacement> = {};
    let row = 0;
    for (const widget of widgets
      .filter(rendersTile)
      .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id))) {
      placements[widget.id] = { x: 0, y: row, w: 1, h: Math.max(1, widget.h) };
      row += Math.max(1, widget.h);
    }
    return placements;
  }

  const scale = columns / BASE_COLUMNS;
  const placements: Record<string, WidgetPlacement> = {};
  for (const widget of widgets) {
    placements[widget.id] = {
      x: Math.min(Math.max(0, Math.round(widget.x * scale)), columns - 1),
      y: widget.y,
      w: Math.min(columns, Math.max(1, Math.round(widget.w * scale))),
      h: widget.h
    };
  }
  return placements;
}

function clampPlacement(
  placement: WidgetPlacement,
  widget: WidgetInstance,
  columns: number,
  rows: number
): WidgetPlacement {
  const minW = Math.min(Math.max(widget.minW || 1, 1), columns);
  const minH = Math.min(Math.max(widget.minH || 1, 1), rows);
  const w = clampNumber(placement.w || widget.w || minW, minW, columns);
  const h = clampNumber(placement.h || widget.h || minH, minH, rows);
  return {
    x: clampNumber(placement.x || 0, 0, columns - w),
    y: clampNumber(placement.y || 0, 0, rows - h),
    w,
    h,
    hidden: Boolean(placement.hidden)
  };
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(Math.max(Math.floor(value || min), min), max);
}

export function selectLayoutVariant(
  layout: WidgetLayout,
  width: number,
  height: number,
  preferredId = ''
): LayoutVariant {
  const withVariants = ensureLayoutVariants(layout);
  const preferred = withVariants.variants?.find(
    (variant) => variant.id === preferredId
  );
  if (preferred) {
    return preferred;
  }

  const orientation = width >= height ? 'landscape' : 'portrait';
  const candidates = (withVariants.variants ?? []).filter((variant) => {
    const minHeight = variant.minHeight ?? 0;
    return (
      width >= variant.minWidth &&
      height >= minHeight &&
      ((variant.orientation ?? 'any') === 'any' ||
        variant.orientation === orientation)
    );
  });

  return (
    candidates.sort(
      (a, b) =>
        b.minWidth - a.minWidth || (b.minHeight ?? 0) - (a.minHeight ?? 0)
    )[0] ??
    withVariants.variants?.find(
      (variant) => variant.id === withVariants.defaultVariant
    ) ??
    withVariants.variants![0]
  );
}

export function widgetsForVariant(
  layout: WidgetLayout,
  variantId: string
): WidgetInstance[] {
  const withVariants = ensureLayoutVariants(layout);
  const variant =
    withVariants.variants?.find((candidate) => candidate.id === variantId) ??
    selectLayoutVariant(withVariants, 1280, 800);
  return withVariants.widgets
    .filter((widget) => {
      const placement = variant.placements[widget.id];
      return rendersTile(widget) && !placement?.hidden;
    })
    .map((widget) => {
      const placement = variant.placements[widget.id];
      return {
        ...widget,
        ...placement,
        minW: Math.min(widget.minW, variant.columns),
        minH: Math.min(widget.minH, variant.rows)
      };
    })
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));
}

export function isHeadless(widget: WidgetInstance): boolean {
  return widget.mode === 'headless';
}

/** A widget draws a tile when it is present and not headless. */
export function rendersTile(widget: WidgetInstance): boolean {
  return widget.visible && !isHeadless(widget);
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
      rendersTile(widget) ? Math.max(row, widget.y + widget.h) : row,
    0
  );
}

export function sizeFromDimensions(
  w: number,
  h: number
): WidgetInstance['size'] {
  if (w >= 9 || h >= 3) {
    return 'large';
  }
  if (w >= 6 && h >= 2) {
    return 'medium';
  }
  if (w >= 6) {
    return 'wide';
  }
  return 'small';
}

export function clampWidget(widget: WidgetInstance): void {
  widget.minW = Math.min(Math.max(widget.minW || 1, 1), BASE_COLUMNS);
  widget.minH = Math.min(Math.max(widget.minH || 1, 1), MAX_ROWS);
  widget.w = Math.min(
    Math.max(widget.w || widget.minW, widget.minW),
    BASE_COLUMNS
  );
  widget.h = Math.min(Math.max(widget.h || widget.minH, widget.minH), MAX_ROWS);
  widget.x = Math.min(Math.max(widget.x, 0), BASE_COLUMNS - widget.w);
  widget.y = Math.min(Math.max(widget.y, 0), 99 - widget.h + 1);
  widget.size = sizeFromDimensions(widget.w, widget.h);
  widget.mode = widget.mode === 'headless' ? 'headless' : 'ui';
  widget.settings = widget.settings ?? {};
  widget.connectionRefs = widget.connectionRefs ?? {};
}

export function packLayout(layout: WidgetLayout, activeId = ''): WidgetLayout {
  const next = cloneLayout(layout);
  // Only tiles participate in grid packing; headless widgets occupy no space.
  const tiles = next.widgets.filter(rendersTile);
  const ordered = tiles.sort((a, b) => {
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
    for (let x = 0; x <= BASE_COLUMNS - w; x += 1) {
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
  item: WidgetCatalogItem,
  mode: WidgetMode = 'ui'
): { layout: WidgetLayout; error?: string; widgetId?: string } {
  const next = ensureLayoutVariants(layout);
  const targetRow = nextWidgetRow(next);
  let widget = next.widgets.find((candidate) => candidate.kind === item.kind);
  if (widget && !item.allowMultiple) {
    widget.visible = true;
    widget.mode = mode;
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
      mode,
      settings: {},
      connectionRefs: {},
      visible: true
    };
    next.widgets = [...next.widgets, widget];
  }
  widget.x = 0;
  widget.y = targetRow;
  for (const variant of next.variants ?? []) {
    variant.placements[widget.id] = clampPlacement(
      {
        x: 0,
        y: nextVariantRow(next, variant.id),
        w: Math.min(item.defaultW, variant.columns),
        h: Math.min(item.defaultH, variant.rows)
      },
      widget,
      variant.columns,
      variant.rows
    );
  }
  return {
    layout: packLayout(next, widget.id),
    widgetId: widget.id
  };
}

function nextVariantRow(layout: WidgetLayout, variantId: string): number {
  const variant = layout.variants?.find(
    (candidate) => candidate.id === variantId
  );
  if (!variant) return 0;
  return layout.widgets.reduce((row, widget) => {
    if (!rendersTile(widget)) return row;
    const placement = variant.placements[widget.id];
    if (!placement || placement.hidden) return row;
    return Math.max(row, placement.y + placement.h);
  }, 0);
}

export function overlaps(a: WidgetInstance, b: WidgetInstance): boolean {
  return (
    a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y
  );
}

export function pushDown(
  widgets: WidgetInstance[],
  target: WidgetInstance,
  activeId: string,
  pushed = new Set<string>()
): void {
  pushed.add(target.id);
  for (const other of widgets) {
    if (other.id === target.id || !rendersTile(other) || pushed.has(other.id)) {
      continue;
    }
    if (overlaps(target, other)) {
      other.y = target.y + target.h;
      pushDown(widgets, other, activeId, pushed);
    }
  }
}

export function resolveOverlaps(
  widgets: WidgetInstance[],
  activeId: string
): void {
  const active = widgets.find((w) => w.id === activeId);
  if (!active || !rendersTile(active)) {
    return;
  }
  pushDown(widgets, active, activeId);
}

export function moveWidget(
  layout: WidgetLayout,
  widgetId: string,
  x: number,
  y: number,
  variantId = ''
): WidgetLayout {
  const next = ensureLayoutVariants(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  if (variantId) {
    const variant = next.variants?.find((item) => item.id === variantId);
    if (!variant) return layout;
    const current = variant.placements[widgetId] ?? widget;
    variant.placements[widgetId] = clampPlacement(
      { ...current, x, y },
      widget,
      variant.columns,
      variant.rows
    );
    resolveVariantOverlaps(next, variantId, widgetId);
    return next;
  }
  widget.x = x;
  widget.y = y;
  clampWidget(widget);
  resolveOverlaps(next.widgets, widgetId);
  return next;
}

export function resizeWidget(
  layout: WidgetLayout,
  widgetId: string,
  w: number,
  h: number,
  variantId = ''
): WidgetLayout {
  const next = ensureLayoutVariants(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  if (variantId) {
    const variant = next.variants?.find((item) => item.id === variantId);
    if (!variant) return layout;
    const current = variant.placements[widgetId] ?? widget;
    variant.placements[widgetId] = clampPlacement(
      { ...current, w, h },
      widget,
      variant.columns,
      variant.rows
    );
    resolveVariantOverlaps(next, variantId, widgetId);
    return next;
  }
  widget.w = w;
  widget.h = h;
  clampWidget(widget);
  resolveOverlaps(next.widgets, widgetId);
  return next;
}

export function removeWidget(
  layout: WidgetLayout,
  widgetId: string
): WidgetLayout {
  const next = ensureLayoutVariants(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  widget.visible = false;
  for (const variant of next.variants ?? []) {
    if (variant.placements[widgetId]) {
      variant.placements[widgetId].hidden = true;
    }
  }
  return next;
}

/**
 * Moves a tile earlier (-1) or later (+1) in reading order. Used for
 * reorder-only editing on phones where fine drag placement is disabled.
 */
export function reorderWidget(
  layout: WidgetLayout,
  widgetId: string,
  direction: -1 | 1,
  variantId = ''
): WidgetLayout {
  const next = ensureLayoutVariants(layout);
  const variant = variantId
    ? next.variants?.find((item) => item.id === variantId)
    : undefined;
  const tiles = (variant ? widgetsForVariant(next, variant.id) : next.widgets)
    .filter(rendersTile)
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));
  const index = tiles.findIndex((item) => item.id === widgetId);
  const swapIndex = index + direction;
  if (index === -1 || swapIndex < 0 || swapIndex >= tiles.length) {
    return layout;
  }
  // Reassign y in the new order, full-width-stacked, then pack.
  const reordered = [...tiles];
  const [moved] = reordered.splice(index, 1);
  reordered.splice(swapIndex, 0, moved);
  let row = 0;
  for (const tile of reordered) {
    if (variant) {
      const widget = next.widgets.find((item) => item.id === tile.id);
      if (widget) {
        variant.placements[tile.id] = clampPlacement(
          { ...(variant.placements[tile.id] ?? tile), x: 0, y: row },
          widget,
          variant.columns,
          variant.rows
        );
      }
    } else {
      tile.x = 0;
      tile.y = row;
    }
    row += tile.h;
  }
  return variant ? next : packLayout(next);
}

/** Sets a widget's mode. Switching to ui re-packs it onto the grid. */
export function setWidgetMode(
  layout: WidgetLayout,
  widgetId: string,
  mode: WidgetMode
): WidgetLayout {
  const next = ensureLayoutVariants(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  if (widget.mode === mode) {
    return layout;
  }
  widget.mode = mode;
  if (mode === 'ui') {
    widget.y = nextWidgetRow(next);
    for (const variant of next.variants ?? []) {
      const placement = variant.placements[widgetId] ?? widget;
      variant.placements[widgetId] = clampPlacement(
        {
          ...placement,
          y: nextVariantRow(next, variant.id),
          hidden: false
        },
        widget,
        variant.columns,
        variant.rows
      );
    }
  } else {
    for (const variant of next.variants ?? []) {
      if (variant.placements[widgetId]) {
        variant.placements[widgetId].hidden = true;
      }
    }
  }
  return packLayout(next, mode === 'ui' ? widgetId : '');
}

/** Updates a widget's settings (and optionally title). */
export function updateWidget(
  layout: WidgetLayout,
  widgetId: string,
  patch: Partial<
    Pick<WidgetInstance, 'title' | 'settings' | 'connectionRefs' | 'mode'>
  >
): WidgetLayout {
  const next = cloneLayout(layout);
  const widget = next.widgets.find((item) => item.id === widgetId);
  if (!widget) {
    return layout;
  }
  if (patch.title !== undefined) {
    widget.title = patch.title;
  }
  if (patch.settings !== undefined) {
    widget.settings = patch.settings;
  }
  if (patch.connectionRefs !== undefined) {
    widget.connectionRefs = patch.connectionRefs;
  }
  if (patch.mode !== undefined) {
    widget.mode = patch.mode;
  }
  return next;
}

export function setVariantGridSize(
  layout: WidgetLayout,
  variantId: string,
  columns: number,
  rows: number
): WidgetLayout {
  const next = ensureLayoutVariants(layout);
  const variant = next.variants?.find(
    (candidate) => candidate.id === variantId
  );
  if (!variant) return layout;
  variant.columns = clampNumber(columns, MIN_GRID_COLUMNS, MAX_GRID_COLUMNS);
  variant.rows = clampNumber(rows, MIN_GRID_ROWS, MAX_GRID_ROWS);
  for (const widget of next.widgets) {
    const placement = variant.placements[widget.id] ?? widget;
    variant.placements[widget.id] = clampPlacement(
      placement,
      widget,
      variant.columns,
      variant.rows
    );
  }
  return next;
}

function resolveVariantOverlaps(
  layout: WidgetLayout,
  variantId: string,
  activeId: string
): void {
  const variant = layout.variants?.find(
    (candidate) => candidate.id === variantId
  );
  if (!variant) return;
  const widgets = widgetsForVariant(layout, variantId);
  const active = widgets.find((widget) => widget.id === activeId);
  if (!active) return;
  const pushed = new Set<string>([activeId]);
  for (const other of widgets) {
    if (other.id === activeId || pushed.has(other.id)) continue;
    if (overlaps(active, other)) {
      pushVariantDown(layout, variant, other, active.y + active.h, pushed);
    }
  }
}

function pushVariantDown(
  layout: WidgetLayout,
  variant: LayoutVariant,
  widget: WidgetInstance,
  y: number,
  pushed: Set<string>
): void {
  pushed.add(widget.id);
  const base = layout.widgets.find((item) => item.id === widget.id);
  if (!base) return;
  variant.placements[widget.id] = clampPlacement(
    { ...(variant.placements[widget.id] ?? widget), y },
    base,
    variant.columns,
    variant.rows
  );
  const moved = { ...widget, ...variant.placements[widget.id] };
  for (const other of widgetsForVariant(layout, variant.id)) {
    if (other.id === widget.id || pushed.has(other.id)) continue;
    if (overlaps(moved, other)) {
      pushVariantDown(layout, variant, other, moved.y + moved.h, pushed);
    }
  }
}

/**
 * Proportionally remaps a base (12-column) layout onto `targetColumns` for a
 * narrower screen. The stored layout is never mutated. Widths/positions scale
 * by targetColumns/BASE_COLUMNS; widgets are re-flowed top-to-bottom so they
 * never overlap or overflow. Headless widgets are passed through untouched.
 */
export function remapLayout(
  layout: WidgetLayout,
  targetColumns: number
): WidgetLayout {
  if (targetColumns >= BASE_COLUMNS) {
    return layout;
  }
  const cols = Math.max(1, Math.floor(targetColumns));
  const next = cloneLayout(layout);
  const scale = cols / BASE_COLUMNS;

  const tiles = next.widgets
    .filter(rendersTile)
    .sort((a, b) => a.y - b.y || a.x - b.x || a.id.localeCompare(b.id));

  // Track the next free row per column.
  const columnHeights = new Array<number>(cols).fill(0);

  for (const widget of tiles) {
    let w = Math.max(1, Math.round(widget.w * scale));
    w = Math.min(w, cols);
    const h = Math.max(1, widget.h);

    // Find the left-most x where a w-wide block fits with the lowest top.
    let bestX = 0;
    let bestY = Number.POSITIVE_INFINITY;
    for (let x = 0; x <= cols - w; x += 1) {
      let top = 0;
      for (let c = x; c < x + w; c += 1) {
        top = Math.max(top, columnHeights[c]);
      }
      if (top < bestY) {
        bestY = top;
        bestX = x;
      }
    }
    if (!Number.isFinite(bestY)) {
      bestX = 0;
      bestY = Math.max(0, ...columnHeights);
    }

    widget.x = bestX;
    widget.y = bestY;
    widget.w = w;
    widget.h = h;
    for (let c = bestX; c < bestX + w; c += 1) {
      columnHeights[c] = bestY + h;
    }
  }

  return next;
}

/** Chooses a responsive target column count for a viewport width. */
export function columnsForWidth(width: number): number {
  if (width >= 1024) {
    return BASE_COLUMNS; // desktop / wall
  }
  if (width >= 768) {
    return 6; // tablet
  }
  if (width >= 480) {
    return 4; // large phone / small tablet
  }
  return 2; // phone
}
