package voice

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

const (
	WyomingEventTranscribe      = "transcribe"
	WyomingEventTranscript      = "transcript"
	WyomingEventTranscriptStart = "transcript-start"
	WyomingEventTranscriptChunk = "transcript-chunk"
	WyomingEventTranscriptStop  = "transcript-stop"
	WyomingEventAudioStart      = "audio-start"
	WyomingEventAudioChunk      = "audio-chunk"
	WyomingEventAudioStop       = "audio-stop"
)

var errWyomingSTTProviderUnavailable = errors.New("wyoming STT provider unavailable")

type STTResult struct {
	Text       string        `json:"text"`
	ProviderID string        `json:"providerId"`
	ModelID    string        `json:"modelId,omitempty"`
	Language   string        `json:"language,omitempty"`
	Duration   time.Duration `json:"duration"`
}

type ProviderHealth struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type STTProvider interface {
	Transcribe(ctx context.Context, utterance CapturedUtterance) (STTResult, error)
}

type WyomingSTTProvider struct {
	ProviderID  string
	Endpoint    string
	ModelID     string
	Language    string
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
}

func (p WyomingSTTProvider) Health(ctx context.Context) ProviderHealth {
	network, address, err := wyomingTCPAddress(p.Endpoint)
	if err != nil {
		return ProviderHealth{Status: "misconfigured", Reason: "invalid_endpoint"}
	}
	dial := p.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: 2 * time.Second}).DialContext
	}
	conn, err := dial(ctx, network, address)
	if err != nil {
		return ProviderHealth{Status: "offline", Reason: "unreachable"}
	}
	_ = conn.Close()
	return ProviderHealth{Status: "available"}
}

func (p WyomingSTTProvider) Transcribe(ctx context.Context, utterance CapturedUtterance) (STTResult, error) {
	if len(utterance.Frames) == 0 {
		return STTResult{}, errors.New("utterance audio is required")
	}
	network, address, err := wyomingTCPAddress(p.Endpoint)
	if err != nil {
		return STTResult{}, err
	}
	dial := p.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: 3 * time.Second}).DialContext
	}
	conn, err := dial(ctx, network, address)
	if err != nil {
		return STTResult{}, errWyomingSTTProviderUnavailable
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})
	defer stopClose()

	first := utterance.Frames[0]
	if err := writeWyomingEvent(conn, wyomingEvent{
		Type: WyomingEventTranscribe,
		Data: map[string]any{
			"name":     safeIdentifier(p.ModelID),
			"language": safeIdentifier(p.Language),
		},
	}); err != nil {
		return STTResult{}, errWyomingSTTProviderUnavailable
	}
	if err := writeWyomingEvent(conn, wyomingEvent{
		Type: WyomingEventAudioStart,
		Data: map[string]any{
			"rate":      first.SampleRate,
			"width":     sampleWidth(first),
			"channels":  first.Channels,
			"timestamp": 0,
		},
	}); err != nil {
		return STTResult{}, errWyomingSTTProviderUnavailable
	}
	for _, frame := range utterance.Frames {
		if err := writeWyomingEvent(conn, wyomingEvent{
			Type: WyomingEventAudioChunk,
			Data: map[string]any{
				"rate":      frame.SampleRate,
				"width":     sampleWidth(frame),
				"channels":  frame.Channels,
				"timestamp": frameTimestampMillis(utterance.StartedAt, frame.Timestamp),
			},
			Payload: frame.PCM,
		}); err != nil {
			return STTResult{}, errWyomingSTTProviderUnavailable
		}
	}
	if err := writeWyomingEvent(conn, wyomingEvent{
		Type: WyomingEventAudioStop,
		Data: map[string]any{
			"timestamp": int(utterance.EndedAt.Sub(utterance.StartedAt).Milliseconds()),
		},
	}); err != nil {
		return STTResult{}, errWyomingSTTProviderUnavailable
	}

	reader := bufio.NewReader(conn)
	var chunks []string
	language := safeIdentifier(p.Language)
	for {
		event, err := readWyomingEvent(reader)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return STTResult{}, ctx.Err()
			}
			return STTResult{}, errWyomingSTTProviderUnavailable
		}
		switch event.Type {
		case WyomingEventTranscript:
			text := sanitizeText(stringValue(event.Data["text"]))
			if text == "" {
				return STTResult{}, errors.New("wyoming STT transcript was empty")
			}
			if eventLanguage := safeIdentifier(stringValue(event.Data["language"])); eventLanguage != "" {
				language = eventLanguage
			}
			return STTResult{
				Text:       text,
				ProviderID: safeIdentifier(p.ProviderID),
				ModelID:    safeIdentifier(p.ModelID),
				Language:   language,
				Duration:   utterance.EndedAt.Sub(utterance.StartedAt),
			}, nil
		case WyomingEventTranscriptStart:
			if eventLanguage := safeIdentifier(stringValue(event.Data["language"])); eventLanguage != "" {
				language = eventLanguage
			}
		case WyomingEventTranscriptChunk:
			chunk := sanitizeTranscriptChunk(stringValue(event.Data["text"]))
			if chunk != "" {
				chunks = append(chunks, chunk)
			}
		case WyomingEventTranscriptStop:
			text := strings.TrimSpace(strings.Join(chunks, ""))
			if text == "" {
				return STTResult{}, errors.New("wyoming STT transcript was empty")
			}
			return STTResult{
				Text:       text,
				ProviderID: safeIdentifier(p.ProviderID),
				ModelID:    safeIdentifier(p.ModelID),
				Language:   language,
				Duration:   utterance.EndedAt.Sub(utterance.StartedAt),
			}, nil
		}
	}
}

func sampleWidth(frame AudioFrame) int {
	if frame.SampleWidth > 0 {
		return frame.SampleWidth
	}
	return 2
}

func frameTimestampMillis(start, ts time.Time) int {
	if ts.IsZero() || start.IsZero() {
		return 0
	}
	return int(ts.Sub(start).Milliseconds())
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func sanitizeTranscriptChunk(value string) string {
	return secretPattern.ReplaceAllString(value, "$1=[redacted]")
}
