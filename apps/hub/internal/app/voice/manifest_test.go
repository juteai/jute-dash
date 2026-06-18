package voice

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateProviderManifestAcceptsWakeWordProvider(t *testing.T) {
	manifest := validWakeWordManifest()

	if problems := ValidateProviderManifest(manifest); len(problems) != 0 {
		t.Fatalf("expected valid wake provider manifest, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsUndeclaredWakeModel(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.WakeWord.DefaultModelID = "missing-model"

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "defaultModelId must reference") {
		t.Fatalf("expected undeclared model problem, got %v", problems)
	}
}

func TestDecodeProviderManifestRejectsTrailingJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "provider_manifests", "go_whisper_http_json.json"))
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}

	_, err = DecodeProviderManifest(string(raw) + `{"rawAudioPcm":"secret"}`)

	if err == nil || !strings.Contains(err.Error(), "trailing JSON data") {
		t.Fatalf("expected trailing JSON decode error, got %v", err)
	}
}

func TestValidateProviderManifestRejectsUnsafeWakeModelPath(t *testing.T) {
	for _, path := range []string{
		"../models/hey-jute.tflite",
		"/opt/models/hey-jute.tflite",
		"https://example.com/hey-jute.tflite",
	} {
		manifest := validWakeWordManifest()
		manifest.WakeWord.Models[0].Path = path

		problems := ValidateProviderManifest(manifest)

		if !hasProblem(problems, "path must be a relative provider-pack asset path") {
			t.Fatalf("expected unsafe model path problem for %q, got %v", path, problems)
		}
	}
}

func TestValidateProviderManifestRejectsDuplicateWakeModelIDs(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.WakeWord.Models = append(manifest.WakeWord.Models, WakeWordModelManifest{
		ID:          "hey-jute",
		Path:        "assets/hey-jute-v2.tflite",
		Phrase:      "Hey Jute",
		Languages:   []string{"en"},
		Sensitivity: 0.6,
	})

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "wakeWord.models[1].id must be unique") {
		t.Fatalf("expected duplicate wake model problem, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsDuplicateTTSVoiceIDs(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Kind = ProviderKindTTS
	manifest.WakeWord = WakeWordManifest{}
	manifest.TTS = TTSManifest{
		DefaultVoiceID: "amy",
		Voices: []TTSVoiceManifest{
			{ID: "amy", Label: "Amy", Locale: "en-GB"},
			{ID: "amy", Label: "Amy duplicate", Locale: "en-US"},
		},
	}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "tts.voices[1].id must be unique") {
		t.Fatalf("expected duplicate TTS voice problem, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsUnsafeRemoteEndpoint(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Transport.Endpoint = "tcp://voice.example.com:10400"

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "loopback or LAN-scoped") {
		t.Fatalf("expected unsafe endpoint problem, got %v", problems)
	}
}

func TestValidateProviderManifestRequiresSTTCommandPlaceholders(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "provider_manifests", "go_whisper_command.json"))
	if err != nil {
		t.Fatalf("read go-whisper command manifest: %v", err)
	}
	base, err := DecodeProviderManifest(string(raw))
	if err != nil {
		t.Fatalf("decode go-whisper command manifest: %v", err)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing model placeholder",
			args: []string{"transcribe", "--format", "json", "{inputPath}"},
			want: "transport.args must include {modelId} for STT-capable command providers",
		},
		{
			name: "missing input placeholder",
			args: []string{"transcribe", "--format", "json", "--model", "{modelId}"},
			want: "transport.args must include {inputPath} for STT-capable command providers",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := base
			manifest.Transport.Args = tt.args

			problems := ValidateProviderManifest(manifest)

			if !hasProblem(problems, tt.want) {
				t.Fatalf("expected command placeholder problem containing %q, got %v", tt.want, problems)
			}
		})
	}
}

func TestValidateProviderManifestRequiresSTTTTSCommandPlaceholders(t *testing.T) {
	manifest := validSTTTTSManifest()
	manifest.Transport = TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/stt-tts-sidecar",
		Args:    []string{"transcribe", "--format", "json"},
	}

	problems := ValidateProviderManifest(manifest)

	for _, want := range []string{
		"transport.args must include {modelId} for STT-capable command providers",
		"transport.args must include {inputPath} for STT-capable command providers",
	} {
		if !hasProblem(problems, want) {
			t.Fatalf("expected hybrid command placeholder problem %q, got %v", want, problems)
		}
	}
}

func TestValidateProviderManifestRequiresWakeCommandInputPlaceholder(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "provider_manifests", "microwakeword_experimental.json"))
	if err != nil {
		t.Fatalf("read microWakeWord manifest: %v", err)
	}
	manifest, err := DecodeProviderManifest(string(raw))
	if err != nil {
		t.Fatalf("decode microWakeWord manifest: %v", err)
	}
	manifest.Transport = TransportManifest{
		Type:    "command",
		Command: "/usr/local/bin/microwakeword",
		Args:    []string{"detect", "--model", "okay-nabu", "--json"},
	}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "transport.args must include {inputPath} for wake-word command providers") {
		t.Fatalf("expected wake command input placeholder problem, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsEndpointCredentials(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		transport TransportManifest
		offline   bool
		want      string
	}{
		{
			name: "wyoming userinfo",
			kind: ProviderKindWakeWord,
			transport: TransportManifest{
				Type:     "wyoming",
				Endpoint: "tcp://token:secret@127.0.0.1:10400",
			},
			offline: true,
			want:    "loopback or LAN-scoped",
		},
		{
			name: "http-json userinfo",
			kind: ProviderKindTTS,
			transport: TransportManifest{
				Type:     "http-json",
				Endpoint: "https://token:secret@tts.example.com/v1",
			},
			offline: false,
			want:    "local/LAN HTTP or HTTPS",
		},
		{
			name: "http-json token query",
			kind: ProviderKindTTS,
			transport: TransportManifest{
				Type:     "http-json",
				Endpoint: "https://tts.example.com/v1?api_key=secret",
			},
			offline: false,
			want:    "local/LAN HTTP or HTTPS",
		},
		{
			name: "http-json generic key query",
			kind: ProviderKindTTS,
			transport: TransportManifest{
				Type:     "http-json",
				Endpoint: "https://tts.example.com/v1?key=secret",
			},
			offline: false,
			want:    "local/LAN HTTP or HTTPS",
		},
		{
			name: "http-json secret-looking query value",
			kind: ProviderKindTTS,
			transport: TransportManifest{
				Type:     "http-json",
				Endpoint: "https://tts.example.com/v1?profile=bearer%20abc123",
			},
			offline: false,
			want:    "local/LAN HTTP or HTTPS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validWakeWordManifest()
			manifest.Kind = tt.kind
			manifest.Transport = tt.transport
			manifest.Capabilities.Offline = tt.offline
			if tt.kind == ProviderKindTTS {
				manifest.WakeWord = WakeWordManifest{}
				manifest.TTS = TTSManifest{
					DefaultVoiceID: "amy",
					Voices: []TTSVoiceManifest{{
						ID:     "amy",
						Label:  "Amy",
						Locale: "en-GB",
					}},
				}
			}

			problems := ValidateProviderManifest(manifest)

			if !hasProblem(problems, tt.want) {
				t.Fatalf("expected endpoint credential problem containing %q, got %v", tt.want, problems)
			}
		})
	}
}

func TestValidateProviderManifestAcceptsCredentialReferences(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Kind = ProviderKindTTS
	manifest.WakeWord = WakeWordManifest{}
	manifest.Transport = TransportManifest{
		Type:     "http-json",
		Endpoint: "https://tts.example.com/v1",
	}
	manifest.Capabilities.Offline = false
	manifest.Credentials = []CredentialManifest{{
		ID:       "apiKey",
		Label:    "API key",
		Source:   "env",
		Env:      "EXAMPLE_TTS_API_KEY",
		Required: true,
	}}
	manifest.TTS = TTSManifest{
		DefaultVoiceID: "amy",
		Voices: []TTSVoiceManifest{{
			ID:     "amy",
			Label:  "Amy",
			Locale: "en-GB",
		}},
	}

	if problems := ValidateProviderManifest(manifest); len(problems) != 0 {
		t.Fatalf("expected valid credential reference, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsUnsafeCredentialDeclarations(t *testing.T) {
	tests := []struct {
		name       string
		credential CredentialManifest
		want       string
	}{
		{
			name:       "missing id",
			credential: CredentialManifest{Label: "API key", Source: "env", Env: "EXAMPLE_API_KEY"},
			want:       "credentials[0].id is required",
		},
		{
			name:       "missing label",
			credential: CredentialManifest{ID: "apiKey", Source: "env", Env: "EXAMPLE_API_KEY"},
			want:       "credentials[0].label is required",
		},
		{
			name:       "unsupported source",
			credential: CredentialManifest{ID: "apiKey", Label: "API key", Source: "inline", Env: "EXAMPLE_API_KEY"},
			want:       "credentials[0].source must be env",
		},
		{
			name:       "missing env",
			credential: CredentialManifest{ID: "apiKey", Label: "API key", Source: "env"},
			want:       "credentials[0].env is required",
		},
		{
			name:       "raw token",
			credential: CredentialManifest{ID: "apiKey", Label: "token=secret", Source: "env", Env: "EXAMPLE_API_KEY"},
			want:       "credentials[0] must reference a secret without embedding credential values",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validWakeWordManifest()
			manifest.Credentials = []CredentialManifest{tt.credential}

			problems := ValidateProviderManifest(manifest)

			if !hasProblem(problems, tt.want) {
				t.Fatalf("expected credential problem containing %q, got %v", tt.want, problems)
			}
		})
	}
}

func TestValidateProviderManifestRejectsDuplicateCredentialIDs(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.Credentials = []CredentialManifest{
		{ID: "apiKey", Label: "API key", Source: "env", Env: "EXAMPLE_API_KEY"},
		{ID: "apiKey", Label: "Second API key", Source: "env", Env: "EXAMPLE_API_KEY_2"},
	}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "credentials[1].id must be unique") {
		t.Fatalf("expected duplicate credential problem, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsMissingLicenseMetadata(t *testing.T) {
	manifest := validWakeWordManifest()
	manifest.License = LicenseManifest{}

	problems := ValidateProviderManifest(manifest)

	if !hasProblem(problems, "license.name is required") ||
		!hasProblem(problems, "license.url is required") {
		t.Fatalf("expected license problems, got %v", problems)
	}
}

func TestValidateProviderManifestRejectsMissingContributionMetadata(t *testing.T) {
	tests := []struct {
		name         string
		contribution ContributionManifest
		want         string
	}{
		{
			name:         "missing source and maintainers",
			contribution: ContributionManifest{},
			want:         "contribution.maintainers must include at least one maintainer",
		},
		{
			name: "blank maintainer",
			contribution: ContributionManifest{
				Source:      "https://example.com/provider",
				Maintainers: []string{""},
			},
			want: "contribution.maintainers[0] is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validWakeWordManifest()
			manifest.Contribution = tt.contribution

			problems := ValidateProviderManifest(manifest)

			if !hasProblem(problems, tt.want) {
				t.Fatalf("expected contribution problem containing %q, got %v", tt.want, problems)
			}
			if strings.TrimSpace(tt.contribution.Source) == "" &&
				!hasProblem(problems, "contribution.source is required") {
				t.Fatalf("expected contribution source problem, got %v", problems)
			}
		})
	}
}

func TestSpikeProviderManifestFixturesValidate(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		wantID           string
		wantKind         string
		wantTransport    string
		wantOffline      bool
		wantRaspberryPi  bool
		wantCommand      string
		wantArgs         []string
		wantWakeModelID  string
		wantWakeModelRel string
	}{
		{
			name:            "go whisper http json",
			path:            "go_whisper_http_json.json",
			wantID:          "org.mutablelogic.go-whisper.local",
			wantKind:        ProviderKindSTT,
			wantTransport:   "http-json",
			wantOffline:     true,
			wantRaspberryPi: false,
		},
		{
			name:            "go whisper command",
			path:            "go_whisper_command.json",
			wantID:          "org.mutablelogic.go-whisper.command",
			wantKind:        ProviderKindSTT,
			wantTransport:   "command",
			wantOffline:     true,
			wantRaspberryPi: false,
			wantCommand:     "/usr/local/bin/gowhisper",
			wantArgs: []string{
				"transcribe",
				"{modelId}",
				"{inputPath}",
				"--format",
				"json",
				"--language",
				"{language}",
			},
		},
		{
			name:             "microwakeword experimental",
			path:             "microwakeword_experimental.json",
			wantID:           "org.pmdroid.microwakeword.local",
			wantKind:         ProviderKindWakeWord,
			wantTransport:    "builtin",
			wantOffline:      true,
			wantRaspberryPi:  false,
			wantWakeModelID:  "okay-nabu",
			wantWakeModelRel: "assets/okay_nabu.tflite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", "provider_manifests", tt.path))
			if err != nil {
				t.Fatalf("read fixture manifest: %v", err)
			}
			manifest, err := DecodeProviderManifest(string(raw))
			if err != nil {
				t.Fatalf("decode fixture manifest: %v", err)
			}
			if problems := ValidateProviderManifest(manifest); len(problems) != 0 {
				t.Fatalf("expected valid fixture manifest, got %v", problems)
			}
			if manifest.ID != tt.wantID ||
				manifest.Kind != tt.wantKind ||
				manifest.Transport.Type != tt.wantTransport ||
				manifest.Capabilities.Offline != tt.wantOffline ||
				manifest.Hardware["raspberryPi"] != tt.wantRaspberryPi {
				t.Fatalf("unexpected manifest summary: %+v", manifest)
			}
			if tt.wantWakeModelID != "" {
				if manifest.WakeWord.DefaultModelID != tt.wantWakeModelID ||
					len(manifest.WakeWord.Models) != 1 ||
					manifest.WakeWord.Models[0].Path != tt.wantWakeModelRel {
					t.Fatalf("unexpected wake-word model metadata: %+v", manifest.WakeWord)
				}
			}
			if tt.wantCommand != "" && manifest.Transport.Command != tt.wantCommand {
				t.Fatalf("unexpected command transport path: %s", manifest.Transport.Command)
			}
			if tt.wantArgs != nil && !reflect.DeepEqual(manifest.Transport.Args, tt.wantArgs) {
				t.Fatalf("unexpected command transport args: %#v", manifest.Transport.Args)
			}
			transportText := manifest.Transport.Endpoint + "\n" +
				manifest.Transport.Command + "\n" +
				strings.Join(manifest.Transport.Args, "\n")
			if strings.Contains(transportText, "token") ||
				strings.Contains(transportText, "secret") {
				t.Fatalf("fixture manifest transport contains credential material: %s", transportText)
			}
		})
	}
}

func TestSpikeProviderCandidatesAreNotProductionDependencies(t *testing.T) {
	rawGoMod, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	rawGoSum, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "go.sum"))
	if err != nil {
		t.Fatalf("read go.sum: %v", err)
	}
	moduleMetadata := string(rawGoMod) + "\n" + string(rawGoSum)
	for _, modulePath := range []string{
		"github.com/mutablelogic/go-whisper",
		"github.com/pmdroid/microwakeword",
		"github.com/tensorflow/tensorflow",
	} {
		if strings.Contains(moduleMetadata, modulePath) {
			t.Fatalf("spike candidate %s must not be a production dependency", modulePath)
		}
	}
}

func validWakeWordManifest() ProviderManifest {
	return ProviderManifest{
		ID:      "org.example.openwakeword",
		Name:    "Example openWakeWord",
		Version: "1.0.0",
		Kind:    ProviderKindWakeWord,
		Transport: TransportManifest{
			Type:     "wyoming",
			Endpoint: "tcp://127.0.0.1:10400",
		},
		Capabilities: ProviderCapabilities{
			Offline:   true,
			Languages: []string{"en", "en-GB"},
		},
		Hardware:    map[string]bool{"cpu": true, "raspberryPi": true},
		Credentials: []CredentialManifest{},
		License: LicenseManifest{
			Name: "Apache-2.0",
			URL:  "https://example.com/license",
		},
		Contribution: ContributionManifest{
			Source:      "https://example.com/openwakeword",
			Maintainers: []string{"example"},
		},
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

func validSTTTTSManifest() ProviderManifest {
	manifest := validWakeWordManifest()
	manifest.ID = "org.example.local-stt-tts"
	manifest.Name = "Example STT/TTS"
	manifest.Kind = ProviderKindSTTTTS
	manifest.Transport = TransportManifest{
		Type:     "http-json",
		Endpoint: "http://127.0.0.1:8088",
	}
	manifest.WakeWord = WakeWordManifest{}
	manifest.TTS = TTSManifest{
		DefaultVoiceID: "amy",
		DefaultModelID: "piper-amy",
		Voices: []TTSVoiceManifest{{
			ID:            "amy",
			Label:         "Amy",
			Locale:        "en-GB",
			ModelID:       "piper-amy",
			Styles:        []string{"neutral"},
			OutputFormats: []string{"audio/wav"},
		}},
	}
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
