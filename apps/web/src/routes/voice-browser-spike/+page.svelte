<script lang="ts">
  import {
    Activity,
    Mic,
    MicOff,
    Play,
    Radio,
    Send,
    Square,
    Clipboard,
    Volume2
  } from 'lucide-svelte';
  import { onMount } from 'svelte';
  import Badge from '$lib/components/ui/Badge.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import {
    browserVoiceSnapshot,
    browserVoiceClosureBundle,
    browserVoiceClosureBundleEvidenceMarkdown,
    browserVoiceClosureBundleJSON,
    browserVoiceReport,
    browserVoiceReportJSON,
    browserVoiceRunMatrix,
    browserVoiceRunMatrixEvidenceMarkdown,
    browserVoiceRunMatrixJSON,
    formatBytes,
    parseBrowserVoiceClosureBundleJSON,
    parseBrowserVoiceRunMatrixJSON,
    parseBrowserVoiceReportsJSON,
    speechRecognitionConstructor,
    type BrowserVoiceHubTranscriptReceipt,
    type BrowserVoiceMeasurement,
    type BrowserVoiceSnapshot,
    type SpeechRecognitionLike
  } from '$lib/browserVoiceSpike';
  import { submitVoiceFinalTranscript } from '$lib/hubClient';

  let snapshot: BrowserVoiceSnapshot | undefined;
  let stream: MediaStream | undefined;
  let audioContext: AudioContext | undefined;
  let analyser: AnalyserNode | undefined;
  let recognition: SpeechRecognitionLike | undefined;
  let hubTranscriptReceipt: BrowserVoiceHubTranscriptReceipt | undefined;
  let recognizing = false;
  let sending = false;
  let transcript = '';
  let hubSubmittedTranscript = '';
  let interimTranscript = '';
  let status = 'Ready';
  let measurements: BrowserVoiceMeasurement[] = [];
  let matrixNotes = '';
  let savedRunsJSON = '';
  let savedMatrixJSON = '';
  let savedClosureBundleJSON = '';
  let reportMeasurements: BrowserVoiceMeasurement[];
  let reportJSON: string;
  let matrixJSON: string;
  let matrixEvidenceMarkdown: string;
  let closureBundleJSON: string;
  let closureBundleEvidenceMarkdown: string;
  let savedRunProblems: string[];
  let savedMatrixProblems: string[];
  let savedClosureBundleProblems: string[];
  let hardwareNotes = '';
  let browserSTTColdStart = '';
  let modelDownloadSize = '';
  let cpuMemoryNotes = '';
  let offlineBehavior = '';

  $: canUseMic = Boolean(
    snapshot?.capabilities.find((item) => item.id === 'microphone')?.available
  );
  $: canUseSpeech = Boolean(
    snapshot?.capabilities.find((item) => item.id === 'speech-recognition')
      ?.available
  );
  $: canUseTTS = Boolean(
    snapshot?.capabilities.find((item) => item.id === 'speech-synthesis')
      ?.available
  );
  $: reportMeasurements = [...measurements, ...manualEvidenceMeasurements()];
  $: currentReport = snapshot
    ? browserVoiceReport({
        snapshot,
        measurements: reportMeasurements,
        platform: navigator.platform,
        standalone: window.matchMedia('(display-mode: standalone)').matches,
        transcriptCaptured: Boolean(transcript.trim()),
        submittedThroughHub:
          Boolean(transcript.trim()) &&
          transcript.trim() === hubSubmittedTranscript,
        hubReceipt:
          transcript.trim() === hubSubmittedTranscript
            ? hubTranscriptReceipt
            : undefined
      })
    : undefined;
  $: savedRunParseResult = parseBrowserVoiceReportsJSON(savedRunsJSON);
  $: savedRunProblems = savedRunParseResult.problems;
  $: savedMatrixParseResult = parseBrowserVoiceRunMatrixJSON(savedMatrixJSON);
  $: savedMatrixProblems = savedMatrixParseResult.problems;
  $: savedClosureBundleParseResult = parseBrowserVoiceClosureBundleJSON(
    savedClosureBundleJSON
  );
  $: savedClosureBundleProblems = savedClosureBundleParseResult.problems;
  $: reportJSON = currentReport ? browserVoiceReportJSON(currentReport) : '';
  $: currentMatrix = currentReport
    ? browserVoiceRunMatrix([...savedRunParseResult.reports, currentReport])
    : undefined;
  $: matrixJSON = currentMatrix ? browserVoiceRunMatrixJSON(currentMatrix) : '';
  $: matrixEvidenceMarkdown = currentMatrix
    ? browserVoiceRunMatrixEvidenceMarkdown(currentMatrix)
    : '';
  $: currentClosureBundle = currentMatrix
    ? browserVoiceClosureBundle(currentMatrix)
    : undefined;
  $: closureBundleJSON = currentClosureBundle
    ? browserVoiceClosureBundleJSON(currentClosureBundle)
    : '';
  $: closureBundleEvidenceMarkdown = currentClosureBundle
    ? browserVoiceClosureBundleEvidenceMarkdown(currentClosureBundle)
    : '';
  $: matrixAcceptance = currentMatrix?.acceptance;
  $: importedMatrixAcceptance = savedMatrixParseResult.matrix?.acceptance;

  onMount(() => {
    refreshSnapshot();
  });

  function refreshSnapshot() {
    snapshot = browserVoiceSnapshot(window);
    const memory = (
      performance as Performance & {
        memory?: { usedJSHeapSize?: number; jsHeapSizeLimit?: number };
      }
    ).memory;
    measurements = [
      {
        label: 'Browser',
        value: navigator.userAgent
      },
      {
        label: 'Secure context',
        value: String(window.isSecureContext),
        detail: 'Microphone APIs require HTTPS or localhost.'
      },
      {
        label: 'Network state',
        value: navigator.onLine ? 'online' : 'offline'
      },
      {
        label: 'JS heap',
        value: formatBytes(memory?.usedJSHeapSize),
        detail: memory?.jsHeapSizeLimit
          ? `limit ${formatBytes(memory.jsHeapSizeLimit)}`
          : 'browser does not expose memory metrics'
      }
    ];
    if (!window.speechSynthesis) {
      measurements = [
        ...measurements,
        {
          label: 'TTS cold start',
          value: 'speechSynthesis unavailable in this browser context',
          detail:
            'browser runtime limitation recorded because preview cannot start'
        }
      ];
    }
    if (!speechRecognitionConstructor(window)) {
      measurements = [
        ...measurements,
        {
          label: 'Browser STT cold start',
          value: 'unavailable',
          detail: 'SpeechRecognition is not exposed in this browser context'
        }
      ];
    }
    matrixNotes = `${navigator.platform || 'unknown platform'} | ${
      window.matchMedia('(display-mode: standalone)').matches
        ? 'standalone/PWA'
        : 'browser tab'
    } | ${navigator.onLine ? 'online' : 'offline'}`;
  }

  async function requestMicrophone() {
    if (!navigator.mediaDevices?.getUserMedia) {
      status = 'Microphone capture is unavailable in this browser context.';
      return;
    }
    const startedAt = performance.now();
    try {
      stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true
        }
      });
      const AudioContextCtor =
        window.AudioContext ??
        (window as Window & { webkitAudioContext?: typeof AudioContext })
          .webkitAudioContext;
      audioContext = new AudioContextCtor();
      const source = audioContext.createMediaStreamSource(stream);
      analyser = audioContext.createAnalyser();
      analyser.fftSize = 2048;
      source.connect(analyser);

      measurements = [
        ...measurements.filter(
          (item) =>
            ![
              'Microphone permission',
              'Sample rate',
              'Audio latency',
              'Input tracks',
              'RMS level'
            ].includes(item.label)
        ),
        {
          label: 'Microphone permission',
          value: `${Math.round(performance.now() - startedAt)} ms`,
          detail: 'elapsed time from user gesture to usable MediaStream'
        },
        {
          label: 'Sample rate',
          value: `${audioContext.sampleRate} Hz`
        },
        {
          label: 'Audio latency',
          value:
            audioContext.baseLatency > 0
              ? `${Math.round(audioContext.baseLatency * 1000)} ms`
              : 'not reported'
        },
        {
          label: 'Input tracks',
          value: String(stream.getAudioTracks().length)
        },
        {
          label: 'RMS level',
          value: measureRMS()
        }
      ];
      status = 'Microphone stream active.';
    } catch (error) {
      status =
        error instanceof Error
          ? `Microphone request failed: ${error.message}`
          : 'Microphone request failed.';
    }
  }

  function measureRMS() {
    if (!analyser) {
      return 'not measured';
    }
    const data = new Uint8Array(analyser.fftSize);
    analyser.getByteTimeDomainData(data);
    let sum = 0;
    for (const sample of data) {
      const centered = (sample - 128) / 128;
      sum += centered * centered;
    }
    return Math.sqrt(sum / data.length).toFixed(4);
  }

  function stopMicrophone() {
    stream?.getTracks().forEach((track) => track.stop());
    stream = undefined;
    analyser = undefined;
    void audioContext?.close();
    audioContext = undefined;
    status = 'Microphone stream stopped.';
  }

  function startRecognition() {
    const Recognition = speechRecognitionConstructor(window);
    if (!Recognition) {
      status = 'SpeechRecognition is unavailable in this browser.';
      return;
    }
    recognition?.abort?.();
    recognition = new Recognition();
    recognition.lang = 'en-GB';
    recognition.interimResults = true;
    recognition.continuous = false;
    recognition.onresult = (event) => {
      let interim = '';
      let finalText = '';
      for (let index = 0; index < event.results.length; index++) {
        const result = event.results[index];
        if (result.isFinal) {
          finalText += result[0].transcript;
        } else {
          interim += result[0].transcript;
        }
      }
      interimTranscript = interim.trim();
      if (finalText.trim()) {
        transcript = finalText.trim();
      }
    };
    recognition.onerror = (event) => {
      status = `Speech recognition failed: ${event.error ?? 'unknown error'}`;
    };
    recognition.onend = () => {
      recognizing = false;
      status = 'Speech recognition stopped.';
    };
    recognizing = true;
    measurements = [
      ...measurements.filter((item) => item.label !== 'Browser STT cold start'),
      {
        label: 'Browser STT cold start',
        value: 'started',
        detail:
          'elapsed final transcript timing should be recorded manually after speech result'
      }
    ];
    recognition.start();
    status = 'Speech recognition listening.';
  }

  function stopRecognition() {
    recognition?.stop();
    recognizing = false;
  }

  function speakPreview() {
    if (!window.speechSynthesis) {
      status = 'speechSynthesis is unavailable.';
      return;
    }
    const startedAt = performance.now();
    const voices = window.speechSynthesis.getVoices();
    const utterance = new SpeechSynthesisUtterance(
      transcript || 'Jute browser voice preview'
    );
    utterance.lang = 'en-GB';
    utterance.onstart = () => {
      measurements = [
        ...measurements.filter(
          (item) => !['TTS cold start', 'TTS voices'].includes(item.label)
        ),
        {
          label: 'TTS cold start',
          value: `${Math.round(performance.now() - startedAt)} ms`
        },
        {
          label: 'TTS voices',
          value: String(voices.length)
        }
      ];
    };
    window.speechSynthesis.cancel();
    window.speechSynthesis.speak(utterance);
    status = 'Playing browser-local speech preview.';
  }

  function manualEvidenceMeasurements(): BrowserVoiceMeasurement[] {
    const items: BrowserVoiceMeasurement[] = [];
    if (hardwareNotes.trim()) {
      items.push({
        label: 'Hardware',
        value: hardwareNotes.trim(),
        detail: 'manual device model, CPU class, memory, or kiosk hardware note'
      });
    }
    if (browserSTTColdStart.trim()) {
      items.push({
        label: 'Browser STT cold start',
        value: browserSTTColdStart.trim(),
        detail: 'manual timing or unsupported/offline note'
      });
    }
    if (modelDownloadSize.trim()) {
      items.push({
        label: 'Model download size',
        value: modelDownloadSize.trim(),
        detail: '0 MB when no WASM or model asset was downloaded'
      });
    }
    if (cpuMemoryNotes.trim()) {
      items.push({
        label: 'CPU',
        value: cpuMemoryNotes.trim(),
        detail: 'manual task manager or OS monitor note'
      });
    }
    if (offlineBehavior.trim()) {
      items.push({
        label: 'Offline behavior',
        value: offlineBehavior.trim(),
        detail: 'manual offline or disconnected-network result'
      });
    }
    return items;
  }

  async function sendFinalTranscript() {
    const text = transcript.trim();
    if (!text) {
      status = 'Enter or capture a final transcript first.';
      return;
    }
    sending = true;
    try {
      const response = await submitVoiceFinalTranscript(fetch, {
        text,
        deviceProfileId: 'browser-spike',
        deviceId: 'browser-spike-display'
      });
      hubSubmittedTranscript = text;
      hubTranscriptReceipt = {
        submittedAt: new Date().toISOString(),
        followupActive: response.followup.active,
        followupTurns: response.followup.turns,
        followupMaxTurns: response.followup.maxTurns,
        ...(response.followup.expiresAt
          ? { followupExpiresAt: response.followup.expiresAt }
          : {})
      };
      status = 'Final transcript sent to the hub voice API.';
    } catch (error) {
      status =
        error instanceof Error
          ? `Hub transcript post failed: ${error.message}`
          : 'Hub transcript post failed.';
    } finally {
      sending = false;
    }
  }

  async function copyReport() {
    if (!reportJSON) {
      status = 'Run a snapshot before copying the report.';
      return;
    }
    try {
      await navigator.clipboard.writeText(reportJSON);
      status = 'Browser voice report copied.';
    } catch {
      status = 'Clipboard unavailable; select and copy the report JSON.';
    }
  }

  async function copyMatrix() {
    if (!matrixJSON) {
      status = 'Run a snapshot before copying the matrix.';
      return;
    }
    try {
      await navigator.clipboard.writeText(matrixJSON);
      status = 'Browser voice matrix copied.';
    } catch {
      status = 'Clipboard unavailable; select and copy the matrix JSON.';
    }
  }

  async function copyMatrixEvidence() {
    if (!matrixEvidenceMarkdown) {
      status = 'Run a snapshot before copying the evidence summary.';
      return;
    }
    try {
      await navigator.clipboard.writeText(matrixEvidenceMarkdown);
      status = 'Browser voice evidence summary copied.';
    } catch {
      status = 'Clipboard unavailable; select and copy the evidence summary.';
    }
  }

  async function copyClosureBundle() {
    if (!closureBundleJSON) {
      status = 'Run a snapshot before copying the closure bundle.';
      return;
    }
    try {
      await navigator.clipboard.writeText(closureBundleJSON);
      status = 'Browser voice closure bundle copied.';
    } catch {
      status =
        'Clipboard unavailable; select and copy the closure bundle JSON.';
    }
  }
</script>

<svelte:head>
  <title>Voice Browser Spike | Jute Dash</title>
</svelte:head>

<main class="browser-spike">
  <section class="toolbar">
    <div>
      <p class="eyebrow">JUT-6 spike</p>
      <h1>Browser Voice Feasibility</h1>
    </div>
    <Badge tone={snapshot?.secureContext ? 'active' : 'warning'}>
      {snapshot?.secureContext ? 'secure context' : 'limited context'}
    </Badge>
  </section>

  <section class="status-band">
    <Activity size={18} />
    <span>{status}</span>
  </section>

  <section class="grid">
    <div class="panel">
      <h2>Capability Matrix</h2>
      <p class="matrix">{matrixNotes}</p>
      <div class="capabilities">
        {#each snapshot?.capabilities ?? [] as item (item.id)}
          <div class="capability">
            <div>
              <strong>{item.label}</strong>
              <span>{item.detail}</span>
            </div>
            <Badge tone={item.available ? 'active' : 'warning'}>
              {item.available ? 'available' : 'missing'}
            </Badge>
          </div>
        {/each}
      </div>
      <Button variant="outline" on:click={refreshSnapshot}>
        <Radio size={16} /> Refresh
      </Button>
    </div>

    <div class="panel">
      <h2>Measurement Run</h2>
      <div class="actions">
        <Button
          on:click={requestMicrophone}
          disabled={!canUseMic || Boolean(stream)}
        >
          <Mic size={16} /> Mic
        </Button>
        <Button variant="outline" on:click={stopMicrophone} disabled={!stream}>
          <MicOff size={16} /> Stop
        </Button>
        <Button
          variant="outline"
          on:click={startRecognition}
          disabled={!canUseSpeech || recognizing}
        >
          <Play size={16} /> STT
        </Button>
        <Button
          variant="outline"
          on:click={stopRecognition}
          disabled={!recognizing}
        >
          <Square size={16} /> Stop
        </Button>
        <Button variant="outline" on:click={speakPreview} disabled={!canUseTTS}>
          <Volume2 size={16} /> TTS
        </Button>
      </div>

      <div class="measurements">
        {#each measurements as item (item.label)}
          <div>
            <span>{item.label}</span>
            <strong>{item.value}</strong>
            {#if item.detail}<small>{item.detail}</small>{/if}
          </div>
        {/each}
      </div>
    </div>

    <div class="panel transcript-panel">
      <h2>Final Transcript Path</h2>
      {#if interimTranscript}
        <p class="interim">{interimTranscript}</p>
      {/if}
      <textarea
        bind:value={transcript}
        rows="7"
        placeholder="Capture with Web Speech or type a fixture transcript"
      ></textarea>
      <Button on:click={sendFinalTranscript} disabled={sending}>
        <Send size={16} />
        {sending ? 'Sending' : 'Send to hub'}
      </Button>
    </div>

    <div class="panel evidence-panel">
      <h2>Manual Evidence</h2>
      <label>
        <span>Device hardware</span>
        <textarea
          bind:value={hardwareNotes}
          rows="3"
          placeholder="Device model, CPU class, RAM, kiosk hardware"
        ></textarea>
      </label>
      <label>
        <span>Browser STT cold start</span>
        <input
          bind:value={browserSTTColdStart}
          placeholder="e.g. 850 ms or unavailable"
        />
      </label>
      <label>
        <span>Model download size</span>
        <input bind:value={modelDownloadSize} placeholder="e.g. 0 MB, 42 MB" />
      </label>
      <label>
        <span>CPU and memory</span>
        <textarea
          bind:value={cpuMemoryNotes}
          rows="3"
          placeholder="Task manager or OS monitor note"
        ></textarea>
      </label>
      <label>
        <span>Offline behavior</span>
        <textarea
          bind:value={offlineBehavior}
          rows="3"
          placeholder="Result with network disconnected or browser offline"
        ></textarea>
      </label>
    </div>

    <div class="panel report-panel">
      <h2>Run Report</h2>
      <Button variant="outline" on:click={copyReport} disabled={!reportJSON}>
        <Clipboard size={16} /> Copy
      </Button>
      <pre>{reportJSON}</pre>
    </div>

    <div class="panel report-panel">
      <h2>Acceptance Matrix</h2>
      <div class="actions">
        <Button variant="outline" on:click={copyMatrix} disabled={!matrixJSON}>
          <Clipboard size={16} /> Copy JSON
        </Button>
        <Button
          variant="outline"
          on:click={copyMatrixEvidence}
          disabled={!matrixEvidenceMarkdown}
        >
          <Clipboard size={16} /> Copy Evidence
        </Button>
        <Button
          variant="outline"
          on:click={copyClosureBundle}
          disabled={!closureBundleJSON}
        >
          <Clipboard size={16} /> Copy Closure
        </Button>
      </div>
      <label class="saved-runs">
        <span>Saved run reports</span>
        <textarea
          bind:value={savedRunsJSON}
          rows="8"
          placeholder="Paste one BrowserVoiceReport JSON object, an array, or a reports object"
        ></textarea>
      </label>
      {#if savedRunProblems.length}
        <div class="problems" role="alert">
          {#each savedRunProblems as problem (problem)}
            <p>{problem}</p>
          {/each}
        </div>
      {/if}
      <label class="saved-runs">
        <span>Saved acceptance matrix</span>
        <textarea
          bind:value={savedMatrixJSON}
          rows="8"
          placeholder="Paste a copied BrowserVoiceRunMatrix JSON artifact"
        ></textarea>
      </label>
      {#if savedMatrixProblems.length}
        <div class="problems" role="alert">
          {#each savedMatrixProblems as problem (problem)}
            <p>{problem}</p>
          {/each}
        </div>
      {/if}
      {#if importedMatrixAcceptance}
        <div
          class:complete={importedMatrixAcceptance.complete}
          class="acceptance"
          role="status"
        >
          <strong>
            {importedMatrixAcceptance.complete
              ? 'Saved matrix complete'
              : 'Saved matrix gaps'}
          </strong>
          {#if importedMatrixAcceptance.problems.length}
            {#each importedMatrixAcceptance.problems as problem (problem)}
              <p>{problem}</p>
            {/each}
          {/if}
        </div>
      {/if}
      <label class="saved-runs">
        <span>Saved closure bundle</span>
        <textarea
          bind:value={savedClosureBundleJSON}
          rows="8"
          placeholder="Paste a copied BrowserVoiceClosureBundle JSON artifact"
        ></textarea>
      </label>
      {#if savedClosureBundleProblems.length}
        <div class="problems" role="alert">
          {#each savedClosureBundleProblems as problem (problem)}
            <p>{problem}</p>
          {/each}
        </div>
      {/if}
      {#if savedClosureBundleParseResult.bundle}
        <div
          class:complete={!savedClosureBundleProblems.length}
          class="acceptance"
          role="status"
        >
          <strong>
            {!savedClosureBundleProblems.length
              ? 'Saved closure bundle complete'
              : 'Saved closure bundle gaps'}
          </strong>
          {#if !savedClosureBundleProblems.length}
            <p>Evidence summary matches the copied matrix.</p>
          {/if}
        </div>
      {/if}
      {#if matrixAcceptance}
        <div
          class:complete={matrixAcceptance.complete}
          class="acceptance"
          role="status"
        >
          <strong>
            {matrixAcceptance.complete
              ? 'Acceptance complete'
              : 'Acceptance gaps'}
          </strong>
          {#if matrixAcceptance.problems.length}
            {#each matrixAcceptance.problems as problem (problem)}
              <p>{problem}</p>
            {/each}
          {/if}
        </div>
      {/if}
      <pre>{closureBundleEvidenceMarkdown}</pre>
      <pre>{matrixEvidenceMarkdown}</pre>
      <pre>{closureBundleJSON}</pre>
      <pre>{matrixJSON}</pre>
    </div>
  </section>
</main>

<style>
  :global(body) {
    margin: 0;
    background: var(--background, #f7f7f5);
    color: var(--foreground, #151514);
  }

  .browser-spike {
    min-height: 100vh;
    padding: 24px;
  }

  .toolbar,
  .status-band,
  .grid,
  .actions,
  .capability,
  .measurements div {
    display: flex;
  }

  .toolbar {
    align-items: flex-end;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 18px;
  }

  .eyebrow {
    margin: 0 0 4px;
    color: var(--muted-foreground, #676762);
    font-size: 0.78rem;
    font-weight: 760;
    text-transform: uppercase;
  }

  h1,
  h2,
  p {
    margin: 0;
  }

  h1 {
    font-size: clamp(1.8rem, 4vw, 3.4rem);
    line-height: 1;
  }

  h2 {
    font-size: 1rem;
  }

  .status-band {
    align-items: center;
    gap: 10px;
    min-height: 44px;
    border-top: 1px solid var(--border, #d9d8d0);
    border-bottom: 1px solid var(--border, #d9d8d0);
    color: var(--muted-foreground, #676762);
  }

  .grid {
    align-items: stretch;
    gap: 16px;
    margin-top: 18px;
  }

  .panel {
    flex: 1 1 0;
    min-width: 0;
    border: 1px solid var(--border, #d9d8d0);
    border-radius: 8px;
    background: var(--surface, #ffffff);
    padding: 16px;
  }

  .matrix {
    margin-top: 6px;
    color: var(--muted-foreground, #676762);
    font-size: 0.9rem;
  }

  .capabilities,
  .measurements {
    display: grid;
    gap: 10px;
    margin: 14px 0;
  }

  .capability,
  .measurements div {
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    min-height: 48px;
    border-top: 1px solid var(--border, #d9d8d0);
    padding-top: 10px;
  }

  .capability div,
  .measurements div {
    min-width: 0;
  }

  .capability span,
  .measurements span,
  .measurements small,
  .interim {
    color: var(--muted-foreground, #676762);
    font-size: 0.86rem;
  }

  .capability span,
  .measurements small {
    display: block;
    margin-top: 3px;
  }

  .actions {
    flex-wrap: wrap;
    gap: 8px;
    margin-top: 14px;
  }

  .transcript-panel,
  .evidence-panel,
  .report-panel {
    display: grid;
    gap: 12px;
  }

  .evidence-panel label {
    display: grid;
    gap: 6px;
  }

  .evidence-panel span {
    color: var(--muted-foreground, #676762);
    font-size: 0.82rem;
    font-weight: 700;
  }

  .report-panel pre {
    max-height: 360px;
    overflow: auto;
    border: 1px solid var(--border, #d9d8d0);
    border-radius: 8px;
    background: color-mix(in srgb, var(--surface, #ffffff) 82%, #000 4%);
    color: inherit;
    font-size: 0.78rem;
    line-height: 1.45;
    margin: 0;
    padding: 12px;
    white-space: pre-wrap;
    word-break: break-word;
  }

  input,
  textarea {
    width: 100%;
    border: 1px solid var(--border, #d9d8d0);
    border-radius: 8px;
    background: var(--surface, #ffffff);
    color: inherit;
    font: inherit;
    padding: 12px;
  }

  textarea {
    resize: vertical;
  }

  .saved-runs {
    display: grid;
    gap: 8px;
    margin-top: 10px;
  }

  .saved-runs span {
    color: var(--muted-foreground, #676762);
    font-size: 0.82rem;
    font-weight: 700;
  }

  .problems {
    display: grid;
    gap: 6px;
    margin-top: 10px;
    border: 1px solid
      color-mix(in srgb, var(--destructive, #b42318) 35%, transparent);
    border-radius: 8px;
    color: var(--destructive, #b42318);
    font-size: 0.85rem;
    padding: 10px;
  }

  .acceptance {
    display: grid;
    gap: 6px;
    border: 1px solid var(--warning, #b7791f);
    border-radius: 8px;
    color: var(--warning, #b7791f);
    font-size: 0.85rem;
    padding: 10px;
  }

  .acceptance.complete {
    border-color: var(--success, #177245);
    color: var(--success, #177245);
  }

  .acceptance p {
    margin: 0;
  }

  textarea:focus-visible {
    outline: 2px solid var(--focus, #1b6f6a);
    outline-offset: 2px;
  }

  @media (max-width: 900px) {
    .browser-spike {
      padding: 16px;
    }

    .toolbar,
    .grid {
      flex-direction: column;
    }
  }
</style>
