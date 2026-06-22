package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
		voiceFirstNonEmpty(req.VoiceID, p.VoiceID, p.ModelID),
		voiceFirstNonEmpty(req.Locale, p.Locale),
	)
	//nolint:gosec // command providers require explicit opt-in and absolute manifest commands.
	cmd := exec.CommandContext(commandCtx, p.Command, args...)
	cmd.Stdin = strings.NewReader(req.Text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	started := time.Now()
	slog.Default().DebugContext(ctx, "voice tts command started",
		"command", p.Command,
		"provider_id", p.ProviderID,
		"voice_id", voiceFirstNonEmpty(req.VoiceID, p.VoiceID),
		"locale", voiceFirstNonEmpty(req.Locale, p.Locale),
		"text_bytes", len(req.Text),
	)
	output, err := cmd.Output()
	duration := time.Since(started)
	if commandCtx.Err() != nil {
		slog.Default().WarnContext(ctx, "voice tts command timed out",
			"command", p.Command,
			"provider_id", p.ProviderID,
			"duration_ms", duration.Milliseconds(),
			"error", commandCtx.Err(),
			"stderr", voiceLogText(stderr.String()),
		)
		return TTSAudioResult{}, errCommandTTSProviderUnavailable
	}
	if err != nil {
		slog.Default().WarnContext(ctx, "voice tts command failed",
			"command", p.Command,
			"provider_id", p.ProviderID,
			"duration_ms", duration.Milliseconds(),
			"error", err,
			"stderr", voiceLogText(stderr.String()),
		)
		return TTSAudioResult{}, errCommandTTSProviderUnavailable
	}
	slog.Default().DebugContext(ctx, "voice tts command completed",
		"command", p.Command,
		"provider_id", p.ProviderID,
		"duration_ms", duration.Milliseconds(),
		"stdout_bytes", len(output),
	)
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
		ProviderID:   safeIdentifier(voiceFirstNonEmpty(out.ProviderID, p.ProviderID)),
		VoiceID:      safeIdentifier(voiceFirstNonEmpty(out.VoiceID, req.VoiceID, p.VoiceID)),
		Locale:       safeIdentifier(voiceFirstNonEmpty(out.Locale, req.Locale, p.Locale)),
		ContentType:  safeIdentifier(voiceFirstNonEmpty(out.ContentType, "audio/wav")),
		SampleRate:   out.SampleRate,
		SampleWidth:  out.SampleWidth,
		Channels:     out.Channels,
		Duration:     durationFromMillis(out.DurationMS, 0),
		PlaybackKind: safeIdentifier(voiceFirstNonEmpty(out.PlaybackKind, "audio")),
	}, nil
}
