package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProviderManifestAcceptsCommandSTTProvider(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "provider_manifests", "go_whisper_command.json"))
	if err != nil {
		t.Fatalf("read command manifest: %v", err)
	}
	manifest, err := DecodeProviderManifest(string(raw))
	if err != nil {
		t.Fatalf("decode command manifest: %v", err)
	}

	if problems := ValidateProviderManifest(manifest); len(problems) != 0 {
		t.Fatalf("expected valid command manifest, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsUnsupportedTransports(t *testing.T) {
	for _, transportType := range []string{"wyoming", "http-json", "builtin"} {
		manifest := validWakeWordManifest()
		manifest.Transport.Type = transportType

		problems := ValidateProviderManifest(manifest)

		if !hasProblem(problems, "transport.type must be command") {
			t.Fatalf("expected unsupported transport problem for %q, got %v", transportType, problems)
		}
	}
}

func TestValidateProviderManifestRequiresAbsoluteCommand(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Transport.Command = "openwakeword"

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "transport.command must be absolute") {
		t.Fatalf("expected absolute command problem, got %v", problems)
	}
}

func TestValidateProviderManifestRequiresSTTCommandPlaceholders(t *testing.T) {
	manifest := validSTTManifest()
	manifest.Transport.Args = []string{"transcribe", "--format", "json", "{inputPath}"}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "transport.args must include {modelId}") {
		t.Fatalf("expected model placeholder problem, got %v", problems)
	}

	manifest.Transport.Args = []string{"transcribe", "--format", "json", "--model", "{modelId}"}
	problems = ValidateProviderManifest(manifest)

	if !hasProblem(problems, "transport.args must include {inputPath}") {
		t.Fatalf("expected input placeholder problem, got %v", problems)
	}
}

func TestValidateProviderManifestRequiresWakeCommandInputPlaceholder(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Transport.Args = []string{"detect", "--json"}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "transport.args must include {inputPath}") {
		t.Fatalf("expected wake input placeholder problem, got %v", problems)
	}
}

func TestDecodeProviderManifestRejectsTrailingJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "provider_manifests", "go_whisper_command.json"))
	if err != nil {
		t.Fatalf("read command manifest: %v", err)
	}

	_, err = DecodeProviderManifest(string(raw) + `{"rawAudioPcm":"secret"}`)

	if err == nil || !strings.Contains(err.Error(), "trailing JSON data") {
		t.Fatalf("expected trailing JSON decode error, got %v", err)
	}
}

func TestValidateProviderManifestRejectsUnsafeWakeModelPath(t *testing.T) {
	for _, path := range []string{"../models/hey-jute.tflite", "/opt/models/hey-jute.tflite", "https://example.com/model"} {
		manifest := validWakeWordManifest()
		manifest.WakeWord.Models[0].Path = path

		problems := ValidateProviderManifest(manifest)

		if !hasProblem(problems, "path must be a relative provider-pack asset path") {
			t.Fatalf("expected unsafe model path problem for %q, got %v", path, problems)
		}
	}
}

func TestValidateProviderManifestRejectsUnsafeCredentialDeclarations(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Credentials = []CredentialManifest{{
		ID:       "apiKey",
		Label:    "token=secret",
		Source:   "env",
		Env:      "EXAMPLE_API_KEY",
		Required: true,
	}}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "must reference a secret") {
		t.Fatalf("expected credential problem, got %v", problems)
	}
}

func validWakeWordManifest() ProviderManifest {
	return ProviderManifest{
		ID:      "org.example.openwakeword",
		Name:    "Example openWakeWord",
		Version: "1.0.0",
		Kind:    ProviderKindWakeWord,
		Transport: TransportManifest{
			Type:    "command",
			Command: "/usr/local/bin/openwakeword",
			Args:    []string{"detect", "{inputPath}", "--model", "{modelId}"},
		},
		Capabilities: ProviderCapabilities{
			Offline:   true,
			Languages: []string{"en", "en-GB"},
		},
		Credentials: []CredentialManifest{},
		WakeWord: WakeWordManifest{
			DefaultModelID: "hey-jute",
			Phrase:         "Hey Jute",
			Languages:      []string{"en", "en-GB"},
			Sensitivity:    0.55,
			Models: []WakeWordModelManifest{{
				ID:          "hey-jute",
				Path:        "assets/hey-jute.tflite",
				Phrase:      "Hey Jute",
				Languages:   []string{"en"},
				Sensitivity: 0.55,
			}},
		},
	}
}

func validSTTManifest() ProviderManifest {
	manifest := validWakeWordManifest()
	manifest.ID = "org.example.local-stt"
	manifest.Name = "Example STT"
	manifest.Kind = ProviderKindSTT
	manifest.Transport = TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/stt",
		Args:    []string{"transcribe", "{modelId}", "{inputPath}"},
	}
	manifest.WakeWord = WakeWordManifest{}
	return manifest
}

func hasProblem(problems []string, fragment string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, fragment) {
			return true
		}
	}
	return false
}
