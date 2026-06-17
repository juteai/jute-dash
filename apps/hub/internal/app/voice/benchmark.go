package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"time"
)

type BenchmarkFixture struct {
	ID                 string
	Description        string
	Utterance          CapturedUtterance
	ExpectWake         *bool
	ExpectedTranscript string
	Language           string
}

type BenchmarkResourceSample struct {
	AllocBytes      uint64        `json:"allocBytes"`
	TotalAllocBytes uint64        `json:"totalAllocBytes"`
	Duration        time.Duration `json:"duration"`
}

type BenchmarkEnvironment struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	GoVersion    string `json:"goVersion"`
	ProviderID   string `json:"providerId"`
	ProviderKind string `json:"providerKind"`
	ModelID      string `json:"modelId,omitempty"`
	ModelHash    string `json:"modelHash,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type BenchmarkReport struct {
	GeneratedAt string                 `json:"generatedAt"`
	Issue       string                 `json:"issue"`
	Kind        string                 `json:"kind"`
	Environment BenchmarkEnvironment   `json:"environment"`
	WakeResults []WakeBenchmarkResult  `json:"wakeResults,omitempty"`
	STTResults  []STTBenchmarkResult   `json:"sttResults,omitempty"`
	Summary     BenchmarkReportSummary `json:"summary"`
	Gaps        []string               `json:"gaps,omitempty"`
}

type PublicBenchmarkReport struct {
	GeneratedAt string                     `json:"generatedAt"`
	Issue       string                     `json:"issue"`
	Kind        string                     `json:"kind"`
	Environment BenchmarkEnvironment       `json:"environment"`
	WakeResults []WakeBenchmarkResult      `json:"wakeResults,omitempty"`
	STTResults  []PublicSTTBenchmarkResult `json:"sttResults,omitempty"`
	Summary     BenchmarkReportSummary     `json:"summary"`
	Gaps        []string                   `json:"gaps,omitempty"`
}

type PublicSTTBenchmarkResult struct {
	FixtureID             string                  `json:"fixtureId"`
	ProviderID            string                  `json:"providerId"`
	ModelID               string                  `json:"modelId,omitempty"`
	Language              string                  `json:"language,omitempty"`
	ExpectedTranscriptSet bool                    `json:"expectedTranscriptSet"`
	TranscriptReturned    bool                    `json:"transcriptReturned"`
	TranscriptMatched     bool                    `json:"transcriptMatched"`
	Latency               time.Duration           `json:"latency"`
	ResourceSample        BenchmarkResourceSample `json:"resourceSample"`
	ErrorCode             string                  `json:"errorCode,omitempty"`
	ProviderReturned      bool                    `json:"providerReturned"`
}

type BenchmarkReportSummary struct {
	Total             int           `json:"total"`
	ProviderFailures  int           `json:"providerFailures"`
	FalseAccepts      int           `json:"falseAccepts,omitempty"`
	FalseRejects      int           `json:"falseRejects,omitempty"`
	TranscriptMatches int           `json:"transcriptMatches,omitempty"`
	AverageLatency    time.Duration `json:"averageLatency"`
}

type BenchmarkComparisonReport struct {
	GeneratedAt       string                 `json:"generatedAt"`
	Issue             string                 `json:"issue"`
	Kind              string                 `json:"kind"`
	CandidateProvider string                 `json:"candidateProvider"`
	BaselineProvider  string                 `json:"baselineProvider"`
	SharedFixtures    []string               `json:"sharedFixtures"`
	CandidateOnly     []string               `json:"candidateOnly,omitempty"`
	BaselineOnly      []string               `json:"baselineOnly,omitempty"`
	CandidateSummary  BenchmarkReportSummary `json:"candidateSummary"`
	BaselineSummary   BenchmarkReportSummary `json:"baselineSummary"`
	Gaps              []string               `json:"gaps,omitempty"`
}

type BenchmarkReportExpectations struct {
	Issue                       string
	Kind                        string
	MinResults                  int
	RequiredFixtureIDs          []string
	RequiredProviderIDContains  string
	RequireModelHash            bool
	AllowGaps                   bool
	AllowProviderFailures       bool
	RequireAllWakeMatches       bool
	RequireAllTranscriptMatches bool
	RequireResourceSamples      bool
}

func BenchmarkAcceptanceExpectations(issue string) (BenchmarkReportExpectations, bool) {
	switch safeIdentifier(issue) {
	case "JUT-11":
		return BenchmarkReportExpectations{
			Issue:                  "JUT-11",
			Kind:                   "wake-word",
			MinResults:             4,
			RequiredFixtureIDs:     []string{"positive-wake", "near-miss", "ambient-room", "conversation-long"},
			RequireModelHash:       true,
			RequireAllWakeMatches:  true,
			RequireResourceSamples: true,
		}, true
	case "JUT-13":
		return BenchmarkReportExpectations{
			Issue:                       "JUT-13",
			Kind:                        "stt",
			MinResults:                  1,
			RequiredFixtureIDs:          []string{"short-command"},
			RequiredProviderIDContains:  "go-whisper",
			RequireModelHash:            true,
			RequireAllTranscriptMatches: true,
			RequireResourceSamples:      true,
		}, true
	default:
		return BenchmarkReportExpectations{}, false
	}
}

func DecodeBenchmarkReport(raw []byte) (BenchmarkReport, error) {
	var report BenchmarkReport
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return BenchmarkReport{}, fmt.Errorf("decode benchmark report: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BenchmarkReport{}, errors.New("decode benchmark report: trailing JSON data")
	}
	return report, nil
}

type WakeBenchmarkProvider interface {
	DetectWake(ctx context.Context, utterance CapturedUtterance) (WakeBenchmarkDetection, error)
}

type WakeBenchmarkDetection struct {
	Detected          bool          `json:"detected"`
	DetectedAt        time.Duration `json:"detectedAt,omitempty"`
	ProviderID        string        `json:"providerId"`
	ModelID           string        `json:"modelId,omitempty"`
	Confidence        float64       `json:"confidence,omitempty"`
	ActivationLatency time.Duration `json:"activationLatency,omitempty"`
}

type WakeBenchmarkResult struct {
	FixtureID        string                  `json:"fixtureId"`
	ProviderID       string                  `json:"providerId"`
	ModelID          string                  `json:"modelId,omitempty"`
	ExpectedWake     *bool                   `json:"expectedWake,omitempty"`
	Detected         bool                    `json:"detected"`
	DetectedAt       time.Duration           `json:"detectedAt,omitempty"`
	Confidence       float64                 `json:"confidence,omitempty"`
	Latency          time.Duration           `json:"latency"`
	ResourceSample   BenchmarkResourceSample `json:"resourceSample"`
	FalseAccept      bool                    `json:"falseAccept"`
	FalseReject      bool                    `json:"falseReject"`
	MatchesExpected  bool                    `json:"matchesExpected"`
	ErrorCode        string                  `json:"errorCode,omitempty"`
	ProviderReturned bool                    `json:"providerReturned"`
}

type STTBenchmarkProvider interface {
	Transcribe(ctx context.Context, utterance CapturedUtterance) (STTResult, error)
}

type STTBenchmarkResult struct {
	FixtureID          string                  `json:"fixtureId"`
	ProviderID         string                  `json:"providerId"`
	ModelID            string                  `json:"modelId,omitempty"`
	Language           string                  `json:"language,omitempty"`
	ExpectedTranscript string                  `json:"expectedTranscript,omitempty"`
	Transcript         string                  `json:"transcript,omitempty"`
	TranscriptMatched  bool                    `json:"transcriptMatched"`
	Latency            time.Duration           `json:"latency"`
	ResourceSample     BenchmarkResourceSample `json:"resourceSample"`
	ErrorCode          string                  `json:"errorCode,omitempty"`
	ProviderReturned   bool                    `json:"providerReturned"`
}

type WakeBenchmarkSuite struct {
	Issue    string
	Provider WakeBenchmarkProvider
	Env      BenchmarkEnvironment
	Fixtures []BenchmarkFixture
	Gaps     []string
}

type STTBenchmarkSuite struct {
	Issue    string
	Provider STTBenchmarkProvider
	Env      BenchmarkEnvironment
	Fixtures []BenchmarkFixture
	Gaps     []string
}

func RunWakeBenchmark(
	ctx context.Context,
	provider WakeBenchmarkProvider,
	fixture BenchmarkFixture,
) WakeBenchmarkResult {
	result := WakeBenchmarkResult{
		FixtureID:       safeFixtureID(fixture.ID),
		ExpectedWake:    fixture.ExpectWake,
		MatchesExpected: fixture.ExpectWake == nil,
	}
	if provider == nil {
		result.ErrorCode = "provider_unavailable"
		return result
	}
	sample, err := measureBenchmark(func() error {
		detection, detectErr := provider.DetectWake(ctx, cloneCapturedUtterance(fixture.Utterance))
		if detectErr != nil {
			return detectErr
		}
		result.ProviderReturned = true
		result.ProviderID = safeIdentifier(detection.ProviderID)
		result.ModelID = safeIdentifier(detection.ModelID)
		result.Detected = detection.Detected
		result.DetectedAt = detection.DetectedAt
		result.Confidence = detection.Confidence
		if detection.ActivationLatency > 0 {
			result.Latency = detection.ActivationLatency
		}
		if fixture.ExpectWake != nil {
			result.FalseAccept = !*fixture.ExpectWake && detection.Detected
			result.FalseReject = *fixture.ExpectWake && !detection.Detected
			result.MatchesExpected = *fixture.ExpectWake == detection.Detected
		}
		return nil
	})
	if result.Latency == 0 {
		result.Latency = sample.Duration
	}
	result.ResourceSample = sample
	if err != nil {
		result.ErrorCode = benchmarkErrorCode(err)
	}
	return result
}

func RunWakeBenchmarkSuite(ctx context.Context, suite WakeBenchmarkSuite) BenchmarkReport {
	results := make([]WakeBenchmarkResult, 0, len(suite.Fixtures))
	for _, fixture := range suite.Fixtures {
		results = append(results, RunWakeBenchmark(ctx, suite.Provider, fixture))
	}
	return NewWakeBenchmarkReport(suite.Issue, suite.Env, results, suite.Gaps)
}

func RunSTTBenchmark(
	ctx context.Context,
	provider STTBenchmarkProvider,
	fixture BenchmarkFixture,
) STTBenchmarkResult {
	result := STTBenchmarkResult{
		FixtureID:          safeFixtureID(fixture.ID),
		ExpectedTranscript: sanitizeText(fixture.ExpectedTranscript),
	}
	if provider == nil {
		result.ErrorCode = "provider_unavailable"
		return result
	}
	sample, err := measureBenchmark(func() error {
		transcript, transcribeErr := provider.Transcribe(ctx, cloneCapturedUtterance(fixture.Utterance))
		if transcribeErr != nil {
			return transcribeErr
		}
		result.ProviderReturned = true
		result.ProviderID = safeIdentifier(transcript.ProviderID)
		result.ModelID = safeIdentifier(transcript.ModelID)
		result.Language = safeIdentifier(transcript.Language)
		result.Transcript = sanitizeText(transcript.Text)
		result.TranscriptMatched = transcriptMatches(fixture.ExpectedTranscript, transcript.Text)
		if transcript.Duration > 0 {
			result.Latency = transcript.Duration
		}
		return nil
	})
	if result.Latency == 0 {
		result.Latency = sample.Duration
	}
	result.ResourceSample = sample
	if err != nil {
		result.ErrorCode = benchmarkErrorCode(err)
	}
	return result
}

func RunSTTBenchmarkSuite(ctx context.Context, suite STTBenchmarkSuite) BenchmarkReport {
	results := make([]STTBenchmarkResult, 0, len(suite.Fixtures))
	for _, fixture := range suite.Fixtures {
		results = append(results, RunSTTBenchmark(ctx, suite.Provider, fixture))
	}
	return NewSTTBenchmarkReport(suite.Issue, suite.Env, results, suite.Gaps)
}

func NewWakeBenchmarkReport(
	issue string,
	env BenchmarkEnvironment,
	results []WakeBenchmarkResult,
	gaps []string,
) BenchmarkReport {
	env = normalizeBenchmarkEnvironment(env, "wake-word")
	return BenchmarkReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Issue:       safeIdentifier(issue),
		Kind:        "wake-word",
		Environment: env,
		WakeResults: append([]WakeBenchmarkResult(nil), results...),
		Summary:     summarizeWakeBenchmark(results),
		Gaps:        sanitizeBenchmarkGaps(gaps),
	}
}

func NewSTTBenchmarkReport(
	issue string,
	env BenchmarkEnvironment,
	results []STTBenchmarkResult,
	gaps []string,
) BenchmarkReport {
	env = normalizeBenchmarkEnvironment(env, "stt")
	return BenchmarkReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Issue:       safeIdentifier(issue),
		Kind:        "stt",
		Environment: env,
		STTResults:  append([]STTBenchmarkResult(nil), results...),
		Summary:     summarizeSTTBenchmark(results),
		Gaps:        sanitizeBenchmarkGaps(gaps),
	}
}

func (r BenchmarkReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func (r BenchmarkReport) PublicReport() PublicBenchmarkReport {
	return PublicBenchmarkReport{
		GeneratedAt: r.GeneratedAt,
		Issue:       safeIdentifier(r.Issue),
		Kind:        safeIdentifier(r.Kind),
		Environment: normalizeBenchmarkEnvironment(r.Environment, r.Kind),
		WakeResults: append([]WakeBenchmarkResult(nil), r.WakeResults...),
		STTResults:  publicSTTBenchmarkResults(r.STTResults),
		Summary:     r.Summary,
		Gaps:        sanitizeBenchmarkGaps(r.Gaps),
	}
}

func (r BenchmarkReport) PublicJSON() ([]byte, error) {
	return json.MarshalIndent(r.PublicReport(), "", "  ")
}

func (r BenchmarkReport) EvidenceMarkdown(validationProblems []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Voice Benchmark Evidence: %s\n\n", safeIdentifier(r.Issue))
	fmt.Fprintf(&b, "- Kind: `%s`\n", safeIdentifier(r.Kind))
	fmt.Fprintf(&b, "- Provider: `%s`\n", safeIdentifier(r.Environment.ProviderID))
	fmt.Fprintf(&b, "- Model: `%s`\n", safeIdentifier(r.Environment.ModelID))
	if strings.TrimSpace(r.Environment.ModelHash) != "" {
		fmt.Fprintf(&b, "- Model hash: `%s`\n", safeIdentifier(r.Environment.ModelHash))
	}
	fmt.Fprintf(
		&b,
		"- Runtime: `%s/%s`, `%s`\n",
		safeIdentifier(r.Environment.OS),
		safeIdentifier(r.Environment.Arch),
		safeIdentifier(r.Environment.GoVersion),
	)
	fmt.Fprintf(
		&b,
		"- Results: %d total, %d provider failures, average latency %s\n",
		r.Summary.Total,
		r.Summary.ProviderFailures,
		r.Summary.AverageLatency,
	)
	switch r.Kind {
	case "wake-word":
		fmt.Fprintf(
			&b,
			"- Wake summary: %d false accepts, %d false rejects\n",
			r.Summary.FalseAccepts,
			r.Summary.FalseRejects,
		)
	case "stt":
		fmt.Fprintf(&b, "- STT summary: %d transcript matches\n", r.Summary.TranscriptMatches)
	}
	if len(r.Gaps) > 0 {
		b.WriteString("\nUnresolved gaps:\n")
		for _, gap := range sanitizeBenchmarkGaps(r.Gaps) {
			fmt.Fprintf(&b, "- %s\n", gap)
		}
	}
	if len(validationProblems) > 0 {
		b.WriteString("\nValidation problems:\n")
		for _, problem := range sanitizeBenchmarkGaps(validationProblems) {
			fmt.Fprintf(&b, "- %s\n", problem)
		}
	}
	if len(r.WakeResults) > 0 {
		b.WriteString("\nWake fixture results:\n")
		for _, result := range r.WakeResults {
			status := "matched"
			if !result.MatchesExpected {
				status = "mismatched"
			}
			if result.ErrorCode != "" {
				status = result.ErrorCode
			}
			fmt.Fprintf(
				&b,
				"- `%s`: %s, detected=%t, latency=%s\n",
				safeFixtureID(result.FixtureID),
				status,
				result.Detected,
				result.Latency,
			)
		}
	}
	if len(r.STTResults) > 0 {
		b.WriteString("\nSTT fixture results:\n")
		for _, result := range r.STTResults {
			status := "matched"
			if !result.TranscriptMatched {
				status = "mismatched"
			}
			if result.ErrorCode != "" {
				status = result.ErrorCode
			}
			fmt.Fprintf(
				&b,
				"- `%s`: %s, language=`%s`, latency=%s\n",
				safeFixtureID(result.FixtureID),
				status,
				safeIdentifier(result.Language),
				result.Latency,
			)
		}
	}
	return strings.TrimSpace(b.String())
}

func CompareBenchmarkReports(candidate, baseline BenchmarkReport) BenchmarkComparisonReport {
	kind := safeIdentifier(firstNonEmpty(candidate.Kind, baseline.Kind))
	issue := safeIdentifier(firstNonEmpty(candidate.Issue, baseline.Issue))
	shared, candidateOnly, baselineOnly := compareBenchmarkFixtureIDs(candidate, baseline, kind)
	gaps := []string{}
	if candidate.Kind != baseline.Kind {
		gaps = append(gaps, "candidate and baseline report kinds differ")
	}
	if candidate.Issue != baseline.Issue {
		gaps = append(gaps, "candidate and baseline report issues differ")
	}
	if strings.TrimSpace(candidate.Environment.ProviderID) != "" &&
		strings.TrimSpace(baseline.Environment.ProviderID) != "" &&
		safeIdentifier(candidate.Environment.ProviderID) == safeIdentifier(baseline.Environment.ProviderID) {
		gaps = append(gaps, "candidate and baseline providers must differ")
	}
	if len(shared) == 0 {
		gaps = append(gaps, "candidate and baseline have no shared fixture IDs")
	}
	if len(candidateOnly) > 0 {
		gaps = append(gaps, "candidate has fixtures missing from baseline")
	}
	if len(baselineOnly) > 0 {
		gaps = append(gaps, "baseline has fixtures missing from candidate")
	}
	gaps = append(gaps, issueSpecificBenchmarkComparisonGaps(issue, candidate, baseline)...)
	gaps = append(gaps, candidate.Gaps...)
	gaps = append(gaps, baseline.Gaps...)
	return BenchmarkComparisonReport{
		GeneratedAt:       time.Now().UTC().Format(time.RFC3339Nano),
		Issue:             issue,
		Kind:              kind,
		CandidateProvider: safeIdentifier(candidate.Environment.ProviderID),
		BaselineProvider:  safeIdentifier(baseline.Environment.ProviderID),
		SharedFixtures:    shared,
		CandidateOnly:     candidateOnly,
		BaselineOnly:      baselineOnly,
		CandidateSummary:  candidate.Summary,
		BaselineSummary:   baseline.Summary,
		Gaps:              sanitizeBenchmarkGaps(gaps),
	}
}

func (r BenchmarkComparisonReport) EvidenceMarkdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Voice Benchmark Comparison: %s\n\n", safeIdentifier(r.Issue))
	fmt.Fprintf(&b, "- Kind: `%s`\n", safeIdentifier(r.Kind))
	fmt.Fprintf(&b, "- Candidate: `%s`\n", safeIdentifier(r.CandidateProvider))
	fmt.Fprintf(&b, "- Baseline: `%s`\n", safeIdentifier(r.BaselineProvider))
	fmt.Fprintf(&b, "- Shared fixtures: %d\n", len(r.SharedFixtures))
	fmt.Fprintf(
		&b,
		"- Candidate summary: %d total, %d provider failures, average latency %s\n",
		r.CandidateSummary.Total,
		r.CandidateSummary.ProviderFailures,
		r.CandidateSummary.AverageLatency,
	)
	fmt.Fprintf(
		&b,
		"- Baseline summary: %d total, %d provider failures, average latency %s\n",
		r.BaselineSummary.Total,
		r.BaselineSummary.ProviderFailures,
		r.BaselineSummary.AverageLatency,
	)
	switch r.Kind {
	case "wake-word":
		fmt.Fprintf(
			&b,
			"- Candidate wake errors: %d false accepts, %d false rejects\n",
			r.CandidateSummary.FalseAccepts,
			r.CandidateSummary.FalseRejects,
		)
		fmt.Fprintf(
			&b,
			"- Baseline wake errors: %d false accepts, %d false rejects\n",
			r.BaselineSummary.FalseAccepts,
			r.BaselineSummary.FalseRejects,
		)
	case "stt":
		fmt.Fprintf(&b, "- Candidate STT matches: %d\n", r.CandidateSummary.TranscriptMatches)
		fmt.Fprintf(&b, "- Baseline STT matches: %d\n", r.BaselineSummary.TranscriptMatches)
	}
	if len(r.SharedFixtures) > 0 {
		b.WriteString("\nShared fixtures:\n")
		for _, fixtureID := range r.SharedFixtures {
			fmt.Fprintf(&b, "- `%s`\n", safeFixtureID(fixtureID))
		}
	}
	if len(r.CandidateOnly) > 0 {
		b.WriteString("\nCandidate-only fixtures:\n")
		for _, fixtureID := range r.CandidateOnly {
			fmt.Fprintf(&b, "- `%s`\n", safeFixtureID(fixtureID))
		}
	}
	if len(r.BaselineOnly) > 0 {
		b.WriteString("\nBaseline-only fixtures:\n")
		for _, fixtureID := range r.BaselineOnly {
			fmt.Fprintf(&b, "- `%s`\n", safeFixtureID(fixtureID))
		}
	}
	if len(r.Gaps) > 0 {
		b.WriteString("\nComparison gaps:\n")
		for _, gap := range sanitizeBenchmarkGaps(r.Gaps) {
			fmt.Fprintf(&b, "- %s\n", gap)
		}
	}
	return strings.TrimSpace(b.String())
}

func issueSpecificBenchmarkComparisonGaps(issue string, candidate, baseline BenchmarkReport) []string {
	switch safeIdentifier(issue) {
	case "JUT-11":
		var gaps []string
		candidateProvider := safeIdentifier(candidate.Environment.ProviderID)
		baselineProvider := safeIdentifier(baseline.Environment.ProviderID)
		if !strings.Contains(candidateProvider, "pmdroid") ||
			!strings.Contains(candidateProvider, "microwakeword") {
			gaps = append(gaps, "JUT-11 candidate provider must be pmdroid/microWakeWord")
		}
		if !strings.Contains(baselineProvider, "openwakeword") &&
			!strings.Contains(baselineProvider, "wyoming-openwakeword") {
			gaps = append(gaps, "JUT-11 baseline provider must be openWakeWord/Wyoming")
		}
		return gaps
	default:
		return nil
	}
}

func compareBenchmarkFixtureIDs(candidate, baseline BenchmarkReport, kind string) ([]string, []string, []string) {
	candidateIDs := benchmarkFixtureIDSet(candidate, kind)
	baselineIDs := benchmarkFixtureIDSet(baseline, kind)
	shared := make([]string, 0)
	candidateOnly := make([]string, 0)
	baselineOnly := make([]string, 0)
	for id := range candidateIDs {
		if baselineIDs[id] {
			shared = append(shared, id)
			continue
		}
		candidateOnly = append(candidateOnly, id)
	}
	for id := range baselineIDs {
		if !candidateIDs[id] {
			baselineOnly = append(baselineOnly, id)
		}
	}
	return sortedStrings(shared), sortedStrings(candidateOnly), sortedStrings(baselineOnly)
}

func benchmarkFixtureIDSet(report BenchmarkReport, kind string) map[string]bool {
	out := map[string]bool{}
	switch kind {
	case "wake-word":
		for _, result := range report.WakeResults {
			out[safeFixtureID(result.FixtureID)] = true
		}
	case "stt":
		for _, result := range report.STTResults {
			out[safeFixtureID(result.FixtureID)] = true
		}
	}
	return out
}

func publicSTTBenchmarkResults(results []STTBenchmarkResult) []PublicSTTBenchmarkResult {
	if len(results) == 0 {
		return nil
	}
	public := make([]PublicSTTBenchmarkResult, len(results))
	for i, result := range results {
		public[i] = PublicSTTBenchmarkResult{
			FixtureID:             safeFixtureID(result.FixtureID),
			ProviderID:            safeIdentifier(result.ProviderID),
			ModelID:               safeIdentifier(result.ModelID),
			Language:              safeIdentifier(result.Language),
			ExpectedTranscriptSet: strings.TrimSpace(result.ExpectedTranscript) != "",
			TranscriptReturned:    strings.TrimSpace(result.Transcript) != "",
			TranscriptMatched:     result.TranscriptMatched,
			Latency:               result.Latency,
			ResourceSample:        result.ResourceSample,
			ErrorCode:             safeIdentifier(result.ErrorCode),
			ProviderReturned:      result.ProviderReturned,
		}
	}
	return public
}

func ValidateBenchmarkReport(report BenchmarkReport, expectations BenchmarkReportExpectations) []string {
	var problems []string
	if !validBenchmarkGeneratedAt(report.GeneratedAt) {
		problems = append(problems, "generatedAt must be RFC3339")
	}
	if want := strings.TrimSpace(expectations.Issue); want != "" && report.Issue != safeIdentifier(want) {
		problems = append(problems, "issue does not match expected benchmark issue")
	}
	if want := strings.TrimSpace(expectations.Kind); want != "" && report.Kind != safeIdentifier(want) {
		problems = append(problems, "kind does not match expected benchmark kind")
	}
	if strings.TrimSpace(report.Environment.ProviderID) == "" {
		problems = append(problems, "environment.providerId is required")
	}
	if want := strings.ToLower(strings.TrimSpace(expectations.RequiredProviderIDContains)); want != "" &&
		!strings.Contains(strings.ToLower(safeIdentifier(report.Environment.ProviderID)), want) {
		problems = append(problems, fmt.Sprintf("environment.providerId must identify %s", want))
	}
	if strings.TrimSpace(report.Environment.ProviderKind) == "" {
		problems = append(problems, "environment.providerKind is required")
	}
	if strings.TrimSpace(report.Environment.ProviderKind) != "" &&
		safeIdentifier(report.Environment.ProviderKind) != safeIdentifier(report.Kind) {
		problems = append(problems, "environment.providerKind does not match benchmark kind")
	}
	if strings.TrimSpace(report.Environment.ModelID) == "" {
		problems = append(problems, "environment.modelId is required")
	}
	if expectations.RequireModelHash && strings.TrimSpace(report.Environment.ModelHash) == "" {
		problems = append(problems, "environment.modelHash is required")
	}
	if expectations.RequireModelHash && strings.TrimSpace(report.Environment.ModelHash) != "" &&
		!validSHA256Reference(report.Environment.ModelHash) {
		problems = append(problems, "environment.modelHash must be sha256:<64 hex characters>")
	}
	if !expectations.AllowGaps && len(report.Gaps) > 0 {
		problems = append(problems, "benchmark report has unresolved gaps")
	}
	if !expectations.AllowProviderFailures && report.Summary.ProviderFailures > 0 {
		problems = append(problems, "benchmark report has provider failures")
	}
	if expectations.MinResults > 0 && report.Summary.Total < expectations.MinResults {
		problems = append(problems, "benchmark report has too few results")
	}
	problems = append(problems, validateRequiredBenchmarkFixtures(report, expectations)...)

	switch report.Kind {
	case "wake-word":
		problems = append(problems, validateWakeBenchmarkReport(report, expectations)...)
	case "stt":
		problems = append(problems, validateSTTBenchmarkReport(report, expectations)...)
	default:
		problems = append(problems, "benchmark report kind must be wake-word or stt")
	}
	return problems
}

func validBenchmarkGeneratedAt(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	for _, placeholder := range []string{"replace-with", "placeholder", "unknown", "not-provided", "todo", "tbd"} {
		if strings.Contains(lower, placeholder) {
			return false
		}
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func validateRequiredBenchmarkFixtures(report BenchmarkReport, expectations BenchmarkReportExpectations) []string {
	if len(expectations.RequiredFixtureIDs) == 0 {
		return nil
	}
	seen := benchmarkFixtureIDSet(report, report.Kind)
	var problems []string
	for _, fixtureID := range expectations.RequiredFixtureIDs {
		fixtureID = safeFixtureID(fixtureID)
		if !seen[fixtureID] {
			problems = append(problems, fmt.Sprintf("benchmark report is missing required fixture %s", fixtureID))
		}
	}
	return problems
}

func validateWakeBenchmarkReport(report BenchmarkReport, expectations BenchmarkReportExpectations) []string {
	var problems []string
	if !wakeSummaryMatchesResults(report.Summary, report.WakeResults) {
		problems = append(problems, "wake summary does not match result details")
	}
	if len(report.WakeResults) != report.Summary.Total {
		problems = append(problems, "wake result count does not match summary total")
	}
	if len(report.WakeResults) == 0 {
		problems = append(problems, "wake benchmark report has no wake results")
	}
	wakeFixtureIDs := make([]string, 0, len(report.WakeResults))
	for _, result := range report.WakeResults {
		wakeFixtureIDs = append(wakeFixtureIDs, result.FixtureID)
		if strings.TrimSpace(result.FixtureID) == "" {
			problems = append(problems, "wake result fixtureId is required")
		}
		if result.ExpectedWake == nil {
			problems = append(problems, "wake result expectedWake is required")
		}
		if strings.TrimSpace(result.ProviderID) == "" && result.ErrorCode == "" {
			problems = append(problems, "wake result providerId is required")
		}
		if strings.TrimSpace(result.ProviderID) != "" &&
			safeIdentifier(result.ProviderID) != safeIdentifier(report.Environment.ProviderID) {
			problems = append(problems, "wake result providerId does not match environment.providerId")
		}
		if strings.TrimSpace(result.ModelID) == "" && result.ErrorCode == "" {
			problems = append(problems, "wake result modelId is required")
		}
		if strings.TrimSpace(result.ModelID) != "" &&
			safeIdentifier(result.ModelID) != safeIdentifier(report.Environment.ModelID) {
			problems = append(problems, "wake result modelId does not match environment.modelId")
		}
		if expectations.RequireAllWakeMatches && !result.MatchesExpected {
			problems = append(problems, "wake result did not match expected detection")
		}
		if expectations.RequireResourceSamples && result.ErrorCode == "" &&
			!benchmarkResourceSampleMeasured(result.ResourceSample) {
			problems = append(problems, "wake result resourceSample.duration is required")
		}
	}
	problems = append(problems, duplicateBenchmarkFixtureProblems("wake", wakeFixtureIDs)...)
	return problems
}

func validateSTTBenchmarkReport(report BenchmarkReport, expectations BenchmarkReportExpectations) []string {
	var problems []string
	if !sttSummaryMatchesResults(report.Summary, report.STTResults) {
		problems = append(problems, "stt summary does not match result details")
	}
	if len(report.STTResults) != report.Summary.Total {
		problems = append(problems, "stt result count does not match summary total")
	}
	if len(report.STTResults) == 0 {
		problems = append(problems, "stt benchmark report has no stt results")
	}
	sttFixtureIDs := make([]string, 0, len(report.STTResults))
	for _, result := range report.STTResults {
		sttFixtureIDs = append(sttFixtureIDs, result.FixtureID)
		if strings.TrimSpace(result.FixtureID) == "" {
			problems = append(problems, "stt result fixtureId is required")
		}
		if strings.TrimSpace(result.ExpectedTranscript) == "" {
			problems = append(problems, "stt result expectedTranscript is required")
		}
		if strings.TrimSpace(result.ProviderID) == "" && result.ErrorCode == "" {
			problems = append(problems, "stt result providerId is required")
		}
		if strings.TrimSpace(result.ProviderID) != "" &&
			safeIdentifier(result.ProviderID) != safeIdentifier(report.Environment.ProviderID) {
			problems = append(problems, "stt result providerId does not match environment.providerId")
		}
		if strings.TrimSpace(result.ModelID) == "" && result.ErrorCode == "" {
			problems = append(problems, "stt result modelId is required")
		}
		if strings.TrimSpace(result.ModelID) != "" &&
			safeIdentifier(result.ModelID) != safeIdentifier(report.Environment.ModelID) {
			problems = append(problems, "stt result modelId does not match environment.modelId")
		}
		if expectations.RequireAllTranscriptMatches && !result.TranscriptMatched {
			problems = append(problems, "stt result transcript did not match expected text")
		}
		if result.TranscriptMatched && strings.TrimSpace(result.Transcript) == "" {
			problems = append(problems, "stt result transcript is required when transcriptMatched is true")
		}
		if expectations.RequireResourceSamples && result.ErrorCode == "" &&
			!benchmarkResourceSampleMeasured(result.ResourceSample) {
			problems = append(problems, "stt result resourceSample.duration is required")
		}
	}
	problems = append(problems, duplicateBenchmarkFixtureProblems("stt", sttFixtureIDs)...)
	return problems
}

func benchmarkResourceSampleMeasured(sample BenchmarkResourceSample) bool {
	return sample.Duration > 0
}

func duplicateBenchmarkFixtureProblems(kind string, fixtureIDs []string) []string {
	seen := map[string]bool{}
	var problems []string
	for _, fixtureID := range fixtureIDs {
		fixtureID = safeFixtureID(fixtureID)
		if seen[fixtureID] {
			problems = append(problems, fmt.Sprintf("%s result has duplicate fixtureId %s", kind, fixtureID))
			continue
		}
		seen[fixtureID] = true
	}
	return problems
}

func validSHA256Reference(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[len("sha256:"):] {
		if (char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F') {
			continue
		}
		return false
	}
	return true
}

func wakeSummaryMatchesResults(summary BenchmarkReportSummary, results []WakeBenchmarkResult) bool {
	computed := summarizeWakeBenchmark(results)
	return summary.Total == computed.Total &&
		summary.ProviderFailures == computed.ProviderFailures &&
		summary.FalseAccepts == computed.FalseAccepts &&
		summary.FalseRejects == computed.FalseRejects &&
		summary.AverageLatency == computed.AverageLatency
}

func sttSummaryMatchesResults(summary BenchmarkReportSummary, results []STTBenchmarkResult) bool {
	computed := summarizeSTTBenchmark(results)
	return summary.Total == computed.Total &&
		summary.ProviderFailures == computed.ProviderFailures &&
		summary.TranscriptMatches == computed.TranscriptMatches &&
		summary.AverageLatency == computed.AverageLatency
}

func measureBenchmark(run func() error) (BenchmarkResourceSample, error) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	err := run()
	duration := time.Since(start)
	runtime.ReadMemStats(&after)
	return BenchmarkResourceSample{
		AllocBytes:      saturatingSub(after.Alloc, before.Alloc),
		TotalAllocBytes: saturatingSub(after.TotalAlloc, before.TotalAlloc),
		Duration:        duration,
	}, err
}

func normalizeBenchmarkEnvironment(env BenchmarkEnvironment, kind string) BenchmarkEnvironment {
	if strings.TrimSpace(env.OS) == "" {
		env.OS = runtime.GOOS
	}
	if strings.TrimSpace(env.Arch) == "" {
		env.Arch = runtime.GOARCH
	}
	if strings.TrimSpace(env.GoVersion) == "" {
		env.GoVersion = runtime.Version()
	}
	env.ProviderID = safeIdentifier(env.ProviderID)
	env.ProviderKind = safeIdentifier(firstNonEmpty(env.ProviderKind, kind))
	env.ModelID = safeIdentifier(env.ModelID)
	env.ModelHash = safeIdentifier(env.ModelHash)
	env.Notes = sanitizeText(env.Notes)
	return env
}

func summarizeWakeBenchmark(results []WakeBenchmarkResult) BenchmarkReportSummary {
	var summary BenchmarkReportSummary
	var totalLatency time.Duration
	for _, result := range results {
		summary.Total++
		totalLatency += result.Latency
		if result.ErrorCode != "" {
			summary.ProviderFailures++
		}
		if result.FalseAccept {
			summary.FalseAccepts++
		}
		if result.FalseReject {
			summary.FalseRejects++
		}
	}
	if summary.Total > 0 {
		summary.AverageLatency = totalLatency / time.Duration(summary.Total)
	}
	return summary
}

func summarizeSTTBenchmark(results []STTBenchmarkResult) BenchmarkReportSummary {
	var summary BenchmarkReportSummary
	var totalLatency time.Duration
	for _, result := range results {
		summary.Total++
		totalLatency += result.Latency
		if result.ErrorCode != "" {
			summary.ProviderFailures++
		}
		if result.TranscriptMatched {
			summary.TranscriptMatches++
		}
	}
	if summary.Total > 0 {
		summary.AverageLatency = totalLatency / time.Duration(summary.Total)
	}
	return summary
}

func sanitizeBenchmarkGaps(gaps []string) []string {
	out := make([]string, 0, len(gaps))
	for _, gap := range gaps {
		if cleaned := sanitizeText(gap); strings.TrimSpace(cleaned) != "" {
			out = append(out, strings.TrimSpace(cleaned))
		}
	}
	return out
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func saturatingSub(after, before uint64) uint64 {
	if after < before {
		return 0
	}
	return after - before
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func safeFixtureID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "fixture"
	}
	return safeIdentifier(value)
}

func benchmarkErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	return "provider_failed"
}

func transcriptMatches(expected, got string) bool {
	expected = strings.Join(strings.Fields(sanitizeText(expected)), " ")
	got = strings.Join(strings.Fields(sanitizeText(got)), " ")
	if expected == "" {
		return got == ""
	}
	return strings.EqualFold(expected, got)
}

func cloneCapturedUtterance(utterance CapturedUtterance) CapturedUtterance {
	utterance.Frames = cloneAudioFrames(utterance.Frames)
	return utterance
}
