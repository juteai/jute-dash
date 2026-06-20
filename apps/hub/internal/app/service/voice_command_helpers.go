package service

import (
	"context"
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
	output, err := cmd.Output()
	if commandCtx.Err() != nil {
		return nil, commandCtx.Err()
	}
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
