# Spotify Widget

The Spotify widget allows Jute Dash to control music playback and view track information for the current user's Spotify account.

## Metadata
- **Kind**: `spotify`
- **Skill ID**: `jute.spotify.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Settings Schema
The widget configuration includes the following settings:

| Setting ID | Type | Label | Description |
|---|---|---|---|
| `client_id` | `string` | Client ID | Your Spotify Developer Application Client ID. |
| `client_secret` | `string` | Client Secret | Your Spotify Developer Application Client Secret (stored securely as a `SecretString`). |

## Authentication Setup
To link your Spotify account:
1. Register a developer application on the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard).
2. Configure the Redirect URI in Spotify's dashboard to point to your Jute Hub callback URL: `http://localhost:8787/api/widgets/spotify/callback`.
3. Input your `client_id` and `client_secret` in the Jute widget settings.
4. Click the **Link Account** button to perform the OAuth authorization code flow.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Reads the currently playing track, artist name, volume, and playback state (playing/paused).
