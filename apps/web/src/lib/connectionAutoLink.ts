import { cloneLayout } from './layout-editor';
import type {
  AdapterConnection,
  WidgetCatalogItem,
  WidgetLayout
} from './types';

export function autoLinkWidgetConnections(
  layout: WidgetLayout,
  catalog: WidgetCatalogItem[],
  connections: AdapterConnection[]
): { changed: boolean; layout: WidgetLayout } {
  const catalogByKind = new Map(catalog.map((item) => [item.kind, item]));
  const next = cloneLayout(layout);
  let changed = false;

  for (const widget of next.widgets) {
    const item = catalogByKind.get(widget.kind);
    const requirements = item?.connectionRequirements ?? [];
    if (requirements.length === 0) continue;

    for (const requirement of requirements) {
      if (widget.connectionRefs?.[requirement.slot]) continue;

      const matches = connections.filter(
        (connection) =>
          connection.enabled !== false && connection.kind === requirement.kind
      );
      if (matches.length !== 1) continue;

      widget.connectionRefs = {
        ...(widget.connectionRefs ?? {}),
        [requirement.slot]: matches[0].id
      };
      changed = true;
    }
  }

  return { changed, layout: next };
}
