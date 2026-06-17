package voice

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBenchmarkToneFixtureBuildsDeterministicMonoUtterance(t *testing.T) {
	start := time.Date(2026, 6, 15, 16, 0, 0, 0, time.UTC)
	first, err := NewBenchmarkToneFixture("tone", "token=secret fixture", BenchmarkAudioSpec{
		Duration:  100 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.4,
		StartedAt: start,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	second, err := NewBenchmarkToneFixture("tone", "token=secret fixture", BenchmarkAudioSpec{
		Duration:  100 * time.Millisecond,
		Frequency: 440,
		Amplitude: 0.4,
		StartedAt: start,
	})
	if err != nil {
		t.Fatalf("build fixture again: %v", err)
	}

	firstPCM := flattenUtterancePCM(first.Utterance)
	secondPCM := flattenUtterancePCM(second.Utterance)
	if !bytes.Equal(firstPCM, secondPCM) {
		t.Fatalf("expected deterministic PCM")
	}
	if first.Description != "token=[redacted] fixture" {
		t.Fatalf("description was not sanitized: %q", first.Description)
	}
	if first.Utterance.SampleRate != BenchmarkSampleRate ||
		first.Utterance.Channels != BenchmarkChannels ||
		first.Utterance.EndedAt.Sub(first.Utterance.StartedAt) != 100*time.Millisecond {
		t.Fatalf("unexpected utterance metadata: %+v", first.Utterance)
	}
	if len(first.Utterance.Frames) != 5 {
		t.Fatalf("expected 20ms framing, got %d frames", len(first.Utterance.Frames))
	}
}

func TestBenchmarkWAVRoundTripAndHash(t *testing.T) {
	fixture, err := NewBenchmarkToneFixture("tone", "fixture", BenchmarkAudioSpec{
		Duration:  60 * time.Millisecond,
		Frequency: 220,
		Amplitude: 0.3,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode wav: %v", err)
	}
	decoded, err := DecodeBenchmarkWAV(raw, fixture.Utterance.StartedAt)
	if err != nil {
		t.Fatalf("decode wav: %v", err)
	}
	if !bytes.Equal(flattenUtterancePCM(fixture.Utterance), flattenUtterancePCM(decoded)) {
		t.Fatalf("WAV round trip changed PCM")
	}
	hash := BenchmarkBytesSHA256(raw)
	if len(hash) != len("sha256:")+64 {
		t.Fatalf("unexpected hash: %s", hash)
	}
}

func TestBenchmarkAudioValidation(t *testing.T) {
	if _, _, _, err := DeterministicPCM16(BenchmarkAudioSpec{Duration: -time.Second}); err == nil {
		t.Fatalf("expected invalid duration error")
	}
	if _, _, _, err := DeterministicPCM16(BenchmarkAudioSpec{Duration: time.Second, Channels: 2}); err == nil {
		t.Fatalf("expected mono validation error")
	}
	if _, err := NewBenchmarkToneFixture("tone", "fixture", BenchmarkAudioSpec{
		Duration:  time.Second,
		Amplitude: 2,
	}); err == nil {
		t.Fatalf("expected amplitude validation error")
	}
	if _, err := DecodeBenchmarkWAV([]byte("not wav"), time.Time{}); err == nil {
		t.Fatalf("expected invalid WAV error")
	}
}

func TestLoadBenchmarkFixtureSetFromManifest(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 6, 15, 16, 30, 0, 0, time.UTC)
	fixture, err := NewBenchmarkToneFixture("ambient", "ambient fixture", BenchmarkAudioSpec{
		Duration:  80 * time.Millisecond,
		Frequency: 0,
		Amplitude: 0,
		StartedAt: start,
	})
	if err != nil {
		t.Fatalf("build fixture: %v", err)
	}
	raw, err := EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		t.Fatalf("encode wav: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "wake"), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wake", "ambient.wav"), raw, 0o600); err != nil {
		t.Fatalf("write fixture wav: %v", err)
	}
	expectWake := false
	rawManifest := fmt.Sprintf(`{
		"issue": "JUT-11",
		"kind": "wake-word",
		"fixtures": [{
			"id": "ambient-room",
			"description": "token=secret ambient room",
			"path": "wake/ambient.wav",
			"sha256": %q,
			"source": "synthetic-test",
			"recordedAt": "2026-06-17T00:00:00Z",
			"consent": true,
			"expectWake": false,
			"language": "en-GB"
		}]
	}`, BenchmarkBytesSHA256(raw))

	manifest, err := DecodeBenchmarkFixtureSetManifest(rawManifest)
	if err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest.Fixtures[0].ExpectWake = &expectWake
	fixtures, problems := LoadBenchmarkFixtureSet(dir, manifest, start)

	if len(problems) != 0 {
		t.Fatalf("expected no fixture load problems, got %v", problems)
	}
	if len(fixtures) != 1 {
		t.Fatalf("expected one fixture, got %d", len(fixtures))
	}
	loaded := fixtures[0]
	if loaded.ID != "ambient-room" ||
		loaded.Description != "token=[redacted] ambient room" ||
		loaded.ExpectWake == nil ||
		*loaded.ExpectWake ||
		loaded.Language != "en-GB" {
		t.Fatalf("unexpected loaded fixture metadata: %+v", loaded)
	}
	if !bytes.Equal(flattenUtterancePCM(fixture.Utterance), flattenUtterancePCM(loaded.Utterance)) {
		t.Fatalf("fixture load changed PCM")
	}
	if manifest.Fixtures[0].Source != "synthetic-test" ||
		manifest.Fixtures[0].RecordedAt != "2026-06-17T00:00:00Z" ||
		manifest.Fixtures[0].Consent == nil ||
		!*manifest.Fixtures[0].Consent {
		t.Fatalf("unexpected fixture provenance metadata: %+v", manifest.Fixtures[0])
	}
}

func TestBenchmarkFixtureSetManifestRejectsUnsafeInputs(t *testing.T) {
	if _, err := DecodeBenchmarkFixtureSetManifest(`{"issue":"JUT-11","kind":"wake-word","extra":true}`); err == nil {
		t.Fatalf("expected unknown field decode error")
	}
	if _, err := DecodeBenchmarkFixtureSetManifest(
		`{"issue":"JUT-11","kind":"wake-word","fixtures":[]}{"rawAudioPcm":"secret"}`,
	); err == nil {
		t.Fatalf("expected trailing JSON decode error")
	}
	if _, err := DecodeBenchmarkFixtureSetManifest(
		`{"issue":"JUT-11","kind":"wake-word","fixtures":[{"id":"fixture","path":"fixture.wav","rawAudioPcm":"secret"}]}`,
	); err == nil {
		t.Fatalf("expected nested unknown field decode error")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not-wav.txt"), []byte("not wav"), 0o600); err != nil {
		t.Fatalf("write invalid fixture: %v", err)
	}
	tests := []struct {
		name     string
		manifest BenchmarkFixtureSetManifest
		want     string
	}{
		{
			name: "missing issue",
			manifest: BenchmarkFixtureSetManifest{
				Kind: "stt",
				Fixtures: []BenchmarkFixtureManifest{{
					ID:   "fixture",
					Path: "not-wav.txt",
				}},
			},
			want: "issue is required",
		},
		{
			name: "unsafe path",
			manifest: BenchmarkFixtureSetManifest{
				Issue: "JUT-13",
				Kind:  "stt",
				Fixtures: []BenchmarkFixtureManifest{{
					ID:   "fixture",
					Path: "../outside.wav",
				}},
			},
			want: "relative fixture asset path",
		},
		{
			name: "hash mismatch",
			manifest: BenchmarkFixtureSetManifest{
				Issue: "JUT-13",
				Kind:  "stt",
				Fixtures: []BenchmarkFixtureManifest{{
					ID:     "fixture",
					Path:   "not-wav.txt",
					SHA256: "sha256:0000",
				}},
			},
			want: "sha256 does not match",
		},
		{
			name: "invalid wav",
			manifest: BenchmarkFixtureSetManifest{
				Issue: "JUT-13",
				Kind:  "stt",
				Fixtures: []BenchmarkFixtureManifest{{
					ID:   "fixture",
					Path: "not-wav.txt",
				}},
			},
			want: "16-bit mono PCM WAV",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, problems := LoadBenchmarkFixtureSet(dir, tt.manifest, time.Time{})
			if !hasProblem(problems, tt.want) {
				t.Fatalf("expected problem containing %q, got %v", tt.want, problems)
			}
		})
	}
}
