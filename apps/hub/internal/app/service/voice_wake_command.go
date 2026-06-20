package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
)

var errCommandWakeProviderUnavailable = errors.New("command wake provider unavailable")

type WakeDetection struct {
	Detected   bool
	ProviderID string
	ModelID    string
	Confidence float64
}

type WakeEventEmitter interface {
	EmitVoiceStateChanged(deviceID string, payload VoiceStatePayload) VoiceEvent
	EmitVoiceWakeDetected(deviceID, conversationID string) VoiceEvent
}

type WakeProvider interface {
	DetectWake(ctx context.Context, utterance CapturedUtterance) (WakeDetection, error)
}

type CommandWakeProvider struct {
	ProviderID string
	Command    string
	Args       []string
	ModelID    string
	Timeout    time.Duration
}

func (p CommandWakeProvider) DetectWake(ctx context.Context, utterance CapturedUtterance) (WakeDetection, error) {
	if len(utterance.Frames) == 0 {
		return WakeDetection{}, errors.New("utterance audio is required")
	}
	if !filepath.IsAbs(p.Command) {
		return WakeDetection{}, errors.New("command wake provider command must be absolute")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	output, err := runAudioCommand(ctx, utterance, "jute-wake-command", p.Command, p.Args, p.ModelID, "", timeout)
	if err != nil {
		return WakeDetection{}, errCommandWakeProviderUnavailable
	}
	return decodeCommandWakeOutput(output, p)
}

func decodeCommandWakeOutput(output []byte, p CommandWakeProvider) (WakeDetection, error) {
	var out struct {
		Detected   bool    `json:"detected"`
		ProviderID string  `json:"providerId"`
		ModelID    string  `json:"modelId"`
		Confidence float64 `json:"confidence"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return WakeDetection{}, errCommandWakeProviderUnavailable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return WakeDetection{}, errCommandWakeProviderUnavailable
	}
	return WakeDetection{
		Detected:   out.Detected,
		ProviderID: safeIdentifier(voiceFirstNonEmpty(out.ProviderID, p.ProviderID)),
		ModelID:    safeIdentifier(voiceFirstNonEmpty(out.ModelID, p.ModelID)),
		Confidence: out.Confidence,
	}, nil
}
