# Browser Voice Feasibility Spike

## Status

Prototype added. The route now emits both a single-run report and an acceptance matrix JSON artifact.

## Upstream Snapshot

Checked on 2026-06-17:

- MDN documents Web Speech as split between `SpeechSynthesis` and `SpeechRecognition`, with recognition able to use platform services by default and on-device recognition guarded by browser support and Permissions Policy.
- MDN marks `SpeechRecognition` as limited availability because it does not work in some widely used browsers, so browser STT remains target-browser evidence rather than a portable provider assumption.
- MDN documents `getUserMedia` as the microphone capture path and requires a secure context; localhost or HTTPS is therefore part of every valid run.
- MDN documents `SpeechSynthesis` as the browser TTS controller, but available voices are device/browser dependent and must be measured in each target shell.
- The attached `xenova/whisper-web` sample remains useful evidence that Transformers.js Whisper can run in-browser, but browser voice remains a display-local experiment until a measured Jute run proves the useful pieces in at least one browser.

These findings reinforce the current recommendation: browser voice is an experimental display-local fallback only, not a provider-pack replacement for the hub-owned voice runtime.

## Closure Gate For JUT-6

Do not move JUT-6 to Done until a copied `BrowserVoiceRunMatrix` artifact validates with `acceptance.complete: true` and the Linear evidence summary is attached. Saved run reports, matrix rows, matrices, and closure bundles must carry real RFC3339 `generatedAt` values with `Z` or a numeric timezone offset; placeholders do not count as evidence. Matrix timestamps must not be earlier than any row run timestamp, saved run and row timestamps must not be earlier than their cited hub receipt `submittedAt`, and closure-bundle timestamps must not be earlier than their matrix timestamp.

One complete browser run is enough for the spike. The run must contain real microphone permission timing, browser STT cold-start or explicit unsupported/unavailable evidence, speechSynthesis cold-start evidence, model download size, captured final transcript evidence, and proof that the final transcript was submitted through `/api/v1/voice/transcripts/final`. Placeholder values do not count. Bare `unsupported` or `unavailable` values only count for browser STT capability evidence; speechSynthesis still needs a measured cold-start or a named browser/runtime limitation.

Additional rows for desktop Chromium, Safari, kiosk/PWA, offline mode, or unknown browsers remain useful comparison data, but they are not required to close JUT-6.

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
- `BrowserVoiceRunMatrix`: one or more reports summarized by detected browser target.

The Acceptance Matrix panel also produces a copyable Markdown evidence summary for Linear comments. The summary is derived from the same `BrowserVoiceRunMatrix`, recalculates acceptance, lists optional missing targets/problems, and summarizes each target row without requiring the full JSON artifact inline.

The Acceptance Matrix panel also accepts pasted saved run reports. Paste one `BrowserVoiceReport`, an array of reports, or `{ "reports": [...] }` from previous browser/device runs, then copy the combined matrix. Invalid pasted JSON is shown inline and is not included in the matrix. Saved run reports and `{ "reports": [...] }` bundles are schema-strict: undeclared fields such as raw audio, provider debug data, internal URLs, or credential notes are rejected instead of ignored.

The same panel accepts a copied `BrowserVoiceRunMatrix` artifact for later validation. The page parses the saved matrix and recalculates the `acceptance` block from the rows and gaps instead of trusting the pasted `acceptance` value, so stale or edited artifacts cannot falsely mark JUT-6 complete. Saved matrix artifacts are also schema-strict and reject undeclared row, acceptance-block, or top-level fields. Revalidation checks that each row target matches the browser/display/online evidence that would have produced it.

The panel also produces a `BrowserVoiceClosureBundle` artifact. The bundle contains the copied matrix
and the exact Markdown evidence summary that should be pasted into Linear. Pasting a saved closure
bundle back into the panel revalidates the matrix and confirms the Markdown still matches the copied
matrix, so stale summaries cannot be used as closure evidence. Closure bundles are schema-strict and
reject undeclared raw audio, provider debug, internal URL, or credential fields. Matrix rows preserve
sanitized measurement evidence for microphone timing, browser STT, speechSynthesis, hardware,
model download size, CPU/memory, and offline behavior, so the copied Linear summary carries rough
values when they are available instead of only `true`/`false` coverage flags. Saved matrices re-check
required `true` measurement flags with matching compact evidence text, the expected measurement
label, and a non-placeholder value. Rows that claim final hub routing must include a valid
`hubTranscriptReceipt` timestamp and follow-up counter evidence copied from the hub response; the
turn count cannot exceed the max turn count, and an active follow-up window must expire after the
submission timestamp. Bundle, matrix, saved run, and row `generatedAt` values must parse as RFC3339
timestamps with `Z` or a numeric timezone offset before the artifact can be accepted; bundles must
be generated at or after their matrix, matrices must be generated at or after their row runs, and
saved run or row artifacts must be generated at or after any cited hub receipt submission.

The Manual Evidence panel records values that the browser cannot reliably measure itself:

- browser STT cold-start timing or an unsupported/offline note;
- WASM/model download size when a future local model path is added; the current spike route records
  `0 MB` automatically because it does not load a WASM or Transformers model;
- optional CPU and memory notes from the browser task manager or OS monitor when browser-native hardware,
  CPU, JS heap, or device-memory hints are absent or too coarse;
- optional device hardware notes such as device model, CPU class, RAM, or kiosk hardware;
- optional offline behavior from a disconnected-network run.

The matrix keeps the recommendation fixed to `experimental_display_local_only`, records whether final transcripts were captured, records whether final transcripts use the hub path, and lists required measurement gaps. It also includes an `acceptance` block with `complete` and `problems` fields. A matrix is not acceptance-complete while required measurement fields are missing, any row has unresolved required gaps, any final transcript was not captured, or any final transcript path bypasses the hub. Placeholder values such as `unknown`, `not reported`, `not measured`, `not tested`, `not provided`, or `untested` do not count as evidence for microphone permission, browser STT cold start, speechSynthesis cold start, or model download size. Bare `unsupported` or `unavailable` values do not count for speechSynthesis. Use real timings, measured values, or explicit browser STT unsupported/unavailable findings before sharing acceptance evidence.
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
- rough JS heap when exposed by the browser;
- browser-native hardware/CPU hints when exposed: platform, logical cores, touch points, and
  `navigator.deviceMemory`;
- manual CPU or memory notes when browser-native values are absent or too coarse;
- browser STT availability, cold-start time, and whether it works offline;
- browser TTS voice count and cold-start time;
- any model download size for WASM or Transformers.js experiments, with `0 MB` for the current
  no-model route;
- optional CPU and memory notes from the browser task manager or OS monitor when browser hints are
  insufficient;
- optional device hardware notes for the measured target.
- optional offline behavior in a disconnected-network or browser-offline run.

When a manual run is complete, copy the run report JSON into Linear or paste it into the Acceptance Matrix panel during the next target run. For a batch of runs, copy the closure bundle after the page combines the pasted saved reports with the current run, then paste the bundle's Markdown evidence summary into the Linear comment. Paste a copied matrix or closure bundle back into the saved evidence fields to re-check acceptance after sharing or storing the artifact. A matrix is complete when it has at least one row and every included row has no required gaps for microphone permission, browser STT cold start, browser TTS cold start, model download size, captured final transcript evidence, hub transcript routing, target consistency, and RFC3339 generated timestamps. A closure bundle is complete only when the matrix is complete, the bundle has an RFC3339 `generatedAt`, and the attached Markdown evidence summary exactly matches the matrix. If browser STT is unsupported, record that as the field value instead of a placeholder; for speechSynthesis, record the measured cold-start or named unavailable browser/runtime condition rather than a bare `unsupported` or `unavailable` value. If the transcript text changes after a successful hub submission, send it again before copying the report so the routed evidence still matches the current transcript.

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
