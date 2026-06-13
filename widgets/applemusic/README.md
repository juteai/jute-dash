# Apple Music Widget

The Apple Music widget allows Jute Dash to control and monitor active Apple Music playback.

## Metadata
- **Kind**: `apple-music`
- **Skill ID**: `jute.applemusic.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Connection Requirement
The widget requires a shared Adapter Connection:

| Slot | Kind | Description |
|---|---|---|
| `account` | `apple-music` | Apple Music developer and user token references. |

## Authentication Setup
To configure Apple Music:
1. Create or update an Apple Music Adapter Connection in Settings `Connections`.
2. Store the developer token and user token as secret references.
3. Choose that shared connection from the Apple Music widget settings sheet.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Reads the currently playing track, artist name, and playback state (playing/paused).
*   **Actions `play`, `pause`, `next`, `previous`, `set_volume`**: Low-risk playback controls through the hub widget action dispatcher.
