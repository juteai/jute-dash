# Zigbee2MQTT Widget

The Zigbee2MQTT widget allows local monitoring and control of Zigbee lights, switches, and sensors via a local MQTT broker.

## Metadata
- **Kind**: `zigbee2mqtt`
- **Skill ID**: `jute.zigbee2mqtt.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Connection Requirement
The widget requires a shared Adapter Connection:

| Slot | Kind | Description |
|---|---|---|
| `broker` | `zigbee2mqtt` | MQTT broker URL, optional username, and password secret reference. |

## Setup & Operation
1. Configure your Zigbee2MQTT coordinator to publish to your MQTT broker.
2. Create or update a Zigbee2MQTT Adapter Connection in Settings `Connections`.
3. Choose that shared connection from the Zigbee2MQTT widget settings sheet.
4. The widget will subscribe to `zigbee2mqtt/bridge/devices` and status update topics through the hub.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Lists all connected Zigbee devices, device names, and their state attributes.
*   **Actions `toggle`, `turn_on`, `turn_off`, `set_brightness`**: Low-risk device controls through the hub widget action dispatcher.
