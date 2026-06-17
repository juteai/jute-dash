package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/app/voice"
)

func main() {
	if code := run(os.Args[1:], os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("jute-voice-satellite", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to satellite runtime JSON config")
	hubURL := flags.String("hub-url", "", "hub base URL")
	satelliteID := flags.String("satellite-id", "", "paired satellite ID")
	authSecretEnv := flags.String("auth-secret-env", "", "environment variable containing the satellite auth proof")
	fixtureWAV := flags.String("fixture-wav", "", "16 kHz mono PCM WAV fixture for CI/local smoke runs")
	transcript := flags.String("transcript", "", "mock STT final transcript to submit after fixture speech")
	conversationID := flags.String("conversation-id", "", "hub voice conversation ID for follow-up turns")
	version := flags.String("version", "0.1.0", "satellite runtime version")
	wakeModelID := flags.String("wake-model-id", "fixture-wake", "safe wake provider/model identifier")
	timeout := flags.Duration("timeout", 15*time.Second, "runtime timeout")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg := voice.SatelliteRuntimeConfig{}
	if *configPath != "" {
		loaded, err := voice.LoadSatelliteRuntimeConfig(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", safeCLIError(err))
			return 2
		}
		cfg = loaded
	}
	mergeFlags(
		&cfg,
		*hubURL,
		*satelliteID,
		*authSecretEnv,
		*fixtureWAV,
		*transcript,
		*conversationID,
		*version,
		*wakeModelID,
		*timeout,
	)
	result, err := (voice.SatelliteRuntime{}).RunFixture(context.Background(), cfg)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", safeCLIError(err))
		return 1
	}
	return 0
}

func mergeFlags(
	cfg *voice.SatelliteRuntimeConfig,
	hubURL string,
	satelliteID string,
	authSecretEnv string,
	fixtureWAV string,
	transcript string,
	conversationID string,
	version string,
	wakeModelID string,
	timeout time.Duration,
) {
	if hubURL != "" {
		cfg.HubURL = hubURL
	}
	if satelliteID != "" {
		cfg.SatelliteID = satelliteID
	}
	if authSecretEnv != "" {
		cfg.AuthSecretEnv = authSecretEnv
	}
	if fixtureWAV != "" {
		cfg.FixtureWAV = fixtureWAV
	}
	if transcript != "" {
		cfg.Transcript = transcript
	}
	if conversationID != "" {
		cfg.ConversationID = conversationID
	}
	if version != "" {
		cfg.Version = version
	}
	if wakeModelID != "" {
		cfg.WakeModelID = wakeModelID
	}
	if timeout > 0 {
		cfg.Timeout = timeout
	}
}

func safeCLIError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if text == "" {
		return "satellite runtime failed"
	}
	switch {
	case strings.Contains(text, "auth_failed"):
		return "auth_failed"
	case strings.Contains(text, "hub_unreachable"):
		return "hub_unreachable"
	case strings.Contains(text, "credential"):
		return "credential_unavailable"
	case strings.Contains(text, "wake"):
		return "wake_provider_unavailable"
	case strings.Contains(text, "clock"):
		return "clock_skew"
	case strings.Contains(text, "audio"), strings.Contains(text, "microphone"), strings.Contains(text, "fixture"):
		return "microphone_unavailable"
	}
	return text
}
