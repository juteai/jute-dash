# Modular Widgets (Spotify, Apple Music, Philips Hue, Zigbee2MQTT) Implementation Plan

> Historical planning artifact. Superseded by [ADR 0003](../../adr/0003-connection-aware-integration-widgets.md), [Widgets](../../architecture/widgets.md), [Widget Skills](../../architecture/widget-skills.md), and [Widget Developer Guidelines](../../developer/widget-guidelines.md). Do not treat the paths, credential-through-settings examples, aggregate widget language, or legacy payload shapes below as current runtime architecture.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cleanly decompose monolithic widgets into four dedicated integrations (Spotify, Apple Music, Philips Hue, and Zigbee2MQTT) while removing all previous mock/simulation code from both the hub and the Svelte frontend views.

**Architecture:** Each widget acts as a standalone native Svelte component and self-registering Go backend package. All widgets implement the `widgets.Widget` Go interface and expose their respective skills to the Jute agent registry.

**Tech Stack:** Go 1.22+, SvelteKit, TypeScript, Vitest, and Go unit tests.

---

## File Refactoring Map

- **Delete**:
  - `widgets/musicplayer/musicplayer.go`
  - `widgets/musicplayer/MusicPlayerWidget.svelte`
  - `widgets/musicplayer/musicplayer_test.go`
  - `widgets/smarthome/smarthome.go`
  - `widgets/smarthome/SmartHomeWidget.svelte`
  - `widgets/smarthome/smarthome_test.go`
- **Create**:
  - `widgets/spotify/spotify.go`
  - `widgets/spotify/spotify_test.go`
  - `widgets/spotify/SpotifyWidget.svelte`
  - `widgets/applemusic/applemusic.go`
  - `widgets/applemusic/applemusic_test.go`
  - `widgets/applemusic/AppleMusicWidget.svelte`
  - `widgets/philipshue/philipshue.go`
  - `widgets/philipshue/philipshue_test.go`
  - `widgets/philipshue/PhilipsHueWidget.svelte`
  - `widgets/zigbee2mqtt/zigbee2mqtt.go`
  - `widgets/zigbee2mqtt/zigbee2mqtt_test.go`
  - `widgets/zigbee2mqtt/Zigbee2MQTTWidget.svelte`
- **Modify**:
  - `apps/hub/cmd/juted/main.go`
  - `widgets/widget-registry.ts`

---

## Tasks

### Task 1: Spotify Widget Package

Create the standalone Spotify widget with settings schema, A2A skill, action handler, and unit tests.

**Files:**
- Create: `widgets/spotify/spotify.go`
- Create: `widgets/spotify/spotify_test.go`

- [ ] **Step 1: Write Spotify backend logic**
  Create `widgets/spotify/spotify.go` with the following structure:
  ```go
  package spotify

  import (
  	"context"
  	"errors"
  	"fmt"
  	"log/slog"

  	"jute-dash/apps/hub/pkg/widgetskills"
  	"jute-dash/widgets"
  )

  const (
  	Kind    = "spotify"
  	SkillID = "jute.spotify.control"
  )

  type SecretString string

  func (s SecretString) LogValue() slog.Value {
  	if s == "" {
  		return slog.StringValue("")
  	}
  	return slog.StringValue("[redacted]")
  }

  type Settings struct {
  	ClientID     string
  	ClientSecret SecretString
  }

  func (s Settings) LogValue() slog.Value {
  	return slog.GroupValue(
  		slog.String("client_id", s.ClientID),
  		slog.Any("client_secret", s.ClientSecret),
  	)
  }

  type SpotifyWidget struct{}

  func NewWidget() *SpotifyWidget {
  	return &SpotifyWidget{}
  }

  func (w *SpotifyWidget) Kind() string {
  	return Kind
  }

  func (w *SpotifyWidget) CatalogInfo() widgets.WidgetCatalogItem {
  	return widgets.WidgetCatalogItem{
  		Kind:          Kind,
  		Name:          "Spotify",
  		Description:   "Control playback and view track info from Spotify.",
  		DefaultTitle:  "Spotify",
  		DefaultW:      6,
  		DefaultH:      2,
  		MinW:          4,
  		MinH:          2,
  		DefaultSize:   "wide",
  		Overflow:      "clip",
  		AllowMultiple: false,
  		SettingsSchema: []widgets.SettingField{
  			{
  				ID:    "client_id",
  				Type:  widgets.SettingString,
  				Label: "Client ID",
  				Help:  "Spotify Developer Client ID.",
  			},
  			{
  				ID:    "client_secret",
  				Type:  widgets.SettingString,
  				Label: "Client Secret",
  				Help:  "Spotify Developer Client Secret.",
  			},
  		},
  	}
  }

  func (w *SpotifyWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
  	slog.Debug( //nolint:sloglint // default permitted for widgets
  		"fetching spotify data",
  	)
  	s := parseSettings(rawSettings)
  	if s.ClientID == "" || string(s.ClientSecret) == "" {
  		return map[string]any{
  			"is_configured": false,
  		}, nil
  	}
  	return map[string]any{
  		"is_configured": true,
  		"track_title":   "Not Playing",
  		"artist_name":   "Unknown",
  		"is_playing":    false,
  		"volume":        50,
  	}, nil
  }

  func (w *SpotifyWidget) Skill() *widgetskills.Definition {
  	return &widgetskills.Definition{
  		SkillID:             SkillID,
  		WidgetKind:          Kind,
  		DisplayName:         "Spotify Control",
  		Summary:             "Read playback status and trigger playback control actions on Spotify.",
  		RequiredPermissions: []string{"agent:skill"},
  		VisibilityPolicy:    "visible_or_focused",
  		ContextFields: []widgetskills.Field{
  			{Name: "track_title", Type: "string", Description: "Currently playing song title.", Sensitivity: "public"},
  			{Name: "artist_name", Type: "string", Description: "Artist name.", Sensitivity: "public"},
  			{Name: "is_playing", Type: "boolean", Description: "Is music active.", Sensitivity: "public"},
  			{Name: "volume", Type: "number", Description: "Player volume.", Sensitivity: "public"},
  		},
  		Actions: []widgetskills.Action{
  			widgetskills.ReadAction("status", "Get playback status", "Read current track and status."),
  		},
  	}
  }

  func (w *SpotifyWidget) InvokeAction(
  	ctx context.Context,
  	snap widgetskills.Snapshot,
  	instanceID string,
  	actionID string,
  	arguments map[string]any,
  ) (map[string]any, error) {
  	slog.Info( //nolint:sloglint
  		"spotify action invoked",
  		"actionID", actionID,
  	)
  	return nil, errors.New("live integration not implemented")
  }

  func parseSettings(raw map[string]any) Settings {
  	s := Settings{}
  	if v, ok := raw["client_id"].(string); ok {
  		s.ClientID = v
  	}
  	if v, ok := raw["client_secret"].(string); ok {
  		s.ClientSecret = SecretString(v)
  	}
  	return s
  }

  func init() {
  	widgets.RegisterWithSkill(&SpotifyWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
  		return map[string]any{
  			"track_title": "Not Playing",
  			"artist_name": "Unknown",
  			"is_playing":  false,
  			"volume":      50,
  		}
  	})
  }
  ```

- [ ] **Step 2: Create unit tests for Spotify**
  Create `widgets/spotify/spotify_test.go`:
  ```go
  package spotify

  import (
  	"context"
  	"testing"
  )

  func TestSpotifyWidgetSettings(t *testing.T) {
  	w := NewWidget()
  	if w.Kind() != "spotify" {
  		t.Errorf("expected kind 'spotify', got %q", w.Kind())
  	}

  	raw := map[string]any{
  		"client_id":     "my_client_id",
  		"client_secret": "my_client_secret",
  	}
  	data, err := w.FetchData(context.Background(), raw)
  	if err != nil {
  		t.Fatalf("FetchData failed: %v", err)
  	}
  	m, ok := data.(map[string]any)
  	if !ok {
  		t.Fatalf("expected map[string]any, got %T", data)
  	}
  	if m["is_configured"] != true {
  		t.Errorf("expected is_configured to be true")
  	}
  }
  ```

- [ ] **Step 3: Verify tests pass**
  Run: `go test -v ./widgets/spotify/...`
  Expected: PASS

- [ ] **Step 4: Commit**
  Run:
  ```bash
  git add widgets/spotify/
  git commit -m "feat: implement Spotify backend widget"
  ```

---

### Task 2: Apple Music Widget Package

Create the Apple Music widget with developer and user tokens schema, A2A skill, actions, and unit tests.

**Files:**
- Create: `widgets/applemusic/applemusic.go`
- Create: `widgets/applemusic/applemusic_test.go`

- [ ] **Step 1: Write Apple Music backend logic**
  Create `widgets/applemusic/applemusic.go`:
  ```go
  package applemusic

  import (
  	"context"
  	"errors"
  	"log/slog"

  	"jute-dash/apps/hub/pkg/widgetskills"
  	"jute-dash/widgets"
  )

  const (
  	Kind    = "apple-music"
  	SkillID = "jute.applemusic.control"
  )

  type SecretString string

  func (s SecretString) LogValue() slog.Value {
  	if s == "" {
  		return slog.StringValue("")
  	}
  	return slog.StringValue("[redacted]")
  }

  type Settings struct {
  	DeveloperToken SecretString
  	UserToken      SecretString
  }

  func (s Settings) LogValue() slog.Value {
  	return slog.GroupValue(
  		slog.Any("developer_token", s.DeveloperToken),
  		slog.Any("user_token", s.UserToken),
  	)
  }

  type AppleMusicWidget struct{}

  func NewWidget() *AppleMusicWidget {
  	return &AppleMusicWidget{}
  }

  func (w *AppleMusicWidget) Kind() string {
  	return Kind
  }

  func (w *AppleMusicWidget) CatalogInfo() widgets.WidgetCatalogItem {
  	return widgets.WidgetCatalogItem{
  		Kind:          Kind,
  		Name:          "Apple Music",
  		Description:   "Control playback and view track info from Apple Music.",
  		DefaultTitle:  "Apple Music",
  		DefaultW:      6,
  		DefaultH:      2,
  		MinW:          4,
  		MinH:          2,
  		DefaultSize:   "wide",
  		Overflow:      "clip",
  		AllowMultiple: false,
  		SettingsSchema: []widgets.SettingField{
  			{
  				ID:    "developer_token",
  				Type:  widgets.SettingString,
  				Label: "Developer Token",
  				Help:  "JWT Developer Token from Apple Developer Account.",
  			},
  			{
  				ID:    "user_token",
  				Type:  widgets.SettingString,
  				Label: "User Token",
  				Help:  "Music User Token from client-side StoreKit.",
  			},
  		},
  	}
  }

  func (w *AppleMusicWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
  	slog.Debug( //nolint:sloglint
  		"fetching apple music data",
  	)
  	s := parseSettings(rawSettings)
  	if string(s.DeveloperToken) == "" {
  		return map[string]any{
  			"is_configured": false,
  		}, nil
  	}
  	return map[string]any{
  		"is_configured": true,
  		"track_title":   "Not Playing",
  		"artist_name":   "Unknown",
  		"is_playing":    false,
  	}, nil
  }

  func (w *AppleMusicWidget) Skill() *widgetskills.Definition {
  	return &widgetskills.Definition{
  		SkillID:             SkillID,
  		WidgetKind:          Kind,
  		DisplayName:         "Apple Music Control",
  		Summary:             "Read playback status and trigger playback control actions on Apple Music.",
  		RequiredPermissions: []string{"agent:skill"},
  		VisibilityPolicy:    "visible_or_focused",
  		ContextFields: []widgetskills.Field{
  			{Name: "track_title", Type: "string", Description: "Currently playing song title.", Sensitivity: "public"},
  			{Name: "artist_name", Type: "string", Description: "Artist name.", Sensitivity: "public"},
  			{Name: "is_playing", Type: "boolean", Description: "Is music active.", Sensitivity: "public"},
  		},
  		Actions: []widgetskills.Action{
  			widgetskills.ReadAction("status", "Get playback status", "Read current track and status."),
  		},
  	}
  }

  func (w *AppleMusicWidget) InvokeAction(
  	ctx context.Context,
  	snap widgetskills.Snapshot,
  	instanceID string,
  	actionID string,
  	arguments map[string]any,
  ) (map[string]any, error) {
  	slog.Info( //nolint:sloglint
  		"apple music action invoked",
  		"actionID", actionID,
  	)
  	return nil, errors.New("live integration not implemented")
  }

  func parseSettings(raw map[string]any) Settings {
  	s := Settings{}
  	if v, ok := raw["developer_token"].(string); ok {
  		s.DeveloperToken = SecretString(v)
  	}
  	if v, ok := raw["user_token"].(string); ok {
  		s.UserToken = SecretString(v)
  	}
  	return s
  }

  func init() {
  	widgets.RegisterWithSkill(&AppleMusicWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
  		return map[string]any{
  			"track_title": "Not Playing",
  			"artist_name": "Unknown",
  			"is_playing":  false,
  		}
  	})
  }
  ```

- [ ] **Step 2: Create unit tests for Apple Music**
  Create `widgets/applemusic/applemusic_test.go`:
  ```go
  package applemusic

  import (
  	"context"
  	"testing"
  )

  func TestAppleMusicWidgetSettings(t *testing.T) {
  	w := NewWidget()
  	if w.Kind() != "apple-music" {
  		t.Errorf("expected kind 'apple-music', got %q", w.Kind())
  	}

  	raw := map[string]any{
  		"developer_token": "my_dev_token",
  	}
  	data, err := w.FetchData(context.Background(), raw)
  	if err != nil {
  		t.Fatalf("FetchData failed: %v", err)
  	}
  	m, ok := data.(map[string]any)
  	if !ok {
  		t.Fatalf("expected map[string]any, got %T", data)
  	}
  	if m["is_configured"] != true {
  		t.Errorf("expected is_configured to be true")
  	}
  }
  ```

- [ ] **Step 3: Verify tests pass**
  Run: `go test -v ./widgets/applemusic/...`
  Expected: PASS

- [ ] **Step 4: Commit**
  Run:
  ```bash
  git add widgets/applemusic/
  git commit -m "feat: implement Apple Music backend widget"
  ```

---

### Task 3: Philips Hue Widget Package

Create the Philips Hue widget with local bridge IP and API key credentials, discovery placeholders, and tests.

**Files:**
- Create: `widgets/philipshue/philipshue.go`
- Create: `widgets/philipshue/philipshue_test.go`

- [ ] **Step 1: Write Philips Hue backend logic**
  Create `widgets/philipshue/philipshue.go`:
  ```go
  package philipshue

  import (
  	"context"
  	"errors"
  	"log/slog"

  	"jute-dash/apps/hub/pkg/widgetskills"
  	"jute-dash/widgets"
  )

  const (
  	Kind    = "philips-hue"
  	SkillID = "jute.philipshue.control"
  )

  type SecretString string

  func (s SecretString) LogValue() slog.Value {
  	if s == "" {
  		return slog.StringValue("")
  	}
  	return slog.StringValue("[redacted]")
  }

  type Settings struct {
  	BridgeIP string
  	APIKey   SecretString
  }

  func (s Settings) LogValue() slog.Value {
  	return slog.GroupValue(
  		slog.String("bridge_ip", s.BridgeIP),
  		slog.Any("api_key", s.APIKey),
  	)
  }

  type PhilipsHueWidget struct{}

  func NewWidget() *PhilipsHueWidget {
  	return &PhilipsHueWidget{}
  }

  func (w *PhilipsHueWidget) Kind() string {
  	return Kind
  }

  func (w *PhilipsHueWidget) CatalogInfo() widgets.WidgetCatalogItem {
  	return widgets.WidgetCatalogItem{
  		Kind:          Kind,
  		Name:          "Philips Hue",
  		Description:   "Control lights and rooms connected to a Philips Hue Bridge.",
  		DefaultTitle:  "Philips Hue",
  		DefaultW:      6,
  		DefaultH:      2,
  		MinW:          4,
  		MinH:          2,
  		DefaultSize:   "wide",
  		Overflow:      "clip",
  		AllowMultiple: false,
  		SettingsSchema: []widgets.SettingField{
  			{
  				ID:    "bridge_ip",
  				Type:  widgets.SettingString,
  				Label: "Bridge IP",
  				Help:  "IP address of the Philips Hue Bridge.",
  			},
  			{
  				ID:    "api_key",
  				Type:  widgets.SettingString,
  				Label: "API Key",
  				Help:  "Authorized API username key.",
  			},
  		},
  	}
  }

  func (w *PhilipsHueWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
  	slog.Debug( //nolint:sloglint
  		"fetching philips hue data",
  	)
  	s := parseSettings(rawSettings)
  	if s.BridgeIP == "" || string(s.APIKey) == "" {
  		return map[string]any{
  			"is_configured": false,
  		}, nil
  	}
  	return map[string]any{
  		"is_configured": true,
  		"devices":       []any{},
  	}, nil
  }

  func (w *PhilipsHueWidget) Skill() *widgetskills.Definition {
  	return &widgetskills.Definition{
  		SkillID:             SkillID,
  		WidgetKind:          Kind,
  		DisplayName:         "Philips Hue Control",
  		Summary:             "Read light statuses and control devices connected to Philips Hue.",
  		RequiredPermissions: []string{"agent:skill"},
  		VisibilityPolicy:    "visible_or_focused",
  		ContextFields: []widgetskills.Field{
  			{Name: "devices", Type: "array", Description: "Discovered Hue devices list.", Sensitivity: "public"},
  		},
  		Actions: []widgetskills.Action{
  			widgetskills.ReadAction("status", "Get light status", "List light entities and states."),
  		},
  	}
  }

  func (w *PhilipsHueWidget) InvokeAction(
  	ctx context.Context,
  	snap widgetskills.Snapshot,
  	instanceID string,
  	actionID string,
  	arguments map[string]any,
  ) (map[string]any, error) {
  	slog.Info( //nolint:sloglint
  		"philips hue action invoked",
  		"actionID", actionID,
  	)
  	return nil, errors.New("live integration not implemented")
  }

  func parseSettings(raw map[string]any) Settings {
  	s := Settings{}
  	if v, ok := raw["bridge_ip"].(string); ok {
  		s.BridgeIP = v
  	}
  	if v, ok := raw["api_key"].(string); ok {
  		s.APIKey = SecretString(v)
  	}
  	return s
  }

  func init() {
  	widgets.RegisterWithSkill(&PhilipsHueWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
  		return map[string]any{
  			"devices": []any{},
  		}
  	})
  }
  ```

- [ ] **Step 2: Create unit tests for Philips Hue**
  Create `widgets/philipshue/philipshue_test.go`:
  ```go
  package philipshue

  import (
  	"context"
  	"testing"
  )

  func TestPhilipsHueWidgetSettings(t *testing.T) {
  	w := NewWidget()
  	if w.Kind() != "philips-hue" {
  		t.Errorf("expected kind 'philips-hue', got %q", w.Kind())
  	}

  	raw := map[string]any{
  		"bridge_ip": "192.168.1.100",
  		"api_key":   "my_api_key",
  	}
  	data, err := w.FetchData(context.Background(), raw)
  	if err != nil {
  		t.Fatalf("FetchData failed: %v", err)
  	}
  	m, ok := data.(map[string]any)
  	if !ok {
  		t.Fatalf("expected map[string]any, got %T", data)
  	}
  	if m["is_configured"] != true {
  		t.Errorf("expected is_configured to be true")
  	}
  }
  ```

- [ ] **Step 3: Verify tests pass**
  Run: `go test -v ./widgets/philipshue/...`
  Expected: PASS

- [ ] **Step 4: Commit**
  Run:
  ```bash
  git add widgets/philipshue/
  git commit -m "feat: implement Philips Hue backend widget"
  ```

---

### Task 4: Zigbee2MQTT Widget Package

Create the local Zigbee2MQTT widget using MQTT brokers, password redactions, and tests.

**Files:**
- Create: `widgets/zigbee2mqtt/zigbee2mqtt.go`
- Create: `widgets/zigbee2mqtt/zigbee2mqtt_test.go`

- [ ] **Step 1: Write Zigbee2MQTT backend logic**
  Create `widgets/zigbee2mqtt/zigbee2mqtt.go`:
  ```go
  package zigbee2mqtt

  import (
  	"context"
  	"errors"
  	"log/slog"

  	"jute-dash/apps/hub/pkg/widgetskills"
  	"jute-dash/widgets"
  )

  const (
  	Kind    = "zigbee2mqtt"
  	SkillID = "jute.zigbee2mqtt.control"
  )

  type SecretString string

  func (s SecretString) LogValue() slog.Value {
  	if s == "" {
  		return slog.StringValue("")
  	}
  	return slog.StringValue("[redacted]")
  }

  type Settings struct {
  	MQTTURL      string
  	MQTTUsername string
  	MQTTPassword SecretString
  }

  func (s Settings) LogValue() slog.Value {
  	return slog.GroupValue(
  		slog.String("mqtt_url", s.MQTTURL),
  		slog.String("mqtt_username", s.MQTTUsername),
  		slog.Any("mqtt_password", s.MQTTPassword),
  	)
  }

  type Zigbee2MQTTWidget struct{}

  func NewWidget() *Zigbee2MQTTWidget {
  	return &Zigbee2MQTTWidget{}
  }

  func (w *Zigbee2MQTTWidget) Kind() string {
  	return Kind
  }

  func (w *Zigbee2MQTTWidget) CatalogInfo() widgets.WidgetCatalogItem {
  	return widgets.WidgetCatalogItem{
  		Kind:          Kind,
  		Name:          "Zigbee2MQTT",
  		Description:   "Monitor and control local smart devices connected via Zigbee2MQTT.",
  		DefaultTitle:  "Zigbee",
  		DefaultW:      6,
  		DefaultH:      2,
  		MinW:          4,
  		MinH:          2,
  		DefaultSize:   "wide",
  		Overflow:      "clip",
  		AllowMultiple: false,
  		SettingsSchema: []widgets.SettingField{
  			{
  				ID:      "mqtt_url",
  				Type:    widgets.SettingString,
  				Label:   "MQTT URL",
  				Default: "mqtt://localhost:1883",
  				Help:    "Address of the MQTT Broker.",
  			},
  			{
  				ID:    "mqtt_username",
  				Type:  widgets.SettingString,
  				Label: "MQTT Username",
  				Help:  "Broker username credentials.",
  			},
  			{
  				ID:    "mqtt_password",
  				Type:  widgets.SettingString,
  				Label: "MQTT Password",
  				Help:  "Broker password credentials.",
  			},
  		},
  	}
  }

  func (w *Zigbee2MQTTWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
  	slog.Debug( //nolint:sloglint
  		"fetching zigbee2mqtt data",
  	)
  	s := parseSettings(rawSettings)
  	if s.MQTTURL == "" {
  		return map[string]any{
  			"is_configured": false,
  		}, nil
  	}
  	return map[string]any{
  		"is_configured": true,
  		"devices":       []any{},
  	}, nil
  }

  func (w *Zigbee2MQTTWidget) Skill() *widgetskills.Definition {
  	return &widgetskills.Definition{
  		SkillID:             SkillID,
  		WidgetKind:          Kind,
  		DisplayName:         "Zigbee2MQTT Control",
  		Summary:             "Control and read status of local Zigbee devices.",
  		RequiredPermissions: []string{"agent:skill"},
  		VisibilityPolicy:    "visible_or_focused",
  		ContextFields: []widgetskills.Field{
  			{Name: "devices", Type: "array", Description: "Connected Zigbee devices list.", Sensitivity: "public"},
  		},
  		Actions: []widgetskills.Action{
  			widgetskills.ReadAction("status", "Get device status", "List Zigbee devices and sensor outputs."),
  		},
  	}
  }

  func (w *Zigbee2MQTTWidget) InvokeAction(
  	ctx context.Context,
  	snap widgetskills.Snapshot,
  	instanceID string,
  	actionID string,
  	arguments map[string]any,
  ) (map[string]any, error) {
  	slog.Info( //nolint:sloglint
  		"zigbee2mqtt action invoked",
  		"actionID", actionID,
  	)
  	return nil, errors.New("live integration not implemented")
  }

  func parseSettings(raw map[string]any) Settings {
  	s := Settings{
  		MQTTURL: "mqtt://localhost:1883",
  	}
  	if v, ok := raw["mqtt_url"].(string); ok && v != "" {
  		s.MQTTURL = v
  	}
  	if v, ok := raw["mqtt_username"].(string); ok {
  		s.MQTTUsername = v
  	}
  	if v, ok := raw["mqtt_password"].(string); ok {
  		s.MQTTPassword = SecretString(v)
  	}
  	return s
  }

  func init() {
  	widgets.RegisterWithSkill(&Zigbee2MQTTWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
  		return map[string]any{
  			"devices": []any{},
  		}
  	})
  }
  ```

- [ ] **Step 2: Create unit tests for Zigbee2MQTT**
  Create `widgets/zigbee2mqtt/zigbee2mqtt_test.go`:
  ```go
  package zigbee2mqtt

  import (
  	"context"
  	"testing"
  )

  func TestZigbee2MQTTWidgetSettings(t *testing.T) {
  	w := NewWidget()
  	if w.Kind() != "zigbee2mqtt" {
  		t.Errorf("expected kind 'zigbee2mqtt', got %q", w.Kind())
  	}

  	raw := map[string]any{
  		"mqtt_url":      "mqtt://localhost:1883",
  		"mqtt_username": "my_user",
  	}
  	data, err := w.FetchData(context.Background(), raw)
  	if err != nil {
  		t.Fatalf("FetchData failed: %v", err)
  	}
  	m, ok := data.(map[string]any)
  	if !ok {
  		t.Fatalf("expected map[string]any, got %T", data)
  	}
  	if m["is_configured"] != true {
  		t.Errorf("expected is_configured to be true")
  	}
  }
  ```

- [ ] **Step 3: Verify tests pass**
  Run: `go test -v ./widgets/zigbee2mqtt/...`
  Expected: PASS

- [ ] **Step 4: Commit**
  Run:
  ```bash
  git add widgets/zigbee2mqtt/
  git commit -m "feat: implement Zigbee2MQTT backend widget"
  ```

---

### Task 5: Svelte Components Creation

Create frontend views for all four widgets.

**Files:**
- Create: `widgets/spotify/SpotifyWidget.svelte`
- Create: `widgets/applemusic/AppleMusicWidget.svelte`
- Create: `widgets/philipshue/PhilipsHueWidget.svelte`
- Create: `widgets/zigbee2mqtt/Zigbee2MQTTWidget.svelte`

- [ ] **Step 1: Create Spotify Svelte Component**
  Create `widgets/spotify/SpotifyWidget.svelte`:
  ```svelte
  <script lang="ts">
    export let data: any = {};
    export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

    $: isConfigured = data?.is_configured ?? false;
    $: isPlaying = data?.is_playing ?? false;
    $: trackTitle = data?.track_title ?? 'Not Playing';
    $: artistName = data?.artist_name ?? 'Unknown';
    $: volume = data?.volume ?? 50;

    async function handlePlayPause() {
      await dispatch(isPlaying ? 'pause' : 'play');
    }

    async function handleNext() {
      await dispatch('next');
    }

    async function handlePrevious() {
      await dispatch('previous');
    }

    async function handleVolume(e: Event) {
      const vol = parseInt((e.target as HTMLInputElement).value, 10);
      await dispatch('set_volume', { volume: vol });
    }
  </script>

  <div class="widget-content">
    {#if !isConfigured}
      <div class="unconfigured">
        <p class="title">Spotify</p>
        <button class="connect-btn" on:click={() => dispatch('connect')}>Connect Spotify</button>
      </div>
    {:else}
      <div class="player">
        <div class="info">
          <p class="track">{trackTitle}</p>
          <p class="artist">{artistName}</p>
        </div>
        <div class="controls">
          <button on:click={handlePrevious}>⏮</button>
          <button on:click={handlePlayPause}>{isPlaying ? '⏸' : '▶'}</button>
          <button on:click={handleNext}>⏭</button>
        </div>
        <div class="vol-slider">
          <span>🔈</span>
          <input type="range" min="0" max="100" value={volume} on:change={handleVolume} />
          <span>🔊</span>
        </div>
      </div>
    {/if}
  </div>

  <style>
    .widget-content {
      padding: 12px;
      height: 100%;
      display: flex;
      flex-direction: column;
      justify-content: center;
    }
    .unconfigured {
      text-align: center;
    }
    .title {
      font-weight: bold;
      margin-bottom: 8px;
    }
    .connect-btn {
      padding: 6px 12px;
      background: var(--foreground);
      color: var(--inverse);
      border: none;
      border-radius: 4px;
      cursor: pointer;
    }
    .player {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .track {
      font-weight: bold;
      font-size: 0.9rem;
    }
    .artist {
      color: var(--muted);
      font-size: 0.8rem;
    }
    .controls {
      display: flex;
      gap: 16px;
      justify-content: center;
    }
    .controls button {
      background: none;
      border: none;
      font-size: 1.2rem;
      cursor: pointer;
      color: var(--foreground);
    }
    .vol-slider {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 0.8rem;
    }
    .vol-slider input {
      flex: 1;
    }
  </style>
  ```

- [ ] **Step 2: Create Apple Music Svelte Component**
  Create `widgets/applemusic/AppleMusicWidget.svelte`:
  ```svelte
  <script lang="ts">
    export let data: any = {};
    export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

    $: isConfigured = data?.is_configured ?? false;
    $: isPlaying = data?.is_playing ?? false;
    $: trackTitle = data?.track_title ?? 'Not Playing';
    $: artistName = data?.artist_name ?? 'Unknown';

    async function handlePlayPause() {
      await dispatch(isPlaying ? 'pause' : 'play');
    }

    async function handleNext() {
      await dispatch('next');
    }
  </script>

  <div class="widget-content">
    {#if !isConfigured}
      <div class="unconfigured">
        <p class="title">Apple Music</p>
        <button class="connect-btn" on:click={() => dispatch('connect')}>Connect Apple Music</button>
      </div>
    {:else}
      <div class="player">
        <div class="info">
          <p class="track">{trackTitle}</p>
          <p class="artist">{artistName}</p>
        </div>
        <div class="controls">
          <button on:click={handlePlayPause}>{isPlaying ? '⏸' : '▶'}</button>
          <button on:click={handleNext}>⏭</button>
        </div>
      </div>
    {/if}
  </div>

  <style>
    .widget-content {
      padding: 12px;
      height: 100%;
      display: flex;
      flex-direction: column;
      justify-content: center;
    }
    .unconfigured {
      text-align: center;
    }
    .title {
      font-weight: bold;
      margin-bottom: 8px;
    }
    .connect-btn {
      padding: 6px 12px;
      background: var(--foreground);
      color: var(--inverse);
      border: none;
      border-radius: 4px;
      cursor: pointer;
    }
    .player {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }
    .track {
      font-weight: bold;
      font-size: 0.9rem;
    }
    .artist {
      color: var(--muted);
      font-size: 0.8rem;
    }
    .controls {
      display: flex;
      gap: 16px;
      justify-content: center;
    }
    .controls button {
      background: none;
      border: none;
      font-size: 1.2rem;
      cursor: pointer;
      color: var(--foreground);
    }
  </style>
  ```

- [ ] **Step 3: Create Philips Hue Svelte Component**
  Create `widgets/philipshue/PhilipsHueWidget.svelte`:
  ```svelte
  <script lang="ts">
    export let data: any = {};
    export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

    $: isConfigured = data?.is_configured ?? false;
    $: devices = data?.devices ?? [];
  </script>

  <div class="widget-content">
    {#if !isConfigured}
      <div class="unconfigured">
        <p class="title">Philips Hue</p>
        <button class="connect-btn" on:click={() => dispatch('link_bridge')}>Link Bridge</button>
      </div>
    {:else}
      <div class="devices-list">
        <p class="title">Philips Hue Lights</p>
        {#if devices.length === 0}
          <p class="empty">No devices discovered yet.</p>
        {:else}
          {#each devices as dev}
            <div class="device-row">
              <span>{dev.name}</span>
              <button on:click={() => dispatch('toggle_light', { device_id: dev.id, state: !dev.state })}>
                {dev.state ? 'ON' : 'OFF'}
              </button>
            </div>
          {/each}
        {/if}
      </div>
    {/if}
  </div>

  <style>
    .widget-content {
      padding: 12px;
      height: 100%;
      overflow-y: auto;
    }
    .unconfigured {
      text-align: center;
      padding-top: 16px;
    }
    .title {
      font-weight: bold;
      margin-bottom: 8px;
    }
    .connect-btn {
      padding: 6px 12px;
      background: var(--foreground);
      color: var(--inverse);
      border: none;
      border-radius: 4px;
      cursor: pointer;
    }
    .devices-list {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }
    .device-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      font-size: 0.85rem;
    }
    .device-row button {
      padding: 2px 8px;
      font-size: 0.75rem;
      cursor: pointer;
    }
    .empty {
      color: var(--muted);
      font-size: 0.8rem;
    }
  </style>
  ```

- [ ] **Step 4: Create Zigbee2MQTT Svelte Component**
  Create `widgets/zigbee2mqtt/Zigbee2MQTTWidget.svelte`:
  ```svelte
  <script lang="ts">
    export let data: any = {};
    export let dispatch: (action: string, args?: any) => Promise<any> = async () => {};

    $: isConfigured = data?.is_configured ?? false;
    $: devices = data?.devices ?? [];
  </script>

  <div class="widget-content">
    {#if !isConfigured}
      <div class="unconfigured">
        <p class="title">Zigbee2MQTT</p>
        <p class="help">Ensure MQTT broker is active.</p>
      </div>
    {:else}
      <div class="devices-list">
        <p class="title">Zigbee Devices</p>
        {#if devices.length === 0}
          <p class="empty">No Zigbee devices discovered.</p>
        {:else}
          {#each devices as dev}
            <div class="device-row">
              <span>{dev.name}</span>
              {#if dev.type === 'switch' || dev.type === 'light'}
                <button on:click={() => dispatch('toggle_device', { device_id: dev.id, state: !dev.state })}>
                  {dev.state ? 'ON' : 'OFF'}
                </button>
              {:else if dev.type === 'sensor'}
                <span class="sensor-val">{dev.value}</span>
              {/if}
            </div>
          {/each}
        {/if}
      </div>
    {/if}
  </div>

  <style>
    .widget-content {
      padding: 12px;
      height: 100%;
      overflow-y: auto;
    }
    .unconfigured {
      text-align: center;
      padding-top: 16px;
    }
    .title {
      font-weight: bold;
      margin-bottom: 4px;
    }
    .help {
      font-size: 0.75rem;
      color: var(--muted);
    }
    .devices-list {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }
    .device-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      font-size: 0.85rem;
    }
    .device-row button {
      padding: 2px 8px;
      font-size: 0.75rem;
      cursor: pointer;
    }
    .sensor-val {
      font-weight: bold;
      color: var(--muted);
    }
    .empty {
      color: var(--muted);
      font-size: 0.8rem;
    }
  </style>
  ```

- [ ] **Step 5: Commit Svelte components**
  Run:
  ```bash
  git add widgets/spotify/SpotifyWidget.svelte widgets/applemusic/AppleMusicWidget.svelte widgets/philipshue/PhilipsHueWidget.svelte widgets/zigbee2mqtt/Zigbee2MQTTWidget.svelte
  git commit -m "feat: add Svelte frontend components for modular widgets"
  ```

---

### Task 6: Hub Integration & Cleanup

Remove the old monolithic widgets and register the new modular ones in the backend and frontend registries.

**Files:**
- Modify: `apps/hub/cmd/juted/main.go`
- Modify: `widgets/widget-registry.ts`
- Delete: `widgets/musicplayer/`
- Delete: `widgets/smarthome/`

- [ ] **Step 1: Update apps/hub/cmd/juted/main.go**
  Modify the imports in `apps/hub/cmd/juted/main.go` (replace `jute-dash/widgets/musicplayer` and `jute-dash/widgets/smarthome` with the new ones).
  Target lines:
  ```diff
  -	_ "jute-dash/widgets/musicplayer"
  -	_ "jute-dash/widgets/smarthome"
  +	_ "jute-dash/widgets/spotify"
  +	_ "jute-dash/widgets/applemusic"
  +	_ "jute-dash/widgets/philipshue"
  +	_ "jute-dash/widgets/zigbee2mqtt"
  ```

- [ ] **Step 2: Update widgets/widget-registry.ts**
  Modify `widgets/widget-registry.ts` to register the new components:
  Target:
  ```diff
  -import MusicPlayerWidget from '$widgets/musicplayer/MusicPlayerWidget.svelte';
  -import SmartHomeWidget from '$widgets/smarthome/SmartHomeWidget.svelte';
  +import SpotifyWidget from '$widgets/spotify/SpotifyWidget.svelte';
  +import AppleMusicWidget from '$widgets/applemusic/AppleMusicWidget.svelte';
  +import PhilipsHueWidget from '$widgets/philipshue/PhilipsHueWidget.svelte';
  +import Zigbee2MQTTWidget from '$widgets/zigbee2mqtt/Zigbee2MQTTWidget.svelte';
  ```
  And map their kind IDs in `widgetRegistry`:
  ```diff
  -  'music-player': MusicPlayerWidget,
  -  'smart-home': SmartHomeWidget,
  +  'spotify': SpotifyWidget,
  +  'apple-music': AppleMusicWidget,
  +  'philips-hue': PhilipsHueWidget,
  +  'zigbee2mqtt': Zigbee2MQTTWidget,
  ```

- [ ] **Step 3: Delete old widgets**
  Delete the obsolete folders:
  ```bash
  rm -rf widgets/musicplayer
  rm -rf widgets/smarthome
  ```

- [ ] **Step 4: Verify whole suite builds and passes tests**
  Run: `make check`
  Expected: Success, clean linting, clean Svelte build, all tests pass.

- [ ] **Step 5: Commit cleanup**
  Run:
  ```bash
  git add apps/hub/cmd/juted/main.go widgets/widget-registry.ts
  git rm -rf widgets/musicplayer/ widgets/smarthome/
  git commit -m "refactor: complete monolithic widget cleanup and register modular widgets"
  ```
