# Zigbee2MQTT Widget

The Zigbee2MQTT widget allows local monitoring and control of Zigbee lights, switches, and sensors via a local MQTT broker.

## Metadata
- **Kind**: `zigbee2mqtt`
- **Skill ID**: `jute.zigbee2mqtt.control`
- **Supported Sizes**: `wide` (6x2), with minimum size `wide` (4x2).

## Settings Schema
The widget configuration includes the following settings:

| Setting ID | Type | Label | Default Value | Description |
|---|---|---|---|---|
| `mqtt_url` | `string` | MQTT URL | `mqtt://localhost:1883` | The URL of your local MQTT broker. |
| `mqtt_username` | `string` | MQTT Username | | Username credentials for broker authentication. |
| `mqtt_password` | `string` | MQTT Password | | Password credentials for broker authentication (stored securely as a `SecretString`). |

## Setup & Operation
1. Configure your Zigbee2MQTT coordinator to publish to your MQTT broker.
2. In Jute's widget settings, input the MQTT connection details.
3. The widget will automatically subscribe to the `zigbee2mqtt/bridge/devices` and status update topics to discover your Zigbee devices and cache their states in memory.

## Agent Capabilities & Actions
Exposed actions for home assistant A2A and MCP agents:
*   **Action `status`**: Lists all connected Zigbee devices, device names, and their state attributes.
*   **Action `control_device`**: Sends commands (e.g. `state: "ON"`, `brightness: 200`) to control individual Zigbee devices.
