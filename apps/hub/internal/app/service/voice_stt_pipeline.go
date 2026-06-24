package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

type STTResult struct {
	Text       string        `json:"text"`
	ProviderID string        `json:"providerId"`
	ModelID    string        `json:"modelId,omitempty"`
	Language   string        `json:"language,omitempty"`
	Duration   time.Duration `json:"duration"`
}

type STTProvider interface {
	Transcribe(ctx context.Context, utterance CapturedUtterance) (STTResult, error)
}

type FinalTranscript struct {
	Text            string        `json:"text"`
	DeviceProfileID string        `json:"deviceProfileId,omitempty"`
	DeviceID        string        `json:"deviceId,omitempty"`
	ProviderID      string        `json:"providerId,omitempty"`
	ModelID         string        `json:"modelId,omitempty"`
	Language        string        `json:"language,omitempty"`
	Duration        time.Duration `json:"duration,omitempty"`
}

func FinalTranscriptFromSTT(result STTResult, deviceProfileID, deviceID string) (FinalTranscript, error) {
	transcript := FinalTranscript{
		Text:            sanitizeText(strings.TrimSpace(result.Text)),
		DeviceProfileID: safeIdentifier(deviceProfileID),
		DeviceID:        safeIdentifier(deviceID),
		ProviderID:      safeIdentifier(result.ProviderID),
		ModelID:         safeIdentifier(result.ModelID),
		Language:        safeIdentifier(result.Language),
		Duration:        result.Duration,
	}
	if transcript.Text == "" {
		return FinalTranscript{}, errors.New("STT transcript is empty")
	}
	return transcript, nil
}
