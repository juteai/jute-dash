import { describe, expect, it, vi } from 'vitest';
import {
  API_BASE,
  fallbackDashboard,
  getAdapterConnectionKinds,
  initialDashboard
} from './hubClient';

describe('fallback dashboard', () => {
  it('marks offline scaffolding as stale and agentless', () => {
    const fallback = fallbackDashboard();

    expect(fallback.connectionState).toBe('offline');
    expect(fallback.stale).toBe(true);
    expect(fallback.agents).toEqual([]);
    expect(fallback.layout.widgets.map((widget) => widget.kind)).toEqual([
      'date-time',
      'weather',
      'chat-history'
    ]);
    expect('weather' in fallback.home).toBe(false);
  });

  it('can create a neutral initial dashboard before client-side hub connection', () => {
    const initial = initialDashboard();

    expect(initial.connectionState).toBe('starting');
    expect(initial.issue).toBeUndefined();
  });
});

describe('adapter connection kinds', () => {
  it('loads typed connection setup metadata from the hub', async () => {
    const fetcher = vi.fn(async () =>
      Response.json({
        kinds: [
          {
            kind: 'spotify',
            displayName: 'Spotify Account',
            fields: [
              {
                id: 'client_secret',
                label: 'Client secret reference',
                type: 'string',
                required: true,
                secret: true
              }
            ]
          }
        ]
      })
    ) as unknown as typeof fetch;

    const kinds = await getAdapterConnectionKinds(fetcher);

    expect(fetcher).toHaveBeenCalledWith(
      `${API_BASE}/api/v1/settings/connection-kinds`
    );
    expect(kinds[0].fields[0]).toMatchObject({
      id: 'client_secret',
      required: true,
      secret: true
    });
  });
});
