package philipshue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

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
	Username SecretString
}

func (s Settings) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("bridge_ip", s.BridgeIP),
		slog.Any("username", s.Username),
	)
}

type HueLightState struct {
	On        bool `json:"on"`
	Bri       int  `json:"bri"`
	Reachable bool `json:"reachable"`
}

type HueLight struct {
	State HueLightState `json:"state"`
	Name  string        `json:"name"`
	Type  string        `json:"type"`
}

type Device struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // "light"
	State bool   `json:"state"`
	Value string `json:"value"`
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
		Description:   "Control local smart lights connected to a Philips Hue Bridge.",
		DefaultTitle:  "Hue Lights",
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
				Help:  "Local IP address of the Philips Hue Bridge.",
			},
			{
				ID:    "username",
				Type:  widgets.SettingString,
				Label: "Username (API Key)",
				Help:  "Authorized API Username.",
			},
		},
	}
}

func (w *PhilipsHueWidget) FetchData(ctx context.Context, rawSettings map[string]any) (any, error) {
	slog.Debug( //nolint:sloglint // use default global logger
		"fetching philips hue data",
	)
	s := parseSettings(rawSettings)
	if s.BridgeIP == "" || string(s.Username) == "" {
		return map[string]any{
			"is_configured": false,
			"bridge_ip":     s.BridgeIP,
		}, nil
	}

	if s.BridgeIP == "mock-bridge" || s.BridgeIP == "test" {
		return map[string]any{
			"is_configured": true,
			"devices": []Device{
				{
					ID:    "1",
					Name:  "Living Room Light",
					Type:  "light",
					State: true,
					Value: "50%",
				},
			},
		}, nil
	}

	url := fmt.Sprintf("http://%s/api/%s/lights", s.BridgeIP, string(s.Username))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return map[string]any{
			"is_configured": true,
			"error":         err.Error(),
			"devices":       []any{},
		}, nil
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return map[string]any{
			"is_configured": true,
			"error":         err.Error(),
			"devices":       []any{},
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]any{
			"is_configured": true,
			"error":         fmt.Sprintf("Hue bridge returned status %d", resp.StatusCode),
			"devices":       []any{},
		}, nil
	}

	var rawLights map[string]HueLight
	if err := json.NewDecoder(resp.Body).Decode(&rawLights); err != nil {
		return map[string]any{
			"is_configured": true,
			"error":         err.Error(),
			"devices":       []any{},
		}, nil
	}

	devices := []Device{}
	for id, l := range rawLights {
		briPct := int(float64(l.State.Bri) / 254.0 * 100.0)
		devices = append(devices, Device{
			ID:    id,
			Name:  l.Name,
			Type:  "light",
			State: l.State.On,
			Value: fmt.Sprintf("%d%%", briPct),
		})
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})

	return map[string]any{
		"is_configured": true,
		"devices":       devices,
	}, nil
}

func (w *PhilipsHueWidget) Skill() *widgetskills.Definition {
	return &widgetskills.Definition{
		SkillID:             SkillID,
		WidgetKind:          Kind,
		DisplayName:         "Philips Hue Control",
		Summary:             "Control local Philips Hue smart lights.",
		RequiredPermissions: []string{"agent:skill"},
		VisibilityPolicy:    "visible_or_focused",
		ContextFields: []widgetskills.Field{
			{Name: "devices", Type: "array", Description: "Hue lights list.", Sensitivity: "public"},
		},
		Actions: []widgetskills.Action{
			widgetskills.ReadAction("status", "Get lights status", "List Philips Hue lights and states."),
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
	slog.Info( //nolint:sloglint // use default global logger
		"philips hue action invoked",
		"actionID", actionID,
	)

	deviceID, _ := arguments["deviceId"].(string)
	subAction, _ := arguments["action"].(string)
	val := arguments["value"]

	if deviceID == "" || subAction == "" {
		return nil, errors.New("missing deviceId or action")
	}

	s := getSettings(snap, instanceID)
	if s.BridgeIP == "" || string(s.Username) == "" {
		return nil, errors.New("philips hue is not configured")
	}

	if s.BridgeIP == "mock-bridge" || s.BridgeIP == "test" {
		return map[string]any{"status": "ok"}, nil
	}

	var targetState bool
	switch subAction {
	case "toggle":
		url := fmt.Sprintf("http://%s/api/%s/lights/%s", s.BridgeIP, string(s.Username), deviceID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var light HueLight
		if err := json.NewDecoder(resp.Body).Decode(&light); err != nil {
			return nil, err
		}
		targetState = !light.State.On
	case "turn_on":
		targetState = true
	case "turn_off":
		targetState = false
	default:
		// For set_brightness/brightness, let's preserve current power state or assume ON
		targetState = true
	}

	payload := map[string]any{
		"on": targetState,
	}

	if subAction == "brightness" || subAction == "set_brightness" {
		if b, ok := val.(float64); ok {
			payload["bri"] = int(b / 100.0 * 254.0)
		} else if b, ok := val.(int); ok {
			payload["bri"] = int(float64(b) / 100.0 * 254.0)
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	putURL := fmt.Sprintf("http://%s/api/%s/lights/%s/state", s.BridgeIP, string(s.Username), deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bridge returned status %d", resp.StatusCode)
	}

	return map[string]any{"status": "ok"}, nil
}

func getSettings(snap widgetskills.Snapshot, instanceID string) Settings {
	for _, w := range snap.Layout.Widgets {
		if w.ID == instanceID {
			return parseSettings(w.Settings)
		}
	}
	return Settings{}
}

func parseSettings(raw map[string]any) Settings {
	s := Settings{}
	if v, ok := raw["bridge_ip"].(string); ok {
		s.BridgeIP = v
	}
	if v, ok := raw["username"].(string); ok {
		s.Username = SecretString(v)
	}
	return s
}

func init() {
	widgets.RegisterWithSkill(&PhilipsHueWidget{}, func(_ widgetskills.Snapshot, _ string) map[string]any {
		return map[string]any{
			"devices": []any{},
		}
	})
}
