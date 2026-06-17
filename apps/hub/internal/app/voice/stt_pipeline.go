package voice

import (
	"context"
	"errors"
	"strings"
	"time"
)

type FinalTranscript struct {
	Text            string        `json:"text"`
	DeviceProfileID string        `json:"deviceProfileId,omitempty"`
	DeviceID        string        `json:"deviceId,omitempty"`
	ProviderID      string        `json:"providerId,omitempty"`
	ModelID         string        `json:"modelId,omitempty"`
	Language        string        `json:"language,omitempty"`
	Duration        time.Duration `json:"duration,omitempty"`
}

type FinalTranscriptSink interface {
	SubmitFinalTranscript(ctx context.Context, transcript FinalTranscript) error
}

type STTTurnProcessor struct {
	provider        STTProvider
	sink            FinalTranscriptSink
	deviceProfileID string
	deviceID        string
}

func NewSTTTurnProcessor(
	provider STTProvider,
	sink FinalTranscriptSink,
	deviceProfileID string,
	deviceID string,
) *STTTurnProcessor {
	return &STTTurnProcessor{
		provider:        provider,
		sink:            sink,
		deviceProfileID: safeIdentifier(deviceProfileID),
		deviceID:        safeIdentifier(deviceID),
	}
}

func (p *STTTurnProcessor) Process(ctx context.Context, utterance CapturedUtterance) (FinalTranscript, error) {
	if p == nil || p.provider == nil {
		return FinalTranscript{}, errors.New("STT provider is unavailable")
	}
	if p.sink == nil {
		return FinalTranscript{}, errors.New("final transcript sink is unavailable")
	}
	result, err := p.provider.Transcribe(ctx, cloneSTTTurnUtterance(utterance))
	if err != nil {
		return FinalTranscript{}, errors.New("STT provider unavailable")
	}
	transcript := FinalTranscript{
		Text:            sanitizeText(strings.TrimSpace(result.Text)),
		DeviceProfileID: p.deviceProfileID,
		DeviceID:        p.deviceID,
		ProviderID:      safeIdentifier(result.ProviderID),
		ModelID:         safeIdentifier(result.ModelID),
		Language:        safeIdentifier(result.Language),
		Duration:        result.Duration,
	}
	if transcript.Text == "" {
		return FinalTranscript{}, errors.New("STT transcript is empty")
	}
	if err := p.sink.SubmitFinalTranscript(ctx, transcript); err != nil {
		return FinalTranscript{}, err
	}
	return transcript, nil
}

func cloneSTTTurnUtterance(utterance CapturedUtterance) CapturedUtterance {
	return CapturedUtterance{
		Frames:     cloneAudioFrames(utterance.Frames),
		StartedAt:  utterance.StartedAt,
		EndedAt:    utterance.EndedAt,
		SampleRate: utterance.SampleRate,
		Channels:   utterance.Channels,
	}
}
