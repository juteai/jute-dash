package voice

import (
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

type SatelliteEventRequest struct {
	Type                     string   `json:"type"`
	State                    string   `json:"state,omitempty"`
	Health                   string   `json:"health,omitempty"`
	Version                  string   `json:"version,omitempty"`
	UpdateChannel            string   `json:"updateChannel,omitempty"`
	WakeModelID              string   `json:"wakeModelId,omitempty"`
	ProviderIDs              []string `json:"providerIds,omitempty"`
	SafeErrorCode            string   `json:"safeErrorCode,omitempty"`
	LocalProcessingLatencyMS int      `json:"localProcessingLatencyMs,omitempty"`
}

var safeSatelliteToken = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func DecodeSatelliteEventRequest(r io.Reader) (SatelliteEventRequest, error) {
	var req SatelliteEventRequest
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		return SatelliteEventRequest{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return SatelliteEventRequest{}, errors.New("trailing JSON data")
	}
	return req, nil
}

func satelliteEventPayload(req SatelliteEventRequest) (string, SatelliteEventPayload, error) {
	return SatelliteEventPayloadFromRequest(req)
}

func SatelliteEventPayloadFromRequest(req SatelliteEventRequest) (string, SatelliteEventPayload, error) {
	eventType := strings.TrimSpace(req.Type)
	if eventType == "" {
		eventType = EventVoiceSatelliteStateChanged
	}
	if !allowedSatelliteEventType(eventType) {
		return "", SatelliteEventPayload{}, errors.New("unsupported satellite event type")
	}

	payload := SatelliteEventPayload{
		State:                    strings.TrimSpace(req.State),
		Health:                   strings.TrimSpace(req.Health),
		Version:                  strings.TrimSpace(req.Version),
		UpdateChannel:            strings.TrimSpace(req.UpdateChannel),
		WakeModelID:              strings.TrimSpace(req.WakeModelID),
		ProviderIDs:              normalizeSatelliteProviderIDs(req.ProviderIDs),
		SafeErrorCode:            strings.TrimSpace(req.SafeErrorCode),
		LocalProcessingLatencyMS: req.LocalProcessingLatencyMS,
	}
	if err := validateSatelliteEventRequiredFields(eventType, payload); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	if payload.LocalProcessingLatencyMS < 0 {
		return "", SatelliteEventPayload{}, errors.New("negative latency")
	}
	if err := validateSatelliteTokenField(payload.State, false); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	if err := validateSatelliteTokenField(payload.Health, false); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	if err := validateSatelliteTokenField(payload.Version, false); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	if err := validateSatelliteTokenField(payload.UpdateChannel, false); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	if err := validateSatelliteTokenField(payload.WakeModelID, false); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	if err := validateSatelliteTokenField(payload.SafeErrorCode, false); err != nil {
		return "", SatelliteEventPayload{}, err
	}
	for _, providerID := range payload.ProviderIDs {
		if err := validateSatelliteTokenField(providerID, true); err != nil {
			return "", SatelliteEventPayload{}, err
		}
	}
	return eventType, payload, nil
}

func allowedSatelliteEventType(eventType string) bool {
	switch eventType {
	case EventVoiceSatelliteStateChanged,
		EventVoiceSatelliteHealthChanged,
		EventVoiceSatelliteWakeDetected,
		EventVoiceSatelliteVersionChanged,
		EventVoiceSatelliteUpdateAvailable:
		return true
	default:
		return false
	}
}

func validateSatelliteEventRequiredFields(eventType string, payload SatelliteEventPayload) error {
	switch eventType {
	case EventVoiceSatelliteStateChanged:
		if payload.State == "" {
			return errors.New("satellite state is required")
		}
	case EventVoiceSatelliteHealthChanged:
		if payload.Health == "" {
			return errors.New("satellite health is required")
		}
	case EventVoiceSatelliteWakeDetected:
		if payload.WakeModelID == "" {
			return errors.New("satellite wake model is required")
		}
	case EventVoiceSatelliteVersionChanged:
		if payload.Version == "" {
			return errors.New("satellite version is required")
		}
	case EventVoiceSatelliteUpdateAvailable:
		if payload.Version == "" && payload.UpdateChannel == "" {
			return errors.New("satellite update version or channel is required")
		}
	}
	return nil
}

func normalizeSatelliteProviderIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return normalized
}

func validateSatelliteTokenField(value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return errors.New("required satellite token is empty")
		}
		return nil
	}
	if secretPattern.MatchString(value) || !safeSatelliteToken.MatchString(value) {
		return errors.New("unsafe satellite token")
	}
	return nil
}
