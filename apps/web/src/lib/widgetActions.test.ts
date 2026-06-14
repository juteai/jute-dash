import { describe, expect, it, vi } from 'vitest';
import { dispatchDisplayWidgetAction } from './widgetActions';

function jsonResponse(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' }
  });
}

describe('dispatchDisplayWidgetAction', () => {
  it('returns read action responses without refreshing dashboard state', async () => {
    const fetcher = vi.fn(async () =>
      jsonResponse({ results: [{ name: 'Glue', uri: 'spotify:track:glue' }] })
    ) as unknown as typeof fetch;
    const refreshAfterMutation = vi.fn(async () => {});

    const result = await dispatchDisplayWidgetAction(
      fetcher,
      'spotify-1',
      'search',
      { query: 'glue', type: 'track' },
      refreshAfterMutation
    );

    expect(result).toEqual({
      results: [{ name: 'Glue', uri: 'spotify:track:glue' }]
    });
    expect(refreshAfterMutation).not.toHaveBeenCalled();
  });

  it('refreshes dashboard state after mutating actions', async () => {
    const fetcher = vi.fn(async () =>
      jsonResponse({ status: 'ok' })
    ) as unknown as typeof fetch;
    const refreshAfterMutation = vi.fn(async () => {});

    const result = await dispatchDisplayWidgetAction(
      fetcher,
      'spotify-1',
      'play',
      {},
      refreshAfterMutation
    );

    expect(result).toEqual({ status: 'ok' });
    expect(refreshAfterMutation).toHaveBeenCalledOnce();
  });
});
