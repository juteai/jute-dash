export type BrowserVoiceCapability = {
  id: string;
  label: string;
  available: boolean;
  detail: string;
};

export type BrowserVoiceSnapshot = {
  userAgent: string;
  secureContext: boolean;
  online: boolean;
  capabilities: BrowserVoiceCapability[];
};

export type BrowserVoiceMeasurement = {
  label: string;
  value: string;
  detail?: string;
};

export type BrowserVoiceReport = {
  generatedAt: string;
  issue: 'JUT-6';
  recommendation: 'experimental_display_local_only';
  browser: {
    userAgent: string;
    platform: string;
    displayMode: 'browser-tab' | 'standalone-pwa';
    secureContext: boolean;
    online: boolean;
  };
  capabilities: BrowserVoiceCapability[];
  measurements: BrowserVoiceMeasurement[];
  finalTranscriptPath: {
    hubEndpoint: string;
    submittedThroughHub: boolean;
    transcriptCaptured: boolean;
    hubReceipt?: BrowserVoiceHubTranscriptReceipt;
  };
  gaps: string[];
};

export type BrowserVoiceHubTranscriptReceipt = {
  submittedAt: string;
  followupActive: boolean;
  followupTurns: number;
  followupMaxTurns: number;
  followupExpiresAt?: string;
};

export type BrowserVoiceMatrixTarget =
  | 'desktop-chromium'
  | 'desktop-safari'
  | 'kiosk-pwa'
  | 'offline-display'
  | 'unknown-browser';

export type BrowserVoiceMatrixRow = {
  target: BrowserVoiceMatrixTarget;
  generatedAt: string;
  browser: string;
  platform: string;
  displayMode: BrowserVoiceReport['browser']['displayMode'];
  online: boolean;
  microphoneMeasured: boolean;
  browserSTTMeasured: boolean;
  ttsMeasured: boolean;
  hardwareMeasured: boolean;
  modelDownloadMeasured: boolean;
  cpuMemoryMeasured: boolean;
  offlineBehaviorMeasured: boolean;
  finalTranscriptThroughHub: boolean;
  finalTranscriptCaptured: boolean;
  hubTranscriptReceipt?: BrowserVoiceHubTranscriptReceipt;
  recommendation: BrowserVoiceReport['recommendation'];
  evidence: BrowserVoiceMatrixRowEvidence;
  gaps: string[];
};

export type BrowserVoiceMatrixRowEvidence = {
  microphone: string;
  browserSTT: string;
  tts: string;
  hardware: string;
  modelDownload: string;
  cpuMemory: string;
  offlineBehavior: string;
};

export type BrowserVoiceRunMatrix = {
  issue: 'JUT-6';
  recommendation: BrowserVoiceReport['recommendation'];
  generatedAt: string;
  targetsCovered: BrowserVoiceMatrixTarget[];
  missingTargets: BrowserVoiceMatrixTarget[];
  rows: BrowserVoiceMatrixRow[];
  gaps: string[];
  acceptance: BrowserVoiceAcceptance;
};

export type BrowserVoiceClosureBundle = {
  issue: 'JUT-6';
  generatedAt: string;
  matrix: BrowserVoiceRunMatrix;
  evidenceMarkdown: string;
};

export type BrowserVoiceReportParseResult = {
  reports: BrowserVoiceReport[];
  problems: string[];
};

export type BrowserVoiceMatrixParseResult = {
  matrix: BrowserVoiceRunMatrix | undefined;
  problems: string[];
};

export type BrowserVoiceClosureBundleParseResult = {
  bundle: BrowserVoiceClosureBundle | undefined;
  problems: string[];
};

export type BrowserVoiceAcceptance = {
  complete: boolean;
  problems: string[];
};

type SpeechRecognitionConstructor = {
  new (): SpeechRecognitionLike;
  available?: () => Promise<unknown>;
  install?: () => Promise<unknown>;
};

export type SpeechRecognitionLike = {
  lang: string;
  interimResults: boolean;
  continuous: boolean;
  start: () => void;
  stop: () => void;
  abort?: () => void;
  onresult:
    | ((event: {
        results: ArrayLike<{
          isFinal: boolean;
          0: { transcript: string };
        }>;
      }) => void)
    | null;
  onerror: ((event: { error?: string }) => void) | null;
  onend: (() => void) | null;
};

export function browserVoiceSnapshot(win: Window): BrowserVoiceSnapshot {
  const nav = win.navigator;
  const recognition = speechRecognitionConstructor(win);
  const audioGlobals = windowWithBrowserAudio(win);
  const hasMediaDevices = Boolean(nav.mediaDevices?.getUserMedia);
  const hasAudioContext = Boolean(
    audioGlobals.AudioContext || audioGlobals.webkitAudioContext
  );
  const hasSpeechSynthesis = Boolean(win.speechSynthesis);
  const hasOnDeviceSpeech = recognition
    ? 'available' in recognition || 'install' in recognition
    : false;

  return {
    userAgent: nav.userAgent,
    secureContext: win.isSecureContext,
    online: nav.onLine,
    capabilities: [
      {
        id: 'microphone',
        label: 'Microphone capture',
        available: hasMediaDevices,
        detail: hasMediaDevices
          ? 'getUserMedia is present; permission still requires a user gesture.'
          : 'getUserMedia is not exposed in this browser context.'
      },
      {
        id: 'web-audio',
        label: 'Web Audio buffering',
        available: hasAudioContext,
        detail: hasAudioContext
          ? 'AudioContext can measure sample rate, analyser levels, and buffering latency.'
          : 'AudioContext is unavailable.'
      },
      {
        id: 'audio-worklet',
        label: 'AudioWorklet VAD path',
        available: Boolean(audioGlobals.AudioWorkletNode),
        detail: audioGlobals.AudioWorkletNode
          ? 'AudioWorkletNode is present for future low-latency VAD experiments.'
          : 'AudioWorkletNode is unavailable; fallback would need ScriptProcessor or hub-side VAD.'
      },
      {
        id: 'speech-synthesis',
        label: 'speechSynthesis preview',
        available: hasSpeechSynthesis,
        detail: hasSpeechSynthesis
          ? 'Browser TTS preview is available for display-local experiments.'
          : 'speechSynthesis is unavailable.'
      },
      {
        id: 'speech-recognition',
        label: 'Web Speech recognition',
        available: Boolean(recognition),
        detail: recognition
          ? 'SpeechRecognition is present; final transcripts must still be posted to the hub.'
          : 'SpeechRecognition is unavailable.'
      },
      {
        id: 'on-device-speech',
        label: 'On-device recognition hooks',
        available: hasOnDeviceSpeech,
        detail: hasOnDeviceSpeech
          ? 'Experimental local recognition hooks are visible.'
          : 'No browser-managed on-device recognition install/availability hooks detected.'
      }
    ]
  };
}

export function speechRecognitionConstructor(
  win: Window
): SpeechRecognitionConstructor | undefined {
  const typed = win as Window &
    typeof globalThis & {
      webkitSpeechRecognition?: SpeechRecognitionConstructor;
      SpeechRecognition?: SpeechRecognitionConstructor;
    };
  return typed.SpeechRecognition ?? typed.webkitSpeechRecognition;
}

function windowWithBrowserAudio(win: Window) {
  return win as Window &
    typeof globalThis & {
      AudioContext?: typeof AudioContext;
      AudioWorkletNode?: typeof AudioWorkletNode;
      webkitAudioContext?: typeof AudioContext;
    };
}

export function formatBytes(value: number | undefined): string {
  if (!value || value <= 0) {
    return 'unknown';
  }
  if (value < 1024 * 1024) {
    return `${Math.round(value / 1024)} KB`;
  }
  return `${Math.round(value / (1024 * 1024))} MB`;
}

export function browserVoiceReport(params: {
  snapshot: BrowserVoiceSnapshot;
  measurements: BrowserVoiceMeasurement[];
  platform: string;
  standalone: boolean;
  generatedAt?: string;
  transcriptCaptured?: boolean;
  submittedThroughHub?: boolean;
  hubReceipt?: BrowserVoiceHubTranscriptReceipt;
}): BrowserVoiceReport {
  const generatedAt = params.generatedAt ?? new Date().toISOString();
  const hubReceipt =
    params.submittedThroughHub && params.hubReceipt
      ? params.hubReceipt
      : undefined;
  return {
    generatedAt,
    issue: 'JUT-6',
    recommendation: 'experimental_display_local_only',
    browser: {
      userAgent: params.snapshot.userAgent,
      platform: params.platform || 'unknown platform',
      displayMode: params.standalone ? 'standalone-pwa' : 'browser-tab',
      secureContext: params.snapshot.secureContext,
      online: params.snapshot.online
    },
    capabilities: params.snapshot.capabilities,
    measurements: params.measurements.map((measurement) => ({
      ...measurement
    })),
    finalTranscriptPath: {
      hubEndpoint: '/api/v1/voice/transcripts/final',
      submittedThroughHub: Boolean(hubReceipt),
      transcriptCaptured: Boolean(params.transcriptCaptured),
      ...(hubReceipt ? { hubReceipt } : {})
    },
    gaps: browserVoiceReportGaps(params.snapshot, params.measurements)
  };
}

export function browserVoiceReportJSON(report: BrowserVoiceReport): string {
  return JSON.stringify(report, null, 2);
}

export function browserVoiceRunMatrix(
  reports: BrowserVoiceReport[],
  generatedAt = new Date().toISOString()
): BrowserVoiceRunMatrix {
  const rows = reports.map(browserVoiceMatrixRow);
  const targetsCovered = uniqueTargets(rows.map((row) => row.target));
  const missingTargets = requiredBrowserVoiceTargets.filter(
    (target) => !targetsCovered.includes(target)
  );
  const gaps = [
    ...missingTargets.map((target) => `${target} run not recorded`),
    ...uniqueStrings(rows.flatMap((row) => row.gaps))
  ];
  const matrix = {
    issue: 'JUT-6' as const,
    recommendation: 'experimental_display_local_only' as const,
    generatedAt,
    targetsCovered,
    missingTargets,
    rows,
    gaps
  };

  return {
    ...matrix,
    acceptance: validateBrowserVoiceRunMatrix(matrix)
  };
}

export function browserVoiceRunMatrixJSON(
  matrix: BrowserVoiceRunMatrix
): string {
  return JSON.stringify(matrix, null, 2);
}

export function browserVoiceClosureBundle(
  matrix: BrowserVoiceRunMatrix,
  generatedAt = new Date().toISOString()
): BrowserVoiceClosureBundle {
  const canonicalMatrix = revalidatedBrowserVoiceRunMatrix(matrix);
  return {
    issue: 'JUT-6',
    generatedAt,
    matrix: canonicalMatrix,
    evidenceMarkdown: browserVoiceRunMatrixEvidenceMarkdown(canonicalMatrix)
  };
}

export function browserVoiceClosureBundleJSON(
  bundle: BrowserVoiceClosureBundle
): string {
  return JSON.stringify(bundle, null, 2);
}

export function parseBrowserVoiceClosureBundleJSON(
  raw: string
): BrowserVoiceClosureBundleParseResult {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { bundle: undefined, problems: [] };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return {
      bundle: undefined,
      problems: [safeBrowserVoiceParseError('closure bundle JSON')]
    };
  }

  if (!isBrowserVoiceClosureBundleCandidate(parsed)) {
    return {
      bundle: undefined,
      problems: ['saved closure bundle is not a JUT-6 browser voice bundle']
    };
  }

  const matrix = revalidatedBrowserVoiceRunMatrix(parsed.matrix);
  const expectedEvidence = browserVoiceRunMatrixEvidenceMarkdown(matrix);
  const problems = [...matrix.acceptance.problems];
  if (!isRFC3339BrowserVoiceTimestamp(parsed.generatedAt)) {
    problems.push('closure bundle generatedAt must be RFC3339');
  } else if (
    isRFC3339BrowserVoiceTimestamp(matrix.generatedAt) &&
    browserVoiceTimestampMillis(parsed.generatedAt) <
      browserVoiceTimestampMillis(matrix.generatedAt)
  ) {
    problems.push(
      'closure bundle generatedAt must not be before matrix generatedAt'
    );
  }
  if (parsed.evidenceMarkdown !== expectedEvidence) {
    problems.push('closure bundle evidenceMarkdown does not match matrix');
  }

  return {
    bundle: {
      issue: 'JUT-6',
      generatedAt: parsed.generatedAt,
      matrix,
      evidenceMarkdown: parsed.evidenceMarkdown
    },
    problems: uniqueStrings(problems)
  };
}

export function browserVoiceClosureBundleEvidenceMarkdown(
  bundle: BrowserVoiceClosureBundle
): string {
  const parsed = parseBrowserVoiceClosureBundleJSON(
    browserVoiceClosureBundleJSON(bundle)
  );
  const problems = parsed.problems;
  const matrix = parsed.bundle?.matrix ?? bundle.matrix;
  const lines = [
    '### Browser Voice Closure Bundle: JUT-6',
    '',
    `- Generated at: \`${bundle.generatedAt}\``,
    `- Matrix generated at: \`${matrix.generatedAt}\``,
    `- Evidence summary attached: \`${bundle.evidenceMarkdown.trim() ? 'true' : 'false'}\``,
    `- Acceptance complete: \`${problems.length === 0}\``
  ];
  if (problems.length > 0) {
    lines.push('', 'Validation problems:');
    for (const problem of problems) {
      lines.push(`- ${problem}`);
    }
  }
  return lines.join('\n').trim();
}

export function browserVoiceRunMatrixEvidenceMarkdown(
  matrix: BrowserVoiceRunMatrix
): string {
  const rowTargetsCovered = uniqueTargets(matrix.rows.map((row) => row.target));
  const rowMissingTargets = requiredBrowserVoiceTargets.filter(
    (target) => !rowTargetsCovered.includes(target)
  );
  const acceptance = validateBrowserVoiceRunMatrix({
    issue: matrix.issue,
    recommendation: matrix.recommendation,
    generatedAt: matrix.generatedAt,
    targetsCovered: matrix.targetsCovered,
    missingTargets: matrix.missingTargets,
    rows: matrix.rows,
    gaps: matrix.gaps
  });
  const lines = [
    '### Browser Voice Evidence: JUT-6',
    '',
    `- Recommendation: \`${matrix.recommendation}\``,
    `- Generated at: \`${matrix.generatedAt}\``,
    `- Targets covered: ${rowTargetsCovered.length ? rowTargetsCovered.map((target) => `\`${target}\``).join(', ') : 'none'}`,
    `- Missing targets: ${rowMissingTargets.length ? rowMissingTargets.map((target) => `\`${target}\``).join(', ') : 'none'}`,
    `- Acceptance: ${acceptance.complete ? 'complete' : 'gaps remain'}`
  ];
  if (acceptance.problems.length > 0) {
    lines.push('', 'Acceptance problems:');
    for (const problem of acceptance.problems) {
      lines.push(`- ${problem}`);
    }
  }
  if (matrix.rows.length > 0) {
    lines.push('', 'Browser run rows:');
    for (const row of matrix.rows) {
      lines.push(
        `- \`${row.target}\`: generatedAt=\`${row.generatedAt}\`, display=\`${row.displayMode}\`, online=${row.online}, mic=${row.microphoneMeasured}, stt=${row.browserSTTMeasured}, tts=${row.ttsMeasured}, hardware=${row.hardwareMeasured}, model=${row.modelDownloadMeasured}, cpuMemory=${row.cpuMemoryMeasured}, offline=${row.offlineBehaviorMeasured}, hubFinal=${row.finalTranscriptThroughHub}, transcript=${row.finalTranscriptCaptured}`
      );
      if (row.hubTranscriptReceipt) {
        lines.push(
          `  - hub receipt: submittedAt=\`${row.hubTranscriptReceipt.submittedAt}\`, followupActive=${row.hubTranscriptReceipt.followupActive}, turns=${row.hubTranscriptReceipt.followupTurns}/${row.hubTranscriptReceipt.followupMaxTurns}`
        );
      }
      lines.push(
        `  - evidence: mic=\`${row.evidence.microphone || 'missing'}\`, stt=\`${row.evidence.browserSTT || 'missing'}\`, tts=\`${row.evidence.tts || 'missing'}\`, hardware=\`${row.evidence.hardware || 'missing'}\`, model=\`${row.evidence.modelDownload || 'missing'}\`, cpuMemory=\`${row.evidence.cpuMemory || 'missing'}\`, offline=\`${row.evidence.offlineBehavior || 'missing'}\``
      );
    }
  }
  return lines.join('\n').trim();
}

export function parseBrowserVoiceReportsJSON(
  raw: string
): BrowserVoiceReportParseResult {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { reports: [], problems: [] };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return {
      reports: [],
      problems: [safeBrowserVoiceParseError('saved runs JSON')]
    };
  }

  const candidates = browserVoiceReportCandidates(parsed);
  const reports: BrowserVoiceReport[] = [];
  const problems: string[] = [];
  candidates.forEach((candidate, index) => {
    if (isBrowserVoiceReport(candidate)) {
      reports.push(candidate);
      return;
    }
    problems.push(`saved run ${index + 1} is not a JUT-6 browser voice report`);
  });

  return { reports, problems };
}

export function parseBrowserVoiceRunMatrixJSON(
  raw: string
): BrowserVoiceMatrixParseResult {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { matrix: undefined, problems: [] };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return {
      matrix: undefined,
      problems: [safeBrowserVoiceParseError('saved matrix JSON')]
    };
  }

  if (!isBrowserVoiceRunMatrixCandidate(parsed)) {
    return {
      matrix: undefined,
      problems: ['saved matrix is not a JUT-6 browser voice matrix']
    };
  }

  const matrix = {
    issue: parsed.issue,
    recommendation: parsed.recommendation,
    generatedAt: parsed.generatedAt,
    targetsCovered: parsed.targetsCovered,
    missingTargets: parsed.missingTargets,
    rows: parsed.rows,
    gaps: parsed.gaps
  };

  return {
    matrix: {
      ...matrix,
      acceptance: validateBrowserVoiceRunMatrix(matrix)
    },
    problems: []
  };
}

export function validateBrowserVoiceRunMatrix(
  matrix: Omit<BrowserVoiceRunMatrix, 'acceptance'>
): BrowserVoiceAcceptance {
  const problems: string[] = [];
  const rowTargetsCovered = uniqueTargets(matrix.rows.map((row) => row.target));
  const rowMissingTargets = requiredBrowserVoiceTargets.filter(
    (target) => !rowTargetsCovered.includes(target)
  );
  if (matrix.issue !== 'JUT-6') {
    problems.push('matrix issue must be JUT-6');
  }
  if (matrix.recommendation !== 'experimental_display_local_only') {
    problems.push(
      'matrix recommendation must remain experimental_display_local_only'
    );
  }
  if (!isRFC3339BrowserVoiceTimestamp(matrix.generatedAt)) {
    problems.push('matrix generatedAt must be RFC3339');
  }
  for (const target of requiredBrowserVoiceTargets) {
    if (!rowTargetsCovered.includes(target)) {
      problems.push(`${target} run not recorded`);
    }
  }
  if (!sameTargets(matrix.targetsCovered, rowTargetsCovered)) {
    problems.push('matrix targetsCovered does not match row targets');
  }
  if (!sameTargets(matrix.missingTargets, rowMissingTargets)) {
    problems.push('matrix missingTargets does not match row targets');
  }
  if (matrix.missingTargets.length > 0) {
    problems.push('matrix has missing required targets');
  }
  if (matrix.gaps.length > 0) {
    problems.push('matrix has unresolved evidence gaps');
  }
  if (matrix.rows.length === 0) {
    problems.push('matrix has no browser run rows');
  }
  const rowTargetCounts = matrix.rows.reduce(
    (counts, row) => counts.set(row.target, (counts.get(row.target) ?? 0) + 1),
    new Map<BrowserVoiceMatrixTarget, number>()
  );
  for (const target of requiredBrowserVoiceTargets) {
    if ((rowTargetCounts.get(target) ?? 0) > 1) {
      problems.push(`${target} has duplicate browser run rows`);
    }
  }
  for (const row of matrix.rows) {
    const prefix = `${row.target} row`;
    const evidenceTarget = classifyBrowserVoiceMatrixRowTarget(row);
    if (!isRFC3339BrowserVoiceTimestamp(row.generatedAt)) {
      problems.push(`${prefix} generatedAt must be RFC3339`);
    } else if (
      isRFC3339BrowserVoiceTimestamp(matrix.generatedAt) &&
      browserVoiceTimestampMillis(matrix.generatedAt) <
        browserVoiceTimestampMillis(row.generatedAt)
    ) {
      problems.push(
        `${prefix} generatedAt must not be after matrix generatedAt`
      );
    }
    if (
      row.hubTranscriptReceipt &&
      isRFC3339BrowserVoiceTimestamp(row.generatedAt) &&
      browserVoiceTimestampMillis(row.generatedAt) <
        browserVoiceTimestampMillis(row.hubTranscriptReceipt.submittedAt)
    ) {
      problems.push(
        `${prefix} generatedAt must not be before hub receipt submittedAt`
      );
    }
    if (row.target !== evidenceTarget) {
      problems.push(
        `${prefix} target does not match browser/display evidence (${evidenceTarget})`
      );
    }
    if (row.target === 'unknown-browser') {
      problems.push(`${prefix} cannot satisfy a required JUT-6 target`);
    }
    if (!isConcreteBrowserVoiceIdentity(row.browser)) {
      problems.push(`${prefix} is missing browser identity evidence`);
    }
    if (!isConcreteBrowserVoiceIdentity(row.platform)) {
      problems.push(`${prefix} is missing platform identity evidence`);
    }
    if (row.target === 'offline-display' && row.online) {
      problems.push(`${prefix} must be recorded while offline`);
    }
    if (row.target === 'kiosk-pwa' && row.displayMode !== 'standalone-pwa') {
      problems.push(`${prefix} must use standalone PWA display mode`);
    }
    if (
      (row.target === 'desktop-chromium' || row.target === 'desktop-safari') &&
      row.displayMode !== 'browser-tab'
    ) {
      problems.push(`${prefix} must use browser-tab display mode`);
    }
    if (
      (row.target === 'desktop-chromium' ||
        row.target === 'desktop-safari' ||
        row.target === 'kiosk-pwa') &&
      !row.online
    ) {
      problems.push(`${prefix} must be recorded while online`);
    }
    if (row.recommendation !== 'experimental_display_local_only') {
      problems.push(`${prefix} has unsupported recommendation`);
    }
    if (
      !row.microphoneMeasured ||
      !hasExpectedRowEvidence(row.evidence.microphone, [
        'Microphone permission'
      ])
    ) {
      problems.push(`${prefix} is missing microphone permission evidence`);
    }
    if (
      !row.browserSTTMeasured ||
      !hasExpectedRowEvidence(
        row.evidence.browserSTT,
        ['Browser STT cold start'],
        {
          requireNumericOrGenericUnsupported: true
        }
      )
    ) {
      problems.push(`${prefix} is missing browser STT evidence`);
    }
    if (
      !row.ttsMeasured ||
      !hasExpectedRowEvidence(row.evidence.tts, ['TTS cold start'], {
        disallowGenericUnsupported: true
      })
    ) {
      problems.push(`${prefix} is missing speechSynthesis evidence`);
    }
    if (
      !row.hardwareMeasured ||
      !hasExpectedRowEvidence(row.evidence.hardware, [
        'Hardware',
        'Device hardware'
      ])
    ) {
      problems.push(`${prefix} is missing device hardware evidence`);
    }
    if (
      !row.modelDownloadMeasured ||
      !hasExpectedRowEvidence(row.evidence.modelDownload, [
        'Model download size'
      ])
    ) {
      problems.push(`${prefix} is missing model download evidence`);
    }
    if (
      !row.cpuMemoryMeasured ||
      !hasExpectedRowEvidence(
        row.evidence.cpuMemory,
        ['CPU', 'Memory', 'JS heap'],
        {
          requireNumericMeasurement: true
        }
      )
    ) {
      problems.push(`${prefix} is missing CPU or memory evidence`);
    }
    if (
      !row.offlineBehaviorMeasured ||
      !hasExpectedRowEvidence(
        row.evidence.offlineBehavior,
        ['Offline behavior'],
        {
          disallowGenericUnsupported: true
        }
      )
    ) {
      problems.push(`${prefix} is missing offline behavior evidence`);
    }
    if (!row.finalTranscriptThroughHub) {
      problems.push(`${prefix} does not prove hub transcript routing`);
    }
    if (
      row.finalTranscriptThroughHub &&
      !isBrowserVoiceHubTranscriptReceipt(row.hubTranscriptReceipt)
    ) {
      problems.push(`${prefix} is missing hub transcript receipt evidence`);
    }
    if (!row.finalTranscriptCaptured) {
      problems.push(`${prefix} does not include a captured final transcript`);
    }
    if (row.gaps.length > 0) {
      problems.push(`${prefix} has unresolved gaps`);
    }
  }
  return {
    complete: problems.length === 0,
    problems: uniqueStrings(problems)
  };
}

const requiredBrowserVoiceTargets: BrowserVoiceMatrixTarget[] = [
  'desktop-chromium',
  'desktop-safari',
  'kiosk-pwa',
  'offline-display'
];

function browserVoiceReportCandidates(parsed: unknown): unknown[] {
  if (Array.isArray(parsed)) {
    return parsed;
  }
  if (isRecord(parsed) && Array.isArray(parsed.reports)) {
    if (!hasOnlyKeys(parsed, ['reports'])) {
      return [parsed];
    }
    return parsed.reports;
  }
  return [parsed];
}

function revalidatedBrowserVoiceRunMatrix(
  matrix: Omit<BrowserVoiceRunMatrix, 'acceptance'>
): BrowserVoiceRunMatrix {
  const candidate = {
    issue: matrix.issue,
    recommendation: matrix.recommendation,
    generatedAt: matrix.generatedAt,
    targetsCovered: matrix.targetsCovered,
    missingTargets: matrix.missingTargets,
    rows: matrix.rows,
    gaps: matrix.gaps
  };
  return {
    ...candidate,
    acceptance: validateBrowserVoiceRunMatrix(candidate)
  };
}

function isBrowserVoiceReport(value: unknown): value is BrowserVoiceReport {
  if (!isRecord(value)) {
    return false;
  }
  const browser = value.browser;
  const finalTranscriptPath = value.finalTranscriptPath;
  return (
    hasOnlyKeys(value, [
      'generatedAt',
      'issue',
      'recommendation',
      'browser',
      'capabilities',
      'measurements',
      'finalTranscriptPath',
      'gaps'
    ]) &&
    value.issue === 'JUT-6' &&
    value.recommendation === 'experimental_display_local_only' &&
    isRFC3339BrowserVoiceTimestamp(value.generatedAt) &&
    isRecord(browser) &&
    hasOnlyKeys(browser, [
      'userAgent',
      'platform',
      'displayMode',
      'secureContext',
      'online'
    ]) &&
    typeof browser.userAgent === 'string' &&
    typeof browser.platform === 'string' &&
    (browser.displayMode === 'browser-tab' ||
      browser.displayMode === 'standalone-pwa') &&
    typeof browser.secureContext === 'boolean' &&
    typeof browser.online === 'boolean' &&
    Array.isArray(value.capabilities) &&
    value.capabilities.every(isBrowserVoiceCapability) &&
    Array.isArray(value.measurements) &&
    value.measurements.every(isBrowserVoiceMeasurement) &&
    isRecord(finalTranscriptPath) &&
    hasOnlyKeys(finalTranscriptPath, [
      'hubEndpoint',
      'submittedThroughHub',
      'transcriptCaptured',
      'hubReceipt'
    ]) &&
    finalTranscriptPath.hubEndpoint === '/api/v1/voice/transcripts/final' &&
    typeof finalTranscriptPath.submittedThroughHub === 'boolean' &&
    typeof finalTranscriptPath.transcriptCaptured === 'boolean' &&
    (finalTranscriptPath.submittedThroughHub
      ? isBrowserVoiceHubTranscriptReceipt(finalTranscriptPath.hubReceipt)
      : finalTranscriptPath.hubReceipt === undefined) &&
    browserVoiceReportHubReceiptChronologyIsValid(
      value.generatedAt,
      finalTranscriptPath
    ) &&
    isStringArray(value.gaps)
  );
}

function browserVoiceReportHubReceiptChronologyIsValid(
  generatedAt: string,
  finalTranscriptPath: Record<string, unknown>
): boolean {
  if (!finalTranscriptPath.submittedThroughHub) {
    return true;
  }
  const receipt = finalTranscriptPath.hubReceipt;
  return (
    isBrowserVoiceHubTranscriptReceipt(receipt) &&
    browserVoiceTimestampMillis(generatedAt) >=
      browserVoiceTimestampMillis(receipt.submittedAt)
  );
}

function isBrowserVoiceCapability(
  value: unknown
): value is BrowserVoiceCapability {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, ['id', 'label', 'available', 'detail']) &&
    typeof value.id === 'string' &&
    typeof value.label === 'string' &&
    typeof value.available === 'boolean' &&
    typeof value.detail === 'string'
  );
}

function isBrowserVoiceMeasurement(
  value: unknown
): value is BrowserVoiceMeasurement {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, ['label', 'value', 'detail']) &&
    typeof value.label === 'string' &&
    typeof value.value === 'string' &&
    (value.detail === undefined || typeof value.detail === 'string')
  );
}

function isBrowserVoiceRunMatrixCandidate(
  value: unknown
): value is Omit<BrowserVoiceRunMatrix, 'acceptance'> {
  if (!isRecord(value)) {
    return false;
  }

  return (
    hasOnlyKeys(value, [
      'issue',
      'recommendation',
      'generatedAt',
      'targetsCovered',
      'missingTargets',
      'rows',
      'gaps',
      'acceptance'
    ]) &&
    value.issue === 'JUT-6' &&
    value.recommendation === 'experimental_display_local_only' &&
    typeof value.generatedAt === 'string' &&
    isBrowserVoiceTargetArray(value.targetsCovered) &&
    isBrowserVoiceTargetArray(value.missingTargets) &&
    Array.isArray(value.rows) &&
    value.rows.every(isBrowserVoiceMatrixRow) &&
    isStringArray(value.gaps) &&
    isBrowserVoiceAcceptance(value.acceptance)
  );
}

function isBrowserVoiceClosureBundleCandidate(
  value: unknown
): value is BrowserVoiceClosureBundle {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, [
      'issue',
      'generatedAt',
      'matrix',
      'evidenceMarkdown'
    ]) &&
    value.issue === 'JUT-6' &&
    typeof value.generatedAt === 'string' &&
    isBrowserVoiceRunMatrixCandidate(value.matrix) &&
    typeof value.evidenceMarkdown === 'string'
  );
}

function isBrowserVoiceAcceptance(
  value: unknown
): value is BrowserVoiceAcceptance {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, ['complete', 'problems']) &&
    typeof value.complete === 'boolean' &&
    isStringArray(value.problems)
  );
}

function isBrowserVoiceMatrixRow(
  value: unknown
): value is BrowserVoiceMatrixRow {
  if (!isRecord(value)) {
    return false;
  }

  return (
    hasOnlyKeys(value, [
      'target',
      'generatedAt',
      'browser',
      'platform',
      'displayMode',
      'online',
      'microphoneMeasured',
      'browserSTTMeasured',
      'ttsMeasured',
      'hardwareMeasured',
      'modelDownloadMeasured',
      'cpuMemoryMeasured',
      'offlineBehaviorMeasured',
      'finalTranscriptThroughHub',
      'finalTranscriptCaptured',
      'hubTranscriptReceipt',
      'recommendation',
      'evidence',
      'gaps'
    ]) &&
    isBrowserVoiceMatrixTarget(value.target) &&
    typeof value.generatedAt === 'string' &&
    typeof value.browser === 'string' &&
    typeof value.platform === 'string' &&
    (value.displayMode === 'browser-tab' ||
      value.displayMode === 'standalone-pwa') &&
    typeof value.online === 'boolean' &&
    typeof value.microphoneMeasured === 'boolean' &&
    typeof value.browserSTTMeasured === 'boolean' &&
    typeof value.ttsMeasured === 'boolean' &&
    typeof value.hardwareMeasured === 'boolean' &&
    typeof value.modelDownloadMeasured === 'boolean' &&
    typeof value.cpuMemoryMeasured === 'boolean' &&
    typeof value.offlineBehaviorMeasured === 'boolean' &&
    typeof value.finalTranscriptThroughHub === 'boolean' &&
    typeof value.finalTranscriptCaptured === 'boolean' &&
    (value.hubTranscriptReceipt === undefined ||
      isBrowserVoiceHubTranscriptReceipt(value.hubTranscriptReceipt)) &&
    value.recommendation === 'experimental_display_local_only' &&
    isBrowserVoiceMatrixRowEvidence(value.evidence) &&
    isStringArray(value.gaps)
  );
}

function isBrowserVoiceHubTranscriptReceipt(
  value: unknown
): value is BrowserVoiceHubTranscriptReceipt {
  if (
    !isRecord(value) ||
    !hasOnlyKeys(value, [
      'submittedAt',
      'followupActive',
      'followupTurns',
      'followupMaxTurns',
      'followupExpiresAt'
    ]) ||
    !isRFC3339BrowserVoiceTimestamp(value.submittedAt) ||
    typeof value.followupActive !== 'boolean' ||
    typeof value.followupTurns !== 'number' ||
    !Number.isInteger(value.followupTurns) ||
    value.followupTurns < 0 ||
    typeof value.followupMaxTurns !== 'number' ||
    !Number.isInteger(value.followupMaxTurns) ||
    value.followupMaxTurns <= 0 ||
    value.followupTurns > value.followupMaxTurns
  ) {
    return false;
  }
  if (value.followupActive && value.followupExpiresAt === undefined) {
    return false;
  }
  if (value.followupExpiresAt === undefined) {
    return true;
  }
  if (!isRFC3339BrowserVoiceTimestamp(value.followupExpiresAt)) {
    return false;
  }
  return Date.parse(value.followupExpiresAt) > Date.parse(value.submittedAt);
}

function isBrowserVoiceMatrixRowEvidence(
  value: unknown
): value is BrowserVoiceMatrixRowEvidence {
  return (
    isRecord(value) &&
    hasOnlyKeys(value, [
      'microphone',
      'browserSTT',
      'tts',
      'hardware',
      'modelDownload',
      'cpuMemory',
      'offlineBehavior'
    ]) &&
    typeof value.microphone === 'string' &&
    typeof value.browserSTT === 'string' &&
    typeof value.tts === 'string' &&
    typeof value.hardware === 'string' &&
    typeof value.modelDownload === 'string' &&
    typeof value.cpuMemory === 'string' &&
    typeof value.offlineBehavior === 'string'
  );
}

function isBrowserVoiceTargetArray(
  value: unknown
): value is BrowserVoiceMatrixTarget[] {
  return Array.isArray(value) && value.every(isBrowserVoiceMatrixTarget);
}

function isBrowserVoiceMatrixTarget(
  value: unknown
): value is BrowserVoiceMatrixTarget {
  return (
    value === 'desktop-chromium' ||
    value === 'desktop-safari' ||
    value === 'kiosk-pwa' ||
    value === 'offline-display' ||
    value === 'unknown-browser'
  );
}

function hasExpectedRowEvidence(
  value: string,
  labels: string[],
  options: {
    disallowGenericUnsupported?: boolean;
    requireNumericMeasurement?: boolean;
    requireNumericOrGenericUnsupported?: boolean;
  } = {}
): boolean {
  const chunks = value
    .split(';')
    .map((chunk) => chunk.trim())
    .filter(Boolean);
  return chunks.some((chunk) => {
    const label = labels.find((candidate) =>
      chunk.toLowerCase().startsWith(`${candidate.toLowerCase()}:`)
    );
    if (!label) {
      return false;
    }
    const measured = chunk
      .slice(label.length + 1)
      .trim()
      .toLowerCase();
    if (!measured) {
      return false;
    }
    if (
      [
        'unknown',
        'not reported',
        'not measured',
        'not tested',
        'untested',
        'not provided',
        'started'
      ].includes(measured)
    ) {
      return false;
    }
    if (
      options.disallowGenericUnsupported &&
      ['unsupported', 'unavailable'].includes(measured)
    ) {
      return false;
    }
    if (options.requireNumericMeasurement && !/\d/.test(measured)) {
      return false;
    }
    if (
      options.requireNumericOrGenericUnsupported &&
      !/\d/.test(measured) &&
      !['unsupported', 'unavailable'].includes(measured)
    ) {
      return false;
    }
    return true;
  });
}

function isStringArray(value: unknown): value is string[] {
  return (
    Array.isArray(value) && value.every((item) => typeof item === 'string')
  );
}

function isRFC3339BrowserVoiceTimestamp(value: unknown): value is string {
  if (typeof value !== 'string') {
    return false;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return false;
  }
  const lower = trimmed.toLowerCase();
  if (
    lower.includes('replace-with') ||
    lower.includes('placeholder') ||
    lower.includes('unknown') ||
    lower.includes('not-provided') ||
    lower.includes('todo') ||
    lower.includes('tbd')
  ) {
    return false;
  }
  if (
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(
      trimmed
    )
  ) {
    return false;
  }
  const parsed = Date.parse(trimmed);
  if (Number.isNaN(parsed)) {
    return false;
  }
  return true;
}

function browserVoiceTimestampMillis(value: string): number {
  return Date.parse(value);
}

function safeBrowserVoiceParseError(label: string): string {
  return `${label} could not be parsed`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function hasOnlyKeys(
  value: Record<string, unknown>,
  allowed: string[]
): boolean {
  const allowedKeys = new Set(allowed);
  return Object.keys(value).every((key) => allowedKeys.has(key));
}

function browserVoiceMatrixRow(
  report: BrowserVoiceReport
): BrowserVoiceMatrixRow {
  const measurementsByLabel = new Map(
    report.measurements.map((measurement) => [
      measurement.label,
      measurement.value
    ])
  );
  const target = classifyBrowserVoiceTarget(report);
  const microphoneMeasured = hasSubstantiveMeasurement(
    measurementsByLabel,
    'Microphone permission'
  );
  const browserSTTMeasured = hasSubstantiveMeasurement(
    measurementsByLabel,
    'Browser STT cold start'
  );
  const ttsMeasured = hasSubstantiveMeasurement(
    measurementsByLabel,
    'TTS cold start'
  );
  const hardwareMeasured =
    hasSubstantiveMeasurement(measurementsByLabel, 'Hardware') ||
    hasSubstantiveMeasurement(measurementsByLabel, 'Device hardware');
  const cpuMemoryMeasured =
    hasSubstantiveMeasurement(measurementsByLabel, 'CPU') ||
    hasSubstantiveMeasurement(measurementsByLabel, 'Memory') ||
    hasSubstantiveMeasurement(measurementsByLabel, 'JS heap');
  const modelDownloadMeasured = hasSubstantiveMeasurement(
    measurementsByLabel,
    'Model download size'
  );
  const offlineBehaviorMeasured = hasSubstantiveMeasurement(
    measurementsByLabel,
    'Offline behavior'
  );
  const evidence = {
    microphone: measurementEvidence(measurementsByLabel, [
      'Microphone permission'
    ]),
    browserSTT: measurementEvidence(measurementsByLabel, [
      'Browser STT cold start'
    ]),
    tts: measurementEvidence(measurementsByLabel, ['TTS cold start']),
    hardware: measurementEvidence(measurementsByLabel, [
      'Hardware',
      'Device hardware'
    ]),
    modelDownload: measurementEvidence(measurementsByLabel, [
      'Model download size'
    ]),
    cpuMemory: measurementEvidence(measurementsByLabel, [
      'CPU',
      'Memory',
      'JS heap'
    ]),
    offlineBehavior: measurementEvidence(measurementsByLabel, [
      'Offline behavior'
    ])
  };
  const rowGaps = [
    ...report.gaps,
    ...(!microphoneMeasured ? ['microphone permission not measured'] : []),
    ...(!browserSTTMeasured ? ['browser STT cold start not measured'] : []),
    ...(!ttsMeasured ? ['speechSynthesis cold start not measured'] : []),
    ...(!hardwareMeasured ? ['device hardware note not measured'] : []),
    ...(!modelDownloadMeasured ? ['model download size not measured'] : []),
    ...(!cpuMemoryMeasured ? ['CPU or memory note not measured'] : []),
    ...(!offlineBehaviorMeasured ? ['offline behavior note not measured'] : []),
    ...(!report.finalTranscriptPath.submittedThroughHub
      ? ['final transcript hub path not proven']
      : []),
    ...(!report.finalTranscriptPath.transcriptCaptured
      ? ['final transcript capture not proven']
      : [])
  ];

  return {
    target,
    generatedAt: report.generatedAt,
    browser: report.browser.userAgent,
    platform: report.browser.platform,
    displayMode: report.browser.displayMode,
    online: report.browser.online,
    microphoneMeasured,
    browserSTTMeasured,
    ttsMeasured,
    hardwareMeasured,
    modelDownloadMeasured,
    cpuMemoryMeasured,
    offlineBehaviorMeasured,
    finalTranscriptThroughHub: report.finalTranscriptPath.submittedThroughHub,
    finalTranscriptCaptured: report.finalTranscriptPath.transcriptCaptured,
    ...(report.finalTranscriptPath.hubReceipt
      ? { hubTranscriptReceipt: report.finalTranscriptPath.hubReceipt }
      : {}),
    recommendation: report.recommendation,
    evidence,
    gaps: uniqueStrings(rowGaps)
  };
}

function measurementEvidence(
  measurementsByLabel: Map<string, string>,
  labels: string[]
): string {
  return labels
    .flatMap((label) => {
      const value = measurementsByLabel.get(label);
      if (!value || !hasSubstantiveMeasurement(measurementsByLabel, label)) {
        return [];
      }
      return `${label}: ${safeBrowserVoiceEvidenceValue(value)}`;
    })
    .filter(Boolean)
    .join('; ');
}

function safeBrowserVoiceEvidenceValue(value: string): string {
  const redacted = value
    .replace(/https?:\/\/\S+/gi, '[redacted-url]')
    .replace(/\/[^\s,;]+/g, '[redacted-path]')
    .replace(
      /\b(token|secret|credential|password|api[-_ ]?key)\s*[:=]\s*\S+/gi,
      '$1 redacted'
    );
  let safe = '';
  for (const char of redacted) {
    if (/^[a-zA-Z0-9 .,:;()_+\-[\]]$/.test(char)) {
      safe += char;
    } else {
      safe += ' ';
    }
    if (safe.length >= 160) {
      break;
    }
  }
  return safe.trim().replace(/\s+/g, ' ');
}

function classifyBrowserVoiceTarget(
  report: BrowserVoiceReport
): BrowserVoiceMatrixTarget {
  if (!report.browser.online) {
    return 'offline-display';
  }
  if (report.browser.displayMode === 'standalone-pwa') {
    return 'kiosk-pwa';
  }
  const userAgent = report.browser.userAgent;
  if (/\b(Chrome|Chromium|Edg)\//.test(userAgent)) {
    return 'desktop-chromium';
  }
  if (
    /Safari\//.test(userAgent) &&
    !/\b(Chrome|Chromium|Edg)\//.test(userAgent)
  ) {
    return 'desktop-safari';
  }
  return 'unknown-browser';
}

function classifyBrowserVoiceMatrixRowTarget(
  row: Pick<BrowserVoiceMatrixRow, 'browser' | 'displayMode' | 'online'>
): BrowserVoiceMatrixTarget {
  if (!row.online) {
    return 'offline-display';
  }
  if (row.displayMode === 'standalone-pwa') {
    return 'kiosk-pwa';
  }
  if (/\b(Chrome|Chromium|Edg)\//.test(row.browser)) {
    return 'desktop-chromium';
  }
  if (
    /Safari\//.test(row.browser) &&
    !/\b(Chrome|Chromium|Edg)\//.test(row.browser)
  ) {
    return 'desktop-safari';
  }
  return 'unknown-browser';
}

function browserVoiceReportGaps(
  snapshot: BrowserVoiceSnapshot,
  measurements: BrowserVoiceMeasurement[]
): string[] {
  const measurementsByLabel = new Map(
    measurements.map((measurement) => [measurement.label, measurement.value])
  );
  const gaps: string[] = [];
  if (
    !hasSubstantiveMeasurement(measurementsByLabel, 'Microphone permission')
  ) {
    gaps.push('microphone permission and setup latency not measured');
  }
  if (!hasSubstantiveMeasurement(measurementsByLabel, 'TTS cold start')) {
    gaps.push('speechSynthesis preview cold-start not measured');
  }
  if (
    !hasSubstantiveMeasurement(measurementsByLabel, 'Browser STT cold start')
  ) {
    gaps.push('browser speech recognition cold-start not measured');
  }
  if (!hasSubstantiveMeasurement(measurementsByLabel, 'Model download size')) {
    gaps.push('WASM/model download size not measured');
  }
  if (
    !hasSubstantiveMeasurement(measurementsByLabel, 'Hardware') &&
    !hasSubstantiveMeasurement(measurementsByLabel, 'Device hardware')
  ) {
    gaps.push('device hardware not measured');
  }
  if (
    !hasSubstantiveMeasurement(measurementsByLabel, 'CPU') &&
    !hasSubstantiveMeasurement(measurementsByLabel, 'Memory') &&
    !hasSubstantiveMeasurement(measurementsByLabel, 'JS heap')
  ) {
    gaps.push('CPU or memory note not measured');
  }
  if (!hasSubstantiveMeasurement(measurementsByLabel, 'Offline behavior')) {
    gaps.push(
      snapshot.online
        ? 'offline behavior not measured in this run'
        : 'offline behavior note not recorded for this offline run'
    );
  }
  return gaps;
}

function hasSubstantiveMeasurement(
  measurementsByLabel: Map<string, string>,
  label: string
): boolean {
  const value = measurementsByLabel.get(label)?.trim().toLowerCase();
  return Boolean(
    value &&
    ![
      'unknown',
      'not reported',
      'not measured',
      'not tested',
      'untested',
      'not provided',
      'started'
    ].includes(value) &&
    !isSecretShapedMeasurement(value)
  );
}

function isConcreteBrowserVoiceIdentity(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return Boolean(
    normalized &&
    ![
      'unknown',
      'unknown browser',
      'unknown platform',
      'not reported',
      'not measured',
      'not tested',
      'untested',
      'not provided',
      'replace-with-browser',
      'replace-with-platform',
      'placeholder',
      'todo',
      'tbd'
    ].includes(normalized) &&
    !normalized.includes('replace-with') &&
    !normalized.includes('placeholder')
  );
}

function isSecretShapedMeasurement(value: string): boolean {
  return /\b(token|secret|credential|password|api[-_ ]?key)\s*[:=]\s*\S+/i.test(
    value
  );
}

function uniqueTargets(
  targets: BrowserVoiceMatrixTarget[]
): BrowserVoiceMatrixTarget[] {
  return [...new Set(targets)];
}

function sameTargets(
  left: BrowserVoiceMatrixTarget[],
  right: BrowserVoiceMatrixTarget[]
): boolean {
  const leftSet = new Set(left);
  const rightSet = new Set(right);
  if (leftSet.size !== rightSet.size) {
    return false;
  }
  return [...leftSet].every((target) => rightSet.has(target));
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.filter(Boolean))];
}
