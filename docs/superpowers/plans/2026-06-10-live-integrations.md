# Live Integrations (Spotify, Apple Music, Philips Hue, Zigbee2MQTT) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the real backend communication logic for the Spotify (OAuth), Apple Music (JWT/HTTP API), Philips Hue (HTTP API & Registration), and Zigbee2MQTT (MQTT client) integrations.

**Architecture:** Connect to the MQTT broker for Zigbee2MQTT via `paho.mqtt.golang` using a thread-safe connection manager. Communicate with Philips Hue and Apple Music via standard HTTP requests. Implement OAuth login and callback endpoints for Spotify directly on the Go hub server to store tokens securely.

**Tech Stack:** Go 1.25+, SvelteKit, MQTT, REST HTTP APIs.

---

## File Refactoring Map

- **Modify**:
  - `go.mod` (add `github.com/eclipse/paho.mqtt.golang`)
  - `widgets/zigbee2mqtt/zigbee2mqtt.go`
  - `widgets/zigbee2mqtt/zigbee2mqtt_test.go`
  - `widgets/philipshue/philipshue.go`
  - `widgets/philipshue/philipshue_test.go`
  - `widgets/spotify/spotify.go`
  - `widgets/spotify/spotify_test.go`
  - `widgets/applemusic/applemusic.go`
  - `widgets/applemusic/applemusic_test.go`
  - `apps/hub/internal/app/server.go`
  - `widgets/spotify/SpotifyWidget.svelte`
  - `widgets/philipshue/PhilipsHueWidget.svelte`

---

## Tasks

### Task 1: Zigbee2MQTT Backend MQTT Connection & Caching

Add MQTT client dependency and implement a persistent, thread-safe connection manager in Go to connect, subscribe, cache states, and publish actions.

**Files:**
- Modify: `go.mod`
- Modify: `widgets/zigbee2mqtt/zigbee2mqtt.go`
- Modify: `widgets/zigbee2mqtt/zigbee2mqtt_test.go`

- [ ] **Step 1: Install MQTT Dependency**

Run: `go get github.com/eclipse/paho.mqtt.golang`
Expected: `go.mod` and `go.sum` are updated successfully.

- [ ] **Step 2: Implement Zigbee2MQTT MQTT client manager**

Modify [widgets/zigbee2mqtt/zigbee2mqtt.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/zigbee2mqtt/zigbee2mqtt.go) to import `mqtt "github.com/eclipse/paho.mqtt.golang"` and implement:
```go
type mqttClient struct {
	client  mqtt.Client
	devices []any
	mu      sync.RWMutex
}
```
Update `FetchData` to connect/reconnect on-demand based on `mqtt_url`, subscribe to `zigbee2mqtt/bridge/devices`, parse the incoming JSON, and return the cached list. Update `InvokeAction` to publish commands to `zigbee2mqtt/<device_friendly_name>/set`.

- [ ] **Step 3: Update Zigbee2MQTT backend unit tests**

Modify [widgets/zigbee2mqtt/zigbee2mqtt_test.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/zigbee2mqtt/zigbee2mqtt_test.go) to verify connection setup and command publishing using a mock MQTT broker or local unit tests.

- [ ] **Step 4: Run tests**

Run: `go test -v ./widgets/zigbee2mqtt/...`
Expected: Tests pass.

- [ ] **Step 5: Commit changes**

Run:
```bash
git add go.mod go.sum widgets/zigbee2mqtt/
git commit -m "feat(zigbee2mqtt): implement persistent MQTT client and state caching"
```

---

### Task 2: Philips Hue Bridge Link & Control API

Implement local HTTP Bridge commands and username registration handler.

**Files:**
- Modify: `widgets/philipshue/philipshue.go`
- Modify: `widgets/philipshue/philipshue_test.go`
- Modify: `apps/hub/internal/app/server.go`

- [ ] **Step 1: Implement Philips Hue HTTP client**

Modify [widgets/philipshue/philipshue.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/philipshue/philipshue.go) to perform HTTP GET requests to `http://<bridge_ip>/api/<username>/lights` in `FetchData` and PUT requests to `http://<bridge_ip>/api/<username>/lights/<id>/state` in `InvokeAction`.

- [ ] **Step 2: Add Link Bridge endpoint in Go hub**

Modify [apps/hub/internal/app/server.go](file:///Users/craighutcheon/Repos/Other/jute-dash/apps/hub/internal/app/server.go) to register `mux.HandleFunc("/api/widgets/philips-hue/register", server.handleHueRegister)`:
```go
func (s *Server) handleHueRegister(w http.ResponseWriter, r *http.Request) {
    // 1. Decode bridge_ip and instance_id from JSON POST.
    // 2. POST to http://<bridge_ip>/api with body {"devicetype": "jute_dash"}
    // 3. If registered, save username back to layoutStore widget settings.
    // 4. Return the new username.
}
```

- [ ] **Step 3: Update Philips Hue unit tests**

Modify [widgets/philipshue/philipshue_test.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/philipshue/philipshue_test.go) to test local HTTP state mapping.

- [ ] **Step 4: Run tests**

Run: `go test -v ./widgets/philipshue/...`
Expected: Tests pass.

- [ ] **Step 5: Commit changes**

Run:
```bash
git add widgets/philipshue/ apps/hub/internal/app/server.go
git commit -m "feat(hue): implement local Hue Bridge control and auto-registration API"
```

---

### Task 3: Spotify OAuth Flow & Player Actions

Implement the complete OAuth flow handler on the hub server and Web API playback controls in Go.

**Files:**
- Modify: `widgets/spotify/spotify.go`
- Modify: `widgets/spotify/spotify_test.go`
- Modify: `apps/hub/internal/app/server.go`

- [ ] **Step 1: Implement Hub OAuth endpoints**

Modify [apps/hub/internal/app/server.go](file:///Users/craighutcheon/Repos/Other/jute-dash/apps/hub/internal/app/server.go) to register `/api/widgets/spotify/auth` and `/api/widgets/spotify/callback`.
*   `/auth`: Reads the Client ID from layout settings, redirects the user to Spotify Accounts service.
*   `/callback`: Receives the authorization code, exchanges it for `access_token` and `refresh_token`, saves them to SQLite layout database under the target instance, and redirects back to `/`.

- [ ] **Step 2: Implement Spotify API client and token refresh**

Modify [widgets/spotify/spotify.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/spotify/spotify.go) to include request utilities that inject the `Authorization: Bearer <access_token>` header. If the response status is `401 Unauthorized`, make a POST request to Spotify with the `refresh_token` to get a new `access_token`, update the layout settings in the SQLite database, and retry.

- [ ] **Step 3: Implement Spotify Player Actions**

Update `InvokeAction` in [widgets/spotify/spotify.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/spotify/spotify.go) to trigger Spotify player control actions (Play, Pause, Skip Next, Skip Previous, Set Volume).

- [ ] **Step 4: Run tests**

Run: `go test -v ./widgets/spotify/...`
Expected: Tests pass.

- [ ] **Step 5: Commit changes**

Run:
```bash
git add widgets/spotify/ apps/hub/internal/app/server.go
git commit -m "feat(spotify): implement backend Spotify OAuth authentication and Web API player controls"
```

---

### Task 4: Apple Music API Client Implementation

Implement standard HTTP request handlers using Developer JWT and User Tokens.

**Files:**
- Modify: `widgets/applemusic/applemusic.go`
- Modify: `widgets/applemusic/applemusic_test.go`

- [ ] **Step 1: Implement Apple Music HTTP Client**

Modify [widgets/applemusic/applemusic.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/applemusic/applemusic.go) to make HTTP requests to the Apple Music player APIs passing `Authorization: Bearer <developer_token>` and `Music-User-Token: <user_token>`.

- [ ] **Step 2: Update Apple Music backend unit tests**

Modify [widgets/applemusic/applemusic_test.go](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/applemusic/applemusic_test.go) to verify client headers and requests.

- [ ] **Step 3: Run tests**

Run: `go test -v ./widgets/applemusic/...`
Expected: Tests pass.

- [ ] **Step 4: Commit changes**

Run:
```bash
git add widgets/applemusic/
git commit -m "feat(applemusic): implement Apple Music player HTTP client"
```

---

### Task 5: Frontend Settings Sheets Integration

Add buttons in the frontend widget configuration forms to link Spotify accounts and Hue Bridge registration.

**Files:**
- Modify: `widgets/spotify/SpotifyWidget.svelte`
- Modify: `widgets/philipshue/PhilipsHueWidget.svelte`

- [ ] **Step 1: Add Link Spotify Account button**

Modify [widgets/spotify/SpotifyWidget.svelte](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/spotify/SpotifyWidget.svelte) to display a "Link Account" button when `is_configured` is false or when configuring settings, redirecting the user to `/api/widgets/spotify/auth?instance_id=<id>`.

- [ ] **Step 2: Add Link Bridge button**

Modify [widgets/philipshue/PhilipsHueWidget.svelte](file:///Users/craighutcheon/Repos/Other/jute-dash/widgets/philipshue/PhilipsHueWidget.svelte) to render a button "Link Bridge". Clicking it calls the hub registration endpoint `/api/widgets/philips-hue/register` in the background and retrieves the username.

- [ ] **Step 3: Verify workspace builds**

Run: `make check`
Expected: Zero lint or build errors.

- [ ] **Step 4: Commit changes**

Run:
```bash
git add widgets/spotify/SpotifyWidget.svelte widgets/philipshue/PhilipsHueWidget.svelte
git commit -m "feat(frontend): integrate Spotify and Hue linking helpers in settings UI"
```
