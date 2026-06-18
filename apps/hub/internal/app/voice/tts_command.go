package voice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var errCommandTTSProviderUnavailable = errors.New("command TTS provider unavailable")

type CommandTTSProvider struct {
	ProviderID string
	Command    string
	Args       []string
	ModelID    string
	VoiceID    string
	Locale     string
	Timeout    time.Duration
}

func (p CommandTTSProvider) Synthesize(ctx context.Context, req TTSRequest) (TTSAudioResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return TTSAudioResult{}, errors.New("TTS text is required")
	}
	if !filepath.IsAbs(p.Command) {
		return TTSAudioResult{}, errors.New("command TTS provider command must be absolute")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := commandArgs(
		p.Args,
		"",
		firstNonEmpty(req.VoiceID, p.VoiceID, p.ModelID),
		firstNonEmpty(req.Locale, p.Locale),
	)
	for i := range args {
		args[i] = strings.ReplaceAll(args[i], "{text}", req.Text)
	}
	//nolint:gosec // command providers require explicit opt-in and absolute manifest commands.
	cmd := exec.CommandContext(commandCtx, p.Command, args...)
	output, err := cmd.Output()
	if commandCtx.Err() != nil || err != nil {
		return TTSAudioResult{}, errCommandTTSProviderUnavailable
	}
	return decodeCommandTTSOutput(output, p, req)
}

func decodeCommandTTSOutput(output []byte, p CommandTTSProvider, req TTSRequest) (TTSAudioResult, error) {
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
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return TTSAudioResult{}, errCommandTTSProviderUnavailable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return TTSAudioResult{}, errCommandTTSProviderUnavailable
	}
	return TTSAudioResult{
		ProviderID:   safeIdentifier(firstNonEmpty(out.ProviderID, p.ProviderID)),
		VoiceID:      safeIdentifier(firstNonEmpty(out.VoiceID, req.VoiceID, p.VoiceID)),
		Locale:       safeIdentifier(firstNonEmpty(out.Locale, req.Locale, p.Locale)),
		ContentType:  safeIdentifier(firstNonEmpty(out.ContentType, "audio/wav")),
		SampleRate:   out.SampleRate,
		SampleWidth:  out.SampleWidth,
		Channels:     out.Channels,
		Duration:     durationFromMillis(out.DurationMS, 0),
		PlaybackKind: safeIdentifier(firstNonEmpty(out.PlaybackKind, "audio")),
	}, nil
}

func commandArgs(args []string, inputPath string, modelID string, language string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		arg = strings.ReplaceAll(arg, "{inputPath}", inputPath)
		arg = strings.ReplaceAll(arg, "{modelId}", modelID)
		arg = strings.ReplaceAll(arg, "{language}", language)
		out = append(out, arg)
	}
	return out
}
