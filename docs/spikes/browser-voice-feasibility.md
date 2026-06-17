# Browser Voice Feasibility Spike

## Status

Prototype added. The route now emits both a single-run report and an acceptance matrix JSON artifact. Device/browser benchmark values are still pending manual runs.

## Upstream Snapshot

Checked on 2026-06-17:

- MDN documents Web Speech as split between `SpeechSynthesis` and `SpeechRecognition`, with recognition able to use platform services by default and on-device recognition guarded by browser support and Permissions Policy.
- MDN marks `SpeechRecognition` as limited availability because it does not work in some widely used browsers, so browser STT remains target-browser evidence rather than a portable provider assumption.
- MDN documents `getUserMedia` as the microphone capture path and requires a secure context; localhost or HTTPS is therefore part of every valid run.
- MDN documents `SpeechSynthesis` as the browser TTS controller, but available voices are device/browser dependent and must be measured in each target shell.
- The attached `xenova/whisper-web` sample remains useful evidence that Transformers.js Whisper can run in-browser, but any Jute recommendation still needs local cold-start, model-size, CPU/memory, and offline measurements from the target display hardware.

These findings reinforce the current recommendation: browser voice is an experimental display-local fallback only, not a provider-pack replacement for the hub-owned voice runtime.

## Closure Gate For JUT-6

Do not move JUT-6 to Done until a copied `BrowserVoiceRunMatrix` artifact validates with `acceptance.complete: true` and the Linear evidence summary is attached. Saved run reports, matrix rows, matrices, and closure bundles must carry real RFC3339 `generatedAt` values with `Z` or a numeric timezone offset; placeholders do not count as evidence. Matrix timestamps must not be earlier than any row run timestamp, saved run and row timestamps must not be earlier than their cited hub receipt `submittedAt`, and closure-bundle timestamps must not be earlier than their matrix timestamp. The matrix must include measured rows for:

- `desktop-chromium`
- `desktop-safari`
- `kiosk-pwa`
- `offline-display`

Each required target must have exactly one acceptance row. Duplicate rows for the same required target are rejected because they make the copied evidence summary ambiguous. Each row must contain real microphone permission timing, browser STT cold-start or explicit unsupported/unavailable evidence, speechSynthesis cold-start evidence, model download size, CPU/memory notes, device hardware notes, concrete offline behavior, captured final transcript evidence, and proof that the final transcript was submitted through `/api/v1/voice/transcripts/final`. Placeholder values do not count. Bare `unsupported` or `unavailable` values only count for browser STT capability evidence; speechSynthesis and offline behavior rows need a measured cold-start, a named browser/runtime limitation, or a concrete disconnected-network result. `unknown-browser` rows are useful exploratory runs, but they cannot satisfy a required JUT-6 target.

## Decision

Browser-side voice should remain an experimental display-local fallback for v1. It is useful for manual push-to-talk experiments and browser `speechSynthesis` preview, but it is not a canonical provider-pack runtime and must not replace the local Jute Voice Service for wake word, VAD, STT, TTS, follow-up windows, or headless satellites.

## Prototype

The throwaway Svelte prototype lives at:

```text
apps/web/src/routes/voice-browser-spike/+page.svelte
```

The reusable report helpers live at:

```text
apps/web/src/lib/browserVoiceSpike.ts
```

It measures:

- secure-context and online/offline state;
- microphone permission and `getUserMedia` setup latency;
- Web Audio sample rate, input track count, base latency, and RMS level;
- `AudioWorkletNode` availability for future VAD experiments;
- browser `speechSynthesis` availability, voice count, and preview start latency;
- Web Speech recognition availability and captured final transcript text.

Final transcript submission goes through `POST /api/v1/voice/transcripts/final` via the web hub client. The prototype does not call A2A agents directly. A run report marks `finalTranscriptPath.submittedThroughHub` as `true` only after the current transcript is accepted by that hub API and the report carries a validated `hubReceipt`; typing or capturing transcript text without a successful hub submission remains an explicit evidence gap. Successful submissions record a safe `hubReceipt` with the submission timestamp and follow-up counters from the hub response, without transcript text, agent messages, raw URLs, or credentials. Helper-created or hand-edited reports that set `submittedThroughHub: true` without a real receipt are downgraded back to unresolved hub-routing evidence.

The route produces two copyable JSON artifacts:

- `BrowserVoiceReport`: one measured browser/device run.
- `BrowserVoiceRunMatrix`: one or more reports summarized against the acceptance targets `desktop-chromium`, `desktop-safari`, `kiosk-pwa`, and `offline-display`.

The Acceptance Matrix panel also produces a copyable Markdown evidence summary for Linear comments. The summary is derived from the same `BrowserVoiceRunMatrix`, recalculates acceptance, lists missing targets/problems, and summarizes each target row without requiring the full JSON artifact inline.

The Acceptance Matrix panel also accepts pasted saved run reports. Paste one `BrowserVoiceReport`, an array of reports, or `{ "reports": [...] }` from previous browser/device runs, then copy the combined matrix. Invalid pasted JSON is shown inline and is not included in the matrix. Saved run reports and `{ "reports": [...] }` bundles are schema-strict: undeclared fields such as raw audio, provider debug data, internal URLs, or credential notes are rejected instead of ignored.

The same panel accepts a copied `BrowserVoiceRunMatrix` artifact for later validation. The page parses the saved matrix and recalculates the `acceptance` block from the rows and gaps instead of trusting the pasted `acceptance` value, so stale or edited artifacts cannot falsely mark JUT-6 complete. Saved matrix artifacts are also schema-strict and reject undeclared row, acceptance-block, or top-level fields. Revalidation checks target semantics too: desktop rows must be online browser-tab runs, kiosk/PWA rows must be standalone PWA runs, offline-display rows must be recorded while offline, each required target may appear only once, and each row target must match the browser/display/online evidence that would have produced it.

The panel also produces a `BrowserVoiceClosureBundle` artifact. The bundle contains the copied matrix
and the exact Markdown evidence summary that should be pasted into Linear. Pasting a saved closure
bundle back into the panel revalidates the matrix and confirms the Markdown still matches the copied
matrix, so stale summaries cannot be used as closure evidence. Closure bundles are schema-strict and
reject undeclared raw audio, provider debug, internal URL, or credential fields. Matrix rows preserve
sanitized measurement evidence for microphone timing, browser STT, speechSynthesis, hardware,
model download size, CPU/memory, and offline behavior, so the copied Linear summary carries the
rough values JUT-6 asks for instead of only `true`/`false` coverage flags. Saved matrices re-check
that every `true` measurement flag has matching compact evidence text with the expected measurement
label and a non-placeholder value. Rows that claim final hub routing must include a valid
`hubTranscriptReceipt` timestamp and follow-up counter evidence copied from the hub response; the
turn count cannot exceed the max turn count, and an active follow-up window must expire after the
submission timestamp. Bundle, matrix, saved run, and row `generatedAt` values must parse as RFC3339
timestamps with `Z` or a numeric timezone offset before the artifact can be accepted; bundles must
be generated at or after their matrix, matrices must be generated at or after their row runs, and
saved run or row artifacts must be generated at or after any cited hub receipt submission.

The Manual Evidence panel records values that the browser cannot reliably measure itself:

- browser STT cold-start timing or an unsupported/offline note;
- WASM/model download size, including `0 MB` when no local model asset is downloaded;
- CPU and memory notes from the browser task manager or OS monitor;
- device hardware notes such as device model, CPU class, RAM, or kiosk hardware;
- offline behavior from a disconnected-network run.

The matrix keeps the recommendation fixed to `experimental_display_local_only`, records whether final transcripts were captured, records whether final transcripts use the hub path, and lists missing targets or measurement fields as explicit gaps. It also includes an `acceptance` block with `complete` and `problems` fields. A matrix is not acceptance-complete while any target or manual evidence field is missing, any row has unresolved gaps, any final transcript was not captured, or any final transcript path bypasses the hub. Placeholder values such as `unknown`, `not reported`, `not measured`, `not tested`, `not provided`, or `untested` do not count as evidence for microphone permission, browser STT cold start, speechSynthesis cold start, model download size, CPU/memory, device hardware, or offline behavior. Bare `unsupported` or `unavailable` values also do not count for speechSynthesis or offline behavior. Use real timings, measured values, explicit browser STT unsupported/unavailable findings, concrete hardware notes, or a concrete disconnected-network result before sharing acceptance evidence.
Credential-shaped values such as `token=...`, `password=...`, or `api-key=...` are also rejected as
measurement evidence; remove secrets and record the measured result instead.

## Initial Browser Matrix

| Target                           | Expected result                                                                                               | Notes                                                                                                                                                  |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Desktop Chrome/Edge on localhost | Microphone, Web Audio, `speechSynthesis`, and prefixed Web Speech recognition are expected to be available.   | Web Speech recognition may use browser/platform services and should be treated as online/implementation-dependent unless on-device hooks are detected. |
| Desktop Safari on localhost      | Microphone, Web Audio, and `speechSynthesis` are expected to be useful; recognition support must be measured. | Safari behavior differs across macOS/iOS and PWA mode.                                                                                                 |
| Kiosk/PWA display                | Microphone and `speechSynthesis` must be measured in the actual kiosk shell.                                  | Display mode, permission persistence, and autoplay policies are the important unknowns.                                                                |
| Offline display                  | Microphone/Web Audio should remain measurable.                                                                | Browser STT must be considered unavailable unless the run proves local model availability.                                                             |

## Measurement Fields

Record these values for each run:

- browser, version, OS, display mode, and manual device model/CPU/RAM/kiosk hardware note;
- secure context and online/offline state;
- microphone permission latency;
- sample rate, base latency, and input tracks;
- rough JS heap when exposed by the browser, plus a manual CPU or memory note when the browser reports `unknown`;
- browser STT availability, cold-start time, and whether it works offline;
- browser TTS voice count and cold-start time;
- any model download size for WASM or Transformers.js experiments;
- CPU and memory notes from the browser task manager or OS monitor;
- device hardware notes for the measured target.
- offline behavior in a disconnected-network or browser-offline run.

When a manual run is complete, copy the run report JSON into Linear or paste it into the Acceptance Matrix panel during the next target run. For a batch of runs, copy the closure bundle after the page combines the pasted saved reports with the current run, then paste the bundle's Markdown evidence summary into the Linear comment. Paste a copied matrix or closure bundle back into the saved evidence fields to re-check acceptance after sharing or storing the artifact. A matrix is complete only when it has no missing targets and no gaps for microphone permission, browser STT cold start, browser TTS cold start, model download size, CPU/memory notes, device hardware notes, offline behavior, captured final transcript evidence, hub transcript routing, target semantics, and RFC3339 generated timestamps. A closure bundle is complete only when the matrix is complete, the bundle has an RFC3339 `generatedAt`, and the attached Markdown evidence summary exactly matches the matrix. If browser STT is unsupported, record that as the field value instead of a placeholder; for speechSynthesis and offline behavior, record the measured cold-start, named unavailable browser/runtime condition, or concrete disconnected-network behavior rather than a bare `unsupported` or `unavailable` value. If the transcript text changes after a successful hub submission, send it again before copying the report so the routed evidence still matches the current transcript.

## Recommendation

Keep browser voice as experimental only:

- Push-to-talk capture can be useful for display-only demos.
- Browser `speechSynthesis` can preview local display TTS behavior, but canonical TTS remains the hub/provider path.
- Browser STT should not be advertised as reliable or offline until benchmark runs prove support in the target browser and kiosk shell.
- Browser wake-word detection should be rejected for v1 background listening. It conflicts with headless satellite requirements and depends on browser permission/runtime behavior.

## Sources

- [MDN Web Speech API](https://developer.mozilla.org/en-US/docs/Web/API/Web_Speech_API)
- [MDN SpeechRecognition](https://developer.mozilla.org/en-US/docs/Web/API/SpeechRecognition)
- [MDN getUserMedia](https://developer.mozilla.org/en-US/docs/Web/API/MediaDevices/getUserMedia)
- [MDN AudioWorklet](https://developer.mozilla.org/en-US/docs/Web/API/AudioWorklet)
- [MDN SpeechSynthesis](https://developer.mozilla.org/en-US/docs/Web/API/SpeechSynthesis)
- [xenova/whisper-web](https://github.com/xenova/whisper-web)
