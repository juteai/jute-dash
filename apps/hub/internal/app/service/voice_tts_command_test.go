package service

import (
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestCommandTTSProviderSendsTextOnStdin(t *testing.T) {
	result, err := (CommandTTSProvider{
		ProviderID: "piper-command",
		Command:    os.Args[0],
		Args:       []string{"-test.run=TestCommandTTSProviderHelper", "--", "{modelId}", "{language}"},
		VoiceID:    "amy",
		Locale:     "en-GB",
	}).Synthesize(t.Context(), TTSRequest{Text: "hello from stdin"})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}
	if result.ProviderID != "piper-command" || result.VoiceID != "amy" || result.Locale != "en-GB" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCommandTTSProviderHelper(t *testing.T) {
	voiceID, locale := ttsHelperArgs()
	if voiceID == "" {
		return
	}
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	if string(raw) != "hello from stdin" {
		t.Fatalf("unexpected stdin text: %q", raw)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
		"providerId": "piper-command",
		"voiceId":    voiceID,
		"locale":     locale,
	})
	os.Exit(0)
}

func ttsHelperArgs() (string, string) {
	for i, arg := range os.Args {
		if arg == "--" && i+2 < len(os.Args) {
			return os.Args[i+1], os.Args[i+2]
		}
	}
	return "", ""
}
