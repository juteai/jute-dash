package voice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
)

var errCommandSTTProviderUnavailable = errors.New("command STT provider unavailable")

type CommandSTTProvider struct {
	ProviderID string
	Command    string
	Args       []string
	ModelID    string
	Language   string
	Timeout    time.Duration
}

func (p CommandSTTProvider) Transcribe(ctx context.Context, utterance CapturedUtterance) (STTResult, error) {
	if len(utterance.Frames) == 0 {
		return STTResult{}, errors.New("utterance audio is required")
	}
	if !filepath.IsAbs(p.Command) {
		return STTResult{}, errors.New("command STT provider command must be absolute")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	output, err := runAudioCommand(
		ctx,
		utterance,
		"jute-stt-command",
		p.Command,
		p.Args,
		p.ModelID,
		p.Language,
		timeout,
	)
	if err != nil {
		return STTResult{}, errCommandSTTProviderUnavailable
	}
	return decodeCommandSTTOutput(output, p, utterance.EndedAt.Sub(utterance.StartedAt))
}

func decodeCommandSTTOutput(output []byte, p CommandSTTProvider, fallbackDuration time.Duration) (STTResult, error) {
	var out struct {
		Text       string  `json:"text"`
		Transcript string  `json:"transcript"`
		ProviderID string  `json:"providerId"`
		ModelID    string  `json:"modelId"`
		Language   string  `json:"language"`
		DurationMS float64 `json:"durationMs"`
		Duration   string  `json:"duration"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return STTResult{}, errCommandSTTProviderUnavailable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return STTResult{}, errCommandSTTProviderUnavailable
	}
	text := sanitizeText(firstNonEmpty(out.Text, out.Transcript))
	if text == "" {
		return STTResult{}, errors.New("command STT transcript was empty")
	}
	duration := durationFromMillis(out.DurationMS, fallbackDuration)
	if out.DurationMS <= 0 && strings.TrimSpace(out.Duration) != "" {
		if parsed, err := time.ParseDuration(strings.TrimSpace(out.Duration)); err == nil {
			duration = parsed
		}
	}
	return STTResult{
		Text:       text,
		ProviderID: safeIdentifier(firstNonEmpty(out.ProviderID, p.ProviderID)),
		ModelID:    safeIdentifier(firstNonEmpty(out.ModelID, p.ModelID)),
		Language:   safeIdentifier(firstNonEmpty(out.Language, p.Language)),
		Duration:   duration,
	}, nil
}
