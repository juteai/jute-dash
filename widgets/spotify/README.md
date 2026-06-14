# Spotify Widget

The Spotify widget allows Jute Dash to control music playback and view track information for the current user's Spotify account.

## Metadata
- **Kind**: `spotify`
- **Skill ID**: `jute.spotify.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Connection Requirement
The widget requires a shared Adapter Connection:

| Slot | Kind | Description |
|---|---|---|
| `account` | `spotify` | Spotify login and OAuth token references. |

## Authentication Setup
To link your Spotify account:
1. Create or update a Spotify Adapter Connection in Settings `Connections`.
2. For self-hosted or development installs, register a developer application on the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard), add `http://127.0.0.1:8787/api/v1/integrations/spotify/callback` as an allowed redirect URI, and enter the Spotify Client ID in the connection settings. Spotify rejects `localhost` redirect URIs; the hub callback uses the explicit loopback IP. A client secret is not required for local login because Jute uses Authorization Code with PKCE.
3. If the hub is configured with a Jute-managed Spotify app (`JUTE_SPOTIFY_CLIENT_ID`), the Client ID field can be left empty.
4. Click **Login with Spotify** in Settings `Connections`. The hub exchanges the OAuth code and stores the access and refresh tokens as encrypted local `db:` secret references.
5. Choose that shared connection from the Spotify widget settings sheet when the widget instance is not already linked.

Spotify OAuth tokens are not stored in widget settings, YAML, browser storage, display payloads, A2A context, or MCP context. The Display can request a short-lived access token from the hub only for Spotify's Web Playback SDK; it is used in memory by the browser player and is not persisted by Jute.

## Browser Playback
The Spotify widget can activate a Jute Dash browser player using Spotify's Web Playback SDK. Browser playback requires:

- a linked Spotify Account connection;
- a Spotify Premium account;
- the `streaming` and `user-top-read` OAuth scopes granted during login;
- HTTPS for local display development.

Once the browser player is activated, the widget transfers Spotify playback to the Jute Dash display and existing playback controls continue through the hub action dispatcher.

For Spotify's local HTTPS requirement, run the display with:

```sh
cd apps/web
npm run dev:https
```

The browser may ask you to accept the local self-signed certificate.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Reads the currently playing track, artist name, volume, and playback state (playing/paused).
*   **Actions `play`, `pause`, `next`, `previous`, `set_volume`, `transfer_playback`**: Low-risk playback controls through the hub widget action dispatcher.
