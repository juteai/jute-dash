# Philips Hue Widget

The Philips Hue widget allows local control of light states, brightness, and colors on Hue smart lights connected to a local Philips Hue Bridge.

## Metadata
- **Kind**: `philips-hue`
- **Skill ID**: `jute.philipshue.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Connection Requirement
The widget requires a shared Adapter Connection:

| Slot | Kind | Description |
|---|---|---|
| `bridge` | `philips-hue` | Hue bridge IP and username/API key secret reference. |

## Authentication Setup
To link your Philips Hue Bridge:
1. Create or update a Philips Hue Adapter Connection in Settings `Connections`.
2. Store the bridge IP as non-secret settings and the username/API key as a secret reference.
3. Choose that shared connection from the Philips Hue widget settings sheet.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Lists the current connection status, all detected lights, and their state (on/off, brightness).
*   **Actions `toggle`, `turn_on`, `turn_off`, `set_brightness`**: Low-risk light controls through the hub widget action dispatcher.
