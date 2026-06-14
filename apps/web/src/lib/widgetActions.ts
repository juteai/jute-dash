import { dispatchWidgetAction } from './hubClient';

const readOnlyWidgetActions = new Set(['search', 'status']);

export async function dispatchDisplayWidgetAction(
  fetcher: typeof fetch,
  widgetId: string,
  action: string,
  args: Record<string, unknown> = {},
  refreshAfterMutation: () => Promise<void> = async () => {}
): Promise<unknown> {
  const result = await dispatchWidgetAction(fetcher, widgetId, action, args);
  if (!readOnlyWidgetActions.has(action)) {
    await refreshAfterMutation();
  }
  return result;
}

export function createDisplayWidgetDispatcher(
  fetcher: typeof fetch,
  widgetId: string
) {
  return (action: string, args: Record<string, unknown> = {}) =>
    dispatchDisplayWidgetAction(
      fetcher,
      widgetId,
      action,
      args,
      refreshAfterWidgetMutation
    );
}

async function refreshAfterWidgetMutation() {
  const { hubStream } = await import('$lib/hubStream');
  await hubStream.refreshAfterMutation(fetch);
  if (typeof window === 'undefined') return;
  for (const delay of [750, 1750, 3500]) {
    window.setTimeout(() => {
      void hubStream.refreshAfterMutation(fetch);
    }, delay);
  }
}
