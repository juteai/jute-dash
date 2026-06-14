# Design Spec: Modular Widgets Integration (Spotify, Apple Music, Philips Hue, Zigbee2MQTT)

> Historical planning artifact. Superseded by [ADR 0003](../../adr/0003-connection-aware-integration-widgets.md), [Widgets](../../architecture/widgets.md), [Widget Skills](../../architecture/widget-skills.md), and [Widget Developer Guidelines](../../developer/widget-guidelines.md). Do not treat the paths, credential-through-settings examples, aggregate widget language, or legacy payload shapes below as current runtime architecture.

We are refactoring the monolithic `music-player` and `smart-home` widgets into four dedicated, specific widgets. This maximizes modularity, keeps settings schemas highly focused, and simplifies backend API calls.

## Directory & File Refactoring

### 1. Deleted Components
We will delete the following unified widget directories:
*   `widgets/musicplayer/`
*   `widgets/smarthome/`

### 2. New Components
We will introduce four new native widgets under the `widgets/` directory:
*   **Spotify** (`widgets/spotify/`): Go package `spotify`, Svelte component `SpotifyWidget.svelte`, and unit tests `spotify_test.go`.
*   **Apple Music** (`widgets/applemusic/`): Go package `applemusic`, Svelte component `AppleMusicWidget.svelte`, and unit tests `applemusic_test.go`.
*   **Philips Hue** (`widgets/philipshue/`): Go package `philipshue`, Svelte component `PhilipsHueWidget.svelte`, and unit tests `philipshue_test.go`.
*   **Zigbee2MQTT** (`widgets/zigbee2mqtt/`): Go package `zigbee2mqtt`, Svelte component `Zigbee2MQTTWidget.svelte`, and unit tests `zigbee2mqtt_test.go`.

---

## Registry Configurations

### 1. Go Hub Auto-Registration
In `apps/hub/cmd/juted/main.go`, we will declare blank imports for the new widget packages to register them during compilation:
```go
import (
	_ "jute-dash/widgets/spotify"
	_ "jute-dash/widgets/applemusic"
	_ "jute-dash/widgets/philipshue"
	_ "jute-dash/widgets/zigbee2mqtt"
)
```

### 2. Frontend Svelte Component Registry
In `widgets/widget-registry.ts`, we register the new components:
```typescript
import SpotifyWidget from '$widgets/spotify/SpotifyWidget.svelte';
import AppleMusicWidget from '$widgets/applemusic/AppleMusicWidget.svelte';
import PhilipsHueWidget from '$widgets/philipshue/PhilipsHueWidget.svelte';
import Zigbee2MQTTWidget from '$widgets/zigbee2mqtt/Zigbee2MQTTWidget.svelte';

export const widgetRegistry = {
  'spotify': SpotifyWidget,
  'apple-music': AppleMusicWidget,
  'philips-hue': PhilipsHueWidget,
  'zigbee2mqtt': Zigbee2MQTTWidget,
  // ... existing widgets
};
```

---

## Settings Schemas & Security

Each widget specifies its own settings schema. Credentials will utilize a package-private `SecretString` type to redact secrets from Go hub logs.

### 1. Schemas

*   **Spotify** (`spotify`):
    *   `client_id` (`SettingString`): Spotify Developer Client ID.
    *   `client_secret` (`SettingString`): Spotify Developer Client Secret (secured).
*   **Apple Music** (`apple-music`):
    *   `developer_token` (`SettingString`): JWT Developer Token (secured).
    *   `user_token` (`SettingString`): User authentication token (secured).
*   **Philips Hue** (`philips-hue`):
    *   `bridge_ip` (`SettingString`): Local IP of the Philips Hue Bridge.
    *   `api_key` (`SettingString`): Generated Hue username key (secured).
*   **Zigbee2MQTT** (`zigbee2mqtt`):
    *   `mqtt_url` (`SettingString`): Local MQTT broker address (default: `mqtt://localhost:1883`).
    *   `mqtt_username` (`SettingString`): MQTT username.
    *   `mqtt_password` (`SettingString`): MQTT password (secured).

### 2. Security Redaction
```go
type SecretString string
func (s SecretString) LogValue() slog.Value {
	if s == "" {
		return slog.StringValue("")
	}
	return slog.StringValue("[redacted]")
}
```

---

## Svelte Views & UX Flows

All components use Jute design tokens to support high-contrast WOB/BOW styling and layout sizing.

### 1. Automated Setup UX
*   **Philips Hue**: Auto-discovery via local network mDNS scans. Initiating pairing displays a physical Link button prompt with polling logic to fetch and save the API key automatically.
*   **Spotify**: Opens a browser popup directing to the hub's local OAuth callback `/api/v1/oauth/spotify/callback`, automatically exchanges auth code, saves access/refresh tokens in SQLite, and closes.
*   **Apple Music**: Launches Apple's native secure StoreKit authentication popup using MusicKit JS and saves the returned user token to the hub.
*   **Zigbee2MQTT**: Defaults to localhost, and auto-discovers connected Zigbee devices by reading MQTT configuration payloads.

---

## Go Backend & A2A Skills

### 1. Spotify
*   **Skill ID**: `jute.spotify.control`
*   **Actions**: `play`, `pause`, `next`, `previous`, `set_volume`
*   **Context**: `track_title`, `artist_name`, `is_playing`, `volume`

### 2. Apple Music
*   **Skill ID**: `jute.applemusic.control`
*   **Actions**: `play`, `pause`, `next`, `previous`
*   **Context**: `track_title`, `artist_name`, `is_playing`

### 3. Philips Hue
*   **Skill ID**: `jute.philipshue.control`
*   **Actions**: `toggle_light`, `set_brightness`
*   **Context**: `devices` (array of lights with name, id, state, brightness)

### 4. Zigbee2MQTT
*   **Skill ID**: `jute.zigbee2mqtt.control`
*   **Actions**: `toggle_device`
*   **Context**: `devices` (array of devices with name, id, type, state, sensors)

---

## Verification Plan

### Automated Tests
*   `go test -v ./widgets/spotify/... ./widgets/applemusic/... ./widgets/philipshue/... ./widgets/zigbee2mqtt/...`
*   Verify layout checks and compilation via `make check`.
