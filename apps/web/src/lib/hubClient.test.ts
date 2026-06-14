import { describe, expect, it, vi } from 'vitest';
import {
  API_BASE,
  fallbackDashboard,
  getAdapterConnectionKinds,
  getSpotifyWebPlaybackToken,
  initialDashboard,
  spotifyCallbackDisplayURL,
  spotifyCallbackParams,
  spotifyOAuthRedirectURI,
  spotifyAuthURL
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

describe('spotify auth URL', () => {
  it('points setup to the hub-owned OAuth route', () => {
    expect(spotifyAuthURL('spotify-main')).toBe(
      `${API_BASE}/api/v1/integrations/spotify/auth?connectionId=spotify-main`
    );
    expect(spotifyAuthURL('spotify-main', 'spotify-widget-1')).toBe(
      `${API_BASE}/api/v1/integrations/spotify/auth?connectionId=spotify-main&widgetInstanceId=spotify-widget-1`
    );
    expect(
      spotifyAuthURL('spotify-main', undefined, 'https://localhost:5173')
    ).toBe(
      `${API_BASE}/api/v1/integrations/spotify/auth?connectionId=spotify-main&returnUri=https%3A%2F%2Flocalhost%3A5173`
    );
  });

  it('uses the hub loopback callback as Spotify redirect URI', () => {
    expect(spotifyOAuthRedirectURI()).toBe(
      `${API_BASE}/api/v1/integrations/spotify/callback`
    );
  });

  it('parses and cleans display callback query strings', () => {
    expect(spotifyCallbackParams('?code=abc&state=xyz')).toEqual({
      code: 'abc',
      state: 'xyz'
    });
    expect(
      spotifyCallbackDisplayURL(
        '/',
        '?code=abc&state=xyz&theme=dark',
        '#top',
        'linked'
      )
    ).toBe('/?theme=dark&spotify=linked#top');
  });
});

describe('spotify web playback token', () => {
  it('loads a display-scoped token from the hub', async () => {
    const fetcher = vi.fn(async () =>
      Response.json({
        accessToken: 'access-token',
        expiresAt: 123,
        scope: 'streaming'
      })
    ) as unknown as typeof fetch;

    const token = await getSpotifyWebPlaybackToken(fetcher, 'spotify-main');

    expect(fetcher).toHaveBeenCalledWith(
      `${API_BASE}/api/v1/integrations/spotify/web-playback-token?connectionId=spotify-main`
    );
    expect(token).toEqual({
      accessToken: 'access-token',
      expiresAt: 123,
      scope: 'streaming'
    });
  });
});
