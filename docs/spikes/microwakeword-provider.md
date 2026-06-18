# microWakeWord Provider Spike

## Status

Provider manifest validation, benchmark tooling, native build failure evidence, a complete
packaging matrix, and an ESPHome model hash are in place. Model runtime compatibility and the audio
fixture benchmark against openWakeWord are still pending.

## Upstream Snapshot

Verified on 2026-06-17 against
[pmdroid/microwakeword](https://github.com/pmdroid/microwakeword),
[ESPHome micro-wake-word-models](https://github.com/esphome/micro-wake-word-models), and
[OHF micro-wake-word](https://github.com/OHF-Voice/micro-wake-word):

- `pmdroid/microwakeword` is still a small MIT-licensed Go library, not a packaged service. It has
  very little repository activity, one public release, and no production-style binary distribution.
- The upstream README requires Go, GCC, Bazel, Git, TensorFlow Lite C, and the TensorFlow audio
  microfrontend. Its Makefile downloads TensorFlow v2.19.0 and KissFFT, builds native libraries, and
  installs shared libraries under `/usr/local/lib` by default.
- The repository includes a microphone example, but Jute still needs fixture-driven tests before any
  live microphone trial has product value.
- ESPHome's model repository hosts `.tflite` files for the ESPHome `micro_wake_word` component and
  documents `okay_nabu` as a model reference name, but Jute has not yet proved those model assets are
  directly compatible with pmdroid's expected config/runtime shape.
- OHF's `micro-wake-word` project remains relevant as the training framework lineage, not as a
  drop-in Jute runtime provider.

This snapshot supports the current decision to defer production adoption and keep
`wyoming-openwakeword` as the wake-word baseline until pmdroid/microWakeWord has reproducible build,
model-compatibility, and fixture benchmark evidence.

## Decision

Defer `pmdroid/microwakeword` as a production provider for v1. Keep `wyoming-openwakeword` as the first wake-word baseline, and treat microWakeWord as an experimental provider-pack candidate until Jute has reproducible builds, model compatibility proof, and benchmark results against the Wyoming/openWakeWord path.

Do not add `github.com/pmdroid/microwakeword` or TensorFlow Lite dependencies to production code as part of this spike.

## Rationale

`pmdroid/microwakeword` is attractive because it exposes a Go API over TensorFlow Lite and the TensorFlow audio microfrontend. It can process streaming 16 kHz mono PCM in 10 ms chunks and uses `.tflite` microWakeWord models, which lines up with Jute's local wake-word direction.

The tradeoff is that it is not a pure Go dependency. The upstream build path pulls TensorFlow, KissFFT, Bazel, cgo, and shared libraries, then installs TensorFlow Lite C and microfrontend artifacts into a system prefix. That is too heavy for the hub binary and too fragile for v1 distribution without a provider-pack wrapper and reproducible packaging.

## Candidate Provider Shape

If this path is revisited, prefer an isolated provider pack over a direct hub import:

This experimental manifest draft is kept as a machine-validated fixture at:

```text
apps/hub/internal/app/voice/testdata/provider_manifests/microwakeword_experimental.json
```

```json
{
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
    "url": "https://github.com/pmdroid/microwakeword/blob/main/LICENSE"
  },
  "contribution": {
    "source": "https://github.com/pmdroid/microwakeword",
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
      }
    ]
  }
}
```

The manifest above is intentionally marked `experimental`. The `builtin` transport only becomes reasonable if the local Jute Voice Service owns the native dependency bundle and exposes the same wake-word contract as other providers. Until then, a separate sidecar process is safer than linking native TensorFlow Lite into the hub.

`TestSpikeProviderManifestFixturesValidate` decodes this fixture with unknown-field and trailing-JSON rejection, then runs it through the hub manifest validator. `TestSpikeProviderCandidatesAreNotProductionDependencies` also checks that pmdroid/microWakeWord and TensorFlow have not been added to `go.mod` or `go.sum`. Those tests keep Raspberry Pi support marked false, prove model paths remain provider-pack-relative, and make sure the spike cannot drift into unsafe remote/global-path model loading or accidental production dependency adoption.

If this spike later switches from `builtin` to a trusted `command` provider pack, the manifest must
declare an absolute `transport.command` and include `{inputPath}` in `transport.args` so the voice
service can pass the captured wake-word audio explicitly. Bare command names and command manifests
without an audio input placeholder are rejected by validation.

## Build And Run Notes

### macOS

Expected blockers:

- Bazel and a C compiler are required.
- TensorFlow v2.19.0 and KissFFT are downloaded during the upstream Makefile build.
- Shared libraries are installed under `/usr/local/lib` by default.
- cgo must find TensorFlow Lite C headers and libraries at build and runtime.
- Microphone examples also depend on the Go microphone/PortAudio stack.

Validation steps:

```sh
git clone https://github.com/pmdroid/microwakeword.git
cd microwakeword
make INSTALL_PREFIX=/tmp/jute-microwakeword
make install INSTALL_PREFIX=/tmp/jute-microwakeword
CGO_ENABLED=1 \
  CGO_CFLAGS="-I/tmp/jute-microwakeword/include" \
  CGO_LDFLAGS="-L/tmp/jute-microwakeword/lib" \
  go test ./...
```

Do not require `sudo make install` for a Jute provider pack. The provider must be relocatable inside a Jute-managed provider directory.

#### Native Consumer Build Attempt

Checked on 2026-06-17 from a disposable module under `/private/tmp`, without adding
`github.com/pmdroid/microwakeword` to Jute production dependencies:

```sh
mkdir -p /private/tmp/jute-microwakeword-build
cd /private/tmp/jute-microwakeword-build
go mod init jute-microwakeword-build
env GOMODCACHE=/private/tmp/jute-microwakeword-modcache \
  GOCACHE=/tmp/jute-dash-go-build-cache \
  go get github.com/pmdroid/microwakeword@v0.0.1
env GOMODCACHE=/private/tmp/jute-microwakeword-modcache \
  GOCACHE=/tmp/jute-dash-go-build-cache \
  go test github.com/pmdroid/microwakeword
```

The module fetch succeeded. The package build failed before tests could run because the TensorFlow
Lite audio microfrontend header was not installed:

```text
pkg/microfrontend/microfrontend.go:7:11: fatal error:
'tensorflow/lite/experimental/microfrontend/lib/frontend.h' file not found
```

This confirms the pmdroid path is not a normal Go-only dependency. A Jute provider-pack recipe must
first build or bundle the TensorFlow Lite C and audio microfrontend artifacts, then set include and
library paths explicitly. This failed build is valid packaging evidence, but it does not satisfy the
JUT-11 closure gate because no model loaded, no wake detection ran, and no openWakeWord baseline
comparison was produced.

Generate a repeatable, privacy-safe Markdown summary for this failed build with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -build-evidence \
  -issue JUT-11 \
  -kind wake-word \
  -provider-id pmdroid-microwakeword \
  -build-target native-consumer \
  -build-command-id go-test-package \
  -build-status failed \
  -build-exit-code 1 \
  -build-error-code missing-header \
  -build-missing tensorflow-lite-microfrontend-header \
  -environment-notes "consumer package build failed before tests because TensorFlow Lite audio microfrontend headers were absent"
```

The command exits non-zero for failed builds and prints `Closure evidence: false` with the validation
problem `provider build did not succeed`. Use that Markdown in Linear as native packaging evidence,
not as JUT-11 completion evidence.

#### Upstream Make Attempt

Checked again on 2026-06-18 from a disposable clone under `/tmp/jute-microwakeword-jut11`:

```sh
git clone --depth 1 https://github.com/pmdroid/microwakeword.git /tmp/jute-microwakeword-jut11/microwakeword
cd /tmp/jute-microwakeword-jut11/microwakeword
make INSTALL_PREFIX=/tmp/jute-microwakeword-jut11/install
```

The upstream Makefile downloaded TensorFlow v2.19.0 and KissFFT, then failed in the `build` target
before Bazel ran:

```text
Checking if build is already done...
/bin/sh: -c: line 1: syntax error: unexpected end of file
make: *** [build] Error 2
```

The failure occurs in the continued shell block around `Makefile` lines 67-76, where comment lines
are mixed into the same continued `if` block. The local machine also lacks `bazel`, `pkg-config`,
and installed TensorFlow Lite microfrontend headers, so this remains negative build evidence rather
than runnable provider evidence.

Record the cross-target packaging matrix separately so Linear evidence shows which required targets
have concrete findings and which remain unsupported:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -packaging-evidence \
  -issue JUT-11 \
  -kind wake-word \
  -provider-id pmdroid-microwakeword \
  -packaging-targets macos-native=failed,linux-native=blocked,raspberry-pi-arm64=unsupported \
  -environment-notes "macOS native upstream make failed before provider build; Linux blocked pending container recipe; Raspberry Pi unsupported until ARM64 TensorFlow Lite microfrontend build and latency are proven"
```

The JUT-11 packaging matrix requires at least one evaluated target with notes for failures, blocked
targets, or unsupported targets. macOS, Linux, and Raspberry Pi rows remain useful comparison data
when feasible, but they are not all required to close the spike. On 2026-06-18 this matrix validated
with `packagingEvidenceComplete: true`, but it is still not closure evidence by itself; model
compatibility proof and the pmdroid-vs-openWakeWord fixture comparison remain required.

Before moving the Linear spike to Done, combine the generated public JSON artifacts into one closure
bundle and validate the whole evidence set:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -closure-bundle-template JUT-11 > microwakeword-closure-bundle.json

go run ./apps/hub/cmd/jute-voice-benchmark \
  -closure-bundle microwakeword-closure-bundle.json
```

The generated bundle is a schema-correct skeleton, not passing evidence. Replace placeholder hashes,
target statuses, model evidence, candidate benchmark report, and baseline benchmark report with real
generated artifacts before using the validation summary in Linear.

For JUT-11, the closure bundle requires:

- a decision block with status `adopt-optional-provider`, `defer`, or `reject`, plus a
  non-placeholder rationale grounded in the build, packaging, model, and benchmark evidence;
- a provider manifest artifact that decodes with the hub manifest parser, passes
  `ValidateProviderManifest`, identifies the pmdroid microWakeWord candidate, uses kind
  `wake-word`, includes `pmdroid` in `contribution.maintainers`, declares offline capability, and
  has no credential requirements;
- a fixture manifest artifact that decodes with the strict benchmark fixture parser, declares all
  required wake fixtures with expected wake/no-wake flags, source, RFC3339 `recordedAt`, consent,
  concrete source, and unique real `sha256:<64 hex>` fixture hashes;
- generated build, packaging, and model evidence artifacts must not carry `problems`, and their
  completion flags must remain true (`closureEvidence` for build/model rows and
  `packagingEvidenceComplete` for packaging);
- every generated build, packaging, model, benchmark, and baseline artifact must declare
  `issue: "JUT-11"`; evidence generated for another provider spike cannot satisfy this closure
  bundle;
- at least one pmdroid/microWakeWord build evidence row with a concrete OS/arch/toolchain runtime
  and RFC3339 `generatedAt`; `defer` and `adopt-optional-provider` require successful build
  evidence, while `reject` may use generated failed, blocked, or interrupted build evidence;
- at least one evaluated packaging target, with notes explaining every target that is `failed`,
  `blocked`, or `unsupported`, plus a concrete OS/arch/toolchain runtime and RFC3339 `generatedAt`;
  `hardware.raspberryPi` may only be true when the `raspberry-pi-arm64` packaging target succeeded,
  and a succeeded target must be reflected in the provider manifest; a decision of
  `adopt-optional-provider` requires at least one packaging target to have succeeded;
- for `defer` or `adopt-optional-provider`, compatible and loaded model evidence for at least one
  ESPHome model and one OHF-trained or OHF-compatible model, each with a concrete
  OS/arch/toolchain runtime, RFC3339 `generatedAt`, and a matching `wakeWord.models[].id` entry in
  the provider manifest;
- for `defer` or `adopt-optional-provider`, one strict pmdroid/microWakeWord benchmark report that
  passes the JUT-11 acceptance preset and uses the same `modelId`/`modelHash` as one compatible,
  loaded model evidence row, with every closure fixture manifest row measured, no undeclared result
  fixture IDs, and matching expected wake/no-wake flags, plus concrete OS/arch/Go runtime that
  matches the model evidence runtime and RFC3339 `generatedAt`;
- for `defer` or `adopt-optional-provider`, one strict openWakeWord/Wyoming baseline report over
  every declared fixture ID and expected wake/no-wake flag, plus concrete OS/arch/Go runtime and
  RFC3339 `generatedAt`;
- for `defer` or `adopt-optional-provider`, a passing pmdroid-vs-openWakeWord comparison.

The closure bundle summary is privacy-safe: it reports counts, accepted gates, and validation
problems without raw audio paths or provider debug notes.

### Linux

Expected blockers:

- Bazel/TensorFlow build time can be large.
- Distribution packages need `glibc`, cgo, and dynamic loader paths aligned with the target image.
- Container builds should copy only runtime artifacts, headers needed for provider builds, model assets, and the provider executable.

Validation steps:

```sh
make INSTALL_PREFIX=/opt/jute/microwakeword
LD_LIBRARY_PATH=/opt/jute/microwakeword/lib go test ./...
```

Prefer a containerized build stage that produces a small runtime image. Do not install TensorFlow build trees on the target appliance.

#### Linux Container Consumer Build Attempt

Checked on 2026-06-18 in a disposable `golang:1.25` Linux container, without adding
`github.com/pmdroid/microwakeword` to Jute production dependencies:

```sh
docker run --rm golang:1.25 sh -lc '
  GO=/usr/local/go/bin/go
  mkdir -p /tmp/jute-microwakeword-linux-build
  cd /tmp/jute-microwakeword-linux-build
  $GO mod init jute-microwakeword-linux-build
  $GO get github.com/pmdroid/microwakeword@v0.0.1
  $GO test github.com/pmdroid/microwakeword
'
```

The module fetch succeeded. The package build failed before tests could run because the TensorFlow
Lite audio microfrontend header was not installed:

```text
pkg/microfrontend/microfrontend.go:7:11: fatal error:
tensorflow/lite/experimental/microfrontend/lib/frontend.h: No such file or directory
```

The public build evidence generated inside the same container records runtime `linux/arm64
go1-25-11`, target `linux-native-container`, status `failed`, and missing dependency
`tensorflow-lite-microfrontend-header`. This is stronger Linux packaging evidence than the earlier
blocked placeholder, but it still does not satisfy JUT-11 closure because no model loaded, no wake
detection ran, and no openWakeWord baseline comparison was produced.

#### Linux Container Upstream Make Attempt

Checked on 2026-06-18 in the same disposable `golang:1.25` Linux container:

```sh
docker run --rm golang:1.25 sh -lc '
  mkdir -p /tmp/jute-microwakeword-linux-make
  cd /tmp/jute-microwakeword-linux-make
  git clone --depth 1 https://github.com/pmdroid/microwakeword.git microwakeword
  cd microwakeword
  make INSTALL_PREFIX=/tmp/jute-microwakeword-linux-make/install
'
```

The container had `git`, `make`, and `gcc`. The upstream Makefile cloned TensorFlow v2.19.0 and
KissFFT, then failed in the `build` target because Bazel was not installed:

```text
Building library...
/bin/sh: 8: bazel: not found
make: *** [Makefile:67: build] Error 127
```

This confirms the Linux provider-pack recipe must install/pin Bazel before it can even reach the
TensorFlow Lite and audio microfrontend build products. It is still failed build evidence, not
runnable provider evidence.

#### Linux Container Bazelisk And Python 3.12 Attempt

Checked on 2026-06-18 in the same disposable `golang:1.25` Linux container with Bazel supplied by
Bazelisk:

```sh
docker run --rm golang:1.25 sh -lc '
  GO=/usr/local/go/bin/go
  mkdir -p /tmp/bin /tmp/jute-microwakeword-linux-py312
  $GO install github.com/bazelbuild/bazelisk@latest
  cp /go/bin/bazelisk /tmp/bin/bazel
  export PATH=/tmp/bin:$PATH
  export HERMETIC_PYTHON_VERSION=3.12
  cd /tmp/jute-microwakeword-linux-py312
  git clone --depth 1 https://github.com/pmdroid/microwakeword.git microwakeword
  cd microwakeword
  make INSTALL_PREFIX=/tmp/jute-microwakeword-linux-py312/install
'
```

Without `HERMETIC_PYTHON_VERSION=3.12`, TensorFlow selected Python 3.13 and failed because
TensorFlow v2.19.0 only provides requirement lockfiles for Python 3.9, 3.10, 3.11, and 3.12.

With `HERMETIC_PYTHON_VERSION=3.12`, Bazel built the TensorFlow Lite audio microfrontend target:

```text
Target //tensorflow/lite/experimental/microfrontend/lib:microfrontend up-to-date:
  bazel-bin/tensorflow/lite/experimental/microfrontend/lib/libmicrofrontend.a
  bazel-bin/tensorflow/lite/experimental/microfrontend/lib/libmicrofrontend.pic.a
  bazel-bin/tensorflow/lite/experimental/microfrontend/lib/libmicrofrontend.so
```

The build then advanced to `//tensorflow/lite/c:tensorflowlite_c` and failed when the Bazel server
terminated while compiling TensorFlow Lite C:

```text
Server terminated abruptly (error code: 14, error message: 'Socket closed')
make: *** [Makefile:67: build] Error 37
```

The public build evidence generated inside the same container records runtime `linux/arm64
go1-25-11`, target `linux-native-upstream-make-bazelisk-python312`, status `failed`, exit code
`37`, and error code `bazel-server-socket-closed`. This proves the provider-pack recipe needs
Bazelisk or pinned Bazel plus a Python 3.12 hermetic setting, and still needs a successful
TensorFlow Lite C build before pmdroid can load a model.

Retrying through a wrapper that inserts `--jobs=1 --local_ram_resources=2048` after the Bazel
`build` subcommand avoided the Bazel server socket failure and compiled much further:

```sh
make INSTALL_PREFIX=/tmp/jute-microwakeword-linux-lowjobs-wrap/install \
  BAZEL=/tmp/bin/bazelwrap
```

The microfrontend target still built successfully, but TensorFlow Lite C failed compiling an XNNPACK
ARM64 FP16 pooling kernel with GCC 14:

```text
external/XNNPACK/src/f16-pavgpool/f16-pavgpool-9x-minmax-neonfp16arith-c8.c:31:71:
error: passing argument 1 of 'vld1q_dup_u16' from incompatible pointer type
expected 'const uint16_t *' but argument is of type 'const xnn_float16 *'
Target //tensorflow/lite/c:tensorflowlite_c failed to build
make: *** [Makefile:67: build] Error 1
```

The public build evidence generated inside the same container records runtime `linux/arm64
go1-25-11`, target `linux-native-upstream-make-bazelisk-python312-jobs1`, status `failed`, exit
code `1`, and error code `xnnpack-neonfp16arith-gcc14-pointer-type`. This moves the Linux recipe's
next blocker from resource limits to C toolchain compatibility: pinning Bazel and Python is not
enough; the provider pack must also pin or patch a TensorFlow/XNNPACK/GCC combination that builds
on ARM64.

Re-running the same low-jobs upstream Makefile path in `golang:1.25-bookworm` changed the compiler
from GCC 14 to GCC 12 and avoided the XNNPACK pointer-type failure. The TensorFlow Lite C build
advanced to 1,320 of 1,462 actions, then failed when `gcc` killed `cc1plus` while compiling
`tensorflow/lite/kernels/cast.cc`:

```text
gcc: fatal error: Killed signal terminated program cc1plus
compilation terminated.
Target //tensorflow/lite/c:tensorflowlite_c failed to build
make: *** [Makefile:67: build] Error 1
```

The public build evidence generated inside `golang:1.25-bookworm` records runtime `linux/arm64
go1-25-11`, target `linux-arm64-bookworm-go125-upstream-make-bazelisk-python312-jobs1`, status
`failed`, exit code `2`, and error code `resource-exhausted-cc1plus-killed`. This narrows the
Linux recipe again: GCC 12 avoids the observed ARM64 FP16 compiler incompatibility, but the
provider-pack build still needs a runner with enough memory or a leaner TensorFlow Lite C build
configuration before pmdroid can load a model.

An attempted `linux/amd64` Docker run under local emulation did not produce provider evidence. The
emulated Go toolchain crashed while compiling the local evidence helper:

```text
math: /usr/local/go/pkg/tool/linux_amd64/asm: signal: segmentation fault
```

Treat AMD64 status as unmeasured until it runs on native AMD64 hardware or a more reliable CI runner;
do not infer that the ARM64 XNNPACK/GCC failure applies to AMD64.

### Raspberry Pi

Raspberry Pi support is unproven for Jute. It needs a native ARM64 build or cross-compiled provider artifact, plus a real latency and CPU benchmark on representative hardware.

The manifest should keep `raspberryPi: false` until:

- the closure packaging matrix has a succeeded `raspberry-pi-arm64` target;
- TensorFlow Lite C and microfrontend libraries build reproducibly for ARM64;
- audio fixture detection latency is acceptable;
- always-listening CPU cost stays below the target satellite budget;
- false accept/false reject behavior is measured against the openWakeWord baseline.

## Model Compatibility Notes

The ESPHome model collection hosts `.tflite` files for the ESPHome `micro_wake_word` component. Those assets are promising, but Jute still needs compatibility tests against pmdroid's JSON config format and runtime assumptions.

Checked on 2026-06-18, `models/v2/okay_nabu.json` declares:

- wake word: `Okay Nabu`;
- model file: `okay_nabu.tflite`;
- trained languages: `en`;
- feature step size: `10`;
- sliding window size: `5`;
- tensor arena size: `26080`;
- minimum ESPHome version: `2024.7.0`.

The downloaded `models/v2/okay_nabu.tflite` file is 59 KB with hash
`sha256:0689abe1912a95a3318a0d8cb2e67bad0cbcfe3e24dd6e050c75debddfb6f891`.
This is real model asset evidence, but runtime compatibility is still blocked because pmdroid did
not build or load the model locally.

Each accepted model must be packaged inside the provider pack and referenced through the `wakeWord.models[].path` manifest field. Remote model URLs, absolute paths, and parent-directory traversal are still rejected by Jute manifest validation.

Record each candidate model with a provider model evidence artifact. For example, an ESPHome model
that has a file hash but has not yet loaded in pmdroid should be recorded as blocked rather than
treated as compatible:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -model-evidence \
  -issue JUT-11 \
  -kind wake-word \
  -provider-id pmdroid-microwakeword \
  -model-id okay-nabu \
  -model-hash sha256:replace-with-model-hash \
  -model-source esphome \
  -model-format tflite \
  -model-compatibility blocked \
  -model-runtime-status not-run \
  -environment-notes "native dependencies missing before model load"
```

The command exits non-zero until the hash is a real `sha256:<64 hex>` value, compatibility is
`compatible`, and the runtime status is `loaded`. JUT-11 closure still needs at least one ESPHome
model and one OHF-trained or OHF-compatible model with successful model evidence before the benchmark
comparison can be considered complete.

## Benchmark Plan

The benchmark should compare pmdroid/microWakeWord against the existing `wyoming-openwakeword` baseline using identical fixture audio.

The repo now has a provider-neutral fixture harness in:

```text
apps/hub/internal/app/voice/benchmark.go
```

Use `RunWakeBenchmark` or `RunWakeBenchmarkSuite` to adapt pmdroid/microWakeWord and the openWakeWord/Wyoming baseline to the same result shape. The harness records fixture ID, provider/model IDs, detection result, expected-result flags, latency, and allocation samples without storing or emitting PCM bytes.

Wrap each provider run in `NewWakeBenchmarkReport("JUT-11", ...)` and attach the JSON output from `BenchmarkReport.JSON()` to Linear or this document. The report should include provider ID, model ID, model hash when available, OS/architecture, Go version, result summary, and explicit gaps.

Before treating a run as acceptance evidence, pass the report through `ValidateBenchmarkReport` with `Issue: "JUT-11"`, `Kind: "wake-word"`, `RequireModelHash: true`, `RequireAllWakeMatches: true`, and `RequireResourceSamples: true`. Strict validation should have no unresolved gaps, no provider failures, at least the declared fixture count, unique fixture result IDs, a `sha256:<64 hex>` model hash, result provider/model IDs that match the report environment, a measured `resourceSample.duration` for every successful fixture row, and expected wake/no-wake outcomes for every fixture. Validation recomputes the summary from fixture rows, so copied reports cannot hide provider failures, false accepts, or false rejects by editing aggregate counts.

The benchmark CLI also has an issue-specific acceptance preset for JUT-11 candidate reports. The preset requires the provider ID to identify pmdroid, the named fixture IDs `positive-wake`, `near-miss`, `ambient-room`, and `conversation-long`, a model hash, and matching expected wake/no-wake outcomes:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -report report.json \
  -acceptance-preset
```

For Linear comments, use `BenchmarkReport.EvidenceMarkdown(validationProblems)` alongside the JSON artifact. The Markdown summary intentionally reports fixture IDs, provider/model metadata, validation problems, and aggregate false accept/reject counts without including raw audio or transcript bodies.

The same validation and Markdown output can be generated from a saved report:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -report report.json \
  -acceptance-preset
```

The command exits non-zero when strict validation fails, while still printing the privacy-safe Markdown evidence summary. Saved benchmark report JSON is decoded with unknown-field rejection before validation, so copied artifacts that include raw audio, provider debug fields, or undeclared internals are rejected instead of silently ignored.

After both pmdroid/microWakeWord and the openWakeWord/Wyoming baseline have been run against the same fixture manifest, generate a comparison artifact. The comparison command validates the baseline with the same fixture/model/result requirements while allowing the baseline provider to be openWakeWord/Wyoming instead of pmdroid:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -report microwakeword-report.json \
  -baseline-report openwakeword-report.json \
  -acceptance-preset
```

With `-acceptance-preset`, the command first validates both the pmdroid candidate report and the openWakeWord baseline report against the JUT-11 preset. It fails when either report is missing the required fixture IDs, model hash, matching expected wake/no-wake results, or when the two reports do not share fixture IDs. The comparison also checks provider roles: the candidate provider ID must identify the pmdroid microWakeWord path, and the baseline provider ID must identify openWakeWord through the Wyoming/openWakeWord baseline. The Markdown output reports candidate and baseline providers, shared fixtures, false accepts/rejects, provider failures, and average latency without including raw audio or provider internals. Use this output in the Linear evidence comment next to the individual strict validation summaries.

The comparison also fails if the candidate and baseline provider IDs are the same, if the candidate is a non-pmdroid microWakeWord-like provider, or if the baseline is not openWakeWord/Wyoming. JUT-11 evidence must compare pmdroid/microWakeWord against a distinct openWakeWord/Wyoming baseline, not two copied runs from the same provider or two differently named experimental wake-word candidates.

Use `NewBenchmarkToneFixture`, `BenchmarkUtteranceFromPCM`, `EncodeBenchmarkWAV`, `DecodeBenchmarkWAV`, and `BenchmarkBytesSHA256` to generate or load deterministic 16 kHz mono PCM fixtures. Synthetic tones are not a wake-word quality benchmark, but they verify provider plumbing, payload handling, timing/report generation, and no-audio-leak behavior before real positive/near-miss/ambient fixtures are introduced.

Generate a deterministic plumbing fixture with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -tone-fixture wake/plumbing-tone.wav \
  -tone-duration 500ms \
  -tone-frequency 440 \
  -tone-amplitude 0.25
```

Synthetic tone fixtures are for provider payload/report plumbing only and do not prove wake-word quality.

For repeated comparisons, prefer `RunWakeBenchmarkSuite` so all fixtures, gaps, environment metadata, and result summaries are emitted in a single JSON report.

For real wake-word fixtures, declare a fixture-set manifest and load it with `DecodeBenchmarkFixtureSetManifest` plus `LoadBenchmarkFixtureSet`. The manifest uses relative WAV paths, optional `sha256:` checks, expected wake/no-wake flags, language metadata, fixture source, `recordedAt`, and explicit consent metadata. The loader rejects unknown fields, absolute paths, parent traversal, remote paths, hash mismatches, and non-16-bit mono PCM WAV files. The JUT-11 acceptance preset requires each required fixture to include `source`, RFC3339 `recordedAt`, and `consent: true` before any provider comparison can count as closure evidence.

Generate a starter fixture-set manifest with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark -fixture-template wake-word > wake-fixtures.json
```

Replace the placeholder WAV paths, `sha256:` values, `source`, and `recordedAt` values after recording or selecting the actual positive, near-miss, ambient, and long no-wake fixtures. Keep `consent: true` only for fixtures that are approved for benchmark use.

Validate and hash each WAV fixture with:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark -fixture-hash wake/positive-wake.wav
```

Paste the printed `sha256:` values into `wake-fixtures.json`.

Validate the filled fixture manifest before running providers:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -fixture-manifest wake-fixtures.json \
  -fixture-dir . \
  -acceptance-preset
```

The command verifies the manifest, WAV files, and JUT-11 fixture requirements, then prints a fixture summary without raw audio or transcript bodies. With `-acceptance-preset`, each required wake fixture must declare `expectWake`. Fixture manifests are decoded as strict JSON: duplicate required IDs, unknown fields, and trailing JSON are rejected before any provider report can use the fixture set.

Run a local command-style pmdroid/microWakeWord wrapper over the same fixture manifest with
`-wake-command`. The command path must be absolute; bare command names resolved through `PATH` are
rejected for closure evidence. The benchmark command writes each validated fixture utterance to a temporary WAV,
replaces `{inputPath}`, `{fixtureId}`, `{modelId}`, and `{language}` in `-wake-command-args`, removes the temporary file after the
provider returns, and emits a normal `BenchmarkReport`:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -fixture-manifest wake-fixtures.json \
  -fixture-dir . \
  -wake-command /absolute/path/to/microwakeword-wrapper \
  -wake-command-args-json '["detect", "--model", "{modelId}", "--language", "{language}", "--fixture-id", "{fixtureId}", "--input", "{inputPath}", "--json"]' \
  -provider-id pmdroid-microwakeword \
  -model-id okay-nabu \
  -model-hash sha256:replace-with-model-hash \
  -acceptance-preset > microwakeword-report.json
```

Prefer `-wake-command-args-json` for real evidence so model paths and provider flags are passed as an
exact argv array without shell interpolation or whitespace splitting. The simpler `-wake-command-args`
flag remains available for quick wrappers; do not set both flags in the same run. The command
provider may print `true`/`false` style plain text (`detected`, `not-detected`, `wake`, `no-wake`) or
a JSON object with `detected` or `wake`, plus optional `providerId`, `modelId`, `confidence`,
`detectedAtMs`, `latencyMs`, or `activationLatency`. JSON stdout is decoded strictly: unknown fields,
trailing JSON, raw fixture paths, provider debug payloads, or undeclared internals fail the provider
run and are recorded only as provider failure evidence. The generated report can be validated with
the same JUT-11 preset and compared against the openWakeWord/Wyoming baseline with
`-baseline-report`.

Generate a provider-unavailable fixture report before wiring the real provider, or when documenting recovery after the candidate provider cannot start:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -fixture-manifest wake-fixtures.json \
  -fixture-dir . \
  -fixture-failure-report \
  -acceptance-preset \
  -provider-id pmdroid-microwakeword \
  -model-id okay-nabu \
  -model-hash sha256:replace-with-model-hash > microwakeword-unavailable.json
```

The failure report records each fixture as `provider_unavailable`, keeps raw audio out of the artifact, and can be validated with the same report preset used for real provider runs. With `-acceptance-preset`, this command is expected to exit non-zero because the generated report still has unresolved gaps, provider failures, and unmatched wake expectations. Replace the placeholder model hash before sharing the artifact; placeholder or malformed hashes are reported as acceptance validation problems.

Minimum fixture set:

- one positive wake-word WAV fixture at 16 kHz mono PCM;
- one near-miss speech fixture without the wake phrase;
- one ambient/noise fixture;
- one longer conversational fixture to estimate false accepts per hour.

Measurements:

- model ID, model checksum, and config thresholds;
- cold provider startup time;
- warm detection latency from audio start to activation;
- peak CPU and memory while processing;
- detected timestamp or no-detection result;
- false accept/false reject count across fixtures;
- recovery behavior after reset/cancel.

The benchmark must not require a live microphone. Live microphone behavior can be a separate manual test after fixture parity is proven.

## Closure Gate For JUT-11

Do not close the Linear spike on manifest validation or benchmark harness tests alone. The following
evidence is required:

- macOS and Linux build/run notes from an actual pmdroid/microWakeWord build or a concrete failed
  build, including install prefix, cgo flags, runtime library path, and whether global `/usr/local`
  installation was avoided;
- Raspberry Pi or ARM64 notes from a representative target when feasible, or a concrete reason it
  remains untested and therefore unsupported;
- model compatibility proof for at least one ESPHome model asset and one OHF-trained or
  OHF-compatible model asset, including model IDs, file hashes, and config/runtime assumptions;
- a pmdroid/microWakeWord benchmark report over the required wake fixture set
  (`positive-wake`, `near-miss`, `ambient-room`, `conversation-long`) with a non-placeholder
  `sha256:<64 hex>` model hash and matching expected wake/no-wake outcomes;
- a distinct Wyoming/openWakeWord baseline report over the same fixture IDs;
- `go run ./apps/hub/cmd/jute-voice-benchmark -report microwakeword-report.json
-baseline-report openwakeword-report.json -acceptance-preset` exits zero and produces the
  comparison Markdown used as Linear evidence;
- no `github.com/pmdroid/microwakeword`, TensorFlow, or TensorFlow Lite dependency appears in
  production `go.mod`/`go.sum` unless the final decision explicitly changes from `defer` to
  `adopt as optional provider`.

Compose the generated artifact files into one closure bundle before posting final Linear evidence:

```sh
go run ./apps/hub/cmd/jute-voice-benchmark \
  -closure-bundle-compose JUT-11 \
  -decision-status defer \
  -decision-rationale "Measured build, packaging, model, candidate benchmark, and baseline evidence support deferring pmdroid/microWakeWord for v1." \
  -provider-manifest microwakeword-provider.json \
  -fixture-manifest-artifact wake-fixtures.json \
  -build-evidence-artifacts microwakeword-build.json \
  -packaging-evidence-artifact microwakeword-packaging.json \
  -model-evidence-artifacts esphome-model.json,ohf-model.json \
  -benchmark-report-artifact microwakeword-report.json \
  -baseline-report-artifact openwakeword-report.json > microwakeword-closure-bundle.json

go run ./apps/hub/cmd/jute-voice-benchmark \
  -closure-bundle microwakeword-closure-bundle.json
```

The composer validates the assembled bundle before writing it, so missing artifacts, malformed rows,
placeholder hashes, incomplete packaging, same-provider comparisons, or non-matching benchmark/model
evidence fail before the bundle can be pasted into Linear. Use `-closure-bundle-template JUT-11`
only as a schema reference; the generated template is a skeleton, not passing evidence.

Provider-unavailable reports, synthetic tone fixtures, copied candidate/baseline reports, same-provider
comparisons, missing model hashes, or placeholder hashes are useful guardrail evidence, but they
intentionally fail JUT-11 acceptance and must not be used to close the ticket.

## Security And Runtime Rules

- Do not run training or dependency downloads at Jute runtime.
- Do not install native libraries into global system paths during normal setup.
- Keep model files local to the provider pack.
- Keep raw audio out of logs.
- Emit only wake activation metadata to the hub/display.
- Never call A2A agents from the wake-word provider.

## Follow-Up Work

- Create a disposable native build script that installs under a temp/provider-pack prefix.
- Add a fixture benchmark harness once wake fixture assets exist.
- Test one ESPHome model and one OHF-trained model against the pmdroid runtime.
- Revisit the decision only if native packaging and benchmark results are strong enough to justify a maintained optional provider.
