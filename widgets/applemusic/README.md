# Apple Music Widget

The Apple Music widget allows Jute Dash to control and monitor active Apple Music playback.

## Metadata
- **Kind**: `apple-music`
- **Skill ID**: `jute.applemusic.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Settings Schema
The widget configuration includes the following settings:

| Setting ID | Type | Label | Description |
|---|---|---|---|
| `developer_token` | `string` | Developer Token | Pre-signed Apple Developer JWT token. |
| `user_token` | `string` | User Token | The Apple Music User Token (MusicUserToken) obtained from Apple authorize flow. |

## Authentication Setup
To configure Apple Music:
1. Provide a pre-signed Apple Music Developer Token generated using your Apple Developer account's private MusicKit key.
2. Provide a valid User Token (`Music-User-Token`) that grants access to your personal music library.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Reads the currently playing track, artist name, and playback state (playing/paused).
