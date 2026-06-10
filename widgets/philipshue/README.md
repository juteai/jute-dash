# Philips Hue Widget

The Philips Hue widget allows local control of light states, brightness, and colors on Hue smart lights connected to a local Philips Hue Bridge.

## Metadata
- **Kind**: `philips-hue`
- **Skill ID**: `jute.philipshue.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Settings Schema
The widget configuration includes the following settings:

| Setting ID | Type | Label | Description |
|---|---|---|---|
| `bridge_ip` | `string` | Bridge IP | The local IP address of your Philips Hue Bridge (e.g. `192.168.1.100`). |
| `username` | `string` | Username (API Key) | The authorized API token for communication with the Bridge. |

## Authentication Setup
To link your Philips Hue Bridge:
1. Input your local Bridge IP address.
2. If you already have a Hue username/API key, paste it directly into the `username` field.
3. Alternatively, press the physical link button on your Hue Bridge, then click the **Link Bridge** helper button in the settings panel to automatically register Jute and retrieve the token.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Lists the current connection status, all detected lights, and their state (on/off, brightness).
*   **Action `control_device`**: Controls specific Hue light parameters (e.g., turn on/off or change brightness level).
