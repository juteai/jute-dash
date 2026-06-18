package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"jute-dash/apps/hub/internal/app/voice"
)

func main() {
	if code := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("jute-voice-benchmark", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fixtureTemplate := flags.String("fixture-template", "", "print a fixture-set manifest template: wake-word or stt")
	fixtureManifest := flags.String("fixture-manifest", "", "path to benchmark fixture-set manifest JSON")
	fixtureDir := flags.String(
		"fixture-dir",
		".",
		"base directory for fixture paths when validating a fixture manifest",
	)
	fixtureFailureReport := flags.Bool(
		"fixture-failure-report",
		false,
		"emit a provider_unavailable benchmark report from a fixture manifest",
	)
	wakeCommand := flags.String(
		"wake-command",
		"",
		"run a wake-word command provider over fixture-manifest WAV files; use {inputPath} in wake-command-args or it is appended",
	)
	wakeCommandArgs := flags.String(
		"wake-command-args",
		"",
		"space-separated wake command args; {inputPath} is replaced with the temporary WAV fixture path",
	)
	wakeCommandArgsJSON := flags.String(
		"wake-command-args-json",
		"",
		"JSON array of wake command args; safer than wake-command-args for paths or quoted values",
	)
	wakeCommandTimeout := flags.Duration("wake-command-timeout", 30*time.Second, "per-fixture timeout for wake-command")
	sttCommand := flags.String(
		"stt-command",
		"",
		"run an STT command provider over fixture-manifest WAV files; use {inputPath} in stt-command-args or it is appended",
	)
	sttCommandArgs := flags.String(
		"stt-command-args",
		"",
		"space-separated STT command args; {inputPath} is replaced with the temporary WAV fixture path",
	)
	sttCommandArgsJSON := flags.String(
		"stt-command-args-json",
		"",
		"JSON array of STT command args; safer than stt-command-args for paths or quoted values",
	)
	sttCommandTimeout := flags.Duration("stt-command-timeout", 30*time.Second, "per-fixture timeout for stt-command")
	fixtureHash := flags.String("fixture-hash", "", "path to a 16-bit mono PCM WAV fixture to validate and hash")
	toneFixture := flags.String(
		"tone-fixture",
		"",
		"write a deterministic 16 kHz mono PCM WAV tone fixture to this path",
	)
	toneDuration := flags.Duration("tone-duration", 500*time.Millisecond, "tone fixture duration")
	toneFrequency := flags.Float64("tone-frequency", 440, "tone fixture frequency in Hz; use 0 for silence")
	toneAmplitude := flags.Float64("tone-amplitude", 0.35, "tone fixture amplitude from 0 to 1")
	reportPath := flags.String("report", "", "path to benchmark report JSON; reads stdin when omitted")
	baselineReportPath := flags.String(
		"baseline-report",
		"",
		"path to baseline benchmark report JSON to compare with report",
	)
	closureBundlePath := flags.String("closure-bundle", "", "path to provider spike closure evidence bundle JSON")
	closureBundleTemplate := flags.String(
		"closure-bundle-template",
		"",
		"print a provider closure bundle template: JUT-11 or JUT-13",
	)
	closureBundleCompose := flags.String(
		"closure-bundle-compose",
		"",
		"compose a provider closure bundle from artifact files: JUT-11 or JUT-13",
	)
	composeDecisionStatus := flags.String("decision-status", "", "closure bundle decision status")
	composeDecisionRationale := flags.String("decision-rationale", "", "closure bundle decision rationale")
	composeProviderManifest := flags.String(
		"provider-manifest",
		"",
		"provider manifest JSON artifact path for closure bundle compose",
	)
	composeFixtureManifest := flags.String(
		"fixture-manifest-artifact",
		"",
		"fixture manifest JSON artifact path for closure bundle compose",
	)
	composeBuildEvidence := flags.String(
		"build-evidence-artifacts",
		"",
		"comma-separated provider build evidence JSON artifact paths for closure bundle compose",
	)
	composePackagingEvidence := flags.String(
		"packaging-evidence-artifact",
		"",
		"provider packaging evidence JSON artifact path for closure bundle compose",
	)
	composeModelEvidence := flags.String(
		"model-evidence-artifacts",
		"",
		"comma-separated provider model evidence JSON artifact paths for closure bundle compose",
	)
	composeBenchmarkReport := flags.String(
		"benchmark-report-artifact",
		"",
		"benchmark report JSON artifact path for closure bundle compose",
	)
	composeBaselineReport := flags.String(
		"baseline-report-artifact",
		"",
		"baseline report JSON artifact path for closure bundle compose",
	)
	issue := flags.String("issue", "", "expected Linear issue, e.g. JUT-13")
	kind := flags.String("kind", "", "expected benchmark kind: wake-word or stt")
	providerID := flags.String("provider-id", "", "provider ID to record in generated fixture reports")
	modelID := flags.String("model-id", "", "model ID to record in generated fixture reports")
	modelHash := flags.String("model-hash", "", "model hash to record in generated fixture reports")
	environmentNotes := flags.String("environment-notes", "", "notes to record in generated fixture reports")
	buildEvidence := flags.Bool("build-evidence", false, "emit provider build evidence instead of benchmark evidence")
	buildTarget := flags.String(
		"build-target",
		"",
		"provider build target, e.g. native-cli, native-consumer, docker, raspberry-pi",
	)
	buildCommandID := flags.String("build-command-id", "", "safe command identifier for provider build evidence")
	buildStatus := flags.String("build-status", "", "provider build status: succeeded, failed, blocked, or not-run")
	buildExitCode := flags.Int("build-exit-code", 0, "provider build command exit code")
	buildErrorCode := flags.String("build-error-code", "", "safe provider build error code")
	buildMissing := flags.String(
		"build-missing",
		"",
		"comma-separated missing dependency names for provider build evidence",
	)
	packagingEvidence := flags.Bool(
		"packaging-evidence",
		false,
		"emit provider packaging target matrix evidence instead of benchmark evidence",
	)
	packagingTargets := flags.String(
		"packaging-targets",
		"",
		"comma-separated target=status pairs for packaging evidence",
	)
	modelEvidence := flags.Bool(
		"model-evidence",
		false,
		"emit provider model compatibility evidence instead of benchmark evidence",
	)
	modelSource := flags.String("model-source", "", "safe model source identifier, e.g. esphome, ohf, whisper-cpp")
	modelFormat := flags.String("model-format", "", "safe model format identifier, e.g. tflite, ggml")
	modelCompatibility := flags.String(
		"model-compatibility",
		"",
		"model compatibility status: compatible, incompatible, blocked, or untested",
	)
	modelRuntimeStatus := flags.String(
		"model-runtime-status",
		"",
		"model runtime status: loaded, failed, blocked, or not-run",
	)
	minResults := flags.Int("min-results", 0, "minimum required fixture result count")
	requireModelHash := flags.Bool("require-model-hash", false, "require environment.modelHash")
	allowGaps := flags.Bool("allow-gaps", false, "allow unresolved report gaps")
	allowProviderFailures := flags.Bool("allow-provider-failures", false, "allow provider failures")
	requireWakeMatches := flags.Bool(
		"require-wake-matches",
		false,
		"require all wake results to match expected wake flags",
	)
	requireTranscriptMatches := flags.Bool(
		"require-transcript-matches",
		false,
		"require all STT results to match expected transcripts",
	)
	acceptancePreset := flags.Bool(
		"acceptance-preset",
		false,
		"apply issue-specific acceptance validation defaults for JUT-11 or JUT-13",
	)
	publicJSON := flags.Bool(
		"public-json",
		false,
		"print a redacted benchmark report JSON artifact instead of Markdown evidence",
	)
	if err := flags.Parse(args); err != nil {
		return 2
	}
	explicitFlags := map[string]bool{}
	flags.Visit(func(flag *flag.Flag) {
		explicitFlags[flag.Name] = true
	})
	if *fixtureTemplate != "" {
		if err := writeFixtureTemplate(stdout, *fixtureTemplate, *issue); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		return 0
	}
	if *closureBundleTemplate != "" {
		if err := writeProviderClosureBundleTemplate(stdout, *closureBundleTemplate); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}
		return 0
	}
	if *closureBundleCompose != "" {
		if err := writeComposedProviderClosureBundle(stdout, providerClosureBundleComposeInputs{
			issue:             *closureBundleCompose,
			decisionStatus:    *composeDecisionStatus,
			decisionRationale: *composeDecisionRationale,
			providerManifest:  *composeProviderManifest,
			fixtureManifest:   *composeFixtureManifest,
			buildEvidence:     *composeBuildEvidence,
			packagingEvidence: *composePackagingEvidence,
			modelEvidence:     *composeModelEvidence,
			benchmarkReport:   *composeBenchmarkReport,
			baselineReport:    *composeBaselineReport,
		}); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		return 0
	}
	if *fixtureManifest != "" {
		if *wakeCommand != "" {
			problems, err := writeWakeCommandBenchmarkReport(stdout, wakeCommandBenchmarkInputs{
				manifestPath:     *fixtureManifest,
				fixtureDir:       *fixtureDir,
				acceptancePreset: *acceptancePreset,
				publicJSON:       *publicJSON,
				command:          *wakeCommand,
				args:             *wakeCommandArgs,
				argsJSON:         *wakeCommandArgsJSON,
				timeout:          *wakeCommandTimeout,
				env: voice.BenchmarkEnvironment{
					OS:         runtime.GOOS,
					Arch:       runtime.GOARCH,
					GoVersion:  runtime.Version(),
					ProviderID: *providerID,
					ModelID:    *modelID,
					ModelHash:  *modelHash,
					Notes:      *environmentNotes,
				},
			})
			if err != nil {
				fmt.Fprintf(stderr, "%v\n", err)
				return 1
			}
			if len(problems) > 0 {
				fmt.Fprintf(stderr, "wake command benchmark has %d validation problem(s)\n", len(problems))
				return 1
			}
			return 0
		}
		if *sttCommand != "" {
			problems, err := writeSTTCommandBenchmarkReport(stdout, sttCommandBenchmarkInputs{
				manifestPath:     *fixtureManifest,
				fixtureDir:       *fixtureDir,
				acceptancePreset: *acceptancePreset,
				publicJSON:       *publicJSON,
				command:          *sttCommand,
				args:             *sttCommandArgs,
				argsJSON:         *sttCommandArgsJSON,
				timeout:          *sttCommandTimeout,
				env: voice.BenchmarkEnvironment{
					OS:         runtime.GOOS,
					Arch:       runtime.GOARCH,
					GoVersion:  runtime.Version(),
					ProviderID: *providerID,
					ModelID:    *modelID,
					ModelHash:  *modelHash,
					Notes:      *environmentNotes,
				},
			})
			if err != nil {
				fmt.Fprintf(stderr, "%v\n", err)
				return 1
			}
			if len(problems) > 0 {
				fmt.Fprintf(stderr, "stt command benchmark has %d validation problem(s)\n", len(problems))
				return 1
			}
			return 0
		}
		if *fixtureFailureReport {
			problems, err := writeFixtureFailureReport(
				stdout,
				*fixtureManifest,
				*fixtureDir,
				*acceptancePreset,
				*publicJSON,
				voice.BenchmarkEnvironment{
					ProviderID: *providerID,
					ModelID:    *modelID,
					ModelHash:  *modelHash,
					Notes:      *environmentNotes,
				},
			)
			if err != nil {
				fmt.Fprintf(stderr, "%v\n", err)
				return 1
			}
			if len(problems) > 0 {
				fmt.Fprintf(stderr, "fixture failure report has %d validation problem(s)\n", len(problems))
				return 1
			}
			return 0
		}
		problems, err := validateFixtureManifest(stdout, *fixtureManifest, *fixtureDir, *acceptancePreset)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		if len(problems) > 0 {
			fmt.Fprintf(stderr, "fixture manifest has %d validation problem(s)\n", len(problems))
			return 1
		}
		return 0
	}
	if *fixtureHash != "" {
		if err := writeFixtureHash(stdout, *fixtureHash); err != nil {
			fmt.Fprintf(stderr, "%s\n", safeBenchmarkCLIError(err))
			return 1
		}
		return 0
	}
	if *toneFixture != "" {
		if err := writeToneFixture(stdout, *toneFixture, *toneDuration, *toneFrequency, *toneAmplitude); err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 1
		}
		return 0
	}
	if *closureBundlePath != "" {
		bundle, problems, err := readProviderClosureBundle(*closureBundlePath, *issue)
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", safeBenchmarkCLIError(fmt.Errorf("read closure bundle: %w", err)))
			return 1
		}
		fmt.Fprintln(stdout, bundle.markdown(problems))
		if len(problems) > 0 {
			fmt.Fprintf(stderr, "provider closure bundle has %d validation problem(s)\n", len(problems))
			return 1
		}
		return 0
	}
	if *buildEvidence {
		evidence := newProviderBuildEvidence(providerBuildEvidenceInputs{
			issue:       *issue,
			kind:        *kind,
			providerID:  *providerID,
			target:      *buildTarget,
			commandID:   *buildCommandID,
			status:      *buildStatus,
			exitCode:    *buildExitCode,
			errorCode:   *buildErrorCode,
			missing:     *buildMissing,
			notes:       *environmentNotes,
			publicJSON:  *publicJSON,
			generatedAt: time.Now().UTC(),
			os:          runtime.GOOS,
			arch:        runtime.GOARCH,
			goVersion:   runtime.Version(),
		})
		if *publicJSON {
			raw, err := json.MarshalIndent(evidence, "", "  ")
			if err != nil {
				fmt.Fprintf(stderr, "encode provider build evidence: %v\n", err)
				return 1
			}
			fmt.Fprintln(stdout, string(raw))
		} else {
			fmt.Fprintln(stdout, evidence.markdown())
		}
		if len(evidence.Problems) > 0 {
			fmt.Fprintf(stderr, "provider build evidence has %d validation problem(s)\n", len(evidence.Problems))
			return 1
		}
		return 0
	}
	if *packagingEvidence {
		evidence := newProviderPackagingEvidence(providerPackagingEvidenceInputs{
			issue:       *issue,
			kind:        *kind,
			providerID:  *providerID,
			targets:     *packagingTargets,
			notes:       *environmentNotes,
			generatedAt: time.Now().UTC(),
			os:          runtime.GOOS,
			arch:        runtime.GOARCH,
			goVersion:   runtime.Version(),
		})
		if *publicJSON {
			raw, err := json.MarshalIndent(evidence, "", "  ")
			if err != nil {
				fmt.Fprintf(stderr, "encode provider packaging evidence: %v\n", err)
				return 1
			}
			fmt.Fprintln(stdout, string(raw))
		} else {
			fmt.Fprintln(stdout, evidence.markdown())
		}
		if len(evidence.Problems) > 0 {
			fmt.Fprintf(stderr, "provider packaging evidence has %d validation problem(s)\n", len(evidence.Problems))
			return 1
		}
		return 0
	}
	if *modelEvidence {
		evidence := newProviderModelEvidence(providerModelEvidenceInputs{
			issue:         *issue,
			kind:          *kind,
			providerID:    *providerID,
			modelID:       *modelID,
			modelHash:     *modelHash,
			modelSource:   *modelSource,
			modelFormat:   *modelFormat,
			compatibility: *modelCompatibility,
			runtimeStatus: *modelRuntimeStatus,
			notes:         *environmentNotes,
			generatedAt:   time.Now().UTC(),
			os:            runtime.GOOS,
			arch:          runtime.GOARCH,
			goVersion:     runtime.Version(),
		})
		if *publicJSON {
			raw, err := json.MarshalIndent(evidence, "", "  ")
			if err != nil {
				fmt.Fprintf(stderr, "encode provider model evidence: %v\n", err)
				return 1
			}
			fmt.Fprintln(stdout, string(raw))
		} else {
			fmt.Fprintln(stdout, evidence.markdown())
		}
		if len(evidence.Problems) > 0 {
			fmt.Fprintf(stderr, "provider model evidence has %d validation problem(s)\n", len(evidence.Problems))
			return 1
		}
		return 0
	}

	raw, err := readReport(*reportPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", safeBenchmarkCLIError(fmt.Errorf("read benchmark report: %w", err)))
		return 1
	}
	report, err := voice.DecodeBenchmarkReport(raw)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if *baselineReportPath != "" {
		baselineRaw, err := readReport(*baselineReportPath, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", safeBenchmarkCLIError(fmt.Errorf("read baseline benchmark report: %w", err)))
			return 1
		}
		baseline, err := voice.DecodeBenchmarkReport(baselineRaw)
		if err != nil {
			fmt.Fprintf(stderr, "decode baseline benchmark report: %v\n", err)
			return 1
		}
		if *acceptancePreset {
			if code := validateComparisonAcceptancePreset(report, baseline, stdout, stderr, *issue); code != 0 {
				return code
			}
		}
		comparison := voice.CompareBenchmarkReports(report, baseline)
		fmt.Fprintln(stdout, comparison.EvidenceMarkdown())
		if len(comparison.Gaps) > 0 {
			fmt.Fprintf(stderr, "benchmark comparison has %d gap(s)\n", len(comparison.Gaps))
			return 1
		}
		return 0
	}

	expectations, ok := benchmarkExpectations(report, benchmarkExpectationInputs{
		acceptancePreset:         *acceptancePreset,
		explicitFlags:            explicitFlags,
		issue:                    *issue,
		kind:                     *kind,
		minResults:               *minResults,
		requireModelHash:         *requireModelHash,
		allowGaps:                *allowGaps,
		allowProviderFailures:    *allowProviderFailures,
		requireWakeMatches:       *requireWakeMatches,
		requireTranscriptMatches: *requireTranscriptMatches,
	})
	if !ok {
		fmt.Fprintf(stderr, "no acceptance preset is defined for issue %q\n", firstNonEmpty(*issue, report.Issue))
		return 2
	}
	problems := voice.ValidateBenchmarkReport(report, expectations)
	if *publicJSON {
		raw, err := report.PublicJSON()
		if err != nil {
			fmt.Fprintf(stderr, "encode public benchmark report: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(raw))
	} else {
		fmt.Fprintln(stdout, report.EvidenceMarkdown(problems))
	}
	if len(problems) > 0 {
		fmt.Fprintf(stderr, "benchmark report has %d validation problem(s)\n", len(problems))
		return 1
	}
	return 0
}

type providerBuildEvidenceInputs struct {
	issue       string
	kind        string
	providerID  string
	target      string
	commandID   string
	status      string
	exitCode    int
	errorCode   string
	missing     string
	notes       string
	publicJSON  bool
	generatedAt time.Time
	os          string
	arch        string
	goVersion   string
}

type providerBuildEvidence struct {
	GeneratedAt         string   `json:"generatedAt"`
	Issue               string   `json:"issue"`
	ProviderID          string   `json:"providerId"`
	ProviderKind        string   `json:"providerKind"`
	Target              string   `json:"target"`
	CommandID           string   `json:"commandId"`
	Status              string   `json:"status"`
	ExitCode            int      `json:"exitCode,omitempty"`
	ErrorCode           string   `json:"errorCode,omitempty"`
	MissingDependencies []string `json:"missingDependencies,omitempty"`
	Runtime             string   `json:"runtime"`
	Notes               string   `json:"notes,omitempty"`
	ClosureEvidence     bool     `json:"closureEvidence"`
	Problems            []string `json:"problems,omitempty"`
}

type providerPackagingEvidenceInputs struct {
	issue       string
	kind        string
	providerID  string
	targets     string
	notes       string
	generatedAt time.Time
	os          string
	arch        string
	goVersion   string
}

type providerPackagingEvidence struct {
	GeneratedAt               string                    `json:"generatedAt"`
	Issue                     string                    `json:"issue"`
	ProviderID                string                    `json:"providerId"`
	ProviderKind              string                    `json:"providerKind"`
	Targets                   []providerPackagingTarget `json:"targets"`
	Runtime                   string                    `json:"runtime"`
	Notes                     string                    `json:"notes,omitempty"`
	PackagingEvidenceComplete bool                      `json:"packagingEvidenceComplete"`
	Problems                  []string                  `json:"problems,omitempty"`
}

type providerPackagingTarget struct {
	Target string `json:"target"`
	Status string `json:"status"`
}

type providerModelEvidenceInputs struct {
	issue         string
	kind          string
	providerID    string
	modelID       string
	modelHash     string
	modelSource   string
	modelFormat   string
	compatibility string
	runtimeStatus string
	notes         string
	generatedAt   time.Time
	os            string
	arch          string
	goVersion     string
}

type providerModelEvidence struct {
	GeneratedAt         string   `json:"generatedAt"`
	Issue               string   `json:"issue"`
	ProviderID          string   `json:"providerId"`
	ProviderKind        string   `json:"providerKind"`
	ModelID             string   `json:"modelId"`
	ModelHash           string   `json:"modelHash"`
	ModelSource         string   `json:"modelSource"`
	ModelFormat         string   `json:"modelFormat"`
	CompatibilityStatus string   `json:"compatibilityStatus"`
	RuntimeStatus       string   `json:"runtimeStatus"`
	Runtime             string   `json:"runtime"`
	Notes               string   `json:"notes,omitempty"`
	ClosureEvidence     bool     `json:"closureEvidence"`
	Problems            []string `json:"problems,omitempty"`
}

type providerClosureBundleFile struct {
	Issue             string                     `json:"issue"`
	Decision          providerClosureDecision    `json:"decision"`
	ProviderManifest  json.RawMessage            `json:"providerManifest,omitempty"`
	FixtureManifest   json.RawMessage            `json:"fixtureManifest,omitempty"`
	BuildEvidence     []providerBuildEvidence    `json:"buildEvidence,omitempty"`
	PackagingEvidence *providerPackagingEvidence `json:"packagingEvidence,omitempty"`
	ModelEvidence     []providerModelEvidence    `json:"modelEvidence,omitempty"`
	BenchmarkReport   json.RawMessage            `json:"benchmarkReport,omitempty"`
	BaselineReport    json.RawMessage            `json:"baselineReport,omitempty"`
}

type providerClosureDecision struct {
	Status    string `json:"status"`
	Rationale string `json:"rationale"`
}

type providerClosureBundleComposeInputs struct {
	issue             string
	decisionStatus    string
	decisionRationale string
	providerManifest  string
	fixtureManifest   string
	buildEvidence     string
	packagingEvidence string
	modelEvidence     string
	benchmarkReport   string
	baselineReport    string
}

type providerClosureBundle struct {
	Issue                string
	DecisionStatus       string
	DecisionRationale    string
	ProviderManifestID   string
	ProviderManifestKind string
	ProviderManifestOK   bool
	FixtureManifestCount int
	FixtureManifestOK    bool
	BuildEvidenceCount   int
	PackagingComplete    bool
	ModelEvidenceCount   int
	BenchmarkAccepted    bool
	BaselineAccepted     bool
	ComparisonAccepted   bool
	ClosureBundleSuccess bool
}

func newProviderBuildEvidence(inputs providerBuildEvidenceInputs) providerBuildEvidence {
	evidence := providerBuildEvidence{
		GeneratedAt:         inputs.generatedAt.Format(time.RFC3339Nano),
		Issue:               safeEvidenceIssue(inputs.issue),
		ProviderID:          safeEvidenceProvider(inputs.providerID),
		ProviderKind:        safeEvidenceToken(inputs.kind),
		Target:              safeEvidenceToken(inputs.target),
		CommandID:           safeEvidenceToken(inputs.commandID),
		Status:              safeEvidenceToken(inputs.status),
		ExitCode:            inputs.exitCode,
		ErrorCode:           safeEvidenceToken(inputs.errorCode),
		MissingDependencies: safeEvidenceList(inputs.missing),
		Runtime: fmt.Sprintf(
			"%s/%s %s",
			safeEvidenceToken(inputs.os),
			safeEvidenceToken(inputs.arch),
			safeEvidenceToken(inputs.goVersion),
		),
		Notes: safeEvidenceNote(inputs.notes),
	}
	evidence.Problems = validateProviderBuildEvidence(evidence)
	evidence.ClosureEvidence = len(evidence.Problems) == 0 && evidence.Status == "succeeded"
	return evidence
}

func newProviderPackagingEvidence(inputs providerPackagingEvidenceInputs) providerPackagingEvidence {
	evidence := providerPackagingEvidence{
		GeneratedAt:  inputs.generatedAt.Format(time.RFC3339Nano),
		Issue:        safeEvidenceIssue(inputs.issue),
		ProviderID:   safeEvidenceProvider(inputs.providerID),
		ProviderKind: safeEvidenceToken(inputs.kind),
		Targets:      safeEvidenceTargetStatusList(inputs.targets),
		Runtime: fmt.Sprintf(
			"%s/%s %s",
			safeEvidenceToken(inputs.os),
			safeEvidenceToken(inputs.arch),
			safeEvidenceToken(inputs.goVersion),
		),
		Notes: safeEvidenceNote(inputs.notes),
	}
	evidence.Problems = validateProviderPackagingEvidence(evidence)
	evidence.PackagingEvidenceComplete = len(evidence.Problems) == 0
	return evidence
}

func newProviderModelEvidence(inputs providerModelEvidenceInputs) providerModelEvidence {
	evidence := providerModelEvidence{
		GeneratedAt:         inputs.generatedAt.Format(time.RFC3339Nano),
		Issue:               safeEvidenceIssue(inputs.issue),
		ProviderID:          safeEvidenceProvider(inputs.providerID),
		ProviderKind:        safeEvidenceToken(inputs.kind),
		ModelID:             safeEvidenceToken(inputs.modelID),
		ModelHash:           safeEvidenceModelHash(inputs.modelHash),
		ModelSource:         safeEvidenceToken(inputs.modelSource),
		ModelFormat:         safeEvidenceToken(inputs.modelFormat),
		CompatibilityStatus: safeEvidenceToken(inputs.compatibility),
		RuntimeStatus:       safeEvidenceToken(inputs.runtimeStatus),
		Runtime: fmt.Sprintf(
			"%s/%s %s",
			safeEvidenceToken(inputs.os),
			safeEvidenceToken(inputs.arch),
			safeEvidenceToken(inputs.goVersion),
		),
		Notes: safeEvidenceNote(inputs.notes),
	}
	evidence.Problems = validateProviderModelEvidence(evidence)
	evidence.ClosureEvidence = len(evidence.Problems) == 0
	return evidence
}

func writeComposedProviderClosureBundle(w io.Writer, inputs providerClosureBundleComposeInputs) error {
	bundle, err := composeProviderClosureBundle(inputs)
	if err != nil {
		return err
	}
	summary := providerClosureBundle{
		Issue:              safeEvidenceIssue(bundle.Issue),
		BuildEvidenceCount: len(bundle.BuildEvidence),
		ModelEvidenceCount: len(bundle.ModelEvidence),
	}
	problems := validateProviderClosureBundle(bundle, &summary)
	if len(problems) > 0 {
		return fmt.Errorf(
			"composed closure bundle has %d validation problem(s): %s",
			len(problems),
			strings.Join(problems, "; "),
		)
	}
	raw, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("encode closure bundle: %w", err)
	}
	fmt.Fprintln(w, string(raw))
	return nil
}

func composeProviderClosureBundle(inputs providerClosureBundleComposeInputs) (providerClosureBundleFile, error) {
	issue := safeEvidenceIssue(inputs.issue)
	if issue != "JUT-11" && issue != "JUT-13" {
		return providerClosureBundleFile{}, errors.New("closure-bundle-compose must be JUT-11 or JUT-13")
	}
	if strings.TrimSpace(inputs.decisionStatus) == "" {
		return providerClosureBundleFile{}, errors.New("decision-status is required")
	}
	if strings.TrimSpace(inputs.decisionRationale) == "" {
		return providerClosureBundleFile{}, errors.New("decision-rationale is required")
	}
	providerManifest, err := readRawJSONArtifact(inputs.providerManifest, "provider-manifest")
	if err != nil {
		return providerClosureBundleFile{}, err
	}
	fixtureManifest, err := readRawJSONArtifact(inputs.fixtureManifest, "fixture-manifest-artifact")
	if err != nil {
		return providerClosureBundleFile{}, err
	}
	buildEvidence, err := readBuildEvidenceArtifacts(inputs.buildEvidence)
	if err != nil {
		return providerClosureBundleFile{}, err
	}
	packagingEvidence, err := readPackagingEvidenceArtifact(inputs.packagingEvidence)
	if err != nil {
		return providerClosureBundleFile{}, err
	}
	modelEvidence, err := readModelEvidenceArtifacts(inputs.modelEvidence)
	if err != nil {
		return providerClosureBundleFile{}, err
	}
	benchmarkReport, err := readRawJSONArtifact(inputs.benchmarkReport, "benchmark-report-artifact")
	if err != nil {
		return providerClosureBundleFile{}, err
	}
	var baselineReport json.RawMessage
	if strings.TrimSpace(inputs.baselineReport) != "" {
		baselineReport, err = readRawJSONArtifact(inputs.baselineReport, "baseline-report-artifact")
		if err != nil {
			return providerClosureBundleFile{}, err
		}
	}
	return providerClosureBundleFile{
		Issue: issue,
		Decision: providerClosureDecision{
			Status:    inputs.decisionStatus,
			Rationale: inputs.decisionRationale,
		},
		ProviderManifest:  providerManifest,
		FixtureManifest:   fixtureManifest,
		BuildEvidence:     buildEvidence,
		PackagingEvidence: packagingEvidence,
		ModelEvidence:     modelEvidence,
		BenchmarkReport:   benchmarkReport,
		BaselineReport:    baselineReport,
	}, nil
}

func readRawJSONArtifact(path string, label string) (json.RawMessage, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%s is required", label)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode %s: trailing JSON data", label)
	}
	compact, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", label, err)
	}
	return json.RawMessage(compact), nil
}

func readBuildEvidenceArtifacts(paths string) ([]providerBuildEvidence, error) {
	parts := artifactPathList(paths)
	if len(parts) == 0 {
		return nil, errors.New("build-evidence-artifacts is required")
	}
	rows := make([]providerBuildEvidence, 0, len(parts))
	for index, path := range parts {
		var row providerBuildEvidence
		if err := readTypedJSONArtifact(path, fmt.Sprintf("build-evidence-artifacts[%d]", index), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readPackagingEvidenceArtifact(path string) (*providerPackagingEvidence, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("packaging-evidence-artifact is required")
	}
	var row providerPackagingEvidence
	if err := readTypedJSONArtifact(path, "packaging-evidence-artifact", &row); err != nil {
		return nil, err
	}
	return &row, nil
}

func readModelEvidenceArtifacts(paths string) ([]providerModelEvidence, error) {
	parts := artifactPathList(paths)
	if len(parts) == 0 {
		return nil, errors.New("model-evidence-artifacts is required")
	}
	rows := make([]providerModelEvidence, 0, len(parts))
	for index, path := range parts {
		var row providerModelEvidence
		if err := readTypedJSONArtifact(path, fmt.Sprintf("model-evidence-artifacts[%d]", index), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readTypedJSONArtifact(path string, label string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", label, err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", label, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode %s: trailing JSON data", label)
	}
	return nil
}

func artifactPathList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func readProviderClosureBundle(path string, expectedIssue string) (providerClosureBundle, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return providerClosureBundle{}, nil, err
	}
	var file providerClosureBundleFile
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return providerClosureBundle{}, nil, fmt.Errorf("decode closure bundle: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return providerClosureBundle{}, nil, errors.New("decode closure bundle: trailing JSON data")
	}
	issue := safeEvidenceIssue(firstNonEmpty(expectedIssue, file.Issue))
	bundle := providerClosureBundle{
		Issue:              issue,
		BuildEvidenceCount: len(file.BuildEvidence),
		ModelEvidenceCount: len(file.ModelEvidence),
	}
	problems := validateProviderClosureBundle(file, &bundle)
	bundle.ClosureBundleSuccess = len(problems) == 0
	return bundle, problems, nil
}

func validateProviderClosureBundle(file providerClosureBundleFile, bundle *providerClosureBundle) []string {
	issue := bundle.Issue
	var problems []string
	switch issue {
	case "JUT-11", "JUT-13":
	default:
		problems = append(problems, "closure bundle issue must be JUT-11 or JUT-13")
	}
	if safeEvidenceIssue(file.Issue) != "" && safeEvidenceIssue(file.Issue) != issue {
		problems = append(problems, "closure bundle issue does not match expected issue")
	}
	decision := sanitizeProviderClosureDecision(file.Decision)
	bundle.DecisionStatus = decision.Status
	bundle.DecisionRationale = decision.Rationale
	problems = appendPrefixedProblems(problems, "decision", validateProviderClosureDecision(issue, decision))
	manifest, manifestProblems := validateClosureBundleProviderManifest(issue, file.ProviderManifest)
	if len(manifestProblems) == 0 {
		bundle.ProviderManifestOK = true
		bundle.ProviderManifestID = manifest.ID
		bundle.ProviderManifestKind = manifest.Kind
	}
	problems = appendPrefixedProblems(problems, "manifest", manifestProblems)
	fixtureManifest, fixtureProblems := validateClosureBundleFixtureManifest(issue, file.FixtureManifest)
	bundle.FixtureManifestCount = len(fixtureManifest.Fixtures)
	if len(fixtureProblems) == 0 {
		bundle.FixtureManifestOK = true
	}
	problems = appendPrefixedProblems(problems, "fixtures", fixtureProblems)
	buildProblems := validateClosureBundleBuilds(issue, file.BuildEvidence)
	problems = appendPrefixedProblems(problems, "build", buildProblems)
	if file.PackagingEvidence == nil {
		problems = append(problems, "packagingEvidence is required")
	} else {
		packaging, packagingProblems := validateClosureBundlePackaging(issue, *file.PackagingEvidence)
		if len(packagingProblems) == 0 {
			bundle.PackagingComplete = true
		}
		problems = appendPrefixedProblems(problems, "packaging", packagingProblems)
		if len(manifestProblems) == 0 && len(packagingProblems) == 0 {
			packageManifestProblems := validateClosureBundlePackagingAgainstProviderManifest(issue, packaging, manifest)
			problems = appendPrefixedProblems(problems, "packaging", packageManifestProblems)
		}
		if len(packagingProblems) == 0 {
			decisionPackagingProblems := validateClosureBundleDecisionAgainstPackaging(issue, decision, packaging)
			problems = appendPrefixedProblems(problems, "decision", decisionPackagingProblems)
		}
	}
	validModels, modelProblems := validateClosureBundleModels(issue, file.ModelEvidence)
	problems = appendPrefixedProblems(problems, "model", modelProblems)
	if len(manifestProblems) == 0 {
		modelManifestProblems := validateClosureBundleModelsAgainstProviderManifest(issue, validModels, manifest)
		problems = appendPrefixedProblems(problems, "model", modelManifestProblems)
	}
	report, reportProblems := validateClosureBundleBenchmark(issue, file.BenchmarkReport)
	if len(reportProblems) == 0 {
		bundle.BenchmarkAccepted = true
		modelMatchProblems := validateClosureBundleBenchmarkModel(issue, report, validModels)
		problems = appendPrefixedProblems(problems, "model", modelMatchProblems)
		fixtureMatchProblems := validateClosureBundleReportFixtures(report, fixtureManifest)
		problems = appendPrefixedProblems(problems, "benchmark", fixtureMatchProblems)
	}
	problems = appendPrefixedProblems(problems, "benchmark", reportProblems)
	switch issue {
	case "JUT-11":
		baseline, baselineProblems := validateClosureBundleBenchmark(issue, file.BaselineReport)
		if len(baselineProblems) == 0 {
			bundle.BaselineAccepted = true
			fixtureMatchProblems := validateClosureBundleReportFixtures(baseline, fixtureManifest)
			problems = appendPrefixedProblems(problems, "baseline", fixtureMatchProblems)
		}
		problems = appendPrefixedProblems(problems, "baseline", baselineProblems)
		if len(reportProblems) == 0 && len(baselineProblems) == 0 {
			comparison := voice.CompareBenchmarkReports(report, baseline)
			if len(comparison.Gaps) == 0 {
				bundle.ComparisonAccepted = true
			}
			problems = appendPrefixedProblems(problems, "comparison", comparison.Gaps)
		}
	case "JUT-13":
		if len(file.BaselineReport) > 0 {
			problems = append(problems, "baselineReport is not used for JUT-13 closure")
		}
	}
	return uniqueEvidenceProblems(problems)
}

func validateClosureBundleReportFixtures(
	report voice.BenchmarkReport,
	manifest voice.BenchmarkFixtureSetManifest,
) []string {
	if len(manifest.Fixtures) == 0 {
		return nil
	}
	declared := map[string]voice.BenchmarkFixtureManifest{}
	for _, fixture := range manifest.Fixtures {
		fixtureID := strings.TrimSpace(fixture.ID)
		if fixtureID != "" {
			declared[fixtureID] = fixture
		}
	}
	var problems []string
	measured := map[string]bool{}
	switch report.Kind {
	case "wake-word":
		for _, result := range report.WakeResults {
			fixtureID := strings.TrimSpace(result.FixtureID)
			measured[fixtureID] = true
			fixture, ok := declared[fixtureID]
			if !ok {
				problems = append(
					problems,
					fmt.Sprintf("benchmark fixture %s is not declared in fixtureManifest", fixtureID),
				)
				continue
			}
			if fixture.ExpectWake != nil &&
				(result.ExpectedWake == nil || *fixture.ExpectWake != *result.ExpectedWake) {
				problems = append(
					problems,
					fmt.Sprintf("benchmark fixture %s expectedWake does not match fixtureManifest", fixtureID),
				)
			}
		}
	case "stt":
		for _, result := range report.STTResults {
			fixtureID := strings.TrimSpace(result.FixtureID)
			measured[fixtureID] = true
			fixture, ok := declared[fixtureID]
			if !ok {
				problems = append(
					problems,
					fmt.Sprintf("benchmark fixture %s is not declared in fixtureManifest", fixtureID),
				)
				continue
			}
			if strings.TrimSpace(fixture.ExpectedTranscript) != "" &&
				strings.TrimSpace(fixture.ExpectedTranscript) != strings.TrimSpace(result.ExpectedTranscript) {
				problems = append(
					problems,
					fmt.Sprintf("benchmark fixture %s expectedTranscript does not match fixtureManifest", fixtureID),
				)
			}
		}
	}
	for fixtureID := range declared {
		if !measured[fixtureID] {
			problems = append(
				problems,
				fmt.Sprintf("benchmark fixture %s declared in fixtureManifest was not measured", fixtureID),
			)
		}
	}
	return uniqueEvidenceProblems(problems)
}

func validateClosureBundleProviderManifest(issue string, raw json.RawMessage) (voice.ProviderManifest, []string) {
	if len(raw) == 0 {
		return voice.ProviderManifest{}, []string{"providerManifest is required"}
	}
	manifest, err := voice.DecodeProviderManifest(string(raw))
	if err != nil {
		return voice.ProviderManifest{}, []string{err.Error()}
	}
	problems := voice.ValidateProviderManifest(manifest)
	if !manifest.Capabilities.Offline {
		problems = append(problems, "providerManifest must declare offline capability for closure")
	}
	if len(manifest.Credentials) > 0 {
		problems = append(problems, "providerManifest must not require credentials for local closure evidence")
	}
	switch issue {
	case "JUT-11":
		provider := strings.ToLower(manifest.ID + " " + manifest.Name)
		if manifest.Kind != voice.ProviderKindWakeWord {
			problems = append(problems, "JUT-11 providerManifest kind must be wake-word")
		}
		if !strings.Contains(provider, "pmdroid") {
			problems = append(problems, "JUT-11 providerManifest must identify pmdroid")
		}
		if !providerManifestHasMaintainer(manifest, "pmdroid") {
			problems = append(problems, "JUT-11 providerManifest contribution maintainers must include pmdroid")
		}
		if !strings.Contains(provider, "microwakeword") && !strings.Contains(provider, "micro-wake-word") {
			problems = append(problems, "JUT-11 providerManifest must identify microWakeWord")
		}
	case "JUT-13":
		provider := strings.ToLower(manifest.ID + " " + manifest.Name)
		if manifest.Kind != voice.ProviderKindSTT {
			problems = append(problems, "JUT-13 providerManifest kind must be stt")
		}
		if !strings.Contains(provider, "mutablelogic") {
			problems = append(problems, "JUT-13 providerManifest must identify mutablelogic")
		}
		if !providerManifestHasMaintainer(manifest, "mutablelogic") {
			problems = append(problems, "JUT-13 providerManifest contribution maintainers must include mutablelogic")
		}
		if !strings.Contains(provider, "go-whisper") && !strings.Contains(provider, "gowhisper") {
			problems = append(problems, "JUT-13 providerManifest must identify go-whisper")
		}
		if manifest.Transport.Type != "http-json" && manifest.Transport.Type != "command" {
			problems = append(problems, "JUT-13 providerManifest transport must be http-json or command")
		}
	}
	return manifest, uniqueEvidenceProblems(problems)
}

func providerManifestHasMaintainer(manifest voice.ProviderManifest, maintainer string) bool {
	want := strings.ToLower(strings.TrimSpace(maintainer))
	for _, candidate := range manifest.Contribution.Maintainers {
		if strings.ToLower(strings.TrimSpace(candidate)) == want {
			return true
		}
	}
	return false
}

func validateClosureBundleFixtureManifest(
	issue string,
	raw json.RawMessage,
) (voice.BenchmarkFixtureSetManifest, []string) {
	if len(raw) == 0 {
		return voice.BenchmarkFixtureSetManifest{}, []string{"fixtureManifest is required"}
	}
	manifest, err := voice.DecodeBenchmarkFixtureSetManifest(string(raw))
	if err != nil {
		return voice.BenchmarkFixtureSetManifest{}, []string{err.Error()}
	}
	var problems []string
	if safeEvidenceIssue(manifest.Issue) != issue {
		problems = append(problems, "fixtureManifest issue must match closure issue")
	}
	problems = append(problems, validateFixtureManifestAcceptancePreset(manifest)...)
	problems = append(problems, validateClosureFixtureManifestProvenance(manifest)...)
	return manifest, uniqueEvidenceProblems(problems)
}

func validateClosureFixtureManifestProvenance(manifest voice.BenchmarkFixtureSetManifest) []string {
	var problems []string
	fixtureBySHA := map[string]string{}
	for _, fixture := range manifest.Fixtures {
		fixtureID := strings.TrimSpace(fixture.ID)
		if fixtureID == "" {
			fixtureID = "unknown"
		}
		sha := strings.ToLower(strings.TrimSpace(fixture.SHA256))
		if !isSafeSHA256(sha) {
			problems = append(
				problems,
				fmt.Sprintf("fixture manifest fixture %s must declare sha256:<64 hex characters>", fixtureID),
			)
		} else if previousFixtureID := fixtureBySHA[sha]; previousFixtureID != "" {
			problems = append(
				problems,
				fmt.Sprintf(
					"fixture manifest fixtures %s and %s must not share sha256",
					previousFixtureID,
					fixtureID,
				),
			)
		} else {
			fixtureBySHA[sha] = fixtureID
		}
		source := strings.ToLower(strings.TrimSpace(fixture.Source))
		if source == "" || strings.Contains(source, "replace-with") || strings.Contains(source, "placeholder") {
			problems = append(
				problems,
				fmt.Sprintf("fixture manifest fixture %s must declare concrete source", fixtureID),
			)
		}
	}
	return problems
}

func sanitizeProviderClosureDecision(decision providerClosureDecision) providerClosureDecision {
	return providerClosureDecision{
		Status:    safeEvidenceToken(decision.Status),
		Rationale: safeEvidenceNote(decision.Rationale),
	}
}

func validateProviderClosureDecision(issue string, decision providerClosureDecision) []string {
	var problems []string
	if decision.Status == "" {
		problems = append(problems, "decision status is required")
	}
	if !isConcreteEvidenceRationale(decision.Rationale) {
		problems = append(problems, "decision rationale must explain the measured evidence")
	}
	switch issue {
	case "JUT-11":
		switch decision.Status {
		case "adopt-optional-provider", "defer", "reject":
		default:
			problems = append(problems, "JUT-11 decision status must be adopt-optional-provider, defer, or reject")
		}
	case "JUT-13":
		switch decision.Status {
		case "first-class-provider-pack", "documented-external-provider", "defer":
		default:
			problems = append(
				problems,
				"JUT-13 decision status must be first-class-provider-pack, documented-external-provider, or defer",
			)
		}
	}
	return uniqueEvidenceProblems(problems)
}

func validateClosureBundleBenchmark(issue string, raw json.RawMessage) (voice.BenchmarkReport, []string) {
	if len(raw) == 0 {
		return voice.BenchmarkReport{}, []string{"benchmarkReport is required"}
	}
	report, err := voice.DecodeBenchmarkReport(raw)
	if err != nil {
		return voice.BenchmarkReport{}, []string{err.Error()}
	}
	expectations, ok := voice.BenchmarkAcceptanceExpectations(issue)
	if !ok {
		return report, []string{fmt.Sprintf("no acceptance preset is defined for issue %q", issue)}
	}
	problems := voice.ValidateBenchmarkReport(report, expectations)
	if !isConcreteBenchmarkEnvironmentRuntime(report.Environment) {
		problems = append(problems, "benchmark environment runtime must identify a concrete OS/arch and Go version")
	}
	return report, uniqueEvidenceProblems(problems)
}

func isConcreteBenchmarkEnvironmentRuntime(environment voice.BenchmarkEnvironment) bool {
	runtime := fmt.Sprintf("%s/%s %s", environment.OS, environment.Arch, environment.GoVersion)
	return isConcreteEvidenceRuntime(runtime)
}

func validateClosureBundleBuilds(issue string, builds []providerBuildEvidence) []string {
	if len(builds) == 0 {
		return []string{"buildEvidence is required"}
	}
	var problems []string
	validBuilds := 0
	for index, build := range builds {
		artifactProblems := validateProviderBuildEvidenceArtifact(build)
		if len(artifactProblems) > 0 {
			problems = appendPrefixedProblems(problems, fmt.Sprintf("buildEvidence[%d]", index), artifactProblems)
			continue
		}
		build = sanitizeProviderBuildEvidence(build)
		if build.Issue != issue {
			problems = appendPrefixedProblems(
				problems,
				fmt.Sprintf("buildEvidence[%d]", index),
				[]string{"build evidence issue must match closure issue"},
			)
			continue
		}
		buildProblems := validateProviderBuildEvidence(build)
		if len(buildProblems) > 0 {
			problems = appendPrefixedProblems(problems, fmt.Sprintf("buildEvidence[%d]", index), buildProblems)
			continue
		}
		validBuilds++
	}
	if validBuilds == 0 {
		switch issue {
		case "JUT-11":
			problems = append(
				problems,
				"JUT-11 closure requires at least one successful pmdroid/microWakeWord build evidence row",
			)
		case "JUT-13":
			problems = append(problems, "JUT-13 closure requires at least one successful go-whisper build evidence row")
		}
	}
	return uniqueEvidenceProblems(problems)
}

func validateClosureBundlePackaging(
	issue string,
	evidence providerPackagingEvidence,
) (providerPackagingEvidence, []string) {
	packagingArtifactProblems := validateProviderPackagingEvidenceArtifact(evidence)
	packaging := sanitizeProviderPackagingEvidence(evidence)
	problems := append([]string{}, packagingArtifactProblems...)
	if packaging.Issue != issue {
		problems = append(problems, "packaging evidence issue must match closure issue")
	}
	problems = append(problems, validateProviderPackagingEvidence(packaging)...)
	return packaging, uniqueEvidenceProblems(problems)
}

func validateClosureBundleModels(issue string, models []providerModelEvidence) ([]providerModelEvidence, []string) {
	if len(models) == 0 {
		return nil, []string{"modelEvidence is required"}
	}
	var problems []string
	var validModelEvidence []providerModelEvidence
	validModels := 0
	hasESPHome := false
	hasOHF := false
	for index, model := range models {
		artifactProblems := validateProviderModelEvidenceArtifact(model)
		if len(artifactProblems) > 0 {
			problems = appendPrefixedProblems(problems, fmt.Sprintf("modelEvidence[%d]", index), artifactProblems)
			continue
		}
		model = sanitizeProviderModelEvidence(model)
		if model.Issue != issue {
			problems = appendPrefixedProblems(
				problems,
				fmt.Sprintf("modelEvidence[%d]", index),
				[]string{"model evidence issue must match closure issue"},
			)
			continue
		}
		modelProblems := validateProviderModelEvidence(model)
		if len(modelProblems) > 0 {
			problems = appendPrefixedProblems(problems, fmt.Sprintf("modelEvidence[%d]", index), modelProblems)
			continue
		}
		validModels++
		validModelEvidence = append(validModelEvidence, model)
		source := safeEvidenceToken(model.ModelSource)
		if strings.Contains(source, "esphome") {
			hasESPHome = true
		}
		if strings.Contains(source, "ohf") {
			hasOHF = true
		}
	}
	switch issue {
	case "JUT-11":
		if !hasESPHome {
			problems = append(problems, "JUT-11 closure requires compatible ESPHome model evidence")
		}
		if !hasOHF {
			problems = append(problems, "JUT-11 closure requires compatible OHF model evidence")
		}
	case "JUT-13":
		if validModels == 0 {
			problems = append(problems, "JUT-13 closure requires at least one compatible go-whisper model evidence row")
		}
	}
	return validModelEvidence, uniqueEvidenceProblems(problems)
}

func validateProviderBuildEvidenceArtifact(evidence providerBuildEvidence) []string {
	var problems []string
	if len(evidence.Problems) > 0 {
		problems = append(problems, "generated build evidence artifact must not carry validation problems")
	}
	if !evidence.ClosureEvidence {
		problems = append(problems, "generated build evidence artifact must have closureEvidence true")
	}
	return problems
}

func validateProviderPackagingEvidenceArtifact(evidence providerPackagingEvidence) []string {
	var problems []string
	if len(evidence.Problems) > 0 {
		problems = append(problems, "generated packaging evidence artifact must not carry validation problems")
	}
	if !evidence.PackagingEvidenceComplete {
		problems = append(problems, "generated packaging evidence artifact must have packagingEvidenceComplete true")
	}
	return problems
}

func validateProviderModelEvidenceArtifact(evidence providerModelEvidence) []string {
	var problems []string
	if len(evidence.Problems) > 0 {
		problems = append(problems, "generated model evidence artifact must not carry validation problems")
	}
	if !evidence.ClosureEvidence {
		problems = append(problems, "generated model evidence artifact must have closureEvidence true")
	}
	return problems
}

func validateClosureBundleModelsAgainstProviderManifest(
	issue string,
	models []providerModelEvidence,
	manifest voice.ProviderManifest,
) []string {
	if issue != "JUT-11" || len(models) == 0 {
		return nil
	}
	declared := map[string]bool{}
	for _, model := range manifest.WakeWord.Models {
		modelID := safeEvidenceToken(model.ID)
		if modelID != "" {
			declared[modelID] = true
		}
	}
	var problems []string
	for _, model := range models {
		modelID := safeEvidenceToken(model.ModelID)
		if modelID != "" && !declared[modelID] {
			problems = append(
				problems,
				fmt.Sprintf("JUT-11 model evidence %s must be declared in providerManifest wakeWord.models", modelID),
			)
		}
	}
	return uniqueEvidenceProblems(problems)
}

func validateClosureBundlePackagingAgainstProviderManifest(
	issue string,
	packaging providerPackagingEvidence,
	manifest voice.ProviderManifest,
) []string {
	switch issue {
	case "JUT-11", "JUT-13":
	default:
		return nil
	}
	raspberryPiPackaged := packagingTargetStatus(packaging, "raspberry-pi-arm64") == "succeeded"
	raspberryPiManifest := manifest.Hardware["raspberryPi"]
	switch {
	case raspberryPiManifest && !raspberryPiPackaged:
		return []string{
			"providerManifest hardware.raspberryPi requires succeeded raspberry-pi-arm64 packaging evidence",
		}
	case raspberryPiPackaged && !raspberryPiManifest:
		return []string{
			"succeeded raspberry-pi-arm64 packaging evidence requires providerManifest hardware.raspberryPi",
		}
	default:
		return nil
	}
}

func packagingTargetStatus(packaging providerPackagingEvidence, target string) string {
	for _, candidate := range packaging.Targets {
		if candidate.Target == target {
			return candidate.Status
		}
	}
	return ""
}

func validateClosureBundleDecisionAgainstPackaging(
	issue string,
	decision providerClosureDecision,
	packaging providerPackagingEvidence,
) []string {
	switch issue {
	case "JUT-11":
		if decision.Status != "adopt-optional-provider" {
			return nil
		}
	case "JUT-13":
		if decision.Status == "documented-external-provider" {
			if packagingTargetStatus(packaging, "cpu-only") != "succeeded" &&
				packagingTargetStatus(packaging, "container-linux") != "succeeded" {
				return []string{
					"JUT-13 decision documented-external-provider requires cpu-only or container-linux packaging to have succeeded",
				}
			}
			return nil
		}
		if decision.Status != "first-class-provider-pack" {
			return nil
		}
	default:
		return nil
	}
	var problems []string
	for _, target := range requiredProviderPackagingTargets(issue) {
		if packagingTargetStatus(packaging, target) != "succeeded" {
			problems = append(
				problems,
				fmt.Sprintf(
					"%s decision %s requires packaging target %s to have succeeded",
					issue,
					decision.Status,
					target,
				),
			)
		}
	}
	return uniqueEvidenceProblems(problems)
}

func requiredProviderPackagingTargets(issue string) []string {
	switch issue {
	case "JUT-11":
		return []string{"macos-native", "linux-native", "raspberry-pi-arm64"}
	case "JUT-13":
		return []string{"cpu-only", "macos-metal", "container-linux", "raspberry-pi-arm64"}
	default:
		return nil
	}
}

func validateClosureBundleBenchmarkModel(
	issue string,
	report voice.BenchmarkReport,
	models []providerModelEvidence,
) []string {
	if len(models) == 0 {
		return nil
	}
	reportModelID := safeEvidenceToken(report.Environment.ModelID)
	reportModelHash := safeEvidenceModelHash(report.Environment.ModelHash)
	modelMatched := false
	for _, model := range models {
		if safeEvidenceToken(model.ModelID) == reportModelID &&
			safeEvidenceModelHash(model.ModelHash) == reportModelHash {
			modelMatched = true
			if benchmarkEnvironmentMatchesModelRuntime(report.Environment, model.Runtime) {
				return nil
			}
		}
	}
	if modelMatched {
		switch issue {
		case "JUT-11":
			return []string{
				"JUT-11 benchmark runtime must match compatible pmdroid/microWakeWord model evidence runtime",
			}
		case "JUT-13":
			return []string{"JUT-13 benchmark runtime must match compatible go-whisper model evidence runtime"}
		default:
			return nil
		}
	}
	switch issue {
	case "JUT-11":
		return []string{"JUT-11 benchmark modelId/modelHash must match compatible pmdroid/microWakeWord model evidence"}
	case "JUT-13":
		return []string{"JUT-13 benchmark modelId/modelHash must match compatible go-whisper model evidence"}
	default:
		return nil
	}
}

func benchmarkEnvironmentMatchesModelRuntime(environment voice.BenchmarkEnvironment, modelRuntime string) bool {
	expected := fmt.Sprintf(
		"%s/%s %s",
		strings.TrimSpace(environment.OS),
		strings.TrimSpace(environment.Arch),
		strings.TrimSpace(environment.GoVersion),
	)
	return normalizedEvidenceRuntime(expected) == normalizedEvidenceRuntime(modelRuntime)
}

func normalizedEvidenceRuntime(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastSpace := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func sanitizeProviderBuildEvidence(e providerBuildEvidence) providerBuildEvidence {
	e.Issue = safeEvidenceIssue(e.Issue)
	e.ProviderID = safeEvidenceProvider(e.ProviderID)
	e.ProviderKind = safeEvidenceToken(e.ProviderKind)
	e.Target = safeEvidenceToken(e.Target)
	e.CommandID = safeEvidenceToken(e.CommandID)
	e.Status = safeEvidenceToken(e.Status)
	e.ErrorCode = safeEvidenceToken(e.ErrorCode)
	e.MissingDependencies = sanitizeEvidenceStringList(e.MissingDependencies)
	e.Runtime = safeEvidenceNote(e.Runtime)
	e.Notes = safeEvidenceNote(e.Notes)
	e.Problems = nil
	e.ClosureEvidence = false
	return e
}

func sanitizeProviderPackagingEvidence(e providerPackagingEvidence) providerPackagingEvidence {
	e.Issue = safeEvidenceIssue(e.Issue)
	e.ProviderID = safeEvidenceProvider(e.ProviderID)
	e.ProviderKind = safeEvidenceToken(e.ProviderKind)
	e.Runtime = safeEvidenceNote(e.Runtime)
	e.Notes = safeEvidenceNote(e.Notes)
	targets := make([]providerPackagingTarget, 0, len(e.Targets))
	for _, target := range e.Targets {
		targets = append(targets, providerPackagingTarget{
			Target: safeEvidenceToken(target.Target),
			Status: safeEvidenceToken(target.Status),
		})
	}
	e.Targets = targets
	e.Problems = nil
	e.PackagingEvidenceComplete = false
	return e
}

func sanitizeProviderModelEvidence(e providerModelEvidence) providerModelEvidence {
	e.Issue = safeEvidenceIssue(e.Issue)
	e.ProviderID = safeEvidenceProvider(e.ProviderID)
	e.ProviderKind = safeEvidenceToken(e.ProviderKind)
	e.ModelID = safeEvidenceToken(e.ModelID)
	e.ModelHash = safeEvidenceModelHash(e.ModelHash)
	e.ModelSource = safeEvidenceToken(e.ModelSource)
	e.ModelFormat = safeEvidenceToken(e.ModelFormat)
	e.CompatibilityStatus = safeEvidenceToken(e.CompatibilityStatus)
	e.RuntimeStatus = safeEvidenceToken(e.RuntimeStatus)
	e.Runtime = safeEvidenceNote(e.Runtime)
	e.Notes = safeEvidenceNote(e.Notes)
	e.Problems = nil
	e.ClosureEvidence = false
	return e
}

func validateProviderBuildEvidence(evidence providerBuildEvidence) []string {
	var problems []string
	if !isRFC3339EvidenceTime(evidence.GeneratedAt) {
		problems = append(problems, "generatedAt must be RFC3339")
	}
	if evidence.Issue == "" {
		problems = append(problems, "issue is required")
	}
	if evidence.ProviderID == "" {
		problems = append(problems, "providerId is required")
	}
	if evidence.ProviderKind == "" {
		problems = append(problems, "providerKind is required")
	}
	if evidence.Target == "" {
		problems = append(problems, "build target is required")
	}
	if evidence.CommandID == "" {
		problems = append(problems, "build command ID is required")
	}
	if !isConcreteEvidenceRuntime(evidence.Runtime) {
		problems = append(problems, "runtime must identify a concrete OS/arch and toolchain")
	}
	switch evidence.Status {
	case "succeeded":
	case "failed", "blocked", "not-run":
		problems = append(problems, "provider build did not succeed")
	default:
		problems = append(problems, "build status must be succeeded, failed, blocked, or not-run")
	}
	if evidence.Status == "failed" && evidence.ErrorCode == "" && len(evidence.MissingDependencies) == 0 {
		problems = append(problems, "failed provider build requires errorCode or missingDependencies")
	}
	switch evidence.Issue {
	case "JUT-11":
		provider := strings.ToLower(evidence.ProviderID)
		if !strings.Contains(provider, "pmdroid") || !strings.Contains(provider, "microwakeword") {
			problems = append(problems, "JUT-11 build evidence provider must identify pmdroid/microWakeWord")
		}
		if evidence.ProviderKind != "wake-word" {
			problems = append(problems, "JUT-11 build evidence providerKind must be wake-word")
		}
	case "JUT-13":
		if !strings.Contains(strings.ToLower(evidence.ProviderID), "go-whisper") {
			problems = append(problems, "JUT-13 build evidence provider must identify go-whisper")
		}
		if evidence.ProviderKind != "stt" {
			problems = append(problems, "JUT-13 build evidence providerKind must be stt")
		}
	default:
		problems = append(problems, "build evidence issue must be JUT-11 or JUT-13")
	}
	return uniqueEvidenceProblems(problems)
}

func validateProviderPackagingEvidence(evidence providerPackagingEvidence) []string {
	var problems []string
	if !isRFC3339EvidenceTime(evidence.GeneratedAt) {
		problems = append(problems, "generatedAt must be RFC3339")
	}
	if evidence.Issue == "" {
		problems = append(problems, "issue is required")
	}
	if evidence.ProviderID == "" {
		problems = append(problems, "providerId is required")
	}
	if evidence.ProviderKind == "" {
		problems = append(problems, "providerKind is required")
	}
	if !isConcreteEvidenceRuntime(evidence.Runtime) {
		problems = append(problems, "runtime must identify a concrete OS/arch and toolchain")
	}
	if len(evidence.Targets) == 0 {
		problems = append(problems, "at least one packaging target is required")
	}
	targetsByName := map[string]string{}
	for _, target := range evidence.Targets {
		if target.Target == "" {
			problems = append(problems, "packaging target name is required")
			continue
		}
		if _, exists := targetsByName[target.Target]; exists {
			problems = append(problems, fmt.Sprintf("packaging target %s is duplicated", target.Target))
			continue
		}
		targetsByName[target.Target] = target.Status
		switch target.Status {
		case "succeeded", "failed", "blocked", "unsupported":
		case "not-run":
			problems = append(problems, fmt.Sprintf("packaging target %s has not been evaluated", target.Target))
		default:
			problems = append(
				problems,
				fmt.Sprintf(
					"packaging target %s status must be succeeded, failed, blocked, unsupported, or not-run",
					target.Target,
				),
			)
		}
		if target.Status != "succeeded" && evidence.Notes == "" {
			problems = append(
				problems,
				fmt.Sprintf("packaging target %s requires notes when status is not succeeded", target.Target),
			)
		}
	}
	switch evidence.Issue {
	case "JUT-11":
		provider := strings.ToLower(evidence.ProviderID)
		if !strings.Contains(provider, "pmdroid") || !strings.Contains(provider, "microwakeword") {
			problems = append(problems, "JUT-11 packaging evidence provider must identify pmdroid/microWakeWord")
		}
		if evidence.ProviderKind != "wake-word" {
			problems = append(problems, "JUT-11 packaging evidence providerKind must be wake-word")
		}
		problems = appendMissingPackagingTargets(
			problems,
			targetsByName,
			requiredProviderPackagingTargets(evidence.Issue),
		)
	case "JUT-13":
		if !strings.Contains(strings.ToLower(evidence.ProviderID), "go-whisper") {
			problems = append(problems, "JUT-13 packaging evidence provider must identify go-whisper")
		}
		if evidence.ProviderKind != "stt" {
			problems = append(problems, "JUT-13 packaging evidence providerKind must be stt")
		}
		problems = appendMissingPackagingTargets(
			problems,
			targetsByName,
			requiredProviderPackagingTargets(evidence.Issue),
		)
	default:
		problems = append(problems, "packaging evidence issue must be JUT-11 or JUT-13")
	}
	return uniqueEvidenceProblems(problems)
}

func appendMissingPackagingTargets(problems []string, targetsByName map[string]string, required []string) []string {
	for _, target := range required {
		if _, ok := targetsByName[target]; !ok {
			problems = append(problems, fmt.Sprintf("packaging target %s is required", target))
		}
	}
	return problems
}

func validateProviderModelEvidence(evidence providerModelEvidence) []string {
	var problems []string
	if !isRFC3339EvidenceTime(evidence.GeneratedAt) {
		problems = append(problems, "generatedAt must be RFC3339")
	}
	if evidence.Issue == "" {
		problems = append(problems, "issue is required")
	}
	if evidence.ProviderID == "" {
		problems = append(problems, "providerId is required")
	}
	if evidence.ProviderKind == "" {
		problems = append(problems, "providerKind is required")
	}
	if evidence.ModelID == "" {
		problems = append(problems, "modelId is required")
	}
	if !isSafeSHA256(evidence.ModelHash) {
		problems = append(problems, "modelHash must be sha256:<64 hex characters>")
	}
	if evidence.ModelSource == "" {
		problems = append(problems, "modelSource is required")
	}
	if evidence.ModelFormat == "" {
		problems = append(problems, "modelFormat is required")
	}
	if !isConcreteEvidenceRuntime(evidence.Runtime) {
		problems = append(problems, "runtime must identify a concrete OS/arch and toolchain")
	}
	switch evidence.CompatibilityStatus {
	case "compatible":
	case "incompatible", "blocked", "untested":
		problems = append(problems, "model compatibility is not proven")
	default:
		problems = append(problems, "model compatibility must be compatible, incompatible, blocked, or untested")
	}
	switch evidence.RuntimeStatus {
	case "loaded":
	case "failed", "blocked", "not-run":
		problems = append(problems, "model runtime load is not proven")
	default:
		problems = append(problems, "model runtime status must be loaded, failed, blocked, or not-run")
	}
	switch evidence.Issue {
	case "JUT-11":
		provider := strings.ToLower(evidence.ProviderID)
		if !strings.Contains(provider, "pmdroid") || !strings.Contains(provider, "microwakeword") {
			problems = append(problems, "JUT-11 model evidence provider must identify pmdroid/microWakeWord")
		}
		if evidence.ProviderKind != "wake-word" {
			problems = append(problems, "JUT-11 model evidence providerKind must be wake-word")
		}
		if evidence.ModelFormat != "tflite" {
			problems = append(problems, "JUT-11 model evidence modelFormat must be tflite")
		}
	case "JUT-13":
		if !strings.Contains(strings.ToLower(evidence.ProviderID), "go-whisper") {
			problems = append(problems, "JUT-13 model evidence provider must identify go-whisper")
		}
		if evidence.ProviderKind != "stt" {
			problems = append(problems, "JUT-13 model evidence providerKind must be stt")
		}
	default:
		problems = append(problems, "model evidence issue must be JUT-11 or JUT-13")
	}
	return uniqueEvidenceProblems(problems)
}

func (e providerBuildEvidence) markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Provider Build Evidence: %s\n\n", e.Issue)
	fmt.Fprintf(&b, "- Provider: `%s`\n", e.ProviderID)
	fmt.Fprintf(&b, "- Kind: `%s`\n", e.ProviderKind)
	fmt.Fprintf(&b, "- Target: `%s`\n", e.Target)
	fmt.Fprintf(&b, "- Command ID: `%s`\n", e.CommandID)
	fmt.Fprintf(&b, "- Status: `%s`\n", e.Status)
	if e.ExitCode != 0 {
		fmt.Fprintf(&b, "- Exit code: `%d`\n", e.ExitCode)
	}
	if e.ErrorCode != "" {
		fmt.Fprintf(&b, "- Error code: `%s`\n", e.ErrorCode)
	}
	if len(e.MissingDependencies) > 0 {
		fmt.Fprintf(&b, "- Missing dependencies: `%s`\n", strings.Join(e.MissingDependencies, "`, `"))
	}
	fmt.Fprintf(&b, "- Runtime: `%s`\n", e.Runtime)
	fmt.Fprintf(&b, "- Closure evidence: `%t`\n", e.ClosureEvidence)
	if e.Notes != "" {
		fmt.Fprintf(&b, "- Notes: %s\n", e.Notes)
	}
	if len(e.Problems) > 0 {
		b.WriteString("\nValidation problems:\n")
		for _, problem := range e.Problems {
			fmt.Fprintf(&b, "- %s\n", problem)
		}
	}
	return strings.TrimSpace(b.String())
}

func (e providerPackagingEvidence) markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Provider Packaging Evidence: %s\n\n", e.Issue)
	fmt.Fprintf(&b, "- Provider: `%s`\n", e.ProviderID)
	fmt.Fprintf(&b, "- Kind: `%s`\n", e.ProviderKind)
	for _, target := range e.Targets {
		fmt.Fprintf(&b, "- Target `%s`: `%s`\n", target.Target, target.Status)
	}
	fmt.Fprintf(&b, "- Runtime: `%s`\n", e.Runtime)
	fmt.Fprintf(&b, "- Packaging evidence complete: `%t`\n", e.PackagingEvidenceComplete)
	if e.Notes != "" {
		fmt.Fprintf(&b, "- Notes: %s\n", e.Notes)
	}
	if len(e.Problems) > 0 {
		b.WriteString("\nValidation problems:\n")
		for _, problem := range e.Problems {
			fmt.Fprintf(&b, "- %s\n", problem)
		}
	}
	return strings.TrimSpace(b.String())
}

func (b providerClosureBundle) markdown(problems []string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "### Provider Closure Bundle: %s\n\n", b.Issue)
	fmt.Fprintf(&out, "- Decision: `%s`\n", firstNonEmpty(b.DecisionStatus, "missing"))
	if b.DecisionRationale != "" {
		fmt.Fprintf(&out, "- Decision rationale: %s\n", b.DecisionRationale)
	}
	fmt.Fprintf(&out, "- Provider manifest accepted: `%t`\n", b.ProviderManifestOK)
	if b.ProviderManifestID != "" {
		fmt.Fprintf(&out, "- Provider manifest: `%s` (`%s`)\n", b.ProviderManifestID, b.ProviderManifestKind)
	}
	fmt.Fprintf(&out, "- Fixture manifest accepted: `%t`\n", b.FixtureManifestOK)
	fmt.Fprintf(&out, "- Fixture manifest entries: %d\n", b.FixtureManifestCount)
	fmt.Fprintf(&out, "- Build evidence rows: %d\n", b.BuildEvidenceCount)
	fmt.Fprintf(&out, "- Packaging evidence complete: `%t`\n", b.PackagingComplete)
	fmt.Fprintf(&out, "- Model evidence rows: %d\n", b.ModelEvidenceCount)
	fmt.Fprintf(&out, "- Benchmark accepted: `%t`\n", b.BenchmarkAccepted)
	if b.Issue == "JUT-11" {
		fmt.Fprintf(&out, "- Baseline accepted: `%t`\n", b.BaselineAccepted)
		fmt.Fprintf(&out, "- Comparison accepted: `%t`\n", b.ComparisonAccepted)
	}
	fmt.Fprintf(&out, "- Closure bundle complete: `%t`\n", b.ClosureBundleSuccess)
	if len(problems) > 0 {
		out.WriteString("\nValidation problems:\n")
		for _, problem := range problems {
			fmt.Fprintf(&out, "- %s\n", safeEvidenceNote(problem))
		}
	}
	return strings.TrimSpace(out.String())
}

func (e providerModelEvidence) markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Provider Model Evidence: %s\n\n", e.Issue)
	fmt.Fprintf(&b, "- Provider: `%s`\n", e.ProviderID)
	fmt.Fprintf(&b, "- Kind: `%s`\n", e.ProviderKind)
	fmt.Fprintf(&b, "- Model: `%s`\n", e.ModelID)
	fmt.Fprintf(&b, "- Model hash: `%s`\n", e.ModelHash)
	fmt.Fprintf(&b, "- Source: `%s`\n", e.ModelSource)
	fmt.Fprintf(&b, "- Format: `%s`\n", e.ModelFormat)
	fmt.Fprintf(&b, "- Compatibility: `%s`\n", e.CompatibilityStatus)
	fmt.Fprintf(&b, "- Runtime status: `%s`\n", e.RuntimeStatus)
	fmt.Fprintf(&b, "- Runtime: `%s`\n", e.Runtime)
	fmt.Fprintf(&b, "- Closure evidence: `%t`\n", e.ClosureEvidence)
	if e.Notes != "" {
		fmt.Fprintf(&b, "- Notes: %s\n", e.Notes)
	}
	if len(e.Problems) > 0 {
		b.WriteString("\nValidation problems:\n")
		for _, problem := range e.Problems {
			fmt.Fprintf(&b, "- %s\n", problem)
		}
	}
	return strings.TrimSpace(b.String())
}

func safeEvidenceProvider(value string) string {
	return strings.Trim(safeEvidenceToken(value), "-")
}

func safeEvidenceIssue(value string) string {
	return strings.ToUpper(safeEvidenceToken(value))
}

func safeEvidenceModelHash(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "sha256:") {
		return "sha256:" + safeHex(value[len("sha256:"):])
	}
	return safeEvidenceToken(value)
}

func safeEvidenceToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if allowed {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func safeEvidenceList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		item := safeEvidenceToken(part)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func sanitizeEvidenceStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		item := safeEvidenceToken(value)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func safeEvidenceTargetStatusList(value string) []providerPackagingTarget {
	parts := strings.Split(value, ",")
	out := make([]providerPackagingTarget, 0, len(parts))
	for _, part := range parts {
		name, status, ok := strings.Cut(part, "=")
		if !ok {
			name = part
			status = ""
		}
		target := safeEvidenceToken(name)
		if target == "" {
			continue
		}
		out = append(out, providerPackagingTarget{
			Target: target,
			Status: safeEvidenceToken(status),
		})
	}
	return out
}

func safeHex(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'f':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isSafeSHA256(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, r := range value[len("sha256:"):] {
		if (r < 'a' || r > 'f') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func isConcreteEvidenceRuntime(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, placeholder := range []string{"replace-with", "placeholder", "example", "unknown", "not-provided", "todo", "tbd"} {
		if strings.Contains(value, placeholder) {
			return false
		}
	}
	hasOS := false
	for _, osToken := range []string{"linux", "darwin", "macos", "windows"} {
		if strings.Contains(value, osToken) {
			hasOS = true
			break
		}
	}
	hasArch := false
	for _, archToken := range []string{"amd64", "arm64", "arm", "x64"} {
		if strings.Contains(value, archToken) {
			hasArch = true
			break
		}
	}
	return hasOS && hasArch && strings.Contains(value, "go")
}

func isRFC3339EvidenceTime(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.Contains(strings.ToLower(value), "replace-with") ||
		strings.Contains(strings.ToLower(value), "placeholder") {
		return false
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func isConcreteEvidenceRationale(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) < 24 {
		return false
	}
	for _, placeholder := range []string{"replace-with", "placeholder", "example", "unknown", "not-provided", "todo", "tbd"} {
		if strings.Contains(value, placeholder) {
			return false
		}
	}
	return true
}

func safeEvidenceNote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune(" .,;:()_+-", r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
		if b.Len() >= 240 {
			break
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func uniqueEvidenceProblems(problems []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(problems))
	for _, problem := range problems {
		if problem == "" || seen[problem] {
			continue
		}
		seen[problem] = true
		out = append(out, problem)
	}
	return out
}

func appendPrefixedProblems(problems []string, prefix string, incoming []string) []string {
	for _, problem := range incoming {
		if problem == "" {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s: %s", prefix, problem))
	}
	return problems
}

func validateComparisonAcceptancePreset(
	candidate voice.BenchmarkReport,
	baseline voice.BenchmarkReport,
	stdout io.Writer,
	stderr io.Writer,
	issue string,
) int {
	candidateExpectations, ok := voice.BenchmarkAcceptanceExpectations(firstNonEmpty(issue, candidate.Issue))
	if !ok {
		fmt.Fprintf(stderr, "no acceptance preset is defined for issue %q\n", firstNonEmpty(issue, candidate.Issue))
		return 2
	}
	baselineExpectations, ok := voice.BenchmarkAcceptanceExpectations(firstNonEmpty(issue, baseline.Issue))
	if !ok {
		fmt.Fprintf(
			stderr,
			"no acceptance preset is defined for baseline issue %q\n",
			firstNonEmpty(issue, baseline.Issue),
		)
		return 2
	}
	candidateProblems := voice.ValidateBenchmarkReport(candidate, candidateExpectations)
	baselineProblems := voice.ValidateBenchmarkReport(baseline, baselineExpectations)
	comparisonGaps := []string{}
	if len(candidateProblems) == 0 && len(baselineProblems) == 0 {
		comparison := voice.CompareBenchmarkReports(candidate, baseline)
		comparisonGaps = comparison.Gaps
		if len(comparisonGaps) == 0 {
			return 0
		}
		fmt.Fprintf(stdout, "%s\n\n", comparison.EvidenceMarkdown())
	}
	if len(candidateProblems) > 0 {
		fmt.Fprintf(stdout, "%s\n\n", candidate.EvidenceMarkdown(candidateProblems))
	}
	if len(baselineProblems) > 0 {
		fmt.Fprintf(stdout, "%s\n\n", baseline.EvidenceMarkdown(baselineProblems))
	}
	if len(comparisonGaps) > 0 {
		fmt.Fprintf(stderr, "benchmark comparison has %d gap(s)\n", len(comparisonGaps))
		return 1
	}
	fmt.Fprintf(
		stderr,
		"benchmark comparison preset validation failed: candidate=%d baseline=%d problem(s)\n",
		len(candidateProblems),
		len(baselineProblems),
	)
	return 1
}

type benchmarkExpectationInputs struct {
	acceptancePreset         bool
	explicitFlags            map[string]bool
	issue                    string
	kind                     string
	minResults               int
	requireModelHash         bool
	allowGaps                bool
	allowProviderFailures    bool
	requireWakeMatches       bool
	requireTranscriptMatches bool
}

func benchmarkExpectations(
	report voice.BenchmarkReport,
	inputs benchmarkExpectationInputs,
) (voice.BenchmarkReportExpectations, bool) {
	expectations := voice.BenchmarkReportExpectations{}
	if inputs.acceptancePreset {
		preset, ok := voice.BenchmarkAcceptanceExpectations(firstNonEmpty(inputs.issue, report.Issue))
		if !ok {
			return voice.BenchmarkReportExpectations{}, false
		}
		expectations = preset
	}

	if inputs.explicitFlags["issue"] || !inputs.acceptancePreset {
		expectations.Issue = inputs.issue
	}
	if inputs.explicitFlags["kind"] || !inputs.acceptancePreset {
		expectations.Kind = inputs.kind
	}
	if inputs.explicitFlags["min-results"] || !inputs.acceptancePreset {
		expectations.MinResults = inputs.minResults
	}
	if inputs.explicitFlags["require-model-hash"] || !inputs.acceptancePreset {
		expectations.RequireModelHash = inputs.requireModelHash
	}
	if inputs.explicitFlags["allow-gaps"] || !inputs.acceptancePreset {
		expectations.AllowGaps = inputs.allowGaps
	}
	if inputs.explicitFlags["allow-provider-failures"] || !inputs.acceptancePreset {
		expectations.AllowProviderFailures = inputs.allowProviderFailures
	}
	if inputs.explicitFlags["require-wake-matches"] || !inputs.acceptancePreset {
		expectations.RequireAllWakeMatches = inputs.requireWakeMatches
	}
	if inputs.explicitFlags["require-transcript-matches"] || !inputs.acceptancePreset {
		expectations.RequireAllTranscriptMatches = inputs.requireTranscriptMatches
	}
	return expectations, true
}

func readReport(path string, stdin io.Reader) ([]byte, error) {
	if path == "" || path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func safeBenchmarkCLIError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if strings.Contains(message, "read fixture:") {
		return "fixture_unavailable"
	}
	if strings.Contains(message, "read benchmark report:") {
		return "benchmark_report_unavailable"
	}
	if strings.Contains(message, "read baseline benchmark report:") {
		return "baseline_benchmark_report_unavailable"
	}
	if strings.Contains(message, "read closure bundle:") {
		return "closure_bundle_unavailable"
	}
	return message
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeFixtureTemplate(w io.Writer, kind string, issue string) error {
	if issue == "" {
		switch kind {
		case "wake-word":
			issue = "JUT-11"
		case "stt":
			issue = "JUT-13"
		}
	}
	var manifest voice.BenchmarkFixtureSetManifest
	switch kind {
	case "wake-word":
		expectWake := true
		expectNoWake := false
		manifest = voice.BenchmarkFixtureSetManifest{
			Issue: issue,
			Kind:  "wake-word",
			Fixtures: []voice.BenchmarkFixtureManifest{
				{
					ID:          "positive-wake",
					Description: "Wake phrase fixture, 16 kHz mono PCM WAV",
					Path:        "wake/positive-wake.wav",
					SHA256:      "sha256:replace-with-fixture-hash",
					Source:      "replace-with-fixture-source",
					RecordedAt:  "2026-06-17T00:00:00Z",
					Consent:     boolPtr(true),
					ExpectWake:  &expectWake,
					Language:    "en",
				},
				{
					ID:          "near-miss",
					Description: "Speech without wake phrase, 16 kHz mono PCM WAV",
					Path:        "wake/near-miss.wav",
					SHA256:      "sha256:replace-with-fixture-hash",
					Source:      "replace-with-fixture-source",
					RecordedAt:  "2026-06-17T00:00:00Z",
					Consent:     boolPtr(true),
					ExpectWake:  &expectNoWake,
					Language:    "en",
				},
				{
					ID:          "ambient-room",
					Description: "Ambient/noise fixture, 16 kHz mono PCM WAV",
					Path:        "wake/ambient-room.wav",
					SHA256:      "sha256:replace-with-fixture-hash",
					Source:      "replace-with-fixture-source",
					RecordedAt:  "2026-06-17T00:00:00Z",
					Consent:     boolPtr(true),
					ExpectWake:  &expectNoWake,
					Language:    "en",
				},
				{
					ID:          "conversation-long",
					Description: "Long no-wake conversation fixture, 16 kHz mono PCM WAV",
					Path:        "wake/conversation-long.wav",
					SHA256:      "sha256:replace-with-fixture-hash",
					Source:      "replace-with-fixture-source",
					RecordedAt:  "2026-06-17T00:00:00Z",
					Consent:     boolPtr(true),
					ExpectWake:  &expectNoWake,
					Language:    "en",
				},
			},
		}
	case "stt":
		manifest = voice.BenchmarkFixtureSetManifest{
			Issue: issue,
			Kind:  "stt",
			Fixtures: []voice.BenchmarkFixtureManifest{
				{
					ID:                 "short-command",
					Description:        "Short command fixture, 16 kHz mono PCM WAV",
					Path:               "stt/short-command.wav",
					SHA256:             "sha256:replace-with-fixture-hash",
					Source:             "replace-with-fixture-source",
					RecordedAt:         "2026-06-17T00:00:00Z",
					Consent:            boolPtr(true),
					ExpectedTranscript: "turn on the lights",
					Language:           "en-GB",
				},
			},
		}
	default:
		return errors.New("fixture-template must be wake-word or stt")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}

func writeProviderClosureBundleTemplate(w io.Writer, issue string) error {
	issue = safeEvidenceIssue(issue)
	var bundle providerClosureBundleFile
	switch issue {
	case "JUT-11":
		bundle = providerClosureBundleFile{
			Issue: "JUT-11",
			Decision: providerClosureDecision{
				Status:    "defer",
				Rationale: "replace-with-decision-rationale-from-build-packaging-model-and-benchmark-evidence",
			},
			ProviderManifest: json.RawMessage(jut11ProviderManifestTemplate()),
			FixtureManifest:  json.RawMessage(jut11FixtureManifestTemplate()),
			BuildEvidence: []providerBuildEvidence{
				{
					GeneratedAt:     "2026-06-17T00:00:00Z",
					Issue:           "JUT-11",
					ProviderID:      "pmdroid-microwakeword",
					ProviderKind:    "wake-word",
					Target:          "native-consumer",
					CommandID:       "replace-with-command-id",
					Status:          "not-run",
					ErrorCode:       "replace-with-error-code-or-remove",
					Runtime:         "replace-with-os-arch-go-version",
					ClosureEvidence: false,
				},
			},
			PackagingEvidence: &providerPackagingEvidence{
				GeneratedAt:  "2026-06-17T00:00:00Z",
				Issue:        "JUT-11",
				ProviderID:   "pmdroid-microwakeword",
				ProviderKind: "wake-word",
				Targets: []providerPackagingTarget{
					{Target: "macos-native", Status: "not-run"},
					{Target: "linux-native", Status: "not-run"},
					{Target: "raspberry-pi-arm64", Status: "unsupported"},
				},
				Runtime:                   "replace-with-os-arch-go-version",
				PackagingEvidenceComplete: false,
			},
			ModelEvidence: []providerModelEvidence{
				{
					GeneratedAt:         "2026-06-17T00:00:00Z",
					Issue:               "JUT-11",
					ProviderID:          "pmdroid-microwakeword",
					ProviderKind:        "wake-word",
					ModelID:             "okay-nabu",
					ModelHash:           "sha256:replace-with-model-hash",
					ModelSource:         "esphome",
					ModelFormat:         "tflite",
					CompatibilityStatus: "untested",
					RuntimeStatus:       "not-run",
					Runtime:             "replace-with-os-arch-go-version",
					ClosureEvidence:     false,
				},
				{
					GeneratedAt:         "2026-06-17T00:00:00Z",
					Issue:               "JUT-11",
					ProviderID:          "pmdroid-microwakeword",
					ProviderKind:        "wake-word",
					ModelID:             "replace-with-ohf-model-id",
					ModelHash:           "sha256:replace-with-model-hash",
					ModelSource:         "ohf",
					ModelFormat:         "tflite",
					CompatibilityStatus: "untested",
					RuntimeStatus:       "not-run",
					Runtime:             "replace-with-os-arch-go-version",
					ClosureEvidence:     false,
				},
			},
			BenchmarkReport: json.RawMessage(jut11BenchmarkReportTemplate("pmdroid-microwakeword")),
			BaselineReport:  json.RawMessage(jut11BenchmarkReportTemplate("wyoming-openwakeword")),
		}
	case "JUT-13":
		bundle = providerClosureBundleFile{
			Issue: "JUT-13",
			Decision: providerClosureDecision{
				Status:    "documented-external-provider",
				Rationale: "replace-with-decision-rationale-from-build-packaging-model-and-benchmark-evidence",
			},
			ProviderManifest: json.RawMessage(jut13ProviderManifestTemplate()),
			FixtureManifest:  json.RawMessage(jut13FixtureManifestTemplate()),
			BuildEvidence: []providerBuildEvidence{
				{
					GeneratedAt:     "2026-06-17T00:00:00Z",
					Issue:           "JUT-13",
					ProviderID:      "go-whisper",
					ProviderKind:    "stt",
					Target:          "native-cli",
					CommandID:       "replace-with-command-id",
					Status:          "not-run",
					ErrorCode:       "replace-with-error-code-or-remove",
					Runtime:         "replace-with-os-arch-go-version",
					ClosureEvidence: false,
				},
			},
			PackagingEvidence: &providerPackagingEvidence{
				GeneratedAt:  "2026-06-17T00:00:00Z",
				Issue:        "JUT-13",
				ProviderID:   "go-whisper",
				ProviderKind: "stt",
				Targets: []providerPackagingTarget{
					{Target: "cpu-only", Status: "not-run"},
					{Target: "macos-metal", Status: "not-run"},
					{Target: "container-linux", Status: "not-run"},
					{Target: "raspberry-pi-arm64", Status: "unsupported"},
				},
				Runtime:                   "replace-with-os-arch-go-version",
				PackagingEvidenceComplete: false,
			},
			ModelEvidence: []providerModelEvidence{
				{
					GeneratedAt:         "2026-06-17T00:00:00Z",
					Issue:               "JUT-13",
					ProviderID:          "go-whisper",
					ProviderKind:        "stt",
					ModelID:             "tiny-en",
					ModelHash:           "sha256:replace-with-model-hash",
					ModelSource:         "whisper-cpp",
					ModelFormat:         "ggml",
					CompatibilityStatus: "untested",
					RuntimeStatus:       "not-run",
					Runtime:             "replace-with-os-arch-go-version",
					ClosureEvidence:     false,
				},
			},
			BenchmarkReport: json.RawMessage(jut13BenchmarkReportTemplate()),
		}
	default:
		return errors.New("closure-bundle-template must be JUT-11 or JUT-13")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(bundle)
}

func jut11ProviderManifestTemplate() []byte {
	return []byte(`{
  "id": "org.pmdroid.microwakeword.local",
  "name": "microWakeWord Local",
  "version": "experimental",
  "kind": "wake-word",
  "transport": {
    "type": "builtin",
    "endpoint": "local-voice-service"
  },
  "capabilities": {
    "offline": true,
    "languages": ["en"]
  },
  "hardware": {
    "cpu": true,
    "gpu": false,
    "coreml": false,
    "cuda": false,
    "raspberryPi": false
  },
  "credentials": [],
  "license": {
    "name": "MIT",
    "url": "local-license-reference"
  },
  "contribution": {
    "source": "local-source-reference",
    "maintainers": ["pmdroid"]
  },
  "wakeWord": {
    "defaultModelId": "okay-nabu",
    "phrase": "Okay Nabu",
    "languages": ["en"],
    "sensitivity": 0.55,
    "models": [
      {
        "id": "okay-nabu",
        "path": "assets/okay_nabu.tflite",
        "phrase": "Okay Nabu",
        "languages": ["en"],
        "sensitivity": 0.55
      },
      {
        "id": "replace-with-ohf-model-id",
        "path": "assets/replace-with-ohf-model.tflite",
        "phrase": "replace-with-ohf-phrase",
        "languages": ["en"],
        "sensitivity": 0.55
      }
    ]
  }
}`)
}

func jut13ProviderManifestTemplate() []byte {
	return []byte(`{
  "id": "org.mutablelogic.go-whisper.local",
  "name": "go-whisper Local STT",
  "version": "external",
  "kind": "stt",
  "transport": {
    "type": "http-json",
    "endpoint": "http://127.0.0.1:8081"
  },
  "capabilities": {
    "streaming": true,
    "partialTranscripts": false,
    "offline": true,
    "languages": ["en", "en-GB", "en-US"],
    "inputFormats": ["audio/wav;rate=16000", "audio/pcm;rate=16000"],
    "outputFormats": ["application/json"]
  },
  "hardware": {
    "cpu": true,
    "gpu": true,
    "coreml": false,
    "cuda": true,
    "vulkan": true,
    "metal": true,
    "raspberryPi": false
  },
  "credentials": [],
  "license": {
    "name": "Apache-2.0",
    "url": "local-license-reference"
  },
  "contribution": {
    "source": "local-source-reference",
    "maintainers": ["mutablelogic"]
  }
}`)
}

func jut11FixtureManifestTemplate() []byte {
	return []byte(`{
  "issue": "JUT-11",
  "kind": "wake-word",
  "fixtures": [
    {"id": "positive-wake", "description": "Wake phrase fixture", "path": "wake/positive-wake.wav", "sha256": "sha256:replace-with-fixture-hash", "source": "replace-with-fixture-source", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": true, "language": "en"},
    {"id": "near-miss", "description": "Speech without wake phrase", "path": "wake/near-miss.wav", "sha256": "sha256:replace-with-fixture-hash", "source": "replace-with-fixture-source", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
    {"id": "ambient-room", "description": "Ambient room fixture", "path": "wake/ambient-room.wav", "sha256": "sha256:replace-with-fixture-hash", "source": "replace-with-fixture-source", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"},
    {"id": "conversation-long", "description": "Long no-wake conversation fixture", "path": "wake/conversation-long.wav", "sha256": "sha256:replace-with-fixture-hash", "source": "replace-with-fixture-source", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectWake": false, "language": "en"}
  ]
}`)
}

func jut13FixtureManifestTemplate() []byte {
	return []byte(`{
  "issue": "JUT-13",
  "kind": "stt",
  "fixtures": [
    {"id": "short-command", "description": "Short command fixture", "path": "stt/short-command.wav", "sha256": "sha256:replace-with-fixture-hash", "source": "replace-with-fixture-source", "recordedAt": "2026-06-17T00:00:00Z", "consent": true, "expectedTranscript": "turn on the lights", "language": "en-GB"}
  ]
}`)
}

func jut11BenchmarkReportTemplate(providerID string) []byte {
	report := fmt.Sprintf(`{
  "generatedAt": "2026-06-17T00:00:00Z",
  "issue": "JUT-11",
  "kind": "wake-word",
  "environment": {
    "os": "replace-with-os",
    "arch": "replace-with-arch",
    "goVersion": "replace-with-go-version",
    "providerId": %q,
    "providerKind": "wake-word",
    "modelId": "replace-with-model-id",
    "modelHash": "sha256:replace-with-model-hash"
  },
  "wakeResults": [
    {"fixtureId": "positive-wake", "providerId": %q, "modelId": "replace-with-model-id", "expectedWake": true, "detected": false, "matchesExpected": false, "latency": 0, "resourceSample": {"duration": 0}},
    {"fixtureId": "near-miss", "providerId": %q, "modelId": "replace-with-model-id", "expectedWake": false, "detected": false, "matchesExpected": false, "latency": 0, "resourceSample": {"duration": 0}},
    {"fixtureId": "ambient-room", "providerId": %q, "modelId": "replace-with-model-id", "expectedWake": false, "detected": false, "matchesExpected": false, "latency": 0, "resourceSample": {"duration": 0}},
    {"fixtureId": "conversation-long", "providerId": %q, "modelId": "replace-with-model-id", "expectedWake": false, "detected": false, "matchesExpected": false, "latency": 0, "resourceSample": {"duration": 0}}
  ],
  "summary": {
    "total": 4,
    "providerFailures": 0,
    "falseAccepts": 0,
    "falseRejects": 1,
    "averageLatency": 0
  }
}`, providerID, providerID, providerID, providerID, providerID)
	return []byte(report)
}

func jut13BenchmarkReportTemplate() []byte {
	return []byte(`{
  "generatedAt": "2026-06-17T00:00:00Z",
  "issue": "JUT-13",
  "kind": "stt",
  "environment": {
    "os": "replace-with-os",
    "arch": "replace-with-arch",
    "goVersion": "replace-with-go-version",
    "providerId": "go-whisper",
    "providerKind": "stt",
    "modelId": "tiny-en",
    "modelHash": "sha256:replace-with-model-hash"
  },
  "sttResults": [{
    "fixtureId": "short-command",
    "providerId": "go-whisper",
    "modelId": "tiny-en",
    "expectedTranscript": "turn on the lights",
    "transcript": "",
    "transcriptMatched": false,
    "latency": 0,
    "resourceSample": {"duration": 0},
    "providerReturned": false
  }],
  "summary": {
    "total": 1,
    "providerFailures": 0,
    "transcriptMatches": 0,
    "averageLatency": 0
  }
}`)
}

func validateFixtureManifest(
	w io.Writer,
	manifestPath string,
	fixtureDir string,
	acceptancePreset bool,
) ([]string, error) {
	manifest, fixtures, problems, err := loadFixtureManifest(manifestPath, fixtureDir, acceptancePreset)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(w, fixtureSetEvidenceMarkdown(manifest, len(fixtures), problems))
	return problems, nil
}

func loadFixtureManifest(
	manifestPath string,
	fixtureDir string,
	acceptancePreset bool,
) (voice.BenchmarkFixtureSetManifest, []voice.BenchmarkFixture, []string, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return voice.BenchmarkFixtureSetManifest{}, nil, nil, fmt.Errorf("read fixture manifest: %w", err)
	}
	manifest, err := voice.DecodeBenchmarkFixtureSetManifest(string(raw))
	if err != nil {
		return voice.BenchmarkFixtureSetManifest{}, nil, nil, err
	}
	fixtures, problems := voice.LoadBenchmarkFixtureSet(fixtureDir, manifest, time.Time{})
	if acceptancePreset {
		problems = append(problems, validateFixtureManifestAcceptancePreset(manifest)...)
	}
	return manifest, fixtures, problems, nil
}

type sttCommandBenchmarkInputs struct {
	manifestPath     string
	fixtureDir       string
	acceptancePreset bool
	publicJSON       bool
	command          string
	args             string
	argsJSON         string
	timeout          time.Duration
	env              voice.BenchmarkEnvironment
}

type wakeCommandBenchmarkInputs struct {
	manifestPath     string
	fixtureDir       string
	acceptancePreset bool
	publicJSON       bool
	command          string
	args             string
	argsJSON         string
	timeout          time.Duration
	env              voice.BenchmarkEnvironment
}

type sttCommandProvider struct {
	command    string
	args       []string
	timeout    time.Duration
	providerID string
	modelID    string
	language   string
}

type wakeCommandProvider struct {
	command    string
	args       []string
	timeout    time.Duration
	providerID string
	modelID    string
	language   string
	fixtureID  string
}

type sttCommandOutput struct {
	Text       string  `json:"text"`
	Transcript string  `json:"transcript"`
	ProviderID string  `json:"providerId"`
	ModelID    string  `json:"modelId"`
	Language   string  `json:"language"`
	DurationMS float64 `json:"durationMs"`
	Duration   string  `json:"duration"`
}

type wakeCommandOutput struct {
	Detected          *bool   `json:"detected"`
	Wake              *bool   `json:"wake"`
	ProviderID        string  `json:"providerId"`
	ModelID           string  `json:"modelId"`
	Confidence        float64 `json:"confidence"`
	DetectedAtMS      float64 `json:"detectedAtMs"`
	ActivationLatency string  `json:"activationLatency"`
	LatencyMS         float64 `json:"latencyMs"`
}

func writeWakeCommandBenchmarkReport(w io.Writer, inputs wakeCommandBenchmarkInputs) ([]string, error) {
	inputs.env.ProviderKind = "wake-word"
	if strings.TrimSpace(inputs.env.ProviderID) == "" {
		return nil, errors.New("provider-id is required for wake command benchmarks")
	}
	if strings.TrimSpace(inputs.env.ModelID) == "" {
		return nil, errors.New("model-id is required for wake command benchmarks")
	}
	if strings.TrimSpace(inputs.command) == "" {
		return nil, errors.New("wake-command is required")
	}
	if !filepath.IsAbs(inputs.command) {
		return nil, errors.New("wake-command must be an absolute path")
	}
	commandArgs, err := parseVoiceCommandArgs(inputs.args, inputs.argsJSON, "wake-command")
	if err != nil {
		return nil, err
	}
	manifest, fixtures, problems, err := loadFixtureManifest(
		inputs.manifestPath,
		inputs.fixtureDir,
		inputs.acceptancePreset,
	)
	if err != nil {
		return nil, err
	}
	if manifest.Kind != "wake-word" {
		problems = append(problems, "fixture manifest kind must be wake-word for wake-command")
	}
	if len(problems) > 0 {
		fmt.Fprintln(w, fixtureSetEvidenceMarkdown(manifest, len(fixtures), problems))
		return problems, nil
	}
	results := make([]voice.WakeBenchmarkResult, 0, len(fixtures))
	for _, fixture := range fixtures {
		provider := &wakeCommandProvider{
			command:    inputs.command,
			args:       commandArgs,
			timeout:    inputs.timeout,
			providerID: inputs.env.ProviderID,
			modelID:    inputs.env.ModelID,
			language:   fixture.Language,
			fixtureID:  fixture.ID,
		}
		results = append(results, voice.RunWakeBenchmark(context.Background(), provider, fixture))
	}
	report := voice.NewWakeBenchmarkReport(manifest.Issue, inputs.env, results, nil)
	if inputs.acceptancePreset {
		expectations, ok := voice.BenchmarkAcceptanceExpectations(manifest.Issue)
		if !ok {
			problems = append(problems, fmt.Sprintf("no acceptance preset is defined for issue %q", manifest.Issue))
		} else {
			problems = append(problems, voice.ValidateBenchmarkReport(report, expectations)...)
		}
	}
	var raw []byte
	if inputs.publicJSON {
		raw, err = report.PublicJSON()
	} else {
		raw, err = report.JSON()
	}
	if err != nil {
		return nil, fmt.Errorf("encode wake command benchmark report: %w", err)
	}
	fmt.Fprintln(w, string(raw))
	return problems, nil
}

func writeSTTCommandBenchmarkReport(w io.Writer, inputs sttCommandBenchmarkInputs) ([]string, error) {
	inputs.env.ProviderKind = "stt"
	if strings.TrimSpace(inputs.env.ProviderID) == "" {
		return nil, errors.New("provider-id is required for stt command benchmarks")
	}
	if strings.TrimSpace(inputs.env.ModelID) == "" {
		return nil, errors.New("model-id is required for stt command benchmarks")
	}
	if strings.TrimSpace(inputs.command) == "" {
		return nil, errors.New("stt-command is required")
	}
	if !filepath.IsAbs(inputs.command) {
		return nil, errors.New("stt-command must be an absolute path")
	}
	commandArgs, err := parseVoiceCommandArgs(inputs.args, inputs.argsJSON, "stt-command")
	if err != nil {
		return nil, err
	}
	manifest, fixtures, problems, err := loadFixtureManifest(
		inputs.manifestPath,
		inputs.fixtureDir,
		inputs.acceptancePreset,
	)
	if err != nil {
		return nil, err
	}
	if manifest.Kind != "stt" {
		problems = append(problems, "fixture manifest kind must be stt for stt-command")
	}
	if len(problems) > 0 {
		fmt.Fprintln(w, fixtureSetEvidenceMarkdown(manifest, len(fixtures), problems))
		return problems, nil
	}
	provider := &sttCommandProvider{
		command:    inputs.command,
		args:       commandArgs,
		timeout:    inputs.timeout,
		providerID: inputs.env.ProviderID,
		modelID:    inputs.env.ModelID,
		language:   fixtures[0].Language,
	}
	report := voice.RunSTTBenchmarkSuite(context.Background(), voice.STTBenchmarkSuite{
		Issue:    manifest.Issue,
		Provider: provider,
		Env:      inputs.env,
		Fixtures: fixtures,
	})
	if inputs.acceptancePreset {
		expectations, ok := voice.BenchmarkAcceptanceExpectations(manifest.Issue)
		if !ok {
			problems = append(problems, fmt.Sprintf("no acceptance preset is defined for issue %q", manifest.Issue))
		} else {
			problems = append(problems, voice.ValidateBenchmarkReport(report, expectations)...)
		}
	}
	var raw []byte
	if inputs.publicJSON {
		raw, err = report.PublicJSON()
	} else {
		raw, err = report.JSON()
	}
	if err != nil {
		return nil, fmt.Errorf("encode stt command benchmark report: %w", err)
	}
	fmt.Fprintln(w, string(raw))
	return problems, nil
}

func (p *wakeCommandProvider) DetectWake(
	ctx context.Context,
	utterance voice.CapturedUtterance,
) (voice.WakeBenchmarkDetection, error) {
	if p == nil || strings.TrimSpace(p.command) == "" {
		return voice.WakeBenchmarkDetection{}, errors.New("wake command provider unavailable")
	}
	output, err := runFixtureCommand(
		ctx,
		p.command,
		p.args,
		p.timeout,
		p.fixtureID,
		p.modelID,
		p.language,
		utterance,
		"JUTE_WAKE_COMMAND_HELPER",
	)
	if err != nil {
		return voice.WakeBenchmarkDetection{}, err
	}
	return p.parseOutput(output)
}

func (p *wakeCommandProvider) parseOutput(output []byte) (voice.WakeBenchmarkDetection, error) {
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return voice.WakeBenchmarkDetection{}, errors.New("wake command returned empty result")
	}
	result := voice.WakeBenchmarkDetection{
		ProviderID: p.providerID,
		ModelID:    p.modelID,
	}
	switch strings.ToLower(raw) {
	case "true", "detected", "wake", "yes", "1":
		result.Detected = true
		return result, nil
	case "false", "not-detected", "not_detected", "no-wake", "no_wake", "no", "0":
		result.Detected = false
		return result, nil
	}
	var parsed wakeCommandOutput
	if err := decodeStrictVoiceCommandJSON(raw, &parsed); err != nil {
		return voice.WakeBenchmarkDetection{}, errors.New("wake command returned invalid result")
	}
	detected := parsed.Detected
	if detected == nil {
		detected = parsed.Wake
	}
	if detected == nil {
		return voice.WakeBenchmarkDetection{}, errors.New("wake command result missing detected")
	}
	result.Detected = *detected
	result.ProviderID = safeEvidenceProvider(firstNonEmpty(parsed.ProviderID, p.providerID))
	result.ModelID = safeEvidenceToken(firstNonEmpty(parsed.ModelID, p.modelID))
	result.Confidence = parsed.Confidence
	if parsed.DetectedAtMS > 0 {
		result.DetectedAt = time.Duration(parsed.DetectedAtMS * float64(time.Millisecond))
	}
	if parsed.LatencyMS > 0 {
		result.ActivationLatency = time.Duration(parsed.LatencyMS * float64(time.Millisecond))
	}
	if strings.TrimSpace(parsed.ActivationLatency) != "" {
		if duration, err := time.ParseDuration(strings.TrimSpace(parsed.ActivationLatency)); err == nil {
			result.ActivationLatency = duration
		}
	}
	return result, nil
}

func (p *sttCommandProvider) Transcribe(
	ctx context.Context,
	utterance voice.CapturedUtterance,
) (voice.STTResult, error) {
	if p == nil || strings.TrimSpace(p.command) == "" {
		return voice.STTResult{}, errors.New("stt command provider unavailable")
	}
	output, err := runFixtureCommand(
		ctx,
		p.command,
		p.args,
		p.timeout,
		"",
		p.modelID,
		p.language,
		utterance,
		"JUTE_STT_COMMAND_HELPER",
	)
	if err != nil {
		return voice.STTResult{}, err
	}
	return p.parseOutput(output)
}

func (p *sttCommandProvider) parseOutput(output []byte) (voice.STTResult, error) {
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return voice.STTResult{}, errors.New("stt command returned empty transcript")
	}
	result := voice.STTResult{
		Text:       raw,
		ProviderID: p.providerID,
		ModelID:    p.modelID,
		Language:   p.language,
	}
	if strings.HasPrefix(raw, "{") {
		var parsed sttCommandOutput
		if err := decodeStrictVoiceCommandJSON(raw, &parsed); err != nil {
			return voice.STTResult{}, errors.New("stt command returned invalid JSON")
		}
		result.Text = firstNonEmpty(parsed.Text, parsed.Transcript)
		result.ProviderID = firstNonEmpty(parsed.ProviderID, p.providerID)
		result.ModelID = firstNonEmpty(parsed.ModelID, p.modelID)
		result.Language = firstNonEmpty(parsed.Language, p.language)
		switch {
		case parsed.DurationMS > 0:
			result.Duration = time.Duration(parsed.DurationMS * float64(time.Millisecond))
		case strings.TrimSpace(parsed.Duration) != "":
			if duration, err := time.ParseDuration(strings.TrimSpace(parsed.Duration)); err == nil {
				result.Duration = duration
			}
		}
	}
	if strings.TrimSpace(result.Text) == "" {
		return voice.STTResult{}, errors.New("stt command returned empty transcript")
	}
	result.Text = strings.TrimSpace(result.Text)
	result.ProviderID = safeEvidenceProvider(result.ProviderID)
	result.ModelID = safeEvidenceToken(result.ModelID)
	result.Language = safeEvidenceToken(result.Language)
	return result, nil
}

func decodeStrictVoiceCommandJSON(raw string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}

func runFixtureCommand(
	ctx context.Context,
	command string,
	args []string,
	timeout time.Duration,
	fixtureID string,
	modelID string,
	language string,
	utterance voice.CapturedUtterance,
	helperEnv string,
) ([]byte, error) {
	raw, err := voice.EncodeBenchmarkWAV(utterance)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp("", "jute-voice-command-*.wav")
	if err != nil {
		return nil, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(raw); err != nil {
		_ = temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext( //nolint:gosec // command providers are explicit user opt-in benchmark integrations.
		commandCtx,
		command,
		voiceCommandArgs(
			args,
			tempPath,
			fixtureID,
			modelID,
			language,
		)...)
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	output, err := cmd.Output()
	if commandCtx.Err() != nil {
		return nil, errors.New("voice command timed out")
	}
	if err != nil {
		return nil, errors.New("voice command failed")
	}
	return output, nil
}

func parseVoiceCommandArgs(args string, argsJSON string, label string) ([]string, error) {
	args = strings.TrimSpace(args)
	argsJSON = strings.TrimSpace(argsJSON)
	if args != "" && argsJSON != "" {
		return nil, fmt.Errorf("%s-args and %s-args-json cannot both be set", label, label)
	}
	if argsJSON == "" {
		return strings.Fields(args), nil
	}
	var parsed []string
	decoder := json.NewDecoder(strings.NewReader(argsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode %s-args-json: %w", label, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode %s-args-json: trailing JSON data", label)
	}
	return parsed, nil
}

func voiceCommandArgs(args []string, fixturePath string, fixtureID string, modelID string, language string) []string {
	if len(args) == 0 {
		return []string{fixturePath}
	}
	out := make([]string, 0, len(args)+1)
	replaced := false
	for _, arg := range args {
		if strings.Contains(arg, "{inputPath}") || strings.Contains(arg, "{fixture}") {
			replaced = true
		}
		arg = strings.ReplaceAll(arg, "{inputPath}", fixturePath)
		arg = strings.ReplaceAll(arg, "{fixture}", fixturePath)
		arg = strings.ReplaceAll(arg, "{fixtureId}", fixtureID)
		arg = strings.ReplaceAll(arg, "{modelId}", modelID)
		arg = strings.ReplaceAll(arg, "{language}", language)
		out = append(out, arg)
	}
	if !replaced {
		out = append(out, fixturePath)
	}
	return out
}

func writeFixtureFailureReport(
	w io.Writer,
	manifestPath string,
	fixtureDir string,
	acceptancePreset bool,
	publicJSON bool,
	env voice.BenchmarkEnvironment,
) ([]string, error) {
	if strings.TrimSpace(env.ProviderID) == "" {
		return nil, errors.New("provider-id is required for fixture failure reports")
	}
	if strings.TrimSpace(env.ModelID) == "" {
		return nil, errors.New("model-id is required for fixture failure reports")
	}
	manifest, fixtures, problems, err := loadFixtureManifest(manifestPath, fixtureDir, acceptancePreset)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		fmt.Fprintln(w, fixtureSetEvidenceMarkdown(manifest, len(fixtures), problems))
		return problems, nil
	}
	env.ProviderKind = manifest.Kind
	report, err := fixtureFailureBenchmarkReport(manifest, fixtures, env)
	if err != nil {
		return nil, err
	}
	if acceptancePreset {
		expectations, ok := voice.BenchmarkAcceptanceExpectations(manifest.Issue)
		if !ok {
			problems = append(problems, fmt.Sprintf("no acceptance preset is defined for issue %q", manifest.Issue))
		} else {
			problems = append(problems, voice.ValidateBenchmarkReport(report, expectations)...)
		}
	}
	var raw []byte
	if publicJSON {
		raw, err = report.PublicJSON()
	} else {
		raw, err = report.JSON()
	}
	if err != nil {
		return nil, fmt.Errorf("encode fixture failure report: %w", err)
	}
	fmt.Fprintln(w, string(raw))
	return problems, nil
}

func fixtureFailureBenchmarkReport(
	manifest voice.BenchmarkFixtureSetManifest,
	fixtures []voice.BenchmarkFixture,
	env voice.BenchmarkEnvironment,
) (voice.BenchmarkReport, error) {
	gaps := []string{"provider run not executed; generated provider_unavailable fixture report"}
	switch manifest.Kind {
	case "wake-word":
		return voice.RunWakeBenchmarkSuite(context.Background(), voice.WakeBenchmarkSuite{
			Issue:    manifest.Issue,
			Provider: nil,
			Env:      env,
			Fixtures: fixtures,
			Gaps:     gaps,
		}), nil
	case "stt":
		return voice.RunSTTBenchmarkSuite(context.Background(), voice.STTBenchmarkSuite{
			Issue:    manifest.Issue,
			Provider: nil,
			Env:      env,
			Fixtures: fixtures,
			Gaps:     gaps,
		}), nil
	default:
		return voice.BenchmarkReport{}, errors.New("fixture manifest kind must be wake-word or stt")
	}
}

func validateFixtureManifestAcceptancePreset(manifest voice.BenchmarkFixtureSetManifest) []string {
	expectations, ok := voice.BenchmarkAcceptanceExpectations(manifest.Issue)
	if !ok {
		return []string{fmt.Sprintf("no acceptance preset is defined for issue %q", manifest.Issue)}
	}
	if expectations.Kind != "" && manifest.Kind != expectations.Kind {
		return []string{"fixture manifest kind does not match acceptance preset"}
	}
	entries := map[string]voice.BenchmarkFixtureManifest{}
	seen := map[string]bool{}
	var problems []string
	for _, fixture := range manifest.Fixtures {
		fixtureID := strings.TrimSpace(fixture.ID)
		if fixtureID == "" {
			continue
		}
		if seen[fixtureID] {
			problems = append(problems, fmt.Sprintf("fixture manifest has duplicate fixture %s", fixtureID))
		}
		seen[fixtureID] = true
		entries[fixtureID] = fixture
	}
	for _, fixtureID := range expectations.RequiredFixtureIDs {
		if !seen[fixtureID] {
			problems = append(problems, fmt.Sprintf("fixture manifest is missing required fixture %s", fixtureID))
			continue
		}
		fixture := entries[fixtureID]
		switch expectations.Kind {
		case "wake-word":
			if fixture.ExpectWake == nil {
				problems = append(
					problems,
					fmt.Sprintf("fixture manifest fixture %s must declare expectWake", fixtureID),
				)
			}
		case "stt":
			if strings.TrimSpace(fixture.ExpectedTranscript) == "" {
				problems = append(
					problems,
					fmt.Sprintf("fixture manifest fixture %s must declare expectedTranscript", fixtureID),
				)
			}
		}
		problems = append(problems, validateFixtureProvenance(fixtureID, fixture)...)
	}
	return problems
}

func validateFixtureProvenance(fixtureID string, fixture voice.BenchmarkFixtureManifest) []string {
	var problems []string
	if strings.TrimSpace(fixture.Source) == "" {
		problems = append(problems, fmt.Sprintf("fixture manifest fixture %s must declare source", fixtureID))
	}
	if fixture.Consent == nil || !*fixture.Consent {
		problems = append(problems, fmt.Sprintf("fixture manifest fixture %s must declare consent true", fixtureID))
	}
	if strings.TrimSpace(fixture.RecordedAt) == "" {
		problems = append(problems, fmt.Sprintf("fixture manifest fixture %s must declare recordedAt", fixtureID))
	} else if _, err := time.Parse(time.RFC3339, fixture.RecordedAt); err != nil {
		problems = append(problems, fmt.Sprintf("fixture manifest fixture %s recordedAt must be RFC3339", fixtureID))
	}
	return problems
}

func writeFixtureHash(w io.Writer, fixturePath string) error {
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		return fmt.Errorf("read fixture: %w", err)
	}
	utterance, err := voice.DecodeBenchmarkWAV(raw, time.Time{})
	if err != nil {
		return fmt.Errorf("validate fixture WAV: %w", err)
	}
	fmt.Fprintf(w, "%s  %s  duration=%s sampleRate=%d channels=%d\n",
		voice.BenchmarkBytesSHA256(raw),
		fixturePath,
		utterance.EndedAt.Sub(utterance.StartedAt),
		utterance.SampleRate,
		utterance.Channels,
	)
	return nil
}

func writeToneFixture(
	w io.Writer,
	fixturePath string,
	duration time.Duration,
	frequency float64,
	amplitude float64,
) error {
	fixture, err := voice.NewBenchmarkToneFixture("tone", "deterministic tone fixture", voice.BenchmarkAudioSpec{
		Duration:  duration,
		Frequency: frequency,
		Amplitude: amplitude,
	})
	if err != nil {
		return fmt.Errorf("build tone fixture: %w", err)
	}
	raw, err := voice.EncodeBenchmarkWAV(fixture.Utterance)
	if err != nil {
		return fmt.Errorf("encode tone fixture: %w", err)
	}
	if err := os.WriteFile(fixturePath, raw, 0o600); err != nil {
		return fmt.Errorf("write tone fixture: %w", err)
	}
	fmt.Fprintf(w, "%s  %s  duration=%s sampleRate=%d channels=%d\n",
		voice.BenchmarkBytesSHA256(raw),
		fixturePath,
		fixture.Utterance.EndedAt.Sub(fixture.Utterance.StartedAt),
		fixture.Utterance.SampleRate,
		fixture.Utterance.Channels,
	)
	return nil
}

func fixtureSetEvidenceMarkdown(manifest voice.BenchmarkFixtureSetManifest, loaded int, problems []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Voice Benchmark Fixture Set: %s\n\n", manifest.Issue)
	fmt.Fprintf(&b, "- Kind: `%s`\n", manifest.Kind)
	fmt.Fprintf(&b, "- Fixtures: %d loaded / %d declared\n", loaded, len(manifest.Fixtures))
	b.WriteString("\nFixture entries:\n")
	for _, fixture := range manifest.Fixtures {
		expectation := "metadata-only"
		if fixture.ExpectWake != nil {
			expectation = fmt.Sprintf("expectWake=%t", *fixture.ExpectWake)
		}
		if strings.TrimSpace(fixture.ExpectedTranscript) != "" {
			expectation = "expectedTranscript=set"
		}
		hashState := "sha256=missing"
		if strings.TrimSpace(fixture.SHA256) != "" {
			hashState = "sha256=set"
		}
		consent := "consent=missing"
		if fixture.Consent != nil {
			consent = fmt.Sprintf("consent=%t", *fixture.Consent)
		}
		source := "source=missing"
		if strings.TrimSpace(fixture.Source) != "" {
			source = "source=set"
		}
		recordedAt := "recordedAt=missing"
		if strings.TrimSpace(fixture.RecordedAt) != "" {
			recordedAt = "recordedAt=set"
		}
		fmt.Fprintf(
			&b,
			"- `%s`: path=`%s`, %s, %s, %s, %s, %s, language=`%s`\n",
			fixture.ID,
			fixture.Path,
			hashState,
			source,
			recordedAt,
			consent,
			expectation,
			fixture.Language,
		)
	}
	if len(problems) > 0 {
		b.WriteString("\nValidation problems:\n")
		for _, problem := range problems {
			fmt.Fprintf(&b, "- %s\n", problem)
		}
	}
	return strings.TrimSpace(b.String())
}

func boolPtr(value bool) *bool {
	return &value
}
