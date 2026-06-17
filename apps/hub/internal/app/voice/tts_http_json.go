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

var errHTTPJSONTTSProviderUnavailable = errors.New("http-json TTS provider unavailable")

type HTTPJSONTTSProvider struct {
	ProviderID string
	Endpoint   string
	ModelID    string
	VoiceID    string
	Locale     string
	Client     *http.Client
}

func (p HTTPJSONTTSProvider) Synthesize(ctx context.Context, req TTSRequest) (TTSAudioResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return TTSAudioResult{}, errors.New("TTS text is required")
	}
	body, err := json.Marshal(map[string]any{
		"text":    req.Text,
		"modelId": safeIdentifier(p.ModelID),
		"voiceId": safeIdentifier(firstHTTPJSONNonEmpty(req.VoiceID, p.VoiceID)),
		"locale":  safeIdentifier(firstHTTPJSONNonEmpty(req.Locale, p.Locale)),
	})
	if err != nil {
		return TTSAudioResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint, bytes.NewReader(body))
	if err != nil {
		return TTSAudioResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(request)
	if err != nil {
		return TTSAudioResult{}, errHTTPJSONTTSProviderUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return TTSAudioResult{}, errHTTPJSONTTSProviderUnavailable
	}
	return decodeHTTPJSONTTSResponse(resp.Body, p, req)
}

func decodeHTTPJSONTTSResponse(body io.Reader, p HTTPJSONTTSProvider, req TTSRequest) (TTSAudioResult, error) {
	var out struct {
		ProviderID   string  `json:"providerId"`
		VoiceID      string  `json:"voiceId"`
		Locale       string  `json:"locale"`
		ContentType  string  `json:"contentType"`
		SampleRate   int     `json:"sampleRate"`
		SampleWidth  int     `json:"sampleWidth"`
		Channels     int     `json:"channels"`
		DurationMS   float64 `json:"durationMs"`
		PlaybackKind string  `json:"playbackKind"`
		Audio        struct {
			ContentType string `json:"contentType"`
			Data        string `json:"data"`
		} `json:"audio"`
	}
	decoder := json.NewDecoder(io.LimitReader(body, 4<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return TTSAudioResult{}, errHTTPJSONTTSProviderUnavailable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return TTSAudioResult{}, errHTTPJSONTTSProviderUnavailable
	}
	audio, err := base64.StdEncoding.DecodeString(strings.TrimSpace(out.Audio.Data))
	if err != nil || len(audio) == 0 {
		return TTSAudioResult{}, errHTTPJSONTTSProviderUnavailable
	}
	contentType := firstHTTPJSONNonEmpty(out.Audio.ContentType, out.ContentType, "audio/wav")
	playbackKind := firstHTTPJSONNonEmpty(out.PlaybackKind, "audio")
	return TTSAudioResult{
		Audio:        audio,
		ProviderID:   safeIdentifier(firstHTTPJSONNonEmpty(out.ProviderID, p.ProviderID)),
		VoiceID:      safeIdentifier(firstHTTPJSONNonEmpty(out.VoiceID, req.VoiceID, p.VoiceID)),
		Locale:       safeIdentifier(firstHTTPJSONNonEmpty(out.Locale, req.Locale, p.Locale)),
		ContentType:  safeIdentifier(contentType),
		SampleRate:   out.SampleRate,
		SampleWidth:  out.SampleWidth,
		Channels:     out.Channels,
		Duration:     durationFromMillis(out.DurationMS, 0),
		PlaybackKind: safeIdentifier(playbackKind),
	}, nil
}
