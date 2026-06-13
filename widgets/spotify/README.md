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
| `account` | `spotify` | Spotify account credentials and OAuth token references. |

## Authentication Setup
To link your Spotify account:
1. Register a developer application on the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard).
2. Create or update a Spotify Adapter Connection in Settings `Connections`.
3. Store non-secret provider settings on the connection and secret material as secret references.
4. Choose that shared connection from the Spotify widget settings sheet.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Reads the currently playing track, artist name, volume, and playback state (playing/paused).
*   **Actions `play`, `pause`, `next`, `previous`, `set_volume`**: Low-risk playback controls through the hub widget action dispatcher.
