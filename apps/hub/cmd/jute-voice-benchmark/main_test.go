package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jute-dash/apps/hub/internal/app/voice"
)

func TestVoiceBenchmarkCommandEmitsEvidenceForCompleteReport(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "darwin",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "lights",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"language": "en-GB",
			"expectedTranscript": "turn on token=secret lights",
			"transcript": "turn on token=secret lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-issue", "JUT-13",
		"-kind", "stt",
		"-min-results", "1",
		"-require-model-hash",
		"-require-transcript-matches",
	}, strings.NewReader(report), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Voice Benchmark Evidence: JUT-13") ||
		!strings.Contains(stdout.String(), "`lights`: matched") {
		t.Fatalf("expected evidence markdown, got:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "token=secret") {
		t.Fatalf("evidence markdown leaked transcript secret:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandEmitsPublicJSONWithoutTranscriptBodies(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "darwin",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "lights",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"language": "en-GB",
			"expectedTranscript": "turn on token=secret lights",
			"transcript": "turn on token=secret lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000},
			"providerReturned": true
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-issue", "JUT-13",
		"-kind", "stt",
		"-min-results", "1",
		"-require-model-hash",
		"-require-transcript-matches",
		"-public-json",
	}, strings.NewReader(report), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "turn on") ||
		strings.Contains(stdout.String(), "token=secret") ||
		strings.Contains(stdout.String(), "expectedTranscript\"") ||
		strings.Contains(stdout.String(), "transcript\"") {
		t.Fatalf("public JSON leaked transcript material:\n%s", stdout.String())
	}
	for _, want := range []string{
		`"issue": "JUT-13"`,
		`"expectedTranscriptSet": true`,
		`"transcriptReturned": true`,
		`"transcriptMatched": true`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected public JSON to contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandRejectsUnknownReportFields(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "darwin",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"providerDebug": "token=secret"
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected decode failure, got code %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no evidence output, got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), `json: unknown field "providerDebug"`) {
		t.Fatalf("expected unknown-field stderr, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "token=secret") {
		t.Fatalf("stderr leaked ignored field value: %s", stderr.String())
	}
}

func TestVoiceBenchmarkCommandEmitsProviderBuildEvidence(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-build-evidence",
		"-issue", "JUT-11",
		"-kind", "wake-word",
		"-provider-id", "pmdroid-microwakeword",
		"-build-target", "native-consumer",
		"-build-command-id", "go-test-package",
		"-build-status", "failed",
		"-build-exit-code", "1",
		"-build-error-code", "missing-header",
		"-build-missing", "tensorflow-lite-microfrontend-header, TensorFlow Lite microfrontend header",
		"-environment-notes", "token=secret header missing at /private/tmp/build",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failed build evidence to exit non-zero, got %d", code)
	}
	for _, want := range []string{
		"Provider Build Evidence: JUT-11",
		"Provider: `pmdroid-microwakeword`",
		"Target: `native-consumer`",
		"Status: `failed`",
		"Error code: `missing-header`",
		"Missing dependencies: `tensorflow-lite-microfrontend-header`",
		"Closure evidence: `false`",
		"provider build did not succeed",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected build evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	for _, leaked := range []string{"token=secret", "/private/tmp/build"} {
		if strings.Contains(stdout.String(), leaked) {
			t.Fatalf("build evidence leaked unsafe note %q:\n%s", leaked, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "provider build evidence has 1 validation problem") {
		t.Fatalf("expected build evidence stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandEmitsProviderBuildEvidencePublicJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-build-evidence",
		"-public-json",
		"-issue", "JUT-13",
		"-kind", "stt",
		"-provider-id", "go-whisper",
		"-build-target", "native-cli",
		"-build-command-id", "go-install",
		"-build-status", "succeeded",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful build evidence, got code %d stderr %s", code, stderr.String())
	}
	for _, want := range []string{
		`"issue": "JUT-13"`,
		`"providerId": "go-whisper"`,
		`"target": "native-cli"`,
		`"commandId": "go-install"`,
		`"status": "succeeded"`,
		`"closureEvidence": true`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected build public JSON to contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandEmitsProviderPackagingEvidence(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-packaging-evidence",
		"-issue",
		"JUT-13",
		"-kind",
		"stt",
		"-provider-id",
		"go-whisper",
		"-packaging-targets",
		"cpu-only=failed,macos-metal=blocked,container-linux=failed,raspberry-pi-arm64=unsupported",
		"-environment-notes",
		"checked in /private/tmp with token=secret omitted",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected complete packaging evidence, got code %d stderr %s", code, stderr.String())
	}
	for _, want := range []string{
		"Provider Packaging Evidence: JUT-13",
		"Provider: `go-whisper`",
		"Target `cpu-only`: `failed`",
		"Target `macos-metal`: `blocked`",
		"Target `container-linux`: `failed`",
		"Target `raspberry-pi-arm64`: `unsupported`",
		"Packaging evidence complete: `true`",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected packaging evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	for _, leaked := range []string{"token=secret", "/private/tmp"} {
		if strings.Contains(stdout.String(), leaked) {
			t.Fatalf("packaging evidence leaked unsafe note %q:\n%s", leaked, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr for complete packaging evidence, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsIncompleteProviderPackagingEvidence(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-packaging-evidence",
		"-issue", "JUT-11",
		"-kind", "wake-word",
		"-provider-id", "pmdroid-microwakeword",
		"-packaging-targets", "macos-native=failed,linux-native=not-run",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected incomplete packaging evidence to fail, got %d", code)
	}
	for _, want := range []string{
		"Provider Packaging Evidence: JUT-11",
		"Target `linux-native`: `not-run`",
		"Packaging evidence complete: `false`",
		"packaging target linux-native has not been evaluated",
		"packaging target macos-native requires notes when status is not succeeded",
		"packaging target linux-native requires notes when status is not succeeded",
		"packaging target raspberry-pi-arm64 is required",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected incomplete packaging evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "provider packaging evidence has 4 validation problem") {
		t.Fatalf("expected packaging evidence stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandEmitsProviderPackagingEvidencePublicJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-packaging-evidence",
		"-public-json",
		"-issue", "JUT-11",
		"-kind", "wake-word",
		"-provider-id", "pmdroid-microwakeword",
		"-packaging-targets", "macos-native=failed,linux-native=blocked,raspberry-pi-arm64=unsupported",
		"-environment-notes", "macOS failed, Linux blocked, Raspberry Pi unsupported in this run",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful packaging public JSON, got code %d stderr %s", code, stderr.String())
	}
	for _, want := range []string{
		`"issue": "JUT-11"`,
		`"providerId": "pmdroid-microwakeword"`,
		`"target": "macos-native"`,
		`"status": "failed"`,
		`"packagingEvidenceComplete": true`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected packaging public JSON to contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandEmitsProviderModelEvidence(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-model-evidence",
		"-issue", "JUT-11",
		"-kind", "wake-word",
		"-provider-id", "pmdroid-microwakeword",
		"-model-id", "okay-nabu",
		"-model-hash", "sha256:replace-with-model-hash",
		"-model-source", "ESPHome",
		"-model-format", "tflite",
		"-model-compatibility", "blocked",
		"-model-runtime-status", "not-run",
		"-environment-notes", "raw URL https://example.test/model.tflite token=secret",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected incomplete model evidence to exit non-zero, got %d", code)
	}
	for _, want := range []string{
		"Provider Model Evidence: JUT-11",
		"Provider: `pmdroid-microwakeword`",
		"Model: `okay-nabu`",
		"Source: `esphome`",
		"Format: `tflite`",
		"Compatibility: `blocked`",
		"Runtime status: `not-run`",
		"Closure evidence: `false`",
		"modelHash must be sha256:<64 hex characters>",
		"model compatibility is not proven",
		"model runtime load is not proven",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected model evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	for _, leaked := range []string{"token=secret", "https://example.test/model.tflite"} {
		if strings.Contains(stdout.String(), leaked) {
			t.Fatalf("model evidence leaked unsafe note %q:\n%s", leaked, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "provider model evidence has 3 validation problem") {
		t.Fatalf("expected model evidence stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandEmitsProviderModelEvidencePublicJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-model-evidence",
		"-public-json",
		"-issue", "JUT-13",
		"-kind", "stt",
		"-provider-id", "go-whisper",
		"-model-id", "tiny-en",
		"-model-hash", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-model-source", "whisper-cpp",
		"-model-format", "ggml",
		"-model-compatibility", "compatible",
		"-model-runtime-status", "loaded",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected successful model evidence, got code %d stderr %s", code, stderr.String())
	}
	for _, want := range []string{
		`"issue": "JUT-13"`,
		`"providerId": "go-whisper"`,
		`"modelId": "tiny-en"`,
		`"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
		`"compatibilityStatus": "compatible"`,
		`"runtimeStatus": "loaded"`,
		`"closureEvidence": true`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected model public JSON to contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandAcceptsCompleteJUT13ClosureBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "go-whisper-closure.json")
	bundle := `{
		"issue": "JUT-13",
		"decision": {
			"status": "documented-external-provider",
			"rationale": "Benchmark and packaging evidence support keeping go-whisper documented externally for v1."
		},
		"providerManifest": {
			"id": "org.mutablelogic.go-whisper.local",
			"name": "go-whisper Local STT",
			"version": "external",
			"kind": "stt",
			"transport": {"type": "http-json", "endpoint": "http://127.0.0.1:8081"},
			"capabilities": {"offline": true},
			"hardware": {"cpu": true, "raspberryPi": false},
			"credentials": [],
			"license": {"name": "Apache-2.0", "url": "local-license-reference"},
			"contribution": {"source": "local-source-reference", "maintainers": ["mutablelogic"]}
		},
		"fixtureManifest": {
			"issue": "JUT-13",
			"kind": "stt",
			"fixtures": [
				{"id": "short-command", "path": "stt/short-command.wav", "sha256": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"}
			]
		},
		"buildEvidence": [{
			"generatedAt": "2026-06-17T09:00:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"target": "native-cli",
			"commandId": "go-install",
			"status": "succeeded",
			"runtime": "linux/amd64 go1.25.0",
			"closureEvidence": true
		}],
		"packagingEvidence": {
			"generatedAt": "2026-06-17T09:01:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"targets": [
				{"target": "cpu-only", "status": "succeeded"},
				{"target": "macos-metal", "status": "blocked"},
				{"target": "container-linux", "status": "succeeded"},
				{"target": "raspberry-pi-arm64", "status": "unsupported"}
			],
			"runtime": "linux/amd64 go1.25.0",
			"notes": "safe note token=secret /private/tmp",
			"packagingEvidenceComplete": true
		},
		"modelEvidence": [{
			"generatedAt": "2026-06-17T09:02:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny-en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"modelSource": "whisper-cpp",
			"modelFormat": "ggml",
			"compatibilityStatus": "compatible",
			"runtimeStatus": "loaded",
			"runtime": "linux/amd64 go1.25.0",
			"closureEvidence": true
		}],
		"benchmarkReport": {
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {
				"os": "linux",
				"arch": "amd64",
				"goVersion": "go1.25.0",
				"providerId": "go-whisper",
				"providerKind": "stt",
				"modelId": "tiny-en",
				"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			"sttResults": [{
				"fixtureId": "short-command",
				"providerId": "go-whisper",
				"modelId": "tiny-en",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"latency": 80000000,
				"resourceSample": {"duration": 80000000},
				"providerReturned": true
			}],
			"summary": {
				"total": 1,
				"providerFailures": 0,
				"transcriptMatches": 1,
				"averageLatency": 80000000
			}
		}
	}`
	if err := os.WriteFile(bundlePath, []byte(bundle), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-closure-bundle", bundlePath}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf(
			"expected complete closure bundle, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	for _, want := range []string{
		"Provider Closure Bundle: JUT-13",
		"Decision: `documented-external-provider`",
		"Provider manifest accepted: `true`",
		"Provider manifest: `org.mutablelogic.go-whisper.local` (`stt`)",
		"Fixture manifest accepted: `true`",
		"Fixture manifest entries: 1",
		"Build evidence rows: 1",
		"Packaging evidence complete: `true`",
		"Model evidence rows: 1",
		"Benchmark accepted: `true`",
		"Closure bundle complete: `true`",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected closure bundle output to contain %q:\n%s", want, stdout.String())
		}
	}
	for _, leaked := range []string{"token=secret", "/private/tmp", "turn on the lights"} {
		if strings.Contains(stdout.String(), leaked) {
			t.Fatalf("closure bundle output leaked %q:\n%s", leaked, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandComposesCompleteJUT13ClosureBundle(t *testing.T) {
	dir := t.TempDir()
	source := providerClosureBundleFile{
		Decision: providerClosureDecision{
			Status:    "documented-external-provider",
			Rationale: "Benchmark and packaging evidence support keeping go-whisper documented externally for v1.",
		},
		ProviderManifest: json.RawMessage(`{
			"id": "org.mutablelogic.go-whisper.local",
			"name": "go-whisper Local STT",
			"version": "external",
			"kind": "stt",
			"transport": {"type": "http-json", "endpoint": "http://127.0.0.1:8081"},
			"capabilities": {"offline": true},
			"hardware": {"cpu": true, "raspberryPi": false},
			"credentials": [],
			"license": {"name": "Apache-2.0", "url": "local-license-reference"},
			"contribution": {"source": "local-source-reference", "maintainers": ["mutablelogic"]}
		}`),
		FixtureManifest: json.RawMessage(`{
			"issue": "JUT-13",
			"kind": "stt",
			"fixtures": [
				{"id": "short-command", "path": "stt/short-command.wav", "sha256": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"}
			]
		}`),
		BuildEvidence: []providerBuildEvidence{{
			GeneratedAt:     "2026-06-17T09:00:00Z",
			Issue:           "JUT-13",
			ProviderID:      "go-whisper",
			ProviderKind:    "stt",
			Target:          "native-cli",
			CommandID:       "go-install",
			Status:          "succeeded",
			Runtime:         "linux/amd64 go1.25.0",
			ClosureEvidence: true,
		}},
		PackagingEvidence: &providerPackagingEvidence{
			GeneratedAt:  "2026-06-17T09:01:00Z",
			Issue:        "JUT-13",
			ProviderID:   "go-whisper",
			ProviderKind: "stt",
			Targets: []providerPackagingTarget{
				{Target: "cpu-only", Status: "succeeded"},
				{Target: "macos-metal", Status: "blocked"},
				{Target: "container-linux", Status: "succeeded"},
				{Target: "raspberry-pi-arm64", Status: "unsupported"},
			},
			Runtime:                   "linux/amd64 go1.25.0",
			Notes:                     "macOS Metal blocked and Raspberry Pi unsupported in this run",
			PackagingEvidenceComplete: true,
		},
		ModelEvidence: []providerModelEvidence{{
			GeneratedAt:         "2026-06-17T09:02:00Z",
			Issue:               "JUT-13",
			ProviderID:          "go-whisper",
			ProviderKind:        "stt",
			ModelID:             "tiny-en",
			ModelHash:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ModelSource:         "whisper-cpp",
			ModelFormat:         "ggml",
			CompatibilityStatus: "compatible",
			RuntimeStatus:       "loaded",
			Runtime:             "linux/amd64 go1.25.0",
			ClosureEvidence:     true,
		}},
		BenchmarkReport: json.RawMessage(`{
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {
				"os": "linux",
				"arch": "amd64",
				"goVersion": "go1.25.0",
				"providerId": "go-whisper",
				"providerKind": "stt",
				"modelId": "tiny-en",
				"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			"sttResults": [{
				"fixtureId": "short-command",
				"providerId": "go-whisper",
				"modelId": "tiny-en",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"latency": 80000000,
				"resourceSample": {"duration": 80000000},
				"providerReturned": true
			}],
			"summary": {
				"total": 1,
				"providerFailures": 0,
				"transcriptMatches": 1,
				"averageLatency": 80000000
			}
		}`),
	}
	providerPath := writeRawJSONArtifactForTest(t, dir, "provider.json", source.ProviderManifest)
	fixturePath := writeRawJSONArtifactForTest(t, dir, "fixtures.json", source.FixtureManifest)
	buildPath := writeJSONArtifactForTest(t, dir, "build.json", source.BuildEvidence[0])
	packagingPath := writeJSONArtifactForTest(t, dir, "packaging.json", source.PackagingEvidence)
	modelPath := writeJSONArtifactForTest(t, dir, "model.json", source.ModelEvidence[0])
	reportPath := writeRawJSONArtifactForTest(t, dir, "report.json", source.BenchmarkReport)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-closure-bundle-compose", "JUT-13",
		"-decision-status", source.Decision.Status,
		"-decision-rationale", source.Decision.Rationale,
		"-provider-manifest", providerPath,
		"-fixture-manifest-artifact", fixturePath,
		"-build-evidence-artifacts", buildPath,
		"-packaging-evidence-artifact", packagingPath,
		"-model-evidence-artifacts", modelPath,
		"-benchmark-report-artifact", reportPath,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected compose success, got code %d stderr %s stdout %s", code, stderr.String(), stdout.String())
	}
	var composed providerClosureBundleFile
	if err := json.Unmarshal(stdout.Bytes(), &composed); err != nil {
		t.Fatalf("decode composed closure bundle: %v\n%s", err, stdout.String())
	}
	if composed.Issue != "JUT-13" ||
		composed.Decision.Status != source.Decision.Status ||
		len(composed.ProviderManifest) == 0 ||
		len(composed.FixtureManifest) == 0 ||
		len(composed.BuildEvidence) != 1 ||
		composed.PackagingEvidence == nil ||
		len(composed.ModelEvidence) != 1 ||
		len(composed.BenchmarkReport) == 0 {
		t.Fatalf("unexpected composed bundle: %+v", composed)
	}
	composedPath := filepath.Join(dir, "composed.json")
	if err := os.WriteFile(composedPath, stdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write composed bundle: %v", err)
	}
	var validateStdout, validateStderr bytes.Buffer
	validateCode := run(
		[]string{"-closure-bundle", composedPath},
		strings.NewReader(""),
		&validateStdout,
		&validateStderr,
	)
	if validateCode != 0 {
		t.Fatalf(
			"expected composed bundle to validate, got code %d stderr %s stdout %s",
			validateCode,
			validateStderr.String(),
			validateStdout.String(),
		)
	}
	if !strings.Contains(validateStdout.String(), "Closure bundle complete: `true`") {
		t.Fatalf("expected composed bundle complete output:\n%s", validateStdout.String())
	}
}

func TestVoiceBenchmarkCommandComposesCompleteJUT11ClosureBundleWithBaseline(t *testing.T) {
	dir := t.TempDir()
	source := providerClosureBundleFile{
		Decision: providerClosureDecision{
			Status:    "defer",
			Rationale: "Build, packaging, model, candidate benchmark, and baseline evidence support deferring pmdroid microWakeWord for v1.",
		},
		ProviderManifest: json.RawMessage(`{
			"id": "org.pmdroid.microwakeword.local",
			"name": "microWakeWord Local",
			"version": "experimental",
			"kind": "wake-word",
			"transport": {"type": "builtin", "endpoint": "local-voice-service"},
			"capabilities": {"offline": true, "languages": ["en"]},
			"hardware": {"cpu": true, "raspberryPi": false},
			"credentials": [],
			"license": {"name": "MIT", "url": "local-license-reference"},
			"contribution": {"source": "local-source-reference", "maintainers": ["pmdroid"]},
			"wakeWord": {
				"defaultModelId": "okay-nabu",
				"phrase": "Okay Nabu",
				"languages": ["en"],
				"sensitivity": 0.55,
				"models": [
					{"id": "okay-nabu", "path": "assets/okay-nabu.tflite", "phrase": "Okay Nabu", "languages": ["en"], "sensitivity": 0.55},
					{"id": "ohf-jute", "path": "assets/ohf-jute.tflite", "phrase": "Hey Jute", "languages": ["en"], "sensitivity": 0.55}
				]
			}
		}`),
		FixtureManifest: json.RawMessage(`{
			"issue": "JUT-11",
			"kind": "wake-word",
			"fixtures": [
				{"id": "positive-wake", "path": "wake/positive-wake.wav", "sha256": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": true, "language": "en"},
				{"id": "near-miss", "path": "wake/near-miss.wav", "sha256": "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": false, "language": "en"},
				{"id": "ambient-room", "path": "wake/ambient-room.wav", "sha256": "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": false, "language": "en"},
				{"id": "conversation-long", "path": "wake/conversation-long.wav", "sha256": "sha256:9999999999999999999999999999999999999999999999999999999999999999", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": false, "language": "en"}
			]
		}`),
		BuildEvidence: []providerBuildEvidence{{
			GeneratedAt:     "2026-06-17T09:00:00Z",
			Issue:           "JUT-11",
			ProviderID:      "pmdroid-microwakeword",
			ProviderKind:    "wake-word",
			Target:          "native-consumer",
			CommandID:       "go-test-package",
			Status:          "succeeded",
			Runtime:         "linux/arm64 go1.25.0",
			ClosureEvidence: true,
		}},
		PackagingEvidence: &providerPackagingEvidence{
			GeneratedAt:  "2026-06-17T09:01:00Z",
			Issue:        "JUT-11",
			ProviderID:   "pmdroid-microwakeword",
			ProviderKind: "wake-word",
			Targets: []providerPackagingTarget{
				{Target: "macos-native", Status: "failed"},
				{Target: "linux-native", Status: "succeeded"},
				{Target: "raspberry-pi-arm64", Status: "unsupported"},
			},
			Runtime:                   "linux/arm64 go1.25.0",
			Notes:                     "macOS native failed and Raspberry Pi unsupported in this run",
			PackagingEvidenceComplete: true,
		},
		ModelEvidence: []providerModelEvidence{
			{
				GeneratedAt:         "2026-06-17T09:02:00Z",
				Issue:               "JUT-11",
				ProviderID:          "pmdroid-microwakeword",
				ProviderKind:        "wake-word",
				ModelID:             "okay-nabu",
				ModelHash:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				ModelSource:         "esphome",
				ModelFormat:         "tflite",
				CompatibilityStatus: "compatible",
				RuntimeStatus:       "loaded",
				Runtime:             "linux/arm64 go1.25.0",
				ClosureEvidence:     true,
			},
			{
				GeneratedAt:         "2026-06-17T09:02:01Z",
				Issue:               "JUT-11",
				ProviderID:          "pmdroid-microwakeword",
				ProviderKind:        "wake-word",
				ModelID:             "ohf-jute",
				ModelHash:           "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				ModelSource:         "ohf",
				ModelFormat:         "tflite",
				CompatibilityStatus: "compatible",
				RuntimeStatus:       "loaded",
				Runtime:             "linux/arm64 go1.25.0",
				ClosureEvidence:     true,
			},
		},
		BenchmarkReport: json.RawMessage(`{
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {"os": "linux", "arch": "arm64", "goVersion": "go1.25.0", "providerId": "pmdroid-microwakeword", "providerKind": "wake-word", "modelId": "ohf-jute", "modelHash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "near-miss", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "ambient-room", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "conversation-long", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
			],
			"summary": {"total": 4, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 45000000}
		}`),
		BaselineReport: json.RawMessage(`{
			"generatedAt": "2026-06-17T09:04:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {"os": "linux", "arch": "arm64", "goVersion": "go1.25.0", "providerId": "wyoming-openwakeword", "providerKind": "wake-word", "modelId": "hey-jute", "modelHash": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}},
				{"fixtureId": "near-miss", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}},
				{"fixtureId": "ambient-room", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}},
				{"fixtureId": "conversation-long", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}}
			],
			"summary": {"total": 4, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 70000000}
		}`),
	}
	providerPath := writeRawJSONArtifactForTest(t, dir, "provider.json", source.ProviderManifest)
	fixturePath := writeRawJSONArtifactForTest(t, dir, "fixtures.json", source.FixtureManifest)
	buildPath := writeJSONArtifactForTest(t, dir, "build.json", source.BuildEvidence[0])
	packagingPath := writeJSONArtifactForTest(t, dir, "packaging.json", source.PackagingEvidence)
	esphomeModelPath := writeJSONArtifactForTest(t, dir, "esphome-model.json", source.ModelEvidence[0])
	ohfModelPath := writeJSONArtifactForTest(t, dir, "ohf-model.json", source.ModelEvidence[1])
	reportPath := writeRawJSONArtifactForTest(t, dir, "report.json", source.BenchmarkReport)
	baselinePath := writeRawJSONArtifactForTest(t, dir, "baseline.json", source.BaselineReport)
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-closure-bundle-compose", "JUT-11",
		"-decision-status", source.Decision.Status,
		"-decision-rationale", source.Decision.Rationale,
		"-provider-manifest", providerPath,
		"-fixture-manifest-artifact", fixturePath,
		"-build-evidence-artifacts", buildPath,
		"-packaging-evidence-artifact", packagingPath,
		"-model-evidence-artifacts", esphomeModelPath + "," + ohfModelPath,
		"-benchmark-report-artifact", reportPath,
		"-baseline-report-artifact", baselinePath,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf(
			"expected JUT-11 compose success, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	var composed providerClosureBundleFile
	if err := json.Unmarshal(stdout.Bytes(), &composed); err != nil {
		t.Fatalf("decode composed JUT-11 closure bundle: %v\n%s", err, stdout.String())
	}
	if composed.Issue != "JUT-11" ||
		len(composed.ModelEvidence) != 2 ||
		len(composed.BaselineReport) == 0 ||
		composed.PackagingEvidence == nil ||
		len(composed.BenchmarkReport) == 0 {
		t.Fatalf("unexpected composed JUT-11 bundle: %+v", composed)
	}
	composedPath := filepath.Join(dir, "composed-jut11.json")
	if err := os.WriteFile(composedPath, stdout.Bytes(), 0o600); err != nil {
		t.Fatalf("write composed JUT-11 bundle: %v", err)
	}
	var validateStdout, validateStderr bytes.Buffer
	validateCode := run(
		[]string{"-closure-bundle", composedPath},
		strings.NewReader(""),
		&validateStdout,
		&validateStderr,
	)
	if validateCode != 0 {
		t.Fatalf(
			"expected composed JUT-11 bundle to validate, got code %d stderr %s stdout %s",
			validateCode,
			validateStderr.String(),
			validateStdout.String(),
		)
	}
	for _, want := range []string{
		"Baseline accepted: `true`",
		"Comparison accepted: `true`",
		"Closure bundle complete: `true`",
	} {
		if !strings.Contains(validateStdout.String(), want) {
			t.Fatalf("expected composed JUT-11 validation output to contain %q:\n%s", want, validateStdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandRejectsClosureBundleWithPlaceholderRuntime(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "go-whisper-placeholder-runtime.json")
	bundle := `{
		"issue": "JUT-13",
		"decision": {
			"status": "documented-external-provider",
			"rationale": "Benchmark and packaging evidence support keeping go-whisper documented externally for v1."
		},
		"providerManifest": {
			"id": "org.mutablelogic.go-whisper.local",
			"name": "go-whisper Local STT",
			"version": "external",
			"kind": "stt",
			"transport": {"type": "http-json", "endpoint": "http://127.0.0.1:8081"},
			"capabilities": {"offline": true},
			"hardware": {"cpu": true, "raspberryPi": false},
			"credentials": [],
			"license": {"name": "Apache-2.0", "url": "local-license-reference"},
			"contribution": {"source": "local-source-reference", "maintainers": ["mutablelogic"]}
		},
		"fixtureManifest": {
			"issue": "JUT-13",
			"kind": "stt",
			"fixtures": [
				{"id": "short-command", "path": "stt/short-command.wav", "sha256": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"}
			]
		},
		"buildEvidence": [{
			"generatedAt": "2026-06-17T09:00:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"target": "native-cli",
			"commandId": "go-install",
			"status": "succeeded",
			"runtime": "replace-with-os-arch-go-version",
			"closureEvidence": true
		}],
		"packagingEvidence": {
			"generatedAt": "2026-06-17T09:01:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"targets": [
				{"target": "cpu-only", "status": "succeeded"},
				{"target": "macos-metal", "status": "blocked"},
				{"target": "container-linux", "status": "succeeded"},
				{"target": "raspberry-pi-arm64", "status": "unsupported"}
			],
			"runtime": "linux/amd64 go1.25.0",
			"notes": "macOS Metal blocked and Raspberry Pi unsupported in this run",
			"packagingEvidenceComplete": true
		},
		"modelEvidence": [{
			"generatedAt": "2026-06-17T09:02:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny-en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"modelSource": "whisper-cpp",
			"modelFormat": "ggml",
			"compatibilityStatus": "compatible",
			"runtimeStatus": "loaded",
			"runtime": "linux/amd64 go1.25.0",
			"closureEvidence": true
		}],
		"benchmarkReport": {
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {
				"os": "linux",
				"arch": "amd64",
				"goVersion": "go1.25.0",
				"providerId": "go-whisper",
				"providerKind": "stt",
				"modelId": "tiny-en",
				"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			"sttResults": [{
				"fixtureId": "short-command",
				"providerId": "go-whisper",
				"modelId": "tiny-en",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"latency": 80000000,
				"resourceSample": {"duration": 80000000},
				"providerReturned": true
			}],
			"summary": {
				"total": 1,
				"providerFailures": 0,
				"transcriptMatches": 1,
				"averageLatency": 80000000
			}
		}
	}`
	if err := os.WriteFile(bundlePath, []byte(bundle), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-closure-bundle", bundlePath}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected placeholder runtime to fail, got %d", code)
	}
	for _, want := range []string{
		"Benchmark accepted: `true`",
		"Closure bundle complete: `false`",
		"build: buildEvidence 0 : runtime must identify a concrete OS arch and toolchain",
		"build: JUT-13 closure requires at least one successful go-whisper build evidence row",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected placeholder runtime output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "provider closure bundle has 2 validation problem") {
		t.Fatalf("expected placeholder runtime stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsClosureBundleWhenBenchmarkModelDiffersFromModelEvidence(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "go-whisper-mismatch.json")
	bundle := `{
		"issue": "JUT-13",
		"decision": {
			"status": "documented-external-provider",
			"rationale": "Benchmark and packaging evidence support keeping go-whisper documented externally for v1."
		},
		"providerManifest": {
			"id": "org.mutablelogic.go-whisper.local",
			"name": "go-whisper Local STT",
			"version": "external",
			"kind": "stt",
			"transport": {"type": "http-json", "endpoint": "http://127.0.0.1:8081"},
			"capabilities": {"offline": true},
			"hardware": {"cpu": true, "raspberryPi": false},
			"credentials": [],
			"license": {"name": "Apache-2.0", "url": "local-license-reference"},
			"contribution": {"source": "local-source-reference", "maintainers": ["mutablelogic"]}
		},
		"fixtureManifest": {
			"issue": "JUT-13",
			"kind": "stt",
			"fixtures": [
				{"id": "short-command", "path": "stt/short-command.wav", "sha256": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"}
			]
		},
		"buildEvidence": [{
			"generatedAt": "2026-06-17T09:00:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"target": "native-cli",
			"commandId": "go-install",
			"status": "succeeded",
			"runtime": "linux/amd64 go1.25.0",
			"closureEvidence": true
		}],
		"packagingEvidence": {
			"generatedAt": "2026-06-17T09:01:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"targets": [
				{"target": "cpu-only", "status": "succeeded"},
				{"target": "macos-metal", "status": "blocked"},
				{"target": "container-linux", "status": "succeeded"},
				{"target": "raspberry-pi-arm64", "status": "unsupported"}
			],
			"runtime": "linux/amd64 go1.25.0",
			"notes": "macOS Metal blocked and Raspberry Pi unsupported in this run",
			"packagingEvidenceComplete": true
		},
		"modelEvidence": [{
			"generatedAt": "2026-06-17T09:02:00Z",
			"issue": "JUT-13",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny-en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"modelSource": "whisper-cpp",
			"modelFormat": "ggml",
			"compatibilityStatus": "compatible",
			"runtimeStatus": "loaded",
			"runtime": "linux/amd64 go1.25.0",
			"closureEvidence": true
		}],
		"benchmarkReport": {
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {
				"os": "linux",
				"arch": "amd64",
				"goVersion": "go1.25.0",
				"providerId": "go-whisper",
				"providerKind": "stt",
				"modelId": "tiny-en",
				"modelHash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			},
			"sttResults": [{
				"fixtureId": "short-command",
				"providerId": "go-whisper",
				"modelId": "tiny-en",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"latency": 80000000,
				"resourceSample": {"duration": 80000000},
				"providerReturned": true
			}],
			"summary": {
				"total": 1,
				"providerFailures": 0,
				"transcriptMatches": 1,
				"averageLatency": 80000000
			}
		}
	}`
	if err := os.WriteFile(bundlePath, []byte(bundle), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-closure-bundle", bundlePath}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected model mismatch to fail, got %d", code)
	}
	for _, want := range []string{
		"Benchmark accepted: `true`",
		"Closure bundle complete: `false`",
		"model: JUT-13 benchmark modelId modelHash must match compatible go-whisper model evidence",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected model mismatch output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "provider closure bundle has 1 validation problem") {
		t.Fatalf("expected model mismatch stderr, got %q", stderr.String())
	}
}

func TestProviderClosureBundleRejectsBenchmarkFixturesOutsideManifest(t *testing.T) {
	report := voice.BenchmarkReport{
		Kind: "stt",
		STTResults: []voice.STTBenchmarkResult{
			{FixtureID: "short-command"},
			{FixtureID: "ad-hoc-command"},
		},
	}
	manifest := voice.BenchmarkFixtureSetManifest{
		Fixtures: []voice.BenchmarkFixtureManifest{
			{ID: "short-command"},
		},
	}

	problems := validateClosureBundleReportFixtures(report, manifest)

	if len(problems) != 1 ||
		problems[0] != "benchmark fixture ad-hoc-command is not declared in fixtureManifest" {
		t.Fatalf("expected undeclared fixture problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsManifestFixturesMissingFromBenchmark(t *testing.T) {
	report := voice.BenchmarkReport{
		Kind: "wake-word",
		WakeResults: []voice.WakeBenchmarkResult{
			{FixtureID: "positive-wake"},
		},
	}
	manifest := voice.BenchmarkFixtureSetManifest{
		Fixtures: []voice.BenchmarkFixtureManifest{
			{ID: "positive-wake"},
			{ID: "near-miss"},
		},
	}

	problems := validateClosureBundleReportFixtures(report, manifest)

	if len(problems) != 1 ||
		problems[0] != "benchmark fixture near-miss declared in fixtureManifest was not measured" {
		t.Fatalf("expected unmeasured fixture problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsBenchmarkFixtureExpectationMismatch(t *testing.T) {
	transcriptReport := voice.BenchmarkReport{
		Kind: "stt",
		STTResults: []voice.STTBenchmarkResult{
			{
				FixtureID:          "short-command",
				ExpectedTranscript: "turn on the lights",
			},
		},
	}
	transcriptManifest := voice.BenchmarkFixtureSetManifest{
		Fixtures: []voice.BenchmarkFixtureManifest{
			{
				ID:                 "short-command",
				ExpectedTranscript: "turn off the lights",
			},
		},
	}
	expectWake := false
	reportWake := true
	wakeReport := voice.BenchmarkReport{
		Kind: "wake-word",
		WakeResults: []voice.WakeBenchmarkResult{
			{
				FixtureID:    "positive-wake",
				ExpectedWake: &reportWake,
			},
		},
	}
	wakeManifest := voice.BenchmarkFixtureSetManifest{
		Fixtures: []voice.BenchmarkFixtureManifest{
			{
				ID:         "positive-wake",
				ExpectWake: &expectWake,
			},
		},
	}

	transcriptProblems := validateClosureBundleReportFixtures(transcriptReport, transcriptManifest)
	wakeProblems := validateClosureBundleReportFixtures(wakeReport, wakeManifest)

	if len(transcriptProblems) != 1 ||
		transcriptProblems[0] != "benchmark fixture short-command expectedTranscript does not match fixtureManifest" {
		t.Fatalf("expected transcript mismatch, got %v", transcriptProblems)
	}
	if len(wakeProblems) != 1 ||
		wakeProblems[0] != "benchmark fixture positive-wake expectedWake does not match fixtureManifest" {
		t.Fatalf("expected wake mismatch, got %v", wakeProblems)
	}
}

func TestProviderClosureBundleRejectsCloudOrCredentialManifest(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "org.mutablelogic.go-whisper.cloud",
		"name": "go-whisper Cloud STT",
		"version": "external",
		"kind": "stt",
		"transport": {"type": "http-json", "endpoint": "https://stt.example.test/v1"},
		"capabilities": {"offline": false},
		"hardware": {"cpu": true},
		"credentials": [{"id": "api-key", "label": "API key", "source": "env", "env": "GO_WHISPER_API_KEY", "required": true}],
		"license": {"name": "Apache-2.0", "url": "local-license-reference"},
		"contribution": {"source": "local-source-reference", "maintainers": ["mutablelogic"]}
	}`)

	_, problems := validateClosureBundleProviderManifest("JUT-13", raw)

	for _, want := range []string{
		"providerManifest must declare offline capability for closure",
		"providerManifest must not require credentials for local closure evidence",
	} {
		if !containsProblem(problems, want) {
			t.Fatalf("expected manifest problem %q, got %v", want, problems)
		}
	}
}

func TestProviderClosureBundleRejectsJUT11ManifestWithoutPmdroidIdentity(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "org.example.microwakeword.local",
		"name": "microWakeWord Local",
		"version": "experimental",
		"kind": "wake-word",
		"transport": {"type": "builtin", "endpoint": "local-voice-service"},
		"capabilities": {"offline": true, "languages": ["en"]},
		"hardware": {"cpu": true},
		"credentials": [],
		"license": {"name": "MIT", "url": "local-license-reference"},
		"contribution": {"source": "local-source-reference", "maintainers": ["example"]},
		"wakeWord": {
			"defaultModelId": "okay-nabu",
			"phrase": "Okay Nabu",
			"languages": ["en"],
			"sensitivity": 0.55,
			"models": [
				{"id": "okay-nabu", "path": "assets/okay-nabu.tflite", "phrase": "Okay Nabu", "languages": ["en"], "sensitivity": 0.55}
			]
		}
	}`)

	_, problems := validateClosureBundleProviderManifest("JUT-11", raw)

	if !containsProblem(problems, "JUT-11 providerManifest must identify pmdroid") {
		t.Fatalf("expected pmdroid identity problem, got %v", problems)
	}
	if containsProblem(problems, "JUT-11 providerManifest must identify microWakeWord") {
		t.Fatalf("did not expect microWakeWord identity problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsJUT11ManifestWithoutPmdroidMaintainer(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "org.pmdroid.microwakeword.local",
		"name": "microWakeWord Local",
		"version": "experimental",
		"kind": "wake-word",
		"transport": {"type": "builtin", "endpoint": "local-voice-service"},
		"capabilities": {"offline": true, "languages": ["en"]},
		"hardware": {"cpu": true},
		"credentials": [],
		"license": {"name": "MIT", "url": "local-license-reference"},
		"contribution": {"source": "local-source-reference", "maintainers": ["example"]},
		"wakeWord": {
			"defaultModelId": "okay-nabu",
			"phrase": "Okay Nabu",
			"languages": ["en"],
			"sensitivity": 0.55,
			"models": [
				{"id": "okay-nabu", "path": "assets/okay-nabu.tflite", "phrase": "Okay Nabu", "languages": ["en"], "sensitivity": 0.55}
			]
		}
	}`)

	_, problems := validateClosureBundleProviderManifest("JUT-11", raw)

	if !containsProblem(problems, "JUT-11 providerManifest contribution maintainers must include pmdroid") {
		t.Fatalf("expected pmdroid maintainer problem, got %v", problems)
	}
	if containsProblem(problems, "JUT-11 providerManifest must identify pmdroid") ||
		containsProblem(problems, "JUT-11 providerManifest must identify microWakeWord") {
		t.Fatalf("did not expect provider identity problems, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsJUT13ManifestWithoutMutablelogicIdentity(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "org.example.go-whisper.local",
		"name": "go-whisper Local STT",
		"version": "external",
		"kind": "stt",
		"transport": {"type": "http-json", "endpoint": "http://127.0.0.1:8081"},
		"capabilities": {"offline": true, "languages": ["en"]},
		"hardware": {"cpu": true},
		"credentials": [],
		"license": {"name": "Apache-2.0", "url": "local-license-reference"},
		"contribution": {"source": "local-source-reference", "maintainers": ["example"]}
	}`)

	_, problems := validateClosureBundleProviderManifest("JUT-13", raw)

	if !containsProblem(problems, "JUT-13 providerManifest must identify mutablelogic") {
		t.Fatalf("expected mutablelogic identity problem, got %v", problems)
	}
	if containsProblem(problems, "JUT-13 providerManifest must identify go-whisper") {
		t.Fatalf("did not expect go-whisper identity problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsJUT13ManifestWithoutMutablelogicMaintainer(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "org.mutablelogic.go-whisper.local",
		"name": "go-whisper Local STT",
		"version": "external",
		"kind": "stt",
		"transport": {"type": "http-json", "endpoint": "http://127.0.0.1:8081"},
		"capabilities": {"offline": true, "languages": ["en"]},
		"hardware": {"cpu": true},
		"credentials": [],
		"license": {"name": "Apache-2.0", "url": "local-license-reference"},
		"contribution": {"source": "local-source-reference", "maintainers": ["example"]}
	}`)

	_, problems := validateClosureBundleProviderManifest("JUT-13", raw)

	if !containsProblem(problems, "JUT-13 providerManifest contribution maintainers must include mutablelogic") {
		t.Fatalf("expected mutablelogic maintainer problem, got %v", problems)
	}
	if containsProblem(problems, "JUT-13 providerManifest must identify mutablelogic") ||
		containsProblem(problems, "JUT-13 providerManifest must identify go-whisper") {
		t.Fatalf("did not expect provider identity problems, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsBenchmarkReportWithoutGeneratedAt(t *testing.T) {
	raw := json.RawMessage(`{
		"generatedAt": "replace-with-generated-at",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny-en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny-en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000},
			"providerReturned": true
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`)

	_, problems := validateClosureBundleBenchmark("JUT-13", raw)

	if !containsProblem(problems, "generatedAt must be RFC3339") {
		t.Fatalf("expected generatedAt problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsBenchmarkReportWithoutRuntime(t *testing.T) {
	raw := json.RawMessage(`{
		"generatedAt": "2026-06-17T09:03:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny-en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny-en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000},
			"providerReturned": true
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`)

	_, problems := validateClosureBundleBenchmark("JUT-13", raw)

	if !containsProblem(problems, "benchmark environment runtime must identify a concrete OS/arch and Go version") {
		t.Fatalf("expected benchmark runtime problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsJUT11ModelEvidenceOutsideManifest(t *testing.T) {
	models := []providerModelEvidence{
		{ModelID: "okay-nabu"},
		{ModelID: "ohf-jute"},
	}
	manifest := voice.ProviderManifest{
		WakeWord: voice.WakeWordManifest{
			Models: []voice.WakeWordModelManifest{
				{ID: "okay-nabu"},
			},
		},
	}

	problems := validateClosureBundleModelsAgainstProviderManifest("JUT-11", models, manifest)

	if len(problems) != 1 ||
		problems[0] != "JUT-11 model evidence ohf-jute must be declared in providerManifest wakeWord.models" {
		t.Fatalf("expected undeclared model problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsBenchmarkRuntimeOutsideModelEvidenceRuntime(t *testing.T) {
	report := voice.BenchmarkReport{
		Environment: voice.BenchmarkEnvironment{
			OS:        "linux",
			Arch:      "amd64",
			GoVersion: "go1.25.0",
			ModelID:   "tiny-en",
			ModelHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	models := []providerModelEvidence{{
		ModelID:   "tiny-en",
		ModelHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Runtime:   "darwin/arm64 go1.25.0",
	}}

	problems := validateClosureBundleBenchmarkModel("JUT-13", report, models)

	if len(problems) != 1 ||
		problems[0] != "JUT-13 benchmark runtime must match compatible go-whisper model evidence runtime" {
		t.Fatalf("expected runtime mismatch problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsRaspberryPiManifestWithoutPackaging(t *testing.T) {
	packaging := providerPackagingEvidence{
		Targets: []providerPackagingTarget{
			{Target: "raspberry-pi-arm64", Status: "unsupported"},
		},
	}
	manifest := voice.ProviderManifest{
		Hardware: map[string]bool{"raspberryPi": true},
	}

	problems := validateClosureBundlePackagingAgainstProviderManifest("JUT-13", packaging, manifest)

	if len(problems) != 1 ||
		problems[0] != "providerManifest hardware.raspberryPi requires succeeded raspberry-pi-arm64 packaging evidence" {
		t.Fatalf("expected raspberry pi manifest packaging problem, got %v", problems)
	}
}

func TestProviderClosureBundleRejectsRaspberryPiPackagingWithoutManifestSupport(t *testing.T) {
	packaging := providerPackagingEvidence{
		Targets: []providerPackagingTarget{
			{Target: "raspberry-pi-arm64", Status: "succeeded"},
		},
	}
	manifest := voice.ProviderManifest{
		Hardware: map[string]bool{"raspberryPi": false},
	}

	problems := validateClosureBundlePackagingAgainstProviderManifest("JUT-11", packaging, manifest)

	if len(problems) != 1 ||
		problems[0] != "succeeded raspberry-pi-arm64 packaging evidence requires providerManifest hardware.raspberryPi" {
		t.Fatalf("expected raspberry pi packaging manifest problem, got %v", problems)
	}
}

func TestProviderClosureBundleEvidenceRowsRequireGeneratedAt(t *testing.T) {
	buildProblems := validateProviderBuildEvidence(providerBuildEvidence{
		Issue:        "JUT-13",
		ProviderID:   "go-whisper",
		ProviderKind: "stt",
		Target:       "native-cli",
		CommandID:    "go-test",
		Status:       "succeeded",
		Runtime:      "darwin/arm64 go1.25.0",
	})
	packagingProblems := validateProviderPackagingEvidence(providerPackagingEvidence{
		Issue:        "JUT-13",
		ProviderID:   "go-whisper",
		ProviderKind: "stt",
		Targets: []providerPackagingTarget{
			{Target: "cpu-only", Status: "succeeded"},
			{Target: "macos-metal", Status: "blocked"},
			{Target: "container-linux", Status: "failed"},
			{Target: "raspberry-pi-arm64", Status: "unsupported"},
		},
		Runtime: "darwin/arm64 go1.25.0",
		Notes:   "GPU and container targets require separate provider-pack runners.",
	})
	modelProblems := validateProviderModelEvidence(providerModelEvidence{
		Issue:               "JUT-13",
		ProviderID:          "go-whisper",
		ProviderKind:        "stt",
		ModelID:             "tiny.en",
		ModelHash:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ModelSource:         "whisper",
		ModelFormat:         "gguf",
		CompatibilityStatus: "compatible",
		RuntimeStatus:       "loaded",
		Runtime:             "darwin/arm64 go1.25.0",
	})

	for name, problems := range map[string][]string{
		"build":     buildProblems,
		"packaging": packagingProblems,
		"model":     modelProblems,
	} {
		if !containsProblem(problems, "generatedAt must be RFC3339") {
			t.Fatalf("expected %s generatedAt problem, got %v", name, problems)
		}
	}
}

func TestProviderClosureBundleRejectsGeneratedArtifactsWithStaleProblemsOrFlags(t *testing.T) {
	buildProblems := validateProviderBuildEvidenceArtifact(providerBuildEvidence{
		ClosureEvidence: true,
		Problems:        []string{"provider build did not succeed"},
	})
	packagingProblems := validateProviderPackagingEvidenceArtifact(providerPackagingEvidence{
		PackagingEvidenceComplete: false,
	})
	modelProblems := validateProviderModelEvidenceArtifact(providerModelEvidence{
		ClosureEvidence: false,
		Problems:        []string{"model runtime load is not proven"},
	})

	for _, want := range []string{
		"generated build evidence artifact must not carry validation problems",
	} {
		if !containsProblem(buildProblems, want) {
			t.Fatalf("expected build artifact problem %q, got %v", want, buildProblems)
		}
	}
	if !containsProblem(
		packagingProblems,
		"generated packaging evidence artifact must have packagingEvidenceComplete true",
	) {
		t.Fatalf("expected packaging completion flag problem, got %v", packagingProblems)
	}
	for _, want := range []string{
		"generated model evidence artifact must not carry validation problems",
		"generated model evidence artifact must have closureEvidence true",
	} {
		if !containsProblem(modelProblems, want) {
			t.Fatalf("expected model artifact problem %q, got %v", want, modelProblems)
		}
	}
}

func TestProviderClosureBundleRejectsCrossIssueGeneratedEvidence(t *testing.T) {
	buildProblems := validateClosureBundleBuilds("JUT-11", []providerBuildEvidence{
		{
			GeneratedAt:     "2026-06-17T09:00:00Z",
			Issue:           "JUT-13",
			ProviderID:      "go-whisper",
			ProviderKind:    "stt",
			Target:          "native-cli",
			CommandID:       "go-test",
			Status:          "succeeded",
			Runtime:         "darwin/arm64 go1.25.0",
			ClosureEvidence: true,
		},
	})
	_, packagingProblems := validateClosureBundlePackaging("JUT-13", providerPackagingEvidence{
		GeneratedAt:  "2026-06-17T09:00:00Z",
		Issue:        "JUT-11",
		ProviderID:   "pmdroid-microwakeword",
		ProviderKind: "wake-word",
		Targets: []providerPackagingTarget{
			{Target: "macos-native", Status: "failed"},
			{Target: "linux-native", Status: "failed"},
			{Target: "raspberry-pi-arm64", Status: "unsupported"},
		},
		Runtime:                   "darwin/arm64 go1.25.0",
		Notes:                     "native pmdroid package failed and Raspberry Pi unsupported in this run",
		PackagingEvidenceComplete: true,
	})
	_, modelProblems := validateClosureBundleModels("JUT-11", []providerModelEvidence{
		{
			GeneratedAt:         "2026-06-17T09:00:00Z",
			Issue:               "JUT-13",
			ProviderID:          "go-whisper",
			ProviderKind:        "stt",
			ModelID:             "tiny.en",
			ModelHash:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			ModelSource:         "whisper",
			ModelFormat:         "gguf",
			CompatibilityStatus: "compatible",
			RuntimeStatus:       "loaded",
			Runtime:             "darwin/arm64 go1.25.0",
			ClosureEvidence:     true,
		},
	})

	for _, want := range []string{
		"buildEvidence[0]: build evidence issue must match closure issue",
		"JUT-11 closure requires at least one successful pmdroid/microWakeWord build evidence row",
	} {
		if !containsProblem(buildProblems, want) {
			t.Fatalf("expected build problem %q, got %v", want, buildProblems)
		}
	}
	if !containsProblem(packagingProblems, "packaging evidence issue must match closure issue") {
		t.Fatalf("expected packaging issue mismatch, got %v", packagingProblems)
	}
	for _, want := range []string{
		"modelEvidence[0]: model evidence issue must match closure issue",
		"JUT-11 closure requires compatible ESPHome model evidence",
		"JUT-11 closure requires compatible OHF model evidence",
	} {
		if !containsProblem(modelProblems, want) {
			t.Fatalf("expected model problem %q, got %v", want, modelProblems)
		}
	}
}

func TestProviderClosureBundleRejectsStrongDecisionWithIncompletePackaging(t *testing.T) {
	jut11Problems := validateClosureBundleDecisionAgainstPackaging(
		"JUT-11",
		providerClosureDecision{Status: "adopt-optional-provider"},
		providerPackagingEvidence{
			Targets: []providerPackagingTarget{
				{Target: "macos-native", Status: "failed"},
				{Target: "linux-native", Status: "succeeded"},
				{Target: "raspberry-pi-arm64", Status: "unsupported"},
			},
		},
	)
	jut13Problems := validateClosureBundleDecisionAgainstPackaging(
		"JUT-13",
		providerClosureDecision{Status: "first-class-provider-pack"},
		providerPackagingEvidence{
			Targets: []providerPackagingTarget{
				{Target: "cpu-only", Status: "succeeded"},
				{Target: "macos-metal", Status: "blocked"},
				{Target: "container-linux", Status: "succeeded"},
				{Target: "raspberry-pi-arm64", Status: "unsupported"},
			},
		},
	)
	jut13ExternalProblems := validateClosureBundleDecisionAgainstPackaging(
		"JUT-13",
		providerClosureDecision{Status: "documented-external-provider"},
		providerPackagingEvidence{
			Targets: []providerPackagingTarget{
				{Target: "cpu-only", Status: "failed"},
				{Target: "macos-metal", Status: "blocked"},
				{Target: "container-linux", Status: "failed"},
				{Target: "raspberry-pi-arm64", Status: "unsupported"},
			},
		},
	)

	for _, want := range []string{
		"JUT-11 decision adopt-optional-provider requires packaging target macos-native to have succeeded",
		"JUT-11 decision adopt-optional-provider requires packaging target raspberry-pi-arm64 to have succeeded",
	} {
		if !containsProblem(jut11Problems, want) {
			t.Fatalf("expected JUT-11 decision packaging problem %q, got %v", want, jut11Problems)
		}
	}
	for _, want := range []string{
		"JUT-13 decision first-class-provider-pack requires packaging target macos-metal to have succeeded",
		"JUT-13 decision first-class-provider-pack requires packaging target raspberry-pi-arm64 to have succeeded",
	} {
		if !containsProblem(jut13Problems, want) {
			t.Fatalf("expected JUT-13 decision packaging problem %q, got %v", want, jut13Problems)
		}
	}
	if !containsProblem(
		jut13ExternalProblems,
		"JUT-13 decision documented-external-provider requires cpu-only or container-linux packaging to have succeeded",
	) {
		t.Fatalf("expected JUT-13 documented external packaging problem, got %v", jut13ExternalProblems)
	}
}

func TestVoiceBenchmarkCommandAcceptsJUT11ClosureBundleWhenBenchmarkUsesOneCompatibleModel(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "microwakeword-complete.json")
	bundle := `{
		"issue": "JUT-11",
		"decision": {
			"status": "defer",
			"rationale": "Build, packaging, model, and baseline comparison evidence support deferring production adoption."
		},
		"providerManifest": {
			"id": "org.pmdroid.microwakeword.local",
			"name": "microWakeWord Local",
			"version": "experimental",
			"kind": "wake-word",
			"transport": {"type": "builtin", "endpoint": "local-voice-service"},
			"capabilities": {"offline": true, "languages": ["en"]},
			"hardware": {"cpu": true, "raspberryPi": false},
			"credentials": [],
			"license": {"name": "MIT", "url": "local-license-reference"},
			"contribution": {"source": "local-source-reference", "maintainers": ["pmdroid"]},
			"wakeWord": {
				"defaultModelId": "okay-nabu",
				"phrase": "Okay Nabu",
				"languages": ["en"],
				"sensitivity": 0.55,
				"models": [
					{"id": "okay-nabu", "path": "assets/okay_nabu.tflite", "phrase": "Okay Nabu", "languages": ["en"], "sensitivity": 0.55},
					{"id": "ohf-jute", "path": "assets/ohf_jute.tflite", "phrase": "Hey Jute", "languages": ["en"], "sensitivity": 0.55}
				]
			}
		},
		"fixtureManifest": {
			"issue": "JUT-11",
			"kind": "wake-word",
			"fixtures": [
				{"id": "positive-wake", "path": "wake/positive-wake.wav", "sha256": "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": true, "language": "en"},
				{"id": "near-miss", "path": "wake/near-miss.wav", "sha256": "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": false, "language": "en"},
				{"id": "ambient-room", "path": "wake/ambient-room.wav", "sha256": "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": false, "language": "en"},
				{"id": "conversation-long", "path": "wake/conversation-long.wav", "sha256": "sha256:9999999999999999999999999999999999999999999999999999999999999999", "source": "synthetic-test", "recordedAt": "2026-06-17T09:00:00Z", "consent": true, "expectWake": false, "language": "en"}
			]
		},
		"buildEvidence": [{
			"generatedAt": "2026-06-17T09:00:00Z",
			"issue": "JUT-11",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"target": "native-consumer",
			"commandId": "go-test-package",
			"status": "succeeded",
			"runtime": "linux/arm64 go1.25.0",
			"closureEvidence": true
		}],
		"packagingEvidence": {
			"generatedAt": "2026-06-17T09:01:00Z",
			"issue": "JUT-11",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"targets": [
				{"target": "macos-native", "status": "failed"},
				{"target": "linux-native", "status": "succeeded"},
				{"target": "raspberry-pi-arm64", "status": "unsupported"}
			],
			"runtime": "linux/arm64 go1.25.0",
			"notes": "macOS native failed; Raspberry Pi unsupported in this run",
			"packagingEvidenceComplete": true
		},
		"modelEvidence": [
			{"generatedAt": "2026-06-17T09:02:00Z", "issue": "JUT-11", "providerId": "pmdroid-microwakeword", "providerKind": "wake-word", "modelId": "okay-nabu", "modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "modelSource": "esphome", "modelFormat": "tflite", "compatibilityStatus": "compatible", "runtimeStatus": "loaded", "runtime": "linux/arm64 go1.25.0", "closureEvidence": true},
			{"generatedAt": "2026-06-17T09:02:01Z", "issue": "JUT-11", "providerId": "pmdroid-microwakeword", "providerKind": "wake-word", "modelId": "ohf-jute", "modelHash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "modelSource": "ohf", "modelFormat": "tflite", "compatibilityStatus": "compatible", "runtimeStatus": "loaded", "runtime": "linux/arm64 go1.25.0", "closureEvidence": true}
		],
		"benchmarkReport": {
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {"os": "linux", "arch": "arm64", "goVersion": "go1.25.0", "providerId": "pmdroid-microwakeword", "providerKind": "wake-word", "modelId": "ohf-jute", "modelHash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "near-miss", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "ambient-room", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "conversation-long", "providerId": "pmdroid-microwakeword", "modelId": "ohf-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
			],
			"summary": {"total": 4, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 45000000}
		},
		"baselineReport": {
			"generatedAt": "2026-06-17T09:04:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {"os": "linux", "arch": "arm64", "goVersion": "go1.25.0", "providerId": "wyoming-openwakeword", "providerKind": "wake-word", "modelId": "hey-jute", "modelHash": "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}},
				{"fixtureId": "near-miss", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}},
				{"fixtureId": "ambient-room", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}},
				{"fixtureId": "conversation-long", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 70000000, "resourceSample": {"duration": 70000000}}
			],
			"summary": {"total": 4, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 70000000}
		}
	}`
	if err := os.WriteFile(bundlePath, []byte(bundle), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-closure-bundle", bundlePath}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf(
			"expected complete JUT-11 closure bundle, got %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	for _, want := range []string{
		"Provider Closure Bundle: JUT-11",
		"Decision: `defer`",
		"Provider manifest accepted: `true`",
		"Provider manifest: `org.pmdroid.microwakeword.local` (`wake-word`)",
		"Fixture manifest accepted: `true`",
		"Fixture manifest entries: 4",
		"Benchmark accepted: `true`",
		"Baseline accepted: `true`",
		"Comparison accepted: `true`",
		"Closure bundle complete: `true`",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected complete JUT-11 output to contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandRejectsIncompleteJUT11ClosureBundle(t *testing.T) {
	dir := t.TempDir()
	bundlePath := filepath.Join(dir, "microwakeword-closure.json")
	bundle := `{
		"issue": "JUT-11",
		"packagingEvidence": {
			"generatedAt": "2026-06-17T09:01:00Z",
			"issue": "JUT-11",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"targets": [
				{"target": "macos-native", "status": "failed"},
				{"target": "linux-native", "status": "not-run"}
			],
			"runtime": "linux/amd64 go1.25.0",
			"packagingEvidenceComplete": false
		},
		"modelEvidence": [{
			"generatedAt": "2026-06-17T09:02:00Z",
			"issue": "JUT-11",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"modelId": "okay-nabu",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"modelSource": "esphome",
			"modelFormat": "tflite",
			"compatibilityStatus": "compatible",
			"runtimeStatus": "loaded",
			"runtime": "linux/amd64 go1.25.0",
			"closureEvidence": true
		}],
		"benchmarkReport": {
			"generatedAt": "2026-06-17T09:03:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {
				"os": "linux",
				"arch": "amd64",
				"goVersion": "go1.25.0",
				"providerId": "pmdroid-microwakeword",
				"providerKind": "wake-word",
				"modelId": "okay-nabu",
				"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "near-miss", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "ambient-room", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "conversation-long", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
			],
			"summary": {"total": 4, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 45000000}
		}
	}`
	if err := os.WriteFile(bundlePath, []byte(bundle), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-closure-bundle", bundlePath}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected incomplete closure bundle to fail, got %d", code)
	}
	for _, want := range []string{
		"Provider Closure Bundle: JUT-11",
		"Packaging evidence complete: `false`",
		"Benchmark accepted: `true`",
		"Baseline accepted: `false`",
		"Comparison accepted: `false`",
		"Closure bundle complete: `false`",
		"decision: decision status is required",
		"decision: decision rationale must explain the measured evidence",
		"decision: JUT-11 decision status must be adopt-optional-provider, defer, or reject",
		"manifest: providerManifest is required",
		"fixtures: fixtureManifest is required",
		"build: buildEvidence is required",
		"packaging: generated packaging evidence artifact must have packagingEvidenceComplete true",
		"packaging: packaging target linux-native has not been evaluated",
		"packaging: packaging target macos-native requires notes when status is not succeeded",
		"packaging: packaging target linux-native requires notes when status is not succeeded",
		"packaging: packaging target raspberry-pi-arm64 is required",
		"model: JUT-11 closure requires compatible OHF model evidence",
		"baseline: benchmarkReport is required",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected incomplete closure bundle output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "provider closure bundle has") ||
		!strings.Contains(stderr.String(), "validation problem") {
		t.Fatalf("expected closure bundle stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandComparesCandidateWithBaseline(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	candidate := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"modelId": "okay-nabu"
		},
		"wakeResults": [{
			"fixtureId": "positive-wake",
			"providerId": "pmdroid-microwakeword",
			"modelId": "okay-nabu",
			"expectedWake": true,
			"detected": true,
			"matchesExpected": true,
			"latency": 45000000,
			"resourceSample": {"duration": 45000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"falseAccepts": 0,
			"falseRejects": 0,
			"averageLatency": 45000000
		}
	}`
	baseline := `{
		"generatedAt": "2026-06-15T15:31:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "wyoming-openwakeword",
			"providerKind": "wake-word",
			"modelId": "hey-jute"
		},
		"wakeResults": [{
			"fixtureId": "positive-wake",
			"providerId": "wyoming-openwakeword",
			"modelId": "hey-jute",
			"expectedWake": true,
			"detected": true,
			"matchesExpected": true,
			"latency": 70000000,
			"resourceSample": {"duration": 70000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"falseAccepts": 0,
			"falseRejects": 0,
			"averageLatency": 70000000
		}
	}`
	if err := os.WriteFile(candidatePath, []byte(candidate), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte(baseline), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s stdout %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"Voice Benchmark Comparison: JUT-11",
		"Candidate: `pmdroid-microwakeword`",
		"Baseline: `wyoming-openwakeword`",
		"`positive-wake`",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected comparison evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "token=secret") || strings.Contains(stdout.String(), "raw pcm") {
		t.Fatalf("comparison output leaked unsafe data:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandComparesCandidateWithBaselineUsingAcceptancePreset(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	reportJSON := func(provider string) string {
		return fmt.Sprintf(`{
			"generatedAt": "2026-06-15T15:30:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {
				"os": "linux",
				"arch": "arm64",
				"goVersion": "go1.25.0",
				"providerId": %q,
				"providerKind": "wake-word",
				"modelId": "okay-nabu",
				"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": %q, "modelId": "okay-nabu", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "near-miss", "providerId": %q, "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "ambient-room", "providerId": %q, "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "conversation-long", "providerId": %q, "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
			],
			"summary": {
				"total": 4,
				"providerFailures": 0,
				"falseAccepts": 0,
				"falseRejects": 0,
				"averageLatency": 45000000
			}
		}`, provider, provider, provider, provider, provider)
	}
	if err := os.WriteFile(candidatePath, []byte(reportJSON("pmdroid-microwakeword")), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte(reportJSON("wyoming-openwakeword")), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s stdout %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Voice Benchmark Comparison: JUT-11") ||
		!strings.Contains(stdout.String(), "Shared fixtures: 4") ||
		!strings.Contains(stdout.String(), "Candidate wake errors: 0 false accepts, 0 false rejects") {
		t.Fatalf("expected preset comparison evidence, got:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandComparisonAcceptancePresetRequiresJUT11ProviderRoles(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	reportJSON := func(provider string) string {
		return fmt.Sprintf(`{
			"generatedAt": "2026-06-15T15:30:00Z",
			"issue": "JUT-11",
			"kind": "wake-word",
			"environment": {
				"os": "linux",
				"arch": "arm64",
				"goVersion": "go1.25.0",
				"providerId": %q,
				"providerKind": "wake-word",
				"modelId": "okay-nabu",
				"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			},
			"wakeResults": [
				{"fixtureId": "positive-wake", "providerId": %q, "modelId": "okay-nabu", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "near-miss", "providerId": %q, "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "ambient-room", "providerId": %q, "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
				{"fixtureId": "conversation-long", "providerId": %q, "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
			],
			"summary": {
				"total": 4,
				"providerFailures": 0,
				"falseAccepts": 0,
				"falseRejects": 0,
				"averageLatency": 45000000
			}
		}`, provider, provider, provider, provider, provider)
	}
	if err := os.WriteFile(candidatePath, []byte(reportJSON("wyoming-openwakeword")), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte(reportJSON("wyoming-porcupine")), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf(
			"expected comparison role failure, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	for _, want := range []string{
		"JUT-11 candidate provider must be pmdroid/microWakeWord",
		"JUT-11 baseline provider must be openWakeWord/Wyoming",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected comparison evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "benchmark comparison has") {
		t.Fatalf("expected comparison gap stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandComparisonAcceptancePresetValidatesBothReports(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	candidate := `{
		"generatedAt": "2026-06-17T09:00:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"modelId": "okay-nabu",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"wakeResults": [
			{"fixtureId": "positive-wake", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
			{"fixtureId": "near-miss", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
			{"fixtureId": "ambient-room", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
			{"fixtureId": "conversation-long", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
		],
		"summary": {"total": 4, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 45000000}
	}`
	baseline := `{
		"generatedAt": "2026-06-17T09:01:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"providerId": "wyoming-openwakeword",
			"providerKind": "wake-word",
			"modelId": "hey-jute"
		},
		"wakeResults": [
			{"fixtureId": "positive-wake", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
		],
		"summary": {"total": 1, "providerFailures": 0, "falseAccepts": 0, "falseRejects": 0, "averageLatency": 45000000}
	}`
	if err := os.WriteFile(candidatePath, []byte(candidate), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte(baseline), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected preset comparison validation failure, got code %d", code)
	}
	for _, want := range []string{
		"Voice Benchmark Evidence: JUT-11",
		"Provider: `wyoming-openwakeword`",
		"environment.modelHash is required",
		"benchmark report is missing required fixture near-miss",
		"benchmark report is missing required fixture ambient-room",
		"benchmark report is missing required fixture conversation-long",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected preset failure output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(
		stderr.String(),
		"benchmark comparison preset validation failed: candidate=0 baseline=5 problem",
	) {
		t.Fatalf("expected preset comparison stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandComparisonFailsWhenFixturesDiffer(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	if err := os.WriteFile(candidatePath, []byte(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {"providerId": "micro", "providerKind": "wake-word"},
		"wakeResults": [{"fixtureId": "ambient"}],
		"summary": {"total": 1}
	}`), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {"providerId": "openwakeword", "providerKind": "wake-word"},
		"wakeResults": [{"fixtureId": "positive"}],
		"summary": {"total": 1}
	}`), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected comparison gap failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "candidate and baseline have no shared fixture IDs") ||
		!strings.Contains(stderr.String(), "benchmark comparison has") {
		t.Fatalf("expected comparison gap output, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestVoiceBenchmarkCommandMissingReportDoesNotLeakPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "RAW_REPORT_PATH_SHOULD_NOT_PRINT.json")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-report", path}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failure, got code %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no output, got %q", stdout.String())
	}
	if strings.TrimSpace(stderr.String()) != "benchmark_report_unavailable" {
		t.Fatalf("expected safe report error, got %q", stderr.String())
	}
	for _, leaked := range []string{
		path,
		"RAW_REPORT_PATH_SHOULD_NOT_PRINT",
		"no such file",
		"open ",
	} {
		if strings.Contains(stderr.String(), leaked) {
			t.Fatalf("stderr leaked %q: %s", leaked, stderr.String())
		}
	}
}

func TestVoiceBenchmarkCommandMissingBaselineReportDoesNotLeakPath(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "RAW_BASELINE_PATH_SHOULD_NOT_PRINT.json")
	if err := os.WriteFile(candidatePath, []byte(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {"providerId": "pmdroid-microwakeword", "providerKind": "wake-word"},
		"wakeResults": [{"fixtureId": "positive-wake"}],
		"summary": {"total": 1}
	}`), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failure, got code %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no output, got %q", stdout.String())
	}
	if strings.TrimSpace(stderr.String()) != "baseline_benchmark_report_unavailable" {
		t.Fatalf("expected safe baseline report error, got %q", stderr.String())
	}
	for _, leaked := range []string{
		baselinePath,
		"RAW_BASELINE_PATH_SHOULD_NOT_PRINT",
		"no such file",
		"open ",
	} {
		if strings.Contains(stderr.String(), leaked) {
			t.Fatalf("stderr leaked %q: %s", leaked, stderr.String())
		}
	}
}

func TestVoiceBenchmarkCommandComparisonFailsWhenProvidersMatch(t *testing.T) {
	dir := t.TempDir()
	candidatePath := filepath.Join(dir, "candidate.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	report := `{
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {"providerId": "pmdroid-microwakeword", "providerKind": "wake-word"},
		"wakeResults": [{"fixtureId": "positive-wake"}],
		"summary": {"total": 1}
	}`
	if err := os.WriteFile(candidatePath, []byte(report), 0o600); err != nil {
		t.Fatalf("write candidate: %v", err)
	}
	if err := os.WriteFile(baselinePath, []byte(report), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-report", candidatePath,
		"-baseline-report", baselinePath,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected same-provider comparison failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "candidate and baseline providers must differ") ||
		!strings.Contains(stderr.String(), "benchmark comparison has") {
		t.Fatalf("expected same-provider gap output, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestVoiceBenchmarkCommandFailsIncompleteReportWithEvidence(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "micro",
			"providerKind": "wake-word",
			"modelId": "okay-nabu"
		},
		"wakeResults": [{
			"fixtureId": "ambient",
			"providerId": "micro",
			"modelId": "okay-nabu",
			"expectedWake": false,
			"detected": true,
			"falseAccept": true,
			"matchesExpected": false,
			"latency": 20000000,
			"resourceSample": {"duration": 20000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"falseAccepts": 1,
			"averageLatency": 20000000
		},
		"gaps": ["baseline pending"]
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-issue", "JUT-11",
		"-kind", "wake-word",
		"-min-results", "2",
		"-require-model-hash",
		"-require-wake-matches",
	}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	for _, want := range []string{
		"environment.modelHash is required",
		"benchmark report has unresolved gaps",
		"benchmark report has too few results",
		"`ambient`: mismatched",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsTamperedReportSummary(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"errorCode": "provider_failed"
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "stt summary does not match result details") {
		t.Fatalf("expected tampered summary problem:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsDuplicateReportFixtureRows(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [
			{
				"fixtureId": "short-command",
				"providerId": "go-whisper",
				"modelId": "tiny.en",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"latency": 80000000,
				"resourceSample": {"duration": 80000000}
			},
			{
				"fixtureId": "short-command",
				"providerId": "go-whisper",
				"modelId": "tiny.en",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"latency": 80000000,
				"resourceSample": {"duration": 80000000}
			}
		],
		"summary": {
			"total": 2,
			"providerFailures": 0,
			"transcriptMatches": 2,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "stt result has duplicate fixtureId short-command") {
		t.Fatalf("expected duplicate fixture output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsMatchedSTTWithoutTranscript(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "stt result transcript is required when transcriptMatched is true") {
		t.Fatalf("expected missing transcript problem:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsInvalidModelHash(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:replace-with-model-hash"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "environment.modelHash must be sha256:<64 hex characters>") {
		t.Fatalf("expected invalid model hash output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsMissingResourceSample(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "stt result resourceSample.duration is required") {
		t.Fatalf("expected missing resource sample output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsResultIdentityMismatch(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "wyoming-stt",
			"modelId": "small-en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	for _, want := range []string{
		"stt result providerId does not match environment.providerId",
		"stt result modelId does not match environment.modelId",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected identity mismatch output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "benchmark report has 2 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandAcceptancePresetRequiresJUT13GoWhisperProvider(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "wyoming-stt",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "wyoming-stt",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "environment.providerId must identify go-whisper") {
		t.Fatalf("expected go-whisper provider identity output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsProviderKindMismatch(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "wake-word",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "environment.providerKind does not match benchmark kind") {
		t.Fatalf("expected provider kind mismatch output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsWakeResultIdentityMismatch(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"modelId": "okay-nabu",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"wakeResults": [
			{"fixtureId": "positive-wake", "providerId": "wyoming-openwakeword", "modelId": "hey-jute", "expectedWake": true, "detected": true, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
			{"fixtureId": "near-miss", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
			{"fixtureId": "ambient-room", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}},
			{"fixtureId": "conversation-long", "providerId": "pmdroid-microwakeword", "modelId": "okay-nabu", "expectedWake": false, "detected": false, "matchesExpected": true, "latency": 45000000, "resourceSample": {"duration": 45000000}}
		],
		"summary": {
			"total": 4,
			"providerFailures": 0,
			"falseAccepts": 0,
			"falseRejects": 0,
			"averageLatency": 45000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	for _, want := range []string{
		"wake result providerId does not match environment.providerId",
		"wake result modelId does not match environment.modelId",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected wake identity mismatch output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "benchmark report has 2 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandAppliesAcceptancePreset(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-11",
		"kind": "wake-word",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "pmdroid-microwakeword",
			"providerKind": "wake-word",
			"modelId": "okay-nabu"
		},
		"wakeResults": [{
			"fixtureId": "positive-wake",
			"providerId": "pmdroid-microwakeword",
			"modelId": "okay-nabu",
			"expectedWake": true,
			"detected": false,
			"falseReject": true,
			"matchesExpected": false,
			"latency": 45000000,
			"resourceSample": {"duration": 45000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"falseAccepts": 0,
			"falseRejects": 1,
			"averageLatency": 45000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected preset validation failure, got code %d", code)
	}
	for _, want := range []string{
		"environment.modelHash is required",
		"benchmark report has too few results",
		"benchmark report is missing required fixture near-miss",
		"benchmark report is missing required fixture ambient-room",
		"benchmark report is missing required fixture conversation-long",
		"wake result did not match expected detection",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected preset output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "benchmark report has 6 validation problem") {
		t.Fatalf("expected preset validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsUnknownAcceptancePresetIssue(t *testing.T) {
	report := `{
		"issue": "JUT-99",
		"kind": "stt",
		"environment": {"providerId": "fixture", "providerKind": "stt"},
		"summary": {"total": 0}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected usage failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), `no acceptance preset is defined for issue "JUT-99"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
}

func TestVoiceBenchmarkCommandAcceptancePresetRequiresNamedSTTFixture(t *testing.T) {
	report := `{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"os": "linux",
			"arch": "arm64",
			"goVersion": "go1.25.0",
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en",
			"modelHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		},
		"sttResults": [{
			"fixtureId": "ad-hoc-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true,
			"latency": 80000000,
			"resourceSample": {"duration": 80000000}
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1,
			"averageLatency": 80000000
		}
	}`
	var stdout, stderr bytes.Buffer

	code := run([]string{"-acceptance-preset"}, strings.NewReader(report), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected preset validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "benchmark report is missing required fixture short-command") {
		t.Fatalf("expected missing STT fixture output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "benchmark report has 1 validation problem") {
		t.Fatalf("expected preset validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandPrintsFixtureTemplates(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantIssue     string
		wantKind      string
		wantFixtures  int
		wantFirstPath string
	}{
		{
			name:          "stt",
			args:          []string{"-fixture-template", "stt"},
			wantIssue:     "JUT-13",
			wantKind:      "stt",
			wantFixtures:  1,
			wantFirstPath: "stt/short-command.wav",
		},
		{
			name:          "wake",
			args:          []string{"-fixture-template", "wake-word"},
			wantIssue:     "JUT-11",
			wantKind:      "wake-word",
			wantFixtures:  4,
			wantFirstPath: "wake/positive-wake.wav",
		},
		{
			name:          "custom issue",
			args:          []string{"-fixture-template", "stt", "-issue", "JUT-CUSTOM"},
			wantIssue:     "JUT-CUSTOM",
			wantKind:      "stt",
			wantFixtures:  1,
			wantFirstPath: "stt/short-command.wav",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("expected success, got code %d stderr %s", code, stderr.String())
			}
			var manifest voice.BenchmarkFixtureSetManifest
			if err := json.Unmarshal(stdout.Bytes(), &manifest); err != nil {
				t.Fatalf("decode fixture template: %v\n%s", err, stdout.String())
			}
			if manifest.Issue != tt.wantIssue ||
				manifest.Kind != tt.wantKind ||
				len(manifest.Fixtures) != tt.wantFixtures ||
				manifest.Fixtures[0].Path != tt.wantFirstPath {
				t.Fatalf("unexpected template: %+v", manifest)
			}
			if strings.Contains(stdout.String(), "../") ||
				strings.Contains(stdout.String(), "https://") {
				t.Fatalf("template contains unsafe fixture path:\n%s", stdout.String())
			}
			expectations, ok := voice.BenchmarkAcceptanceExpectations(tt.wantIssue)
			if ok {
				if manifest.Kind != expectations.Kind {
					t.Fatalf("template kind %q does not match preset kind %q", manifest.Kind, expectations.Kind)
				}
				fixturesByID := map[string]voice.BenchmarkFixtureManifest{}
				for _, fixture := range manifest.Fixtures {
					fixturesByID[fixture.ID] = fixture
				}
				for _, fixtureID := range expectations.RequiredFixtureIDs {
					fixture, ok := fixturesByID[fixtureID]
					if !ok {
						t.Fatalf("template is missing required fixture %s", fixtureID)
					}
					switch expectations.Kind {
					case "wake-word":
						if fixture.ExpectWake == nil {
							t.Fatalf("wake fixture %s must declare expectWake", fixtureID)
						}
					case "stt":
						if strings.TrimSpace(fixture.ExpectedTranscript) == "" {
							t.Fatalf("stt fixture %s must declare expectedTranscript", fixtureID)
						}
					}
				}
			}
		})
	}
}

func TestVoiceBenchmarkCommandRejectsUnknownFixtureTemplate(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"-fixture-template", "browser"}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected usage failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "fixture-template must be wake-word or stt") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestVoiceBenchmarkCommandPrintsClosureBundleTemplates(t *testing.T) {
	tests := []struct {
		issue           string
		wantProvider    string
		wantManifest    string
		wantModelRows   int
		wantBenchmark   string
		wantBaselineSet bool
	}{
		{
			issue:           "JUT-13",
			wantProvider:    "go-whisper",
			wantManifest:    "go-whisper",
			wantModelRows:   1,
			wantBenchmark:   `"fixtureId": "short-command"`,
			wantBaselineSet: false,
		},
		{
			issue:           "JUT-11",
			wantProvider:    "pmdroid-microwakeword",
			wantManifest:    "microwakeword",
			wantModelRows:   2,
			wantBenchmark:   `"fixtureId": "positive-wake"`,
			wantBaselineSet: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.issue, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"-closure-bundle-template", tt.issue}, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("expected template success, got code %d stderr %s", code, stderr.String())
			}
			var bundle providerClosureBundleFile
			if err := json.Unmarshal(stdout.Bytes(), &bundle); err != nil {
				t.Fatalf("decode closure bundle template: %v\n%s", err, stdout.String())
			}
			if bundle.Issue != tt.issue {
				t.Fatalf("expected issue %s, got %s", tt.issue, bundle.Issue)
			}
			if bundle.PackagingEvidence == nil {
				t.Fatalf("template missing packaging evidence")
			}
			if len(bundle.ProviderManifest) == 0 {
				t.Fatalf("template missing provider manifest")
			}
			if len(bundle.FixtureManifest) == 0 {
				t.Fatalf("template missing fixture manifest")
			}
			if !strings.Contains(strings.ToLower(string(bundle.ProviderManifest)), tt.wantManifest) {
				t.Fatalf(
					"provider manifest template missing provider %q:\n%s",
					tt.wantManifest,
					string(bundle.ProviderManifest),
				)
			}
			if bundle.PackagingEvidence.ProviderID != tt.wantProvider {
				t.Fatalf("expected provider %s, got %s", tt.wantProvider, bundle.PackagingEvidence.ProviderID)
			}
			if len(bundle.ModelEvidence) != tt.wantModelRows {
				t.Fatalf("expected %d model rows, got %d", tt.wantModelRows, len(bundle.ModelEvidence))
			}
			if !strings.Contains(string(bundle.BenchmarkReport), tt.wantBenchmark) {
				t.Fatalf("benchmark template missing %q:\n%s", tt.wantBenchmark, string(bundle.BenchmarkReport))
			}
			if (len(bundle.BaselineReport) > 0) != tt.wantBaselineSet {
				t.Fatalf("unexpected baseline presence: %t", len(bundle.BaselineReport) > 0)
			}
			if strings.Contains(stdout.String(), "../") ||
				strings.Contains(stdout.String(), "https://") ||
				strings.Contains(stdout.String(), "token=") {
				t.Fatalf("closure template contains unsafe placeholder:\n%s", stdout.String())
			}

			templatePath := filepath.Join(t.TempDir(), "closure-template.json")
			if err := os.WriteFile(templatePath, stdout.Bytes(), 0o600); err != nil {
				t.Fatalf("write closure template: %v", err)
			}
			var validateStdout, validateStderr bytes.Buffer
			validateCode := run(
				[]string{"-closure-bundle", templatePath},
				strings.NewReader(""),
				&validateStdout,
				&validateStderr,
			)
			if validateCode != 1 {
				t.Fatalf("template should fail closure until placeholders are replaced, got %d", validateCode)
			}
			if !strings.Contains(validateStdout.String(), "Provider Closure Bundle: "+tt.issue) ||
				!strings.Contains(validateStdout.String(), "Closure bundle complete: `false`") {
				t.Fatalf("expected closure validation summary, got:\n%s", validateStdout.String())
			}
		})
	}
}

func TestVoiceBenchmarkCommandRejectsUnknownClosureBundleTemplate(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"-closure-bundle-template", "JUT-6"}, strings.NewReader(""), &stdout, &stderr)

	if code != 2 {
		t.Fatalf("expected usage failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "closure-bundle-template must be JUT-11 or JUT-13") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestVoiceBenchmarkCommandValidatesFixtureManifest(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stt"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	fixture, err := voice.NewBenchmarkToneFixture("short-command", "fixture", voice.BenchmarkAudioSpec{
		Duration:  60 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.2,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stt", "short-command.wav"), raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on token=secret lights",
			"language": "en-GB"
		}]
	}`, voice.BenchmarkBytesSHA256(raw))
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s stdout %s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"Voice Benchmark Fixture Set: JUT-13",
		"Fixtures: 1 loaded / 1 declared",
		"`short-command`: path=`stt/short-command.wav`, sha256=set",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected fixture evidence to contain %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "turn on token=secret") {
		t.Fatalf("fixture evidence leaked transcript body:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandValidatesFixtureManifestAcceptancePreset(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stt"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	fixture, err := voice.NewBenchmarkToneFixture("short-command", "fixture", voice.BenchmarkAudioSpec{
		Duration:  60 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.2,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stt", "short-command.wav"), raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on the lights",
			"language": "en-GB"
		}]
	}`, voice.BenchmarkBytesSHA256(raw))
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s stdout %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Fixtures: 1 loaded / 1 declared") ||
		strings.Contains(stdout.String(), "Validation problems") {
		t.Fatalf("unexpected preset fixture manifest output:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandFixtureManifestAcceptancePresetRequiresNamedFixtures(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "wake"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	fixture, err := voice.NewBenchmarkToneFixture("positive-wake", "fixture", voice.BenchmarkAudioSpec{
		Duration:  60 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.2,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wake", "positive-wake.wav"), raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	manifestPath := filepath.Join(dir, "wake-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [{
			"id": "positive-wake",
			"path": "wake/positive-wake.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectWake": true,
			"language": "en"
		}]
	}`, voice.BenchmarkBytesSHA256(raw))
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected fixture preset validation failure, got code %d", code)
	}
	for _, want := range []string{
		"fixture manifest is missing required fixture near-miss",
		"fixture manifest is missing required fixture ambient-room",
		"fixture manifest is missing required fixture conversation-long",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected output to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "fixture manifest has 3 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandFixtureManifestAcceptancePresetRequiresFixtureMetadata(t *testing.T) {
	t.Run("wake fixtures declare expected detection", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "wake"), 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		hashes := map[string]string{}
		for _, fixtureID := range []string{"positive-wake", "near-miss", "ambient-room", "conversation-long"} {
			hashes[fixtureID] = writeToneFixtureForTest(t, dir, filepath.Join("wake", fixtureID+".wav"), fixtureID)
		}
		manifestPath := filepath.Join(dir, "wake-fixtures.json")
		manifest := fmt.Sprintf(`{
			"issue": "JUT-11",
			"kind": "wake-word",
			"fixtures": [
				{"id": "positive-wake", "path": "wake/positive-wake.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": true, "language": "en"},
				{"id": "near-miss", "path": "wake/near-miss.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "language": "en"},
				{"id": "ambient-room", "path": "wake/ambient-room.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
				{"id": "conversation-long", "path": "wake/conversation-long.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"}
			]
		}`, hashes["positive-wake"], hashes["near-miss"], hashes["ambient-room"], hashes["conversation-long"])
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		var stdout, stderr bytes.Buffer

		code := run([]string{
			"-fixture-manifest", manifestPath,
			"-fixture-dir", dir,
			"-acceptance-preset",
		}, strings.NewReader(""), &stdout, &stderr)

		if code != 1 {
			t.Fatalf("expected fixture preset validation failure, got code %d", code)
		}
		if !strings.Contains(stdout.String(), "fixture manifest fixture near-miss must declare expectWake") {
			t.Fatalf("expected missing expectWake output:\n%s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "fixture manifest has 1 validation problem") {
			t.Fatalf("expected validation stderr, got %q", stderr.String())
		}
	})

	t.Run("stt fixtures declare expected transcript", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "stt"), 0o755); err != nil {
			t.Fatalf("create fixture dir: %v", err)
		}
		hash := writeToneFixtureForTest(t, dir, filepath.Join("stt", "short-command.wav"), "short-command")
		manifestPath := filepath.Join(dir, "stt-fixtures.json")
		manifest := fmt.Sprintf(`{
			"issue": "JUT-13",
			"kind": "stt",
			"fixtures": [{
				"id": "short-command",
				"path": "stt/short-command.wav",
				"sha256": %q,
				"source": "synthetic-test",
				"recordedAt": "2026-06-17T00:00:00Z",
				"consent": true,
				"language": "en-GB"
			}]
		}`, hash)
		if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		var stdout, stderr bytes.Buffer

		code := run([]string{
			"-fixture-manifest", manifestPath,
			"-fixture-dir", dir,
			"-acceptance-preset",
		}, strings.NewReader(""), &stdout, &stderr)

		if code != 1 {
			t.Fatalf("expected fixture preset validation failure, got code %d", code)
		}
		if !strings.Contains(
			stdout.String(),
			"fixture manifest fixture short-command must declare expectedTranscript",
		) {
			t.Fatalf("expected missing expectedTranscript output:\n%s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "fixture manifest has 1 validation problem") {
			t.Fatalf("expected validation stderr, got %q", stderr.String())
		}
	})
}

func TestVoiceBenchmarkCommandFixtureManifestAcceptancePresetRejectsDuplicateFixtures(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stt"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	hash := writeToneFixtureForTest(t, dir, filepath.Join("stt", "short-command.wav"), "short-command")
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [
			{"id": "short-command", "path": "stt/short-command.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"},
			{"id": "short-command", "path": "stt/short-command.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"}
		]
	}`, hash, hash)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected fixture preset validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "fixture manifest has duplicate fixture short-command") {
		t.Fatalf("expected duplicate fixture output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "fixture manifest has 1 validation problem") {
		t.Fatalf("expected validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandEmitsPublicFixtureFailureReportWithAcceptanceProblems(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stt"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	fixture, err := voice.NewBenchmarkToneFixture("short-command", "fixture", voice.BenchmarkAudioSpec{
		Duration:  60 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.2,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stt", "short-command.wav"), raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on token=secret lights",
			"language": "en-GB"
		}]
	}`, voice.BenchmarkBytesSHA256(raw))
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-fixture-failure-report",
		"-acceptance-preset",
		"-public-json",
		"-provider-id", "go-whisper",
		"-model-id", "tiny.en",
		"-model-hash", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf(
			"expected acceptance validation failure, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	for _, want := range []string{
		`"issue": "JUT-13"`,
		`"providerId": "go-whisper"`,
		`"modelId": "tiny.en"`,
		`"errorCode": "provider_unavailable"`,
		`"expectedTranscriptSet": true`,
		`"transcriptReturned": false`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected fixture failure report to contain %q:\n%s", want, stdout.String())
		}
	}
	for _, leaked := range []string{
		"turn on",
		"token=secret",
		"token=[redacted]",
		"expectedTranscript\"",
		"transcript\"",
	} {
		if strings.Contains(stdout.String(), leaked) {
			t.Fatalf("fixture failure public report leaked %q:\n%s", leaked, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "fixture failure report has 3 validation problem") {
		t.Fatalf("expected fixture failure validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandFixtureFailureAcceptanceRejectsPlaceholderModelHash(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "stt"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	hash := writeToneFixtureForTest(t, dir, filepath.Join("stt", "short-command.wav"), "short-command")
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on the lights",
			"language": "en-GB"
		}]
	}`, hash)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-fixture-failure-report",
		"-acceptance-preset",
		"-public-json",
		"-provider-id", "go-whisper",
		"-model-id", "tiny.en",
		"-model-hash", "sha256:replace-with-model-hash",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf(
			"expected model hash validation failure, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	for _, want := range []string{
		`"modelHash": "sha256:replace-with-model-hash"`,
		`"errorCode": "provider_unavailable"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected public fixture failure report to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "fixture failure report has 4 validation problem") {
		t.Fatalf("expected fixture failure validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRunsSTTCommandOverFixtures(t *testing.T) {
	dir := t.TempDir()
	hash := writeToneFixtureForTest(t, dir, filepath.Join("stt", "short-command.wav"), "short-command")
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on the lights",
			"language": "en-GB"
		}]
	}`, hash)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-stt-command", os.Args[0],
		"-stt-command-args-json", `["-test.run=TestVoiceBenchmarkCommandSTTCommandHelper","--","{fixture}"]`,
		"-provider-id", "go-whisper-fixture",
		"-model-id", "tiny-en",
		"-model-hash", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf(
			"expected command benchmark success, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	var report voice.BenchmarkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode command benchmark report: %v\n%s", err, stdout.String())
	}
	if report.Issue != "JUT-13" ||
		report.Kind != "stt" ||
		report.Environment.ProviderID != "go-whisper-fixture" ||
		report.Environment.ProviderKind != "stt" ||
		report.Environment.ModelID != "tiny-en" ||
		report.Summary.Total != 1 ||
		report.Summary.ProviderFailures != 0 ||
		report.Summary.TranscriptMatches != 1 {
		t.Fatalf("unexpected command benchmark report: %+v", report)
	}
	if len(report.STTResults) != 1 ||
		report.STTResults[0].FixtureID != "short-command" ||
		report.STTResults[0].ProviderID != "go-whisper-fixture" ||
		report.STTResults[0].ModelID != "tiny-en" ||
		report.STTResults[0].Language != "en-gb" ||
		!report.STTResults[0].TranscriptMatched ||
		!report.STTResults[0].ProviderReturned ||
		report.STTResults[0].ResourceSample.Duration <= 0 {
		t.Fatalf("unexpected command benchmark result: %+v", report.STTResults)
	}
	if strings.Contains(stdout.String(), ".wav") {
		t.Fatalf("benchmark report leaked temporary WAV path:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandSTTCommandHelper(t *testing.T) {
	if os.Getenv("JUTE_STT_COMMAND_HELPER") != "1" {
		return
	}
	fixturePath := ""
	for index, arg := range os.Args {
		if arg == "--" && index+1 < len(os.Args) {
			fixturePath = os.Args[index+1]
			break
		}
	}
	if fixturePath == "" {
		t.Fatal("fixture path missing")
	}
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("fixture path unavailable: %v", err)
	}
	_, _ = os.Stdout.WriteString(
		`{"text":"turn on the lights","providerId":"go-whisper-fixture","modelId":"tiny-en","language":"en-GB","durationMs":42}`,
	)
	os.Exit(0)
}

func TestVoiceBenchmarkCommandRejectsAmbiguousCommandArgs(t *testing.T) {
	dir := t.TempDir()
	hash := writeToneFixtureForTest(t, dir, filepath.Join("stt", "short-command.wav"), "short-command")
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on the lights",
			"language": "en-GB"
		}]
	}`, hash)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-stt-command", os.Args[0],
		"-stt-command-args", "{fixture}",
		"-stt-command-args-json", `["{fixture}"]`,
		"-provider-id", "go-whisper-fixture",
		"-model-id", "tiny-en",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected ambiguous args failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "stt-command-args and stt-command-args-json cannot both be set") {
		t.Fatalf("expected ambiguous args stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsRelativeSTTCommandPath(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", filepath.Join(t.TempDir(), "missing.json"),
		"-stt-command", "go-whisper",
		"-provider-id", "go-whisper",
		"-model-id", "tiny-en",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected relative stt command failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "stt-command must be an absolute path") {
		t.Fatalf("expected absolute path stderr, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
}

func TestVoiceBenchmarkCommandRejectsMalformedCommandArgsJSON(t *testing.T) {
	dir := t.TempDir()
	hash := writeToneFixtureForTest(t, dir, filepath.Join("wake", "positive-wake.wav"), "positive-wake")
	manifestPath := filepath.Join(dir, "wake-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [{
			"id": "positive-wake",
			"path": "wake/positive-wake.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectWake": true,
			"language": "en"
		}]
	}`, hash)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-wake-command", os.Args[0],
		"-wake-command-args-json", `["{fixture}", 42]`,
		"-provider-id", "pmdroid-microwakeword-fixture",
		"-model-id", "okay-nabu",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected malformed args json failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "decode wake-command-args-json") {
		t.Fatalf("expected malformed args json stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandRejectsRelativeWakeCommandPath(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", filepath.Join(t.TempDir(), "missing.json"),
		"-wake-command", "microwakeword",
		"-provider-id", "pmdroid-microwakeword",
		"-model-id", "okay-nabu",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected relative wake command failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "wake-command must be an absolute path") {
		t.Fatalf("expected absolute path stderr, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
}

func TestVoiceBenchmarkCommandRunsWakeCommandOverFixtures(t *testing.T) {
	dir := t.TempDir()
	hashes := map[string]string{}
	for _, fixtureID := range []string{"positive-wake", "near-miss", "ambient-room", "conversation-long"} {
		hashes[fixtureID] = writeToneFixtureForTest(t, dir, filepath.Join("wake", fixtureID+".wav"), fixtureID)
	}
	manifestPath := filepath.Join(dir, "wake-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [
			{"id": "positive-wake", "path": "wake/positive-wake.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": true, "language": "en"},
			{"id": "near-miss", "path": "wake/near-miss.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
			{"id": "ambient-room", "path": "wake/ambient-room.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
			{"id": "conversation-long", "path": "wake/conversation-long.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"}
		]
	}`, hashes["positive-wake"], hashes["near-miss"], hashes["ambient-room"], hashes["conversation-long"])
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest",
		manifestPath,
		"-fixture-dir",
		dir,
		"-wake-command",
		os.Args[0],
		"-wake-command-args-json",
		`["-test.run=TestVoiceBenchmarkCommandWakeCommandHelper","--","{fixtureId}","{fixture}"]`,
		"-provider-id",
		"pmdroid-microwakeword-fixture",
		"-model-id",
		"okay-nabu",
		"-model-hash",
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf(
			"expected wake command benchmark success, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	var report voice.BenchmarkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode wake command benchmark report: %v\n%s", err, stdout.String())
	}
	if report.Issue != "JUT-11" ||
		report.Kind != "wake-word" ||
		report.Environment.ProviderID != "pmdroid-microwakeword-fixture" ||
		report.Environment.ProviderKind != "wake-word" ||
		report.Environment.ModelID != "okay-nabu" ||
		report.Summary.Total != 4 ||
		report.Summary.ProviderFailures != 0 ||
		report.Summary.FalseAccepts != 0 ||
		report.Summary.FalseRejects != 0 {
		t.Fatalf("unexpected wake command benchmark report: %+v", report)
	}
	for _, result := range report.WakeResults {
		if result.ProviderID != "pmdroid-microwakeword-fixture" ||
			result.ModelID != "okay-nabu" ||
			!result.MatchesExpected ||
			!result.ProviderReturned ||
			result.ResourceSample.Duration <= 0 {
			t.Fatalf("unexpected wake command benchmark result: %+v", result)
		}
	}
	if strings.Contains(stdout.String(), ".wav") {
		t.Fatalf("benchmark report leaked temporary WAV path:\n%s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandWakeCommandHelper(t *testing.T) {
	if os.Getenv("JUTE_WAKE_COMMAND_HELPER") != "1" {
		return
	}
	fixtureID := ""
	fixturePath := ""
	for index, arg := range os.Args {
		if arg == "--" && index+2 < len(os.Args) {
			fixtureID = os.Args[index+1]
			fixturePath = os.Args[index+2]
			break
		}
	}
	if fixtureID == "" || fixturePath == "" {
		t.Fatal("fixture arguments missing")
	}
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("fixture path unavailable: %v", err)
	}
	detected := fixtureID == "positive-wake"
	_, _ = os.Stdout.WriteString(fmt.Sprintf(
		`{"detected":%t,"providerId":"pmdroid-microwakeword-fixture","modelId":"okay-nabu","confidence":0.92,"latencyMs":38}`,
		detected,
	))
	os.Exit(0)
}

func TestVoiceBenchmarkCommandRejectsSTTCommandJSONWithUnknownFields(t *testing.T) {
	dir := t.TempDir()
	hash := writeToneFixtureForTest(t, dir, filepath.Join("stt", "short-command.wav"), "short-command")
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-13",
		"kind": "stt",
		"fixtures": [{
			"id": "short-command",
			"path": "stt/short-command.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectedTranscript": "turn on the lights",
			"language": "en-GB"
		}]
	}`, hash)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-stt-command", "/usr/bin/printf",
		"-stt-command-args-json", `["{\"text\":\"turn on the lights\",\"providerDebug\":\"/tmp/raw.wav\"}"]`,
		"-provider-id", "go-whisper-fixture",
		"-model-id", "tiny-en",
		"-model-hash", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf(
			"expected strict STT command JSON failure, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	if strings.Contains(stdout.String(), "providerDebug") || strings.Contains(stdout.String(), "/tmp/raw.wav") {
		t.Fatalf("strict STT command failure leaked provider debug output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stt command benchmark has") {
		t.Fatalf("expected STT command validation stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"errorCode": "provider_failed"`) {
		t.Fatalf("expected provider failure evidence, got stdout %s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandRejectsWakeCommandJSONWithUnknownFields(t *testing.T) {
	dir := t.TempDir()
	hashes := map[string]string{}
	for _, fixtureID := range []string{"positive-wake", "near-miss", "ambient-room", "conversation-long"} {
		hashes[fixtureID] = writeToneFixtureForTest(t, dir, filepath.Join("wake", fixtureID+".wav"), fixtureID)
	}
	manifestPath := filepath.Join(dir, "wake-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [
			{"id": "positive-wake", "path": "wake/positive-wake.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": true, "language": "en"},
			{"id": "near-miss", "path": "wake/near-miss.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
			{"id": "ambient-room", "path": "wake/ambient-room.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
			{"id": "conversation-long", "path": "wake/conversation-long.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"}
		]
	}`, hashes["positive-wake"], hashes["near-miss"], hashes["ambient-room"], hashes["conversation-long"])
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-wake-command", "/usr/bin/printf",
		"-wake-command-args-json", `["{\"detected\":true,\"providerDebug\":\"/tmp/raw.wav\"}"]`,
		"-provider-id", "pmdroid-microwakeword-fixture",
		"-model-id", "okay-nabu",
		"-model-hash", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"-acceptance-preset",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf(
			"expected strict wake command JSON failure, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	if strings.Contains(stdout.String(), "providerDebug") || strings.Contains(stdout.String(), "/tmp/raw.wav") {
		t.Fatalf("strict wake command failure leaked provider debug output:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "wake command benchmark has") {
		t.Fatalf("expected wake command validation stderr, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"errorCode": "provider_failed"`) {
		t.Fatalf("expected provider failure evidence, got stdout %s", stdout.String())
	}
}

func TestVoiceBenchmarkCommandWakeFixtureFailureReportFailsAcceptancePreset(t *testing.T) {
	dir := t.TempDir()
	hashes := map[string]string{}
	for _, fixtureID := range []string{"positive-wake", "near-miss", "ambient-room", "conversation-long"} {
		hashes[fixtureID] = writeToneFixtureForTest(t, dir, filepath.Join("wake", fixtureID+".wav"), fixtureID)
	}
	manifestPath := filepath.Join(dir, "wake-fixtures.json")
	manifest := fmt.Sprintf(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [
			{"id": "positive-wake", "path": "wake/positive-wake.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": true, "language": "en"},
			{"id": "near-miss", "path": "wake/near-miss.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
			{"id": "ambient-room", "path": "wake/ambient-room.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
			{"id": "conversation-long", "path": "wake/conversation-long.wav", "sha256": %q, "source": "synthetic-test", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"}
		]
	}`, hashes["positive-wake"], hashes["near-miss"], hashes["ambient-room"], hashes["conversation-long"])
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
		"-fixture-failure-report",
		"-acceptance-preset",
		"-provider-id", "pmdroid-microwakeword",
		"-model-id", "okay-nabu",
		"-model-hash", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf(
			"expected acceptance validation failure, got code %d stderr %s stdout %s",
			code,
			stderr.String(),
			stdout.String(),
		)
	}
	for _, want := range []string{
		`"issue": "JUT-11"`,
		`"providerId": "pmdroid-microwakeword"`,
		`"modelId": "okay-nabu"`,
		`"errorCode": "provider_unavailable"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected wake fixture failure report to contain %q:\n%s", want, stdout.String())
		}
	}
	if !strings.Contains(stderr.String(), "fixture failure report has 6 validation problem") {
		t.Fatalf("expected wake fixture failure validation stderr, got %q", stderr.String())
	}
}

func TestVoiceBenchmarkCommandFixtureFailureReportRequiresProviderMetadata(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "stt-fixtures.json")
	if err := os.WriteFile(manifestPath, []byte(`{"issue":"JUT-13","kind":"stt","fixtures":[]}`), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-failure-report",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected metadata validation failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "provider-id is required") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout, got %q", stdout.String())
	}
}

func TestVoiceBenchmarkCommandFailsInvalidFixtureManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "wake-fixtures.json")
	manifest := `{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [{
			"id": "ambient",
			"path": "../ambient.wav",
			"expectWake": false
		}]
	}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-fixture-manifest", manifestPath,
		"-fixture-dir", dir,
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected validation failure, got code %d", code)
	}
	if !strings.Contains(stdout.String(), "relative fixture asset path") ||
		!strings.Contains(stderr.String(), "fixture manifest has 1 validation problem") {
		t.Fatalf("expected fixture validation output, stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestVoiceBenchmarkCommandHashesValidFixtureWAV(t *testing.T) {
	dir := t.TempDir()
	fixture, err := voice.NewBenchmarkToneFixture("short-command", "fixture", voice.BenchmarkAudioSpec{
		Duration:  40 * time.Millisecond,
		Frequency: 220,
		Amplitude: 0.2,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	path := filepath.Join(dir, "fixture.wav")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-fixture-hash", path}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s", code, stderr.String())
	}
	for _, want := range []string{
		voice.BenchmarkBytesSHA256(raw),
		path,
		"duration=40ms",
		"sampleRate=16000",
		"channels=1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected hash output to contain %q:\n%s", want, stdout.String())
		}
	}
}

func TestVoiceBenchmarkCommandFixtureHashMissingPathDoesNotLeakPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "RAW_FIXTURE_PATH_SHOULD_NOT_PRINT.wav")
	var stdout, stderr bytes.Buffer

	code := run([]string{"-fixture-hash", path}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failure, got code %d", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no hash output, got %q", stdout.String())
	}
	if strings.TrimSpace(stderr.String()) != "fixture_unavailable" {
		t.Fatalf("expected safe fixture error, got %q", stderr.String())
	}
	for _, leaked := range []string{
		path,
		"RAW_FIXTURE_PATH_SHOULD_NOT_PRINT",
		"no such file",
		"open ",
	} {
		if strings.Contains(stderr.String(), leaked) {
			t.Fatalf("stderr leaked %q: %s", leaked, stderr.String())
		}
	}
}

func TestVoiceBenchmarkCommandRejectsInvalidFixtureHashInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.wav")
	if err := os.WriteFile(path, []byte("not wav"), 0o600); err != nil {
		t.Fatalf("write invalid fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := run([]string{"-fixture-hash", path}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "validate fixture WAV") {
		t.Fatalf("expected validation error, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no hash output, got %q", stdout.String())
	}
}

func TestVoiceBenchmarkCommandWritesToneFixture(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "tone.wav")
	secondPath := filepath.Join(dir, "tone-again.wav")
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-tone-fixture", firstPath,
		"-tone-duration", "40ms",
		"-tone-frequency", "220",
		"-tone-amplitude", "0.2",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected success, got code %d stderr %s", code, stderr.String())
	}
	firstRaw, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read generated tone fixture: %v", err)
	}
	firstUtterance, err := voice.DecodeBenchmarkWAV(firstRaw, time.Time{})
	if err != nil {
		t.Fatalf("decode generated tone fixture: %v", err)
	}
	for _, want := range []string{
		voice.BenchmarkBytesSHA256(firstRaw),
		firstPath,
		"duration=40ms",
		"sampleRate=16000",
		"channels=1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected generated tone output to contain %q:\n%s", want, stdout.String())
		}
	}
	if got := firstUtterance.EndedAt.Sub(firstUtterance.StartedAt); got != 40*time.Millisecond {
		t.Fatalf("expected 40ms tone duration, got %s", got)
	}
	if firstUtterance.SampleRate != 16000 || firstUtterance.Channels != 1 {
		t.Fatalf(
			"unexpected generated tone shape: sampleRate=%d channels=%d",
			firstUtterance.SampleRate,
			firstUtterance.Channels,
		)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"-tone-fixture", secondPath,
		"-tone-duration", "40ms",
		"-tone-frequency", "220",
		"-tone-amplitude", "0.2",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 {
		t.Fatalf("expected second success, got code %d stderr %s", code, stderr.String())
	}
	secondRaw, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second generated tone fixture: %v", err)
	}
	if !bytes.Equal(firstRaw, secondRaw) {
		t.Fatalf("expected deterministic tone fixture bytes")
	}
}

func TestVoiceBenchmarkCommandRejectsInvalidToneFixtureSpec(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tone.wav")
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-tone-fixture", path,
		"-tone-duration", "-1s",
	}, strings.NewReader(""), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("expected failure, got code %d", code)
	}
	if !strings.Contains(stderr.String(), "build tone fixture") {
		t.Fatalf("expected tone fixture validation error, got %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no tone output, got %q", stdout.String())
	}
}

func writeToneFixtureForTest(t *testing.T, dir string, relativePath string, fixtureID string) string {
	t.Helper()
	fixture, err := voice.NewBenchmarkToneFixture(fixtureID, "fixture", voice.BenchmarkAudioSpec{
		Duration:  60 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.2,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	path := filepath.Join(dir, relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return voice.BenchmarkBytesSHA256(raw)
}

func writeRawJSONArtifactForTest(t *testing.T, dir string, name string, raw json.RawMessage) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write raw JSON artifact %s: %v", name, err)
	}
	return path
}

func writeJSONArtifactForTest(t *testing.T, dir string, name string, value any) string {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON artifact %s: %v", name, err)
	}
	return writeRawJSONArtifactForTest(t, dir, name, raw)
}

func containsProblem(problems []string, want string) bool {
	for _, problem := range problems {
		if strings.Contains(problem, want) {
			return true
		}
	}
	return false
}
