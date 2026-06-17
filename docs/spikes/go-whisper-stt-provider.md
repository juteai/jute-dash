# go-whisper STT Provider Spike

## Status

Provider manifests, benchmark validation tooling, and one native packaging failure check are in place.
The real fixture transcription benchmark with a local model is still pending.

## Upstream Snapshot

Verified on 2026-06-17 against
[mutablelogic/go-whisper](https://github.com/mutablelogic/go-whisper):

- the project is still a full `gowhisper` CLI plus HTTP server, not a small library-style adapter;
- the README describes local Whisper models through `whisper.cpp`, model download/cache commands,
  JSON/SRT/VTT/text output, Docker deployment, and GPU acceleration paths including CUDA, Vulkan,
  and Metal;
- the service also exposes optional OpenAI and ElevenLabs transcription paths, which Jute must treat
  as cloud providers requiring explicit opt-in and secret references;
- Docker images are documented for Linux AMD64 and ARM64, but Raspberry Pi-class latency still needs
  a real benchmark before enabling `hardware.raspberryPi` in a provider manifest;
- upstream license notes mention Apache-2.0 for go-whisper, MIT-licensed `whisper.cpp` static
  libraries, and FFmpeg LGPL 2.1 linkage considerations.

This snapshot supports the current Jute decision to keep go-whisper outside the hub as an external
sidecar/provider-pack candidate until benchmark and packaging evidence exists.

## Decision

Treat `mutablelogic/go-whisper` as a documented external STT provider candidate for v1. Do not make it a built-in provider or link it into the Go hub until Jute has benchmark results and a packaging decision.

The preferred integration shape is an `http-json` sidecar provider pack bound to loopback. A `command` provider can be added later for trusted installs, but command providers remain disabled unless explicitly enabled per household or device profile.

## Rationale

`go-whisper` is a full speech-to-text service with local Whisper model support, HTTP serving, model management, Docker packaging, and optional GPU acceleration. That makes it useful as a Jute provider pack candidate, but it should stay outside the hub process:

- the hub must remain headless and provider-agnostic;
- native model, ffmpeg, and GPU dependencies should not become baseline hub dependencies;
- cloud-backed modes must not become implicit fallbacks;
- raw microphone audio should only flow to the selected local sidecar or an explicitly opted-in cloud provider.

## HTTP JSON Manifest Draft

This manifest draft is kept as a machine-validated fixture at:

```text
apps/hub/internal/app/voice/testdata/provider_manifests/go_whisper_http_json.json
```

```json
{
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
    "url": "https://github.com/mutablelogic/go-whisper/blob/main/LICENSE"
  },
  "contribution": {
    "source": "https://github.com/mutablelogic/go-whisper",
    "maintainers": ["mutablelogic"]
  }
}
```

The provider pack adapter should own any exact go-whisper HTTP route mapping. The hub should only depend on the Jute provider contract: submit one captured utterance and receive a final transcript plus metadata.

Cloud modes, if exposed, require a separate manifest with `offline: false`, explicit cloud opt-in, and secret references only.

`TestSpikeProviderManifestFixturesValidate` decodes this fixture with unknown-field and trailing-JSON rejection, then runs it through the hub manifest validator. `TestSpikeProviderCandidatesAreNotProductionDependencies` also checks that the spike candidate has not been added to `go.mod` or `go.sum`. Those tests do not make go-whisper a production dependency; they only prove the proposed provider-pack shape remains compatible with Jute's safe manifest contract.

## Command Manifest Draft

This manifest draft is kept as a machine-validated fixture at:

```text
apps/hub/internal/app/voice/testdata/provider_manifests/go_whisper_command.json
```

Use this shape only after command providers are enabled for the target household or device profile:

```json
{
  "id": "org.mutablelogic.go-whisper.command",
  "name": "go-whisper Command STT",
  "version": "external",
  "kind": "stt",
  "transport": {
    "type": "command",
    "command": "/usr/local/bin/gowhisper",
    "args": ["transcribe", "--format", "json", "--model", "{modelId}", "{inputPath}"]
  },
  "capabilities": {
    "streaming": false,
    "partialTranscripts": false,
    "offline": true,
    "languages": ["en", "en-GB", "en-US"],
    "inputFormats": ["audio/wav;rate=16000"],
    "outputFormats": ["application/json"]
  },
  "hardware": {
    "cpu": true,
    "gpu": false,
    "coreml": false,
    "cuda": false,
    "vulkan": false,
    "metal": false,
    "raspberryPi": false
  },
  "credentials": [],
  "license": {
    "name": "Apache-2.0",
    "url": "https://github.com/mutablelogic/go-whisper/blob/main/LICENSE"
  },
  "contribution": {
    "source": "https://github.com/mutablelogic/go-whisper",
    "maintainers": ["mutablelogic"]
  }
}
```

The fixture validator requires the command path to be absolute and the arguments to remain an explicit argv array with a `{fixture}` placeholder for the temporary WAV path. The adapter must avoid shell interpolation, write temporary audio files under a Jute-controlled runtime directory, and remove those files after transcription.

## Benchmark Plan

The fixture benchmark should run before promoting this to a first-class provider pack.

The repo now has a provider-neutral fixture harness in:

```text
apps/hub/internal/app/voice/benchmark.go
```

Use `RunSTTBenchmark` or `RunSTTBenchmarkSuite` to adapt go-whisper and the Wyoming STT baseline to the same result shape. The harness records fixture ID, provider/model/language metadata, expected transcript match, latency, and allocation samples without storing or emitting PCM bytes.

Wrap each provider run in `NewSTTBenchmarkReport("JUT-13", ...)` and attach the JSON output from `BenchmarkReport.JSON()` to Linear or this document. The report should include provider ID, model ID, model hash when available, OS/architecture, Go version, result summary, and explicit gaps.

Before treating a run as acceptance evidence, pass the report through `ValidateBenchmarkReport` with `Issue: "JUT-13"`, `Kind: "stt"`, `RequiredProviderIDContains: "go-whisper"`, `RequireModelHash: true`, `RequireAllTranscriptMatches: true`, and `RequireResourceSamples: true`. Strict validation should have no unresolved gaps, no provider failures, at least the declared fixture count, unique fixture result IDs, a provider ID that identifies go-whisper, a `sha256:<64 hex>` model hash, result provider/model IDs that match the report environment, a measured `resourceSample.duration` for every successful fixture row, and matching transcripts for every expected fixture. A full local report must include the returned transcript body whenever `transcriptMatched` is true; public JSON evidence strips transcript bodies after validation. Validation recomputes the summary from fixture rows, so copied reports cannot hide provider failures or transcript mismatches by editing aggregate counts.

The benchmark CLI also has an issue-specific acceptance preset for JUT-13, so saved reports can be checked with one flag. The preset requires the named fixture ID `short-command`, a provider ID containing `go-whisper`, a model hash, and a matching expected transcript:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -report report.json \
  -acceptance-preset
```

For Linear comments, use `BenchmarkReport.EvidenceMarkdown(validationProblems)` alongside the JSON artifact. The Markdown summary intentionally reports fixture IDs, provider/model metadata, validation problems, and aggregate match counts without including raw transcript bodies.

For shareable JSON evidence, generate a public copy from the full local report:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -report report.json \
  -acceptance-preset \
  -public-json > report.public.json
```

The public JSON keeps provider/model metadata, latency, resource samples, match booleans, and `expectedTranscriptSet`/`transcriptReturned` flags, but omits the expected and returned transcript bodies. Keep the full `report.json` local unless a future debug process explicitly allows sharing transcript text.

Before running the fixture benchmark, record the local model identity with a provider model evidence
artifact. Replace the hash with the actual model file digest:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -model-evidence \
  -issue JUT-13 \
  -kind stt \
  -provider-id go-whisper \
  -model-id tiny-en \
  -model-hash sha256:replace-with-model-hash \
  -model-source whisper-cpp \
  -model-format ggml \
  -model-compatibility compatible \
  -model-runtime-status loaded
```

The command exits non-zero until `model-hash` is a real `sha256:<64 hex>` value and the model has
actually loaded. Use this artifact next to the benchmark report so reviewers can distinguish
downloaded model identity from transcription quality.

The same validation and Markdown output can be generated from a saved report:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -report report.json \
  -acceptance-preset
```

The command exits non-zero when strict validation fails, while still printing the privacy-safe Markdown evidence summary. Saved benchmark report JSON is decoded with unknown-field rejection before validation, so copied artifacts that include raw audio, provider debug fields, or undeclared transcript fields are rejected instead of silently ignored.

Use `NewBenchmarkToneFixture`, `BenchmarkUtteranceFromPCM`, `EncodeBenchmarkWAV`, `DecodeBenchmarkWAV`, and `BenchmarkBytesSHA256` to generate or load deterministic 16 kHz mono PCM fixtures. Synthetic tones are not a quality benchmark for transcription accuracy, but they verify provider plumbing, payload handling, timing/report generation, and offline failure behavior before household or recorded speech fixtures are introduced.

Generate a deterministic plumbing fixture with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -tone-fixture stt/plumbing-tone.wav \
  -tone-duration 500ms \
  -tone-frequency 440 \
  -tone-amplitude 0.25
```

Synthetic tone fixtures are for provider payload/report plumbing only and do not prove STT quality.

For repeated comparisons, prefer `RunSTTBenchmarkSuite` so all fixtures, gaps, environment metadata, and result summaries are emitted in a single JSON report.

For real speech fixtures, declare a fixture-set manifest and load it with `DecodeBenchmarkFixtureSetManifest` plus `LoadBenchmarkFixtureSet`. The manifest uses relative WAV paths, optional `sha256:` checks, expected transcript text, language metadata, fixture source, `recordedAt`, and explicit consent metadata. The loader rejects unknown fields, absolute paths, parent traversal, remote paths, hash mismatches, and non-16-bit mono PCM WAV files. The JUT-13 acceptance preset requires each required fixture to include `source`, RFC3339 `recordedAt`, and `consent: true` before any provider report can count as closure evidence.

Generate a starter fixture-set manifest with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark -fixture-template stt > stt-fixtures.json
```

Replace the placeholder WAV paths, `sha256:` values, `source`, and `recordedAt` values after recording or selecting the actual 16 kHz mono speech fixtures. Keep `consent: true` only for fixtures that are approved for benchmark use.

Validate and hash each WAV fixture with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark -fixture-hash stt/short-command.wav
```

Paste the printed `sha256:` value into `stt-fixtures.json`.

Validate the filled fixture manifest before running providers:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -fixture-manifest stt-fixtures.json \
  -fixture-dir . \
  -acceptance-preset
```

The command verifies the manifest, WAV files, and JUT-13 fixture requirements, then prints a fixture summary without transcript bodies. With `-acceptance-preset`, the required `short-command` fixture must declare `expectedTranscript`. Fixture manifests are decoded as strict JSON: duplicate required IDs, unknown fields, and trailing JSON are rejected before any provider report can use the fixture set.

Run a local command-style go-whisper sidecar or CLI over the same fixture manifest with
`-stt-command`. The command path must be absolute; bare command names resolved through `PATH` are
rejected for closure evidence. The benchmark command writes each validated fixture utterance to a temporary WAV,
replaces `{fixture}` in `-stt-command-args` with that path, removes the temporary file after the
provider returns, and emits a normal `BenchmarkReport`:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -fixture-manifest stt-fixtures.json \
  -fixture-dir . \
  -stt-command /absolute/path/to/go-whisper-command \
  -stt-command-args-json '["transcribe", "--model", "tiny.en", "--input", "{fixture}", "--json"]' \
  -provider-id go-whisper-command \
  -model-id tiny.en \
  -model-hash sha256:replace-with-model-hash \
  -acceptance-preset > go-whisper-report.json
```

Prefer `-stt-command-args-json` for real evidence so model paths and provider flags are passed as an
exact argv array without shell interpolation or whitespace splitting. The simpler `-stt-command-args`
flag remains available for quick wrappers; do not set both flags in the same run. The command
provider may print either plain transcript text or a JSON object with `text` or `transcript`, plus
optional `providerId`, `modelId`, `language`, `durationMs`, or `duration`. JSON stdout is decoded
strictly: unknown fields, trailing JSON, raw fixture paths, provider debug payloads, or undeclared
transcript fields fail the provider run and are recorded only as provider failure evidence. The full
local `go-whisper-report.json` includes transcript bodies so strict validation can prove the fixture
transcript matched. Generate a redacted sharing copy with `-public-json` after validation when
posting evidence outside the local machine.

Generate a provider-unavailable fixture report before wiring the real sidecar, or when documenting the offline/misconfigured failure mode:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -fixture-manifest stt-fixtures.json \
  -fixture-dir . \
  -fixture-failure-report \
  -acceptance-preset \
  -public-json \
  -provider-id go-whisper \
  -model-id tiny.en \
  -model-hash sha256:replace-with-model-hash > go-whisper-unavailable.public.json
```

The failure report records each fixture as `provider_unavailable` and carries the provider/model metadata without exposing STT transcript bodies in the public JSON. With `-acceptance-preset`, this command is expected to exit non-zero because the generated report still has unresolved gaps and provider failures. Replace the placeholder model hash before sharing the artifact; placeholder or malformed hashes are reported as acceptance validation problems.

Required measurements:

- model name and checksum;
- cold model load time;
- warm transcription latency for a short utterance;
- peak memory and CPU usage;
- transcript comparison against an expected fixture transcript;
- observed failure mode when the sidecar is offline or misconfigured.

Minimum fixture:

- one short 16 kHz mono WAV file checked into test fixtures or generated deterministically;
- one small local model such as a tiny or base English model;
- one repeated warm run to catch unstable latency;
- a comparison against Jute's provider contract, not against go-whisper internals.

The benchmark should not require a microphone, cloud credentials, or live household audio.

## Closure Gate For JUT-13

Do not close the Linear spike on manifest validation alone. The following evidence is required:

- a filled `stt-fixtures.json` containing at least the `short-command` 16 kHz mono WAV fixture with
  a `sha256:` fixture hash and expected transcript;
- a real go-whisper run using a small local model, with a non-placeholder `sha256:<64 hex>` model
  hash and a successful transcript match for `short-command`;
- `go run ./apps/hub/cmd/jute-voice-benchmark -report report.json -acceptance-preset` exits zero;
- CPU-only packaging notes are checked against the actual command or container used for the run;
- Metal/macOS and containerized Linux notes record either a successful command or a concrete
  unavailable/untested finding;
- Raspberry Pi remains `false` in the manifest unless a representative ARM64 run proves acceptable
  latency and resource use.

Provider-unavailable public JSON artifacts and placeholder model hashes are useful negative evidence
for failure-mode documentation, but they intentionally fail the JUT-13 acceptance preset and must not
be used as closure evidence.

## Packaging Notes

CPU-only Docker is the lowest-friction path for local development. Bind the service to loopback, persist model data outside the container, and do not pass cloud provider environment variables by default.

macOS Metal should be treated as a native/provider-pack packaging target, not a hub dependency. Validate local build instructions, model storage location, and whether the provider can run without elevated permissions.

Containerized Linux can use CPU-only, CUDA, or Vulkan variants when available. GPU containers need explicit device access and should degrade to `offline` or `degraded` provider health when the expected accelerator is unavailable.

Raspberry Pi support remains benchmark-required. The manifest fixture must keep `raspberryPi: false` until the closure packaging matrix has a succeeded `raspberry-pi-arm64` target and an ARM64 CPU-only or accelerator-specific package has acceptable wake-to-final-transcript latency on representative hardware.

Because this path may involve native whisper.cpp and ffmpeg dependencies, distribution notes must preserve upstream license notices. Keeping go-whisper as an external sidecar avoids making those dependencies part of the hub binary.

### Native CLI Build Attempt

Checked on 2026-06-17 from the Jute worktree without modifying hub dependencies:

```sh
env GOBIN=/private/tmp/jute-go-whisper-bin \
  GOMODCACHE=/private/tmp/jute-go-whisper-modcache \
  GOCACHE=/tmp/jute-dash-go-build-cache \
  go install github.com/mutablelogic/go-whisper/cmd/gowhisper@v0.0.39
```

The command downloaded upstream Go modules, then failed during cgo/native package discovery:

```text
github.com/mutablelogic/go-media/sys/ffmpeg80: exec: "pkg-config": executable file not found in $PATH
github.com/mutablelogic/go-whisper/sys/whisper: exec: "pkg-config": executable file not found in $PATH
```

`pkg-config` was not available on the runner PATH. This is useful packaging evidence for the native
macOS/Linux path: a Jute provider-pack recipe cannot rely on plain `go install`; it must either
document/install `pkg-config` plus the expected ffmpeg and whisper.cpp development artifacts, or use
a container/provider bundle that carries those native dependencies. This failed build does not satisfy
the fixture benchmark closure gate because no local model was downloaded and no transcription run
executed.

Generate a repeatable, privacy-safe Markdown summary for this failed build with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -build-evidence \
  -issue JUT-13 \
  -kind stt \
  -provider-id go-whisper \
  -build-target native-cli \
  -build-command-id go-install \
  -build-status failed \
  -build-exit-code 1 \
  -build-error-code missing-pkg-config \
  -build-missing pkg-config \
  -environment-notes "go install failed while resolving ffmpeg and whisper native bindings"
```

The command exits non-zero for failed builds and prints `Closure evidence: false` with the validation
problem `provider build did not succeed`. Use that Markdown in Linear as packaging evidence, not as
JUT-13 completion evidence.

Record the cross-target packaging matrix separately so Linear evidence shows which required targets
have concrete findings and which remain unsupported:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -packaging-evidence \
  -issue JUT-13 \
  -kind stt \
  -provider-id go-whisper \
  -packaging-targets cpu-only=failed,macos-metal=blocked,container-linux=not-run,raspberry-pi-arm64=unsupported \
  -environment-notes "native go install failed before model execution; container run still pending"
```

The JUT-13 packaging matrix requires `cpu-only`, `macos-metal`, `container-linux`, and
`raspberry-pi-arm64` target statuses. `not-run` is reported as a validation problem, so the matrix
can document partial coverage without pretending the packaging subcriterion is complete. The
packaging matrix is still not closure evidence by itself; the real STT fixture benchmark and local
model evidence remain required.

Before moving the Linear spike to Done, compose the generated artifact files into one closure bundle
and validate the whole evidence set:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -closure-bundle-compose JUT-13 \
  -decision-status documented-external-provider \
  -decision-rationale "Measured build, packaging, model, and fixture benchmark evidence support documenting go-whisper externally for v1." \
  -provider-manifest go-whisper-provider.json \
  -fixture-manifest-artifact stt-fixtures.json \
  -build-evidence-artifacts go-whisper-build.json \
  -packaging-evidence-artifact go-whisper-packaging.json \
  -model-evidence-artifacts go-whisper-model.json \
  -benchmark-report-artifact go-whisper-report.json > go-whisper-closure-bundle.json

go run ./apps/hub/cmd/jute-voice-benchmark \
  -closure-bundle go-whisper-closure-bundle.json
```

The composer validates the assembled bundle before writing it, so missing artifacts, malformed rows,
placeholder hashes, incomplete packaging, or non-matching benchmark/model evidence fail before the
bundle can be pasted into Linear. Use `-closure-bundle-template JUT-13` only as a schema reference;
the generated template is a skeleton, not passing evidence.

For JUT-13, the closure bundle requires:

- a decision block with status `first-class-provider-pack`, `documented-external-provider`, or `defer`,
  plus a non-placeholder rationale grounded in the build, packaging, model, and benchmark evidence;
- a provider manifest artifact that decodes with the hub manifest parser, passes
  `ValidateProviderManifest`, identifies the mutablelogic go-whisper candidate, uses kind `stt`,
  includes `mutablelogic` in `contribution.maintainers`, and selects `http-json` or `command`
  transport, while declaring offline capability and no credential requirements;
- a fixture manifest artifact that decodes with the strict benchmark fixture parser, declares the
  required `short-command` fixture with expected transcript, source, RFC3339 `recordedAt`, consent,
  concrete source, and real `sha256:<64 hex>` fixture hash;
- generated build, packaging, and model evidence artifacts must not carry `problems`, and their
  completion flags must remain true (`closureEvidence` for build/model rows and
  `packagingEvidenceComplete` for packaging);
- every generated build, packaging, model, and benchmark artifact must declare `issue: "JUT-13"`;
  evidence generated for another provider spike cannot satisfy this closure bundle;
- at least one successful go-whisper build evidence row with a concrete OS/arch/toolchain runtime
  and RFC3339 `generatedAt`;
- a complete packaging matrix for `cpu-only`, `macos-metal`, `container-linux`, and
  `raspberry-pi-arm64`, with notes explaining every target that is `failed`, `blocked`, or
  `unsupported`, plus a concrete OS/arch/toolchain runtime and RFC3339 `generatedAt`;
  `hardware.raspberryPi` may only be true when the `raspberry-pi-arm64` packaging target succeeded,
  and a succeeded target must be reflected in the provider manifest; a decision of
  `documented-external-provider` requires `cpu-only` or `container-linux` packaging to have
  succeeded, and `first-class-provider-pack` requires every required packaging target to have
  succeeded;
- at least one compatible and loaded go-whisper model evidence row with a concrete
  OS/arch/toolchain runtime and RFC3339 `generatedAt`;
- one strict benchmark report that passes the JUT-13 acceptance preset and uses the same
  `modelId`/`modelHash` as a compatible, loaded model evidence row, with every closure fixture
  manifest row measured, no undeclared result fixture IDs, matching the manifest's expected
  transcript, and
  carrying concrete OS/arch/Go runtime that matches the model evidence runtime and RFC3339
  `generatedAt`.

The closure bundle summary is privacy-safe: it reports counts, accepted gates, and validation
problems without transcript bodies, raw audio paths, or provider debug notes.

## Security And Privacy

- Bind the sidecar to loopback by default.
- Never expose cloud API keys through the manifest.
- Keep raw audio out of logs.
- Report health transitions and provider IDs, not transcript bodies.
- Send only final transcripts to the hub.
- Never send raw microphone audio, pre-roll buffers, or partial transcripts to A2A agents.

## Follow-Up Work

- Run the fixture benchmark on CPU-only macOS or Linux.
- Validate a Docker command and a native macOS Metal setup.
- Decide whether Jute ships a provider-pack recipe, a tested external provider pack, or defers go-whisper behind documentation only.
