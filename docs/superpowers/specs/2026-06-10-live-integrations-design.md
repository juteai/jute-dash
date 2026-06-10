# Specification - Live Integrations (Spotify, Apple Music, Philips Hue, Zigbee2MQTT)

This specification outlines the technical design for introducing real, functional backend integrations for the four modular widgets (`spotify`, `apple-music`, `philips-hue`, `zigbee2mqtt`), replacing the temporary stubs.

---

## 1. Zigbee2MQTT Integration

We will implement a real, persistent MQTT client inside the hub that connects to the local broker to control and query Zigbee devices.

### Libraries & Dependencies
*   Use `github.com/eclipse/paho.mqtt.golang` to handle connection, sub/pub, and reconnects.

### Architecture
1.  **Connection Manager**:
    *   Maintain a thread-safe connection pool in the `zigbee2mqtt` package (e.g. `map[string]*MQTTClient` keyed by `instanceID` or MQTT URL).
    *   On `FetchData`, check if a connection for this instance already exists. If not, spin up a client in a separate goroutine.
    *   On settings changes, disconnect the old client and connect a new one.
2.  **Topics & Subscription**:
    *   **Subscribe to `zigbee2mqtt/bridge/devices`**: The broker publishes a list of all joined Zigbee devices. The client parses this list and keeps it in memory.
    *   **Subscribe to `zigbee2mqtt/+`**: Listen for status changes published by individual devices (e.g. `zigbee2mqtt/living_room_bulb`) to retrieve fields like `state` (ON/OFF), `brightness` (0-255), and sensor outputs.
3.  **FetchData & UI State**:
    *   `FetchData` returns the cached list of devices and their last known states from memory, avoiding block-on-fetch.
4.  **Control Actions (`InvokeAction`)**:
    *   Publish to `zigbee2mqtt/<friendly_name>/set` with a JSON payload:
        *   Turn ON/OFF/Toggle: `{"state": "ON"}` / `{"state": "OFF"}` / `{"state": "TOGGLE"}`.
        *   Set Brightness: `{"brightness": <0-255>}`.

---

## 2. Philips Hue Integration

The Philips Hue widget will communicate directly with a local Hue Bridge via HTTP.

### Authentication & Registration
1.  **Manual Configuration**: Settings support `bridge_ip` and `username`.
2.  **Interactive Link Flow**:
    *   Expose a custom action `register_bridge` on the Hue widget.
    *   When the user clicks "Link Bridge" (which dispatches `register_bridge`), the backend sends a `POST` request to `http://<bridge_ip>/api` with the body:
        ```json
        {"devicetype": "jute_dash#local_hub"}
        ```
    *   If successful, the Bridge returns a `username`. The hub automatically saves this `username` back into the widget instance's settings via the layout/settings store.

### Fetch & Control
*   **FetchData**: Send a `GET` request to `http://<bridge_ip>/api/<username>/lights`. Parse the JSON response mapping Hue light models to Jute's standardized device states.
*   **InvokeAction**: Send a `PUT` request to `http://<bridge_ip>/api/<username>/lights/<id>/state` with:
    *   `{"on": true}` or `{"on": false}`.
    *   `{"bri": <0-254>}` (Hue uses 0-254 for brightness).

---

## 3. Spotify Integration

We will support the Spotify Web API using a full, backend-mediated OAuth Authorization Code flow with automatic token refreshes.

### Authentication Flow
1.  **Auth Route (`/api/widgets/spotify/auth`)**:
    *   Redirects the user to:
        ```
        https://accounts.spotify.com/authorize?client_id=<client_id>&response_type=code&redirect_uri=<redirect_uri>&scope=user-read-playback-state%20user-modify-playback-state&state=<instance_id>
        ```
2.  **Callback Route (`/api/widgets/spotify/callback`)**:
    *   Accepts `code` and `state` (which is the `instance_id`).
    *   Exchange the authorization code for an `access_token` and `refresh_token`.
    *   Save these tokens to the widget's settings in the SQLite database.
3.  **Automatic Refreshes**:
    *   When making a Spotify request, if the response is `401 Unauthorized`, automatically exchange the stored `refresh_token` for a new `access_token`, update the widget's persistent settings in the SQLite database, and retry the request.

### Playback Control
*   **FetchData**: `GET https://api.spotify.com/v1/me/player` to read current track, artist, album art, volume, and playback state.
*   **InvokeAction**:
    *   Play: `PUT https://api.spotify.com/v1/me/player/play`
    *   Pause: `PUT https://api.spotify.com/v1/me/player/pause`
    *   Next: `POST https://api.spotify.com/v1/me/player/next`
    *   Previous: `POST https://api.spotify.com/v1/me/player/previous`
    *   Volume: `PUT https://api.spotify.com/v1/me/player/volume?volume_percent=<val>`

---

## 4. Apple Music Integration

Due to Apple Music API rules requiring paid Apple Developer accounts to sign MusicKit requests, Apple Music will use manual token settings.

### Authentication
*   **Settings**: Support configuring a pre-signed `developer_token` (JWT Developer Token) and a `user_token` (Music-User-Token).

### Playback Control
*   Use standard HTTP client calls to the Apple Music API (`https://api.music.apple.com/v1/me/player/...`) passing `Authorization: Bearer <developer_token>` and `Music-User-Token: <user_token>`.
*   **FetchData**: Fetch the current playback status.
*   **InvokeAction**:
    *   Play: `POST /v1/me/player/play`
    *   Pause: `POST /v1/me/player/pause`
    *   Next: `POST /v1/me/player/next`
    *   Previous: `POST /v1/me/player/previous`

---

## 5. YAML Bootstrap Configuration Support

We will ensure that all settings (e.g. MQTT URLs, Hue IPs, Apple Music developer/user tokens, Spotify client IDs) can be bootstrapped from the YAML config file:
*   Bootstrap configurations for widgets in `config.yaml` will be synced into the SQLite database.
*   OAuth keys and credentials must be marked as `SecretString` to ensure they are never exposed in log outputs.
