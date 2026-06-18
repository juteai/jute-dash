package voice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fixtureWakeBenchmarkProvider struct {
	detection WakeBenchmarkDetection
	err       error
	seen      CapturedUtterance
}

func (p *fixtureWakeBenchmarkProvider) DetectWake(
	_ context.Context,
	utterance CapturedUtterance,
) (WakeBenchmarkDetection, error) {
	p.seen = utterance
	return p.detection, p.err
}

type fixtureSTTBenchmarkProvider struct {
	result STTResult
	err    error
	seen   CapturedUtterance
}

func (p *fixtureSTTBenchmarkProvider) Transcribe(_ context.Context, utterance CapturedUtterance) (STTResult, error) {
	p.seen = utterance
	return p.result, p.err
}

func TestRunWakeBenchmarkCapturesExpectedDetectionWithoutAudioLeak(t *testing.T) {
	expectedWake := true
	provider := &fixtureWakeBenchmarkProvider{
		detection: WakeBenchmarkDetection{
			Detected:   true,
			DetectedAt: 350 * time.Millisecond,
			ProviderID: "wyoming-openwakeword",
			ModelID:    "hey-jute",
			Confidence: 0.82,
		},
	}
	fixture := BenchmarkFixture{
		ID:         "positive-wake",
		ExpectWake: &expectedWake,
		Utterance:  benchmarkUtterance(t, []byte("raw pcm secret")),
	}

	result := RunWakeBenchmark(context.Background(), provider, fixture)

	if result.ErrorCode != "" || !result.ProviderReturned {
		t.Fatalf("unexpected benchmark failure: %+v", result)
	}
	if !result.MatchesExpected || result.FalseAccept || result.FalseReject {
		t.Fatalf("unexpected expected-result flags: %+v", result)
	}
	if result.ProviderID != "wyoming-openwakeword" ||
		result.ModelID != "hey-jute" ||
		result.DetectedAt != 350*time.Millisecond ||
		result.Confidence != 0.82 {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	provider.seen.Frames[0].PCM[0] = 'x'
	if string(fixture.Utterance.Frames[0].PCM) != "raw pcm secret" {
		t.Fatalf("benchmark passed mutable fixture audio to provider")
	}
	if strings.Contains(result.FixtureID, "raw pcm") {
		t.Fatalf("result leaked raw audio in fixture id: %+v", result)
	}
}

func TestRunWakeBenchmarkFlagsFalseAcceptAndProviderErrors(t *testing.T) {
	expectedWake := false
	falseAccept := RunWakeBenchmark(context.Background(), &fixtureWakeBenchmarkProvider{
		detection: WakeBenchmarkDetection{Detected: true, ProviderID: "micro"},
	}, BenchmarkFixture{
		ID:         "ambient-room",
		ExpectWake: &expectedWake,
		Utterance:  benchmarkUtterance(t, []byte{0, 1, 2}),
	})
	if !falseAccept.FalseAccept || falseAccept.MatchesExpected {
		t.Fatalf("expected false accept flags, got %+v", falseAccept)
	}

	failed := RunWakeBenchmark(context.Background(), &fixtureWakeBenchmarkProvider{
		err: errors.New("model failed with raw pcm bytes"),
	}, BenchmarkFixture{ID: "positive", Utterance: benchmarkUtterance(t, []byte{3})})
	if failed.ErrorCode != "provider_failed" || failed.ProviderReturned {
		t.Fatalf("unexpected provider failure result: %+v", failed)
	}
}

func TestRunSTTBenchmarkCapturesTranscriptMatchAndSanitizesText(t *testing.T) {
	provider := &fixtureSTTBenchmarkProvider{
		result: STTResult{
			Text:       "turn on token=[redacted] lights",
			ProviderID: "wyoming-stt",
			ModelID:    "small-en",
			Language:   "en-GB",
			Duration:   420 * time.Millisecond,
		},
	}
	fixture := BenchmarkFixture{
		ID:                 "turn-on-lights",
		ExpectedTranscript: "Turn on token=secret lights",
		Utterance:          benchmarkUtterance(t, []byte("utterance")),
	}

	result := RunSTTBenchmark(context.Background(), provider, fixture)

	if result.ErrorCode != "" || !result.ProviderReturned {
		t.Fatalf("unexpected benchmark failure: %+v", result)
	}
	if !result.TranscriptMatched {
		t.Fatalf("expected sanitized transcript match, got %+v", result)
	}
	if result.ExpectedTranscript != "Turn on token=[redacted] lights" ||
		result.Transcript != "turn on token=[redacted] lights" {
		t.Fatalf("expected sanitized transcript fields, got %+v", result)
	}
	if result.ProviderID != "wyoming-stt" ||
		result.ModelID != "small-en" ||
		result.Language != "en-GB" ||
		result.Latency != 420*time.Millisecond {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	provider.seen.Frames[0].PCM[0] = 'x'
	if string(fixture.Utterance.Frames[0].PCM) != "utterance" {
		t.Fatalf("benchmark passed mutable fixture audio to provider")
	}
}

func TestRunSTTBenchmarkHandlesMissingProviderAndMismatch(t *testing.T) {
	missing := RunSTTBenchmark(context.Background(), nil, BenchmarkFixture{ID: "fixture"})
	if missing.ErrorCode != "provider_unavailable" {
		t.Fatalf("unexpected missing provider result: %+v", missing)
	}

	mismatch := RunSTTBenchmark(context.Background(), &fixtureSTTBenchmarkProvider{
		result: STTResult{Text: "turn off the lights", ProviderID: "stt"},
	}, BenchmarkFixture{
		ID:                 "turn-on-lights",
		ExpectedTranscript: "turn on the lights",
		Utterance:          benchmarkUtterance(t, []byte{4}),
	})
	if mismatch.TranscriptMatched {
		t.Fatalf("expected transcript mismatch, got %+v", mismatch)
	}
}

func TestBenchmarkReportsSummarizeRunsAndProduceSafeJSON(t *testing.T) {
	expectedWake := true
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "micro wake",
		ModelID:    "okay nabu",
		Notes:      "token=secret should be redacted",
	}, []WakeBenchmarkResult{
		{
			FixtureID:    "positive",
			ExpectedWake: &expectedWake,
			Detected:     false,
			FalseReject:  true,
			Latency:      100 * time.Millisecond,
			ErrorCode:    "provider_failed",
		},
		{
			FixtureID: "ambient",
			Detected:  true,
			Latency:   200 * time.Millisecond,
		},
	}, []string{"model token=secret still pending"})

	if wakeReport.Issue != "JUT-11" ||
		wakeReport.Kind != "wake-word" ||
		wakeReport.Environment.ProviderID != "micro wake" ||
		wakeReport.Environment.Notes != "token=[redacted] should be redacted" {
		t.Fatalf("unexpected wake report metadata: %+v", wakeReport)
	}
	if wakeReport.Summary.Total != 2 ||
		wakeReport.Summary.ProviderFailures != 1 ||
		wakeReport.Summary.FalseRejects != 1 ||
		wakeReport.Summary.AverageLatency != 150*time.Millisecond {
		t.Fatalf("unexpected wake report summary: %+v", wakeReport.Summary)
	}
	rawJSON, err := wakeReport.JSON()
	if err != nil {
		t.Fatalf("marshal wake report: %v", err)
	}
	if strings.Contains(string(rawJSON), "token=secret") {
		t.Fatalf("report JSON leaked unsanitized note: %s", rawJSON)
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go whisper",
		ModelID:    "tiny.en",
	}, []STTBenchmarkResult{
		{
			FixtureID:         "lights",
			TranscriptMatched: true,
			Latency:           80 * time.Millisecond,
		},
		{
			FixtureID: "weather",
			ErrorCode: "provider_failed",
			Latency:   120 * time.Millisecond,
		},
	}, nil)
	if sttReport.Kind != "stt" ||
		sttReport.Environment.ProviderKind != "stt" ||
		sttReport.Summary.Total != 2 ||
		sttReport.Summary.TranscriptMatches != 1 ||
		sttReport.Summary.ProviderFailures != 1 ||
		sttReport.Summary.AverageLatency != 100*time.Millisecond {
		t.Fatalf("unexpected stt report: %+v", sttReport)
	}
}

func TestBenchmarkPublicReportRedactsSTTTranscriptBodies(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		Language:           "en-GB",
		ExpectedTranscript: "turn on token=secret lights",
		Transcript:         "turn on token=secret lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
		ProviderReturned:   true,
	}}, nil)

	public := report.PublicReport()
	if len(public.STTResults) != 1 {
		t.Fatalf("expected public STT result, got %+v", public)
	}
	if !public.STTResults[0].ExpectedTranscriptSet ||
		!public.STTResults[0].TranscriptReturned ||
		!public.STTResults[0].TranscriptMatched {
		t.Fatalf("unexpected public STT flags: %+v", public.STTResults[0])
	}
	raw, err := report.PublicJSON()
	if err != nil {
		t.Fatalf("marshal public report: %v", err)
	}
	for _, leaked := range []string{
		"turn on",
		"token=secret",
		"token=[redacted]",
		"expectedTranscript\"",
		"transcript\"",
	} {
		if strings.Contains(string(raw), leaked) {
			t.Fatalf("public benchmark report leaked %q:\n%s", leaked, raw)
		}
	}
	if !strings.Contains(string(raw), "expectedTranscriptSet") ||
		!strings.Contains(string(raw), "transcriptReturned") {
		t.Fatalf("public benchmark report omitted evidence flags:\n%s", raw)
	}
}

func TestDecodeBenchmarkReportRejectsUnknownFields(t *testing.T) {
	valid := []byte(`{
		"generatedAt": "2026-06-15T15:30:00Z",
		"issue": "JUT-13",
		"kind": "stt",
		"environment": {
			"providerId": "go-whisper",
			"providerKind": "stt",
			"modelId": "tiny.en"
		},
		"sttResults": [{
			"fixtureId": "short-command",
			"providerId": "go-whisper",
			"modelId": "tiny.en",
			"expectedTranscript": "turn on the lights",
			"transcript": "turn on the lights",
			"transcriptMatched": true
		}],
		"summary": {
			"total": 1,
			"providerFailures": 0,
			"transcriptMatches": 1
		}
	}`)

	if _, err := DecodeBenchmarkReport(valid); err != nil {
		t.Fatalf("expected valid report to decode: %v", err)
	}

	for name, raw := range map[string][]byte{
		"top-level raw audio": []byte(`{
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {"providerId": "go-whisper", "providerKind": "stt", "modelId": "tiny.en"},
			"summary": {"total": 0},
			"rawAudioPcm": "secret"
		}`),
		"nested provider internal": []byte(`{
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {"providerId": "go-whisper", "providerKind": "stt", "modelId": "tiny.en"},
			"sttResults": [{
				"fixtureId": "short-command",
				"expectedTranscript": "turn on the lights",
				"transcript": "turn on the lights",
				"transcriptMatched": true,
				"providerDebug": "token=secret"
			}],
			"summary": {"total": 1, "providerFailures": 0, "transcriptMatches": 1}
		}`),
		"trailing object": []byte(`{
			"issue": "JUT-13",
			"kind": "stt",
			"environment": {"providerId": "go-whisper", "providerKind": "stt", "modelId": "tiny.en"},
			"summary": {"total": 0}
		}{"rawAudioPcm":"secret"}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeBenchmarkReport(raw); err == nil {
				t.Fatalf("expected strict decode failure")
			}
		})
	}
}

func TestCompareBenchmarkReportsProducesSafeBaselineEvidence(t *testing.T) {
	expectedWake := true
	candidate := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "pmdroid-microwakeword",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         45 * time.Millisecond,
	}}, nil)
	baseline := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "wyoming-openwakeword",
		ModelID:    "hey-jute",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "wyoming-openwakeword",
		ModelID:         "hey-jute",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         70 * time.Millisecond,
	}}, nil)

	comparison := CompareBenchmarkReports(candidate, baseline)

	if comparison.Issue != "JUT-11" ||
		comparison.Kind != "wake-word" ||
		comparison.CandidateProvider != "pmdroid-microwakeword" ||
		comparison.BaselineProvider != "wyoming-openwakeword" ||
		len(comparison.SharedFixtures) != 1 ||
		comparison.SharedFixtures[0] != "positive-wake" ||
		len(comparison.Gaps) != 0 {
		t.Fatalf("unexpected comparison: %+v", comparison)
	}
	evidence := comparison.EvidenceMarkdown()
	if !strings.Contains(evidence, "Voice Benchmark Comparison: JUT-11") ||
		!strings.Contains(evidence, "Candidate wake errors: 0 false accepts, 0 false rejects") {
		t.Fatalf("unexpected comparison evidence:\n%s", evidence)
	}
	if strings.Contains(evidence, "raw pcm") || strings.Contains(evidence, "token=secret") {
		t.Fatalf("comparison evidence leaked unsafe data:\n%s", evidence)
	}
}

func TestCompareBenchmarkReportsFlagsFixtureMismatches(t *testing.T) {
	expectedWake := false
	candidate := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
	}, []WakeBenchmarkResult{{
		FixtureID:       "ambient-room",
		ExpectedWake:    &expectedWake,
		MatchesExpected: true,
	}}, nil)
	baseline := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "wyoming-openwakeword",
		ModelID:    "hey-jute",
	}, []WakeBenchmarkResult{{
		FixtureID:       "near-miss",
		ExpectedWake:    &expectedWake,
		MatchesExpected: true,
	}}, []string{"baseline token=secret note"})

	comparison := CompareBenchmarkReports(candidate, baseline)

	for _, want := range []string{
		"candidate and baseline have no shared fixture IDs",
		"candidate has fixtures missing from baseline",
		"baseline has fixtures missing from candidate",
		"baseline token=[redacted] note",
	} {
		if !hasProblem(comparison.Gaps, want) {
			t.Fatalf("expected comparison gap containing %q, got %+v", want, comparison.Gaps)
		}
	}
}

func TestCompareBenchmarkReportsFlagsSameProviderComparison(t *testing.T) {
	expectedWake := true
	candidate := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "pmdroid-microwakeword",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         45 * time.Millisecond,
	}}, nil)
	baseline := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu-alt",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "pmdroid-microwakeword",
		ModelID:         "okay-nabu-alt",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         50 * time.Millisecond,
	}}, nil)

	comparison := CompareBenchmarkReports(candidate, baseline)

	if !hasProblem(comparison.Gaps, "candidate and baseline providers must differ") {
		t.Fatalf("expected same-provider comparison gap, got %+v", comparison.Gaps)
	}
	evidence := comparison.EvidenceMarkdown()
	if !strings.Contains(evidence, "candidate and baseline providers must differ") {
		t.Fatalf("expected evidence to include same-provider gap:\n%s", evidence)
	}
}

func TestCompareBenchmarkReportsRequiresJUT11ProviderRoles(t *testing.T) {
	expectedWake := true
	candidate := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "wyoming-openwakeword",
		ModelID:    "hey-jute",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "wyoming-openwakeword",
		ModelID:         "hey-jute",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         45 * time.Millisecond,
	}}, nil)
	baseline := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "wyoming-porcupine",
		ModelID:    "hey-jute",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "wyoming-porcupine",
		ModelID:         "hey-jute",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         50 * time.Millisecond,
	}}, nil)

	comparison := CompareBenchmarkReports(candidate, baseline)

	for _, want := range []string{
		"JUT-11 candidate provider must be pmdroid/microWakeWord",
		"JUT-11 baseline provider must be openWakeWord/Wyoming",
	} {
		if !hasProblem(comparison.Gaps, want) {
			t.Fatalf("expected JUT-11 provider role gap %q, got %+v", want, comparison.Gaps)
		}
		if !strings.Contains(comparison.EvidenceMarkdown(), want) {
			t.Fatalf(
				"expected evidence to include JUT-11 provider role gap %q:\n%s",
				want,
				comparison.EvidenceMarkdown(),
			)
		}
	}
}

func TestCompareBenchmarkReportsRequiresJUT11CandidateToBePmdroidMicroWakeWord(t *testing.T) {
	expectedWake := true
	candidate := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "ohf-microwakeword",
		ModelID:    "okay-nabu",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "ohf-microwakeword",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         45 * time.Millisecond,
	}}, nil)
	baseline := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "wyoming-openwakeword",
		ModelID:    "hey-jute",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "wyoming-openwakeword",
		ModelID:         "hey-jute",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         50 * time.Millisecond,
	}}, nil)

	comparison := CompareBenchmarkReports(candidate, baseline)

	if !hasProblem(comparison.Gaps, "JUT-11 candidate provider must be pmdroid/microWakeWord") {
		t.Fatalf("expected pmdroid candidate role gap, got %+v", comparison.Gaps)
	}
	if hasProblem(comparison.Gaps, "JUT-11 baseline provider must be openWakeWord/Wyoming") {
		t.Fatalf("did not expect baseline provider role gap, got %+v", comparison.Gaps)
	}
}

func TestBenchmarkSuitesRunFixturesIntoReports(t *testing.T) {
	expectedWake := false
	wakeProvider := &fixtureWakeBenchmarkProvider{
		detection: WakeBenchmarkDetection{
			Detected:   true,
			ProviderID: "wyoming-openwakeword",
			ModelID:    "hey-jute",
		},
	}
	wakeFixtures := []BenchmarkFixture{
		{
			ID:         "ambient",
			ExpectWake: &expectedWake,
			Utterance:  benchmarkUtterance(t, []byte("ambient pcm")),
		},
	}
	wakeReport := RunWakeBenchmarkSuite(context.Background(), WakeBenchmarkSuite{
		Issue:    "JUT-11",
		Provider: wakeProvider,
		Env:      BenchmarkEnvironment{ProviderID: "wyoming-openwakeword"},
		Fixtures: wakeFixtures,
		Gaps:     []string{"real pmdroid run pending"},
	})
	if wakeReport.Issue != "JUT-11" ||
		wakeReport.Kind != "wake-word" ||
		wakeReport.Summary.Total != 1 ||
		wakeReport.Summary.FalseAccepts != 1 ||
		len(wakeReport.WakeResults) != 1 ||
		wakeReport.Gaps[0] != "real pmdroid run pending" {
		t.Fatalf("unexpected wake suite report: %+v", wakeReport)
	}
	wakeProvider.seen.Frames[0].PCM[0] = 'x'
	if string(wakeFixtures[0].Utterance.Frames[0].PCM) != "ambient pcm" {
		t.Fatalf("wake suite passed mutable fixture audio to provider")
	}

	sttProvider := &fixtureSTTBenchmarkProvider{
		result: STTResult{
			Text:       "turn on the lights",
			ProviderID: "wyoming-stt",
			ModelID:    "small-en",
		},
	}
	sttFixtures := []BenchmarkFixture{
		{
			ID:                 "lights",
			ExpectedTranscript: "turn on the lights",
			Utterance:          benchmarkUtterance(t, []byte("speech pcm")),
		},
	}
	sttReport := RunSTTBenchmarkSuite(context.Background(), STTBenchmarkSuite{
		Issue:    "JUT-13",
		Provider: sttProvider,
		Env:      BenchmarkEnvironment{ProviderID: "wyoming-stt"},
		Fixtures: sttFixtures,
		Gaps:     []string{"real go-whisper run pending"},
	})
	if sttReport.Issue != "JUT-13" ||
		sttReport.Kind != "stt" ||
		sttReport.Summary.Total != 1 ||
		sttReport.Summary.TranscriptMatches != 1 ||
		len(sttReport.STTResults) != 1 ||
		sttReport.Gaps[0] != "real go-whisper run pending" {
		t.Fatalf("unexpected stt suite report: %+v", sttReport)
	}
	sttProvider.seen.Frames[0].PCM[0] = 'x'
	if string(sttFixtures[0].Utterance.Frames[0].PCM) != "speech pcm" {
		t.Fatalf("stt suite passed mutable fixture audio to provider")
	}
}

func TestValidateBenchmarkReportAcceptsCompleteWakeAndSTTRuns(t *testing.T) {
	expectedWake := true
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
		ModelHash:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "pmdroid-microwakeword",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         30 * time.Millisecond,
		ResourceSample:  BenchmarkResourceSample{Duration: 30 * time.Millisecond},
	}}, nil)
	if problems := ValidateBenchmarkReport(wakeReport, BenchmarkReportExpectations{
		Issue:                 "JUT-11",
		Kind:                  "wake-word",
		MinResults:            1,
		RequireModelHash:      true,
		RequireAllWakeMatches: true,
	}); len(problems) != 0 {
		t.Fatalf("expected complete wake report, got %v", problems)
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{{
		FixtureID:          "lights",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
		ResourceSample:     BenchmarkResourceSample{Duration: 80 * time.Millisecond},
	}}, nil)
	if problems := ValidateBenchmarkReport(sttReport, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
	}); len(problems) != 0 {
		t.Fatalf("expected complete stt report, got %v", problems)
	}
}

func TestBenchmarkAcceptanceExpectationsProvideIssueDefaults(t *testing.T) {
	wake, ok := BenchmarkAcceptanceExpectations("JUT-11")
	if !ok {
		t.Fatalf("expected JUT-11 acceptance preset")
	}
	if wake.Issue != "JUT-11" ||
		wake.Kind != "wake-word" ||
		wake.MinResults != 4 ||
		len(wake.RequiredFixtureIDs) != 4 ||
		wake.RequiredFixtureIDs[0] != "positive-wake" ||
		wake.RequiredFixtureIDs[3] != "conversation-long" ||
		wake.RequiredProviderIDContains != "pmdroid" ||
		!wake.RequireModelHash ||
		!wake.RequireAllWakeMatches ||
		!wake.RequireResourceSamples ||
		wake.RequireAllTranscriptMatches {
		t.Fatalf("unexpected wake preset: %+v", wake)
	}

	stt, ok := BenchmarkAcceptanceExpectations("JUT-13")
	if !ok {
		t.Fatalf("expected JUT-13 acceptance preset")
	}
	if stt.Issue != "JUT-13" ||
		stt.Kind != "stt" ||
		stt.MinResults != 1 ||
		len(stt.RequiredFixtureIDs) != 1 ||
		stt.RequiredFixtureIDs[0] != "short-command" ||
		stt.RequiredProviderIDContains != "go-whisper" ||
		!stt.RequireModelHash ||
		stt.RequireAllWakeMatches ||
		!stt.RequireAllTranscriptMatches ||
		!stt.RequireResourceSamples {
		t.Fatalf("unexpected stt preset: %+v", stt)
	}

	if _, ok := BenchmarkAcceptanceExpectations("JUT-6"); ok {
		t.Fatalf("did not expect browser spike benchmark preset")
	}
}

func TestValidateBenchmarkReportRequiresJUT11PmdroidProvider(t *testing.T) {
	wake := true
	report := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID:   "wyoming-openwakeword",
		ProviderKind: "wake-word",
		ModelID:      "okay-nabu",
		ModelHash:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, []WakeBenchmarkResult{{
		FixtureID:        "positive-wake",
		ProviderID:       "wyoming-openwakeword",
		ModelID:          "okay-nabu",
		ExpectedWake:     &wake,
		Detected:         true,
		MatchesExpected:  true,
		Latency:          45 * time.Millisecond,
		ResourceSample:   BenchmarkResourceSample{Duration: 45 * time.Millisecond},
		ProviderReturned: true,
	}}, nil)
	expectations, ok := BenchmarkAcceptanceExpectations("JUT-11")
	if !ok {
		t.Fatalf("expected JUT-11 acceptance preset")
	}

	problems := ValidateBenchmarkReport(report, expectations)

	if !hasProblem(problems, "environment.providerId must identify pmdroid") {
		t.Fatalf("expected pmdroid provider identity problem, got %v", problems)
	}
}

func TestValidateBenchmarkReportRequiresJUT13GoWhisperProvider(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID:   "wyoming-stt",
		ProviderKind: "stt",
		ModelID:      "tiny.en",
		ModelHash:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "wyoming-stt",
		ModelID:            "tiny.en",
		Language:           "en",
		ExpectedTranscript: "turn on the kitchen lights",
		Transcript:         "turn on the kitchen lights",
		TranscriptMatched:  true,
		Latency:            350 * time.Millisecond,
		ResourceSample:     BenchmarkResourceSample{Duration: 350 * time.Millisecond},
	}}, nil)
	expectations, ok := BenchmarkAcceptanceExpectations("JUT-13")
	if !ok {
		t.Fatalf("expected JUT-13 acceptance preset")
	}

	problems := ValidateBenchmarkReport(report, expectations)

	if !hasProblem(problems, "environment.providerId must identify go-whisper") {
		t.Fatalf("expected go-whisper provider identity problem, got %v", problems)
	}
}

func TestValidateBenchmarkReportRejectsProviderKindMismatch(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID:   "go-whisper",
		ProviderKind: "wake-word",
		ModelID:      "tiny.en",
		ModelHash:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		Language:           "en",
		ExpectedTranscript: "turn on the kitchen lights",
		Transcript:         "turn on the kitchen lights",
		TranscriptMatched:  true,
		Latency:            350 * time.Millisecond,
		ResourceSample:     BenchmarkResourceSample{Duration: 350 * time.Millisecond},
	}}, nil)
	report.Environment.ProviderKind = "wake-word"
	expectations, ok := BenchmarkAcceptanceExpectations("JUT-13")
	if !ok {
		t.Fatalf("expected JUT-13 acceptance preset")
	}

	problems := ValidateBenchmarkReport(report, expectations)

	if !hasProblem(problems, "environment.providerKind does not match benchmark kind") {
		t.Fatalf("expected provider kind mismatch problem, got %v", problems)
	}
}

func TestValidateBenchmarkReportFlagsIncompleteEvidence(t *testing.T) {
	expectedWake := false
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "micro",
		ModelID:    "okay-nabu",
	}, []WakeBenchmarkResult{{
		FixtureID:    "ambient",
		ExpectedWake: &expectedWake,
		Detected:     true,
		FalseAccept:  true,
		Latency:      10 * time.Millisecond,
		ErrorCode:    "provider_failed",
	}}, []string{"openWakeWord baseline pending"})
	wakeProblems := ValidateBenchmarkReport(wakeReport, BenchmarkReportExpectations{
		Issue:                 "JUT-11",
		Kind:                  "wake-word",
		MinResults:            2,
		RequiredFixtureIDs:    []string{"positive-wake", "near-miss", "ambient-room", "conversation-long"},
		RequireModelHash:      true,
		RequireAllWakeMatches: true,
	})
	for _, want := range []string{
		"environment.modelHash is required",
		"benchmark report has unresolved gaps",
		"benchmark report has provider failures",
		"benchmark report has too few results",
		"benchmark report is missing required fixture positive-wake",
		"benchmark report is missing required fixture near-miss",
		"benchmark report is missing required fixture conversation-long",
		"wake result did not match expected detection",
	} {
		if !hasProblem(wakeProblems, want) {
			t.Fatalf("expected wake validation problem %q, got %v", want, wakeProblems)
		}
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
	}, []STTBenchmarkResult{{
		FixtureID:          "lights",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn off the lights",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		Latency:            70 * time.Millisecond,
	}}, nil)
	sttProblems := ValidateBenchmarkReport(sttReport, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		RequiredFixtureIDs:          []string{"short-command"},
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
	})
	for _, want := range []string{
		"environment.modelHash is required",
		"benchmark report is missing required fixture short-command",
		"stt result transcript did not match expected text",
	} {
		if !hasProblem(sttProblems, want) {
			t.Fatalf("expected stt validation problem %q, got %v", want, sttProblems)
		}
	}
}

func TestValidateBenchmarkReportRejectsInvalidModelHash(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:not-a-real-hash",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
	}}, nil)

	problems := ValidateBenchmarkReport(report, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
	})

	if !hasProblem(problems, "environment.modelHash must be sha256:<64 hex characters>") {
		t.Fatalf("expected invalid model hash problem, got %v", problems)
	}
}

func TestValidateBenchmarkReportRequiresGeneratedAt(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
		ResourceSample:     BenchmarkResourceSample{Duration: 80 * time.Millisecond},
		ProviderReturned:   true,
	}}, nil)
	report.GeneratedAt = "replace-with-generated-at"

	problems := ValidateBenchmarkReport(report, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		RequiredFixtureIDs:          []string{"short-command"},
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
		RequireResourceSamples:      true,
	})

	if !hasProblem(problems, "generatedAt must be RFC3339") {
		t.Fatalf("expected generatedAt validation problem, got %v", problems)
	}
}

func TestValidateBenchmarkReportRejectsMissingResourceSamples(t *testing.T) {
	expectedWake := true
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
		ModelHash:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "pmdroid-microwakeword",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         30 * time.Millisecond,
	}}, nil)

	wakeProblems := ValidateBenchmarkReport(wakeReport, BenchmarkReportExpectations{
		Issue:                  "JUT-11",
		Kind:                   "wake-word",
		MinResults:             1,
		RequireModelHash:       true,
		RequireAllWakeMatches:  true,
		RequireResourceSamples: true,
	})
	if !hasProblem(wakeProblems, "wake result resourceSample.duration is required") {
		t.Fatalf("expected missing wake resource sample problem, got %v", wakeProblems)
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
	}}, nil)

	sttProblems := ValidateBenchmarkReport(sttReport, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
		RequireResourceSamples:      true,
	})
	if !hasProblem(sttProblems, "stt result resourceSample.duration is required") {
		t.Fatalf("expected missing stt resource sample problem, got %v", sttProblems)
	}
}

func TestValidateBenchmarkReportRejectsResultIdentityMismatch(t *testing.T) {
	expectedWake := true
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
		ModelHash:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, []WakeBenchmarkResult{{
		FixtureID:       "positive-wake",
		ProviderID:      "wyoming-openwakeword",
		ModelID:         "hey-jute",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		MatchesExpected: true,
		Latency:         30 * time.Millisecond,
	}}, nil)

	wakeProblems := ValidateBenchmarkReport(wakeReport, BenchmarkReportExpectations{
		Issue:                 "JUT-11",
		Kind:                  "wake-word",
		MinResults:            1,
		RequireModelHash:      true,
		RequireAllWakeMatches: true,
	})
	for _, want := range []string{
		"wake result providerId does not match environment.providerId",
		"wake result modelId does not match environment.modelId",
	} {
		if !hasProblem(wakeProblems, want) {
			t.Fatalf("expected wake validation problem %q, got %v", want, wakeProblems)
		}
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "wyoming-stt",
		ModelID:            "small-en",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
	}}, nil)

	sttProblems := ValidateBenchmarkReport(sttReport, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
	})
	for _, want := range []string{
		"stt result providerId does not match environment.providerId",
		"stt result modelId does not match environment.modelId",
	} {
		if !hasProblem(sttProblems, want) {
			t.Fatalf("expected stt validation problem %q, got %v", want, sttProblems)
		}
	}
}

func TestValidateBenchmarkReportRejectsMatchedSTTWithoutReturnedTranscript(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		ExpectedTranscript: "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
		ResourceSample:     BenchmarkResourceSample{Duration: 80 * time.Millisecond},
	}}, nil)

	problems := ValidateBenchmarkReport(report, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
		RequireResourceSamples:      true,
	})

	if !hasProblem(problems, "stt result transcript is required when transcriptMatched is true") {
		t.Fatalf("expected missing returned transcript problem, got %v", problems)
	}
}

func TestValidateBenchmarkReportRejectsTamperedSummaries(t *testing.T) {
	expectedWake := false
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
		ModelHash:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, []WakeBenchmarkResult{{
		FixtureID:       "ambient-room",
		ProviderID:      "pmdroid-microwakeword",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		FalseAccept:     true,
		MatchesExpected: false,
		Latency:         50 * time.Millisecond,
		ErrorCode:       "provider_failed",
	}}, nil)
	wakeReport.Summary.ProviderFailures = 0
	wakeReport.Summary.FalseAccepts = 0

	wakeProblems := ValidateBenchmarkReport(wakeReport, BenchmarkReportExpectations{
		Issue:                 "JUT-11",
		Kind:                  "wake-word",
		MinResults:            1,
		RequireModelHash:      true,
		AllowProviderFailures: true,
		RequireAllWakeMatches: false,
	})
	if !hasProblem(wakeProblems, "wake summary does not match result details") {
		t.Fatalf("expected tampered wake summary problem, got %v", wakeProblems)
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{{
		FixtureID:          "short-command",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		ExpectedTranscript: "turn on the lights",
		Transcript:         "turn on the lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
		ErrorCode:          "provider_failed",
	}}, nil)
	sttReport.Summary.ProviderFailures = 0
	sttReport.Summary.TranscriptMatches = 0

	sttProblems := ValidateBenchmarkReport(sttReport, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		AllowProviderFailures:       true,
		RequireAllTranscriptMatches: true,
	})
	if !hasProblem(sttProblems, "stt summary does not match result details") {
		t.Fatalf("expected tampered stt summary problem, got %v", sttProblems)
	}
}

func TestValidateBenchmarkReportRejectsDuplicateResultFixtures(t *testing.T) {
	expectedWake := true
	wakeReport := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "pmdroid-microwakeword",
		ModelID:    "okay-nabu",
		ModelHash:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, []WakeBenchmarkResult{
		{
			FixtureID:       "positive-wake",
			ProviderID:      "pmdroid-microwakeword",
			ModelID:         "okay-nabu",
			ExpectedWake:    &expectedWake,
			Detected:        true,
			MatchesExpected: true,
			Latency:         40 * time.Millisecond,
		},
		{
			FixtureID:       "positive-wake",
			ProviderID:      "pmdroid-microwakeword",
			ModelID:         "okay-nabu",
			ExpectedWake:    &expectedWake,
			Detected:        true,
			MatchesExpected: true,
			Latency:         40 * time.Millisecond,
		},
	}, nil)

	wakeProblems := ValidateBenchmarkReport(wakeReport, BenchmarkReportExpectations{
		Issue:                 "JUT-11",
		Kind:                  "wake-word",
		MinResults:            1,
		RequireModelHash:      true,
		RequireAllWakeMatches: true,
	})
	if !hasProblem(wakeProblems, "wake result has duplicate fixtureId positive-wake") {
		t.Fatalf("expected duplicate wake fixture problem, got %v", wakeProblems)
	}

	sttReport := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}, []STTBenchmarkResult{
		{
			FixtureID:          "short-command",
			ProviderID:         "go-whisper",
			ModelID:            "tiny.en",
			ExpectedTranscript: "turn on the lights",
			Transcript:         "turn on the lights",
			TranscriptMatched:  true,
			Latency:            80 * time.Millisecond,
		},
		{
			FixtureID:          "short-command",
			ProviderID:         "go-whisper",
			ModelID:            "tiny.en",
			ExpectedTranscript: "turn on the lights",
			Transcript:         "turn on the lights",
			TranscriptMatched:  true,
			Latency:            80 * time.Millisecond,
		},
	}, nil)

	sttProblems := ValidateBenchmarkReport(sttReport, BenchmarkReportExpectations{
		Issue:                       "JUT-13",
		Kind:                        "stt",
		MinResults:                  1,
		RequireModelHash:            true,
		RequireAllTranscriptMatches: true,
	})
	if !hasProblem(sttProblems, "stt result has duplicate fixtureId short-command") {
		t.Fatalf("expected duplicate stt fixture problem, got %v", sttProblems)
	}
}

func TestBenchmarkReportEvidenceMarkdownSummarizesWithoutRawTranscripts(t *testing.T) {
	report := NewSTTBenchmarkReport("JUT-13", BenchmarkEnvironment{
		ProviderID: "go-whisper",
		ModelID:    "tiny.en",
		ModelHash:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Notes:      "token=secret note",
	}, []STTBenchmarkResult{{
		FixtureID:          "lights",
		ProviderID:         "go-whisper",
		ModelID:            "tiny.en",
		Language:           "en-GB",
		ExpectedTranscript: "turn on token=secret lights",
		Transcript:         "turn on token=secret lights",
		TranscriptMatched:  true,
		Latency:            80 * time.Millisecond,
	}}, []string{"real device token=secret pending"})

	markdown := report.EvidenceMarkdown([]string{"model token=secret missing"})

	for _, want := range []string{
		"Voice Benchmark Evidence: JUT-13",
		"Provider: `go-whisper`",
		"Model hash: `sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`",
		"STT summary: 1 transcript matches",
		"`lights`: matched",
		"real device token=[redacted] pending",
		"model token=[redacted] missing",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected evidence markdown to contain %q:\n%s", want, markdown)
		}
	}
	for _, leaked := range []string{
		"turn on token=secret lights",
		"token=secret",
	} {
		if strings.Contains(markdown, leaked) {
			t.Fatalf("evidence markdown leaked %q:\n%s", leaked, markdown)
		}
	}
}

func TestBenchmarkReportEvidenceMarkdownSummarizesWakeFixtures(t *testing.T) {
	expectedWake := false
	report := NewWakeBenchmarkReport("JUT-11", BenchmarkEnvironment{
		ProviderID: "micro",
		ModelID:    "okay-nabu",
		ModelHash:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, []WakeBenchmarkResult{{
		FixtureID:       "ambient",
		ProviderID:      "micro",
		ModelID:         "okay-nabu",
		ExpectedWake:    &expectedWake,
		Detected:        true,
		FalseAccept:     true,
		MatchesExpected: false,
		Latency:         20 * time.Millisecond,
	}}, nil)

	markdown := report.EvidenceMarkdown(nil)

	for _, want := range []string{
		"Voice Benchmark Evidence: JUT-11",
		"Wake summary: 1 false accepts, 0 false rejects",
		"`ambient`: mismatched, detected=true",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("expected wake evidence markdown to contain %q:\n%s", want, markdown)
		}
	}
}

func benchmarkUtterance(t *testing.T, pcm []byte) CapturedUtterance {
	t.Helper()
	start := time.Date(2026, 6, 15, 15, 0, 0, 0, time.UTC)
	return CapturedUtterance{
		StartedAt:  start,
		EndedAt:    start.Add(100 * time.Millisecond),
		SampleRate: 16000,
		Channels:   1,
		Frames: []AudioFrame{
			{
				PCM:         append([]byte(nil), pcm...),
				SampleRate:  16000,
				SampleWidth: 2,
				Channels:    1,
				Timestamp:   start,
				Duration:    100 * time.Millisecond,
			},
		},
	}
}
