package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

var errHTTPJSONSTTProviderUnavailable = errors.New("http-json STT provider unavailable")

type HTTPJSONSTTProvider struct {
	ProviderID string
	Endpoint   string
	ModelID    string
	Language   string
	Client     *http.Client
}

func (p HTTPJSONSTTProvider) Transcribe(ctx context.Context, utterance CapturedUtterance) (STTResult, error) {
	if len(utterance.Frames) == 0 {
		return STTResult{}, errors.New("utterance audio is required")
	}
	wav, err := EncodeBenchmarkWAV(utterance)
	if err != nil {
		return STTResult{}, err
	}
	body, err := json.Marshal(map[string]any{
		"modelId":  safeIdentifier(p.ModelID),
		"language": safeIdentifier(p.Language),
		"audio": map[string]string{
			"contentType": "audio/wav",
			"data":        base64.StdEncoding.EncodeToString(wav),
		},
	})
	if err != nil {
		return STTResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return STTResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return STTResult{}, errHTTPJSONSTTProviderUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return STTResult{}, errHTTPJSONSTTProviderUnavailable
	}
	var out struct {
		Text       string  `json:"text"`
		Transcript string  `json:"transcript"`
		ProviderID string  `json:"providerId"`
		ModelID    string  `json:"modelId"`
		Language   string  `json:"language"`
		DurationMS float64 `json:"durationMs"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return STTResult{}, errHTTPJSONSTTProviderUnavailable
	}
	text := sanitizeText(firstHTTPJSONNonEmpty(out.Text, out.Transcript))
	if text == "" {
		return STTResult{}, errors.New("http-json STT transcript was empty")
	}
	return STTResult{
		Text:       text,
		ProviderID: safeIdentifier(firstHTTPJSONNonEmpty(out.ProviderID, p.ProviderID)),
		ModelID:    safeIdentifier(firstHTTPJSONNonEmpty(out.ModelID, p.ModelID)),
		Language:   safeIdentifier(firstHTTPJSONNonEmpty(out.Language, p.Language)),
		Duration:   durationFromMillis(out.DurationMS, utterance.EndedAt.Sub(utterance.StartedAt)),
	}, nil
}

func firstHTTPJSONNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func durationFromMillis(ms float64, fallback time.Duration) time.Duration {
	if ms <= 0 {
		return fallback
	}
	return time.Duration(ms * float64(time.Millisecond))
}
