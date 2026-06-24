package service

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"
)

func voiceFirstNonEmpty(values ...string) string {
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

func runAudioCommand(
	ctx context.Context,
	utterance CapturedUtterance,
	prefix string,
	command string,
	args []string,
	modelID string,
	language string,
	timeout time.Duration,
) ([]byte, error) {
	wav, err := EncodeWAV(utterance)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp("", prefix+"-*.wav")
	if err != nil {
		return nil, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(wav); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	//nolint:gosec // command providers require explicit settings opt-in and absolute manifest commands.
	cmd := exec.CommandContext(commandCtx, command, commandArgs(args, tempPath, modelID, language)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	started := time.Now()
	slog.Default().DebugContext(ctx, "voice command started",
		"command", command,
		"model_id", modelID,
		"language", language,
		"frames", len(utterance.Frames),
		"audio_ms", utteranceDuration(utterance).Milliseconds(),
		"pcm_bytes", len(flattenUtterancePCM(utterance)),
	)
	output, err := cmd.Output()
	duration := time.Since(started)
	if commandCtx.Err() != nil {
		slog.Default().WarnContext(ctx, "voice command timed out",
			"command", command,
			"model_id", modelID,
			"duration_ms", duration.Milliseconds(),
			"error", commandCtx.Err(),
			"stderr", voiceLogText(stderr.String()),
		)
		return nil, commandCtx.Err()
	}
	if err != nil {
		slog.Default().WarnContext(ctx, "voice command failed",
			"command", command,
			"model_id", modelID,
			"duration_ms", duration.Milliseconds(),
			"error", err,
			"stderr", voiceLogText(stderr.String()),
		)
		return output, err
	}
	slog.Default().DebugContext(ctx, "voice command completed",
		"command", command,
		"model_id", modelID,
		"duration_ms", duration.Milliseconds(),
		"stdout_bytes", len(output),
	)
	return output, err
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

func utteranceDuration(utterance CapturedUtterance) time.Duration {
	if !utterance.StartedAt.IsZero() && !utterance.EndedAt.IsZero() {
		return utterance.EndedAt.Sub(utterance.StartedAt)
	}
	var total time.Duration
	for _, frame := range utterance.Frames {
		total += frame.Duration
	}
	return total
}

func voiceLogText(value string) string {
	value = sanitizeText(strings.TrimSpace(value))
	if len(value) > 500 {
		return value[:500] + "..."
	}
	return value
}
