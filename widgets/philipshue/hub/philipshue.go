package philipshue

import (
	"context"
	"errors"
	"time"

	"jute-dash/apps/hub/pkg/widgetskills"
	"jute-dash/widgets"
	"jute-dash/widgets/philipshue/hub/internal/provider"
)

const (
	Kind    = "philips-hue"
	SkillID = "jute.philipshue.control"
)

type Device = provider.Device

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
		Fields: []widgets.ConnectionField{
			{
				ID:       "bridge_ip",
				Type:     widgets.ConnectionFieldString,
				Label:    "Bridge IP address",
				Required: true,
			},
			{
				ID:       "username",
				Type:     widgets.ConnectionFieldString,
				Label:    "Bridge username reference",
				Required: true,
				Secret:   true,
				Help:     "Use a secret reference such as env:HUE_USERNAME.",
			},
		},
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
	devices, err := provider.FetchLights(ctx, bridgeIP, username)
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
	if err := provider.ApplyAction(
		ctx,
		bridgeIP,
		username,
		deviceID,
		input.ActionID,
		input.Arguments["value"],
	); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
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
