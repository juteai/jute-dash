package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
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

func FetchLights(ctx context.Context, bridgeIP, username string) ([]Device, error) {
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

func ApplyAction(
	ctx context.Context,
	bridgeIP string,
	username string,
	deviceID string,
	actionID string,
	value any,
) error {
	payload, err := Payload(ctx, bridgeIP, username, deviceID, actionID, value)
	if err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	putURL := fmt.Sprintf("http://%s/api/%s/lights/%s/state", bridgeIP, username, deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bridge returned status %d", resp.StatusCode)
	}
	return nil
}

func Payload(
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
