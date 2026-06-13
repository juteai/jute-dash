package philipshue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
)

const (
	Kind    = "philips-hue"
	SkillID = "jute.philipshue.control"
)

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
	Type  string `json:"type"`
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

func (w *PhilipsHueWidget) RequiredConnections() []widgets.ConnectionRequirement {
	return []widgets.ConnectionRequirement{{
		Slot:        "bridge",
		Kind:        "philips-hue",
		DisplayName: "Philips Hue Bridge",
		Description: "Local Hue Bridge address and API username.",
		Required:    true,
		SecretKeys:  []string{"username"},
	}}
}

func (w *PhilipsHueWidget) CatalogInfo() widgets.WidgetCatalogItem {
	return widgets.WidgetCatalogItem{
		Kind:                   Kind,
		Name:                   "Philips Hue",
		Description:            "Control local smart lights connected to a Philips Hue Bridge.",
		DefaultTitle:           "Hue Lights",
		DefaultW:               6,
		DefaultH:               2,
		MinW:                   4,
		MinH:                   2,
		DefaultSize:            "wide",
		Overflow:               "clip",
		AllowMultiple:          false,
		ConnectionRequirements: w.RequiredConnections(),
	}
}

func (w *PhilipsHueWidget) FetchData(_ context.Context, _ map[string]any) (any, error) {
	return widgets.Unavailable(
		"connection.missing",
		"Connection needed",
		"Choose a Philips Hue Bridge connection in settings.",
	), nil
}

func (w *PhilipsHueWidget) FetchDataWithConnections(
	ctx context.Context,
	input widgets.RuntimeInput,
) (widgets.RuntimePayload, error) {
	bridge, ok := input.Connections["bridge"]
	if !ok {
		return widgets.Unavailable(
			"connection.missing",
			"Connection needed",
			"Choose a Philips Hue Bridge connection in settings.",
		), nil
	}
	bridgeIP, _ := bridge.Settings["bridge_ip"].(string)
	username := bridge.Secrets["username"]
	if bridgeIP == "" || username == "" {
		return widgets.Unavailable(
			"connection.missing_credentials",
			"Bridge credentials unavailable",
			"Update the Philips Hue Bridge connection in settings.",
		), nil
	}
	if bridgeIP == "mock-bridge" || bridgeIP == "test" {
		return widgets.OK(map[string]any{"devices": []Device{{
			ID:    "1",
			Name:  "Living Room Light",
			Type:  "light",
			State: true,
			Value: "50%",
		}}}), nil
	}
	devices, err := fetchLights(ctx, bridgeIP, username)
	if err != nil {
		return widgets.Unavailable( //nolint:nilerr // provider error is mapped to a safe widget issue
			"hue.bridge_unavailable",
			"Hue Bridge unavailable",
			"Jute cannot reach the Philips Hue Bridge.",
		), nil
	}
	return widgets.RuntimePayload{
		Status:    widgets.StatusOK,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      map[string]any{"devices": devices},
	}, nil
}

func fetchLights(ctx context.Context, bridgeIP, username string) ([]Device, error) {
	url := fmt.Sprintf("http://%s/api/%s/lights", bridgeIP, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hue bridge returned status %d", resp.StatusCode)
	}
	var rawLights map[string]HueLight
	if err := json.NewDecoder(resp.Body).Decode(&rawLights); err != nil {
		return nil, err
	}
	devices := []Device{}
	for id, light := range rawLights {
		briPct := int(float64(light.State.Bri) / 254.0 * 100.0)
		devices = append(devices, Device{
			ID:    id,
			Name:  light.Name,
			Type:  "light",
			State: light.State.On,
			Value: fmt.Sprintf("%d%%", briPct),
		})
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})
	return devices, nil
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
			homeAction("toggle", "Toggle light", "Toggle a Philips Hue light."),
			homeAction("turn_on", "Turn light on", "Turn a Philips Hue light on."),
			homeAction("turn_off", "Turn light off", "Turn a Philips Hue light off."),
			homeAction("set_brightness", "Set brightness", "Set Philips Hue light brightness."),
		},
	}
}

func homeAction(id, title, description string) widgetskills.Action {
	return widgetskills.Action{
		ID:                   id,
		Title:                title,
		Description:          description,
		SideEffect:           "home_action",
		RequiresConfirmation: false,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deviceId": map[string]any{"type": "string"},
				"value":    map[string]any{},
			},
			"required":             []string{"deviceId"},
			"additionalProperties": true,
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"status": map[string]any{"type": "string"}},
			"required":   []string{"status"},
		},
	}
}

func (w *PhilipsHueWidget) InvokeActionWithConnections(
	ctx context.Context,
	input widgets.ActionInput,
) (map[string]any, error) {
	deviceID, _ := input.Arguments["deviceId"].(string)
	if deviceID == "" {
		deviceID, _ = input.Arguments["device_id"].(string)
	}
	if deviceID == "" {
		return nil, errors.New("deviceId is required")
	}
	bridge, ok := input.Connections["bridge"]
	if !ok {
		return nil, errors.New("philips hue bridge connection is missing")
	}
	bridgeIP, _ := bridge.Settings["bridge_ip"].(string)
	username := bridge.Secrets["username"]
	if bridgeIP == "" || username == "" {
		return nil, errors.New("philips hue bridge credentials are missing")
	}
	if bridgeIP == "mock-bridge" || bridgeIP == "test" {
		return map[string]any{"status": "ok"}, nil
	}
	payload, err := huePayload(ctx, bridgeIP, username, deviceID, input.ActionID, input.Arguments["value"])
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	putURL := fmt.Sprintf("http://%s/api/%s/lights/%s/state", bridgeIP, username, deviceID)
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

func huePayload(
	ctx context.Context,
	bridgeIP string,
	username string,
	deviceID string,
	actionID string,
	value any,
) (map[string]any, error) {
	payload := map[string]any{}
	switch actionID {
	case "toggle":
		url := fmt.Sprintf("http://%s/api/%s/lights/%s", bridgeIP, username, deviceID)
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
		payload["on"] = !light.State.On
	case "turn_on":
		payload["on"] = true
	case "turn_off":
		payload["on"] = false
	case "set_brightness":
		payload["on"] = true
		switch b := value.(type) {
		case float64:
			payload["bri"] = int(b / 100.0 * 254.0)
		case int:
			payload["bri"] = int(float64(b) / 100.0 * 254.0)
		default:
			return nil, errors.New("brightness value is required")
		}
	default:
		return nil, fmt.Errorf("unknown action: %s", actionID)
	}
	return payload, nil
}

func init() {
	widgets.RegisterWithSkill(&PhilipsHueWidget{}, func(snapshot widgetskills.Snapshot, instID string) map[string]any {
		for _, widget := range snapshot.Layout.Widgets {
			if widget.ID == instID {
				if data, ok := widgets.PayloadData(widget.Data).(map[string]any); ok {
					return data
				}
			}
		}
		return map[string]any{"devices": []any{}}
	})
}
