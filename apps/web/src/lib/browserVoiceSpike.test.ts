import { describe, expect, it } from 'vitest';
import {
  browserVoiceClosureBundle,
  browserVoiceClosureBundleEvidenceMarkdown,
  browserVoiceClosureBundleJSON,
  browserVoiceReport,
  browserVoiceReportJSON,
  browserVoiceRunMatrix,
  browserVoiceRunMatrixEvidenceMarkdown,
  browserVoiceRunMatrixJSON,
  browserVoiceSnapshot,
  formatBytes,
  parseBrowserVoiceClosureBundleJSON,
  parseBrowserVoiceRunMatrixJSON,
  parseBrowserVoiceReportsJSON,
  type BrowserVoiceMatrixTarget,
  validateBrowserVoiceRunMatrix
} from './browserVoiceSpike';

function hubReceiptAt(submittedAt = '2026-06-15T16:00:00.000Z') {
  return {
    submittedAt,
    followupActive: false,
    followupTurns: 0,
    followupMaxTurns: 5
  };
}

function futureGeneratedAt(offsetMs = 1000) {
  return new Date(Date.now() + offsetMs).toISOString();
}

describe('browserVoiceSnapshot', () => {
  it('summarizes browser voice capability availability', () => {
    const win = {
      navigator: {
        userAgent: 'FixtureBrowser/1.0',
        onLine: true,
        mediaDevices: {
          getUserMedia: () => Promise.resolve({})
        }
      },
      isSecureContext: true,
      AudioContext: function AudioContext() {},
      AudioWorkletNode: function AudioWorkletNode() {},
      speechSynthesis: {},
      webkitSpeechRecognition: function SpeechRecognition() {}
    } as unknown as Window;

    const snapshot = browserVoiceSnapshot(win);

    expect(snapshot.userAgent).toBe('FixtureBrowser/1.0');
    expect(snapshot.secureContext).toBe(true);
    expect(snapshot.online).toBe(true);
    expect(
      snapshot.capabilities.find((item) => item.id === 'microphone')?.available
    ).toBe(true);
    expect(
      snapshot.capabilities.find((item) => item.id === 'speech-recognition')
        ?.available
    ).toBe(true);
  });

  it('formats optional memory values for measurement notes', () => {
    expect(formatBytes(undefined)).toBe('unknown');
    expect(formatBytes(10 * 1024)).toBe('10 KB');
    expect(formatBytes(35 * 1024 * 1024)).toBe('35 MB');
  });

  it('builds a copyable JUT-6 report with known measurement gaps', () => {
    const snapshot = {
      userAgent: 'FixtureBrowser/1.0',
      secureContext: true,
      online: true,
      capabilities: [
        {
          id: 'microphone',
          label: 'Microphone capture',
          available: true,
          detail: 'available'
        }
      ]
    };

    const report = browserVoiceReport({
      snapshot,
      measurements: [
        {
          label: 'Microphone permission',
          value: '120 ms'
        }
      ],
      platform: 'FixtureOS',
      standalone: false,
      generatedAt: '2026-06-15T15:10:00.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt('2026-06-15T15:10:00.000Z')
    });

    expect(report.issue).toBe('JUT-6');
    expect(report.recommendation).toBe('experimental_display_local_only');
    expect(report.browser.platform).toBe('FixtureOS');
    expect(report.finalTranscriptPath).toEqual({
      hubEndpoint: '/api/v1/voice/transcripts/final',
      submittedThroughHub: true,
      transcriptCaptured: true,
      hubReceipt: {
        submittedAt: '2026-06-15T15:10:00.000Z',
        followupActive: false,
        followupTurns: 0,
        followupMaxTurns: 5
      }
    });
    expect(report.gaps).toContain(
      'speechSynthesis preview cold-start not measured'
    );
    expect(report.gaps).toContain('offline behavior not measured in this run');
    expect(browserVoiceReportJSON(report)).toContain('"issue": "JUT-6"');
  });

  it('does not treat typed transcripts as hub-routed evidence by default', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'started' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT requires network' }
      ],
      platform: 'FixtureOS',
      standalone: false,
      transcriptCaptured: true
    });
    const matrix = browserVoiceRunMatrix([report]);

    expect(report.finalTranscriptPath).toEqual({
      hubEndpoint: '/api/v1/voice/transcripts/final',
      submittedThroughHub: false,
      transcriptCaptured: true
    });
    expect(matrix.rows[0].finalTranscriptThroughHub).toBe(false);
    expect(matrix.rows[0].gaps).toContain(
      'final transcript hub path not proven'
    );
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row does not prove hub transcript routing'
    );
  });

  it('does not manufacture hub receipts when only hub routing is claimed', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT requires network' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:19:59.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true
    });
    const matrix = browserVoiceRunMatrix([report]);

    expect(report.finalTranscriptPath).toEqual({
      hubEndpoint: '/api/v1/voice/transcripts/final',
      submittedThroughHub: false,
      transcriptCaptured: true
    });
    expect(matrix.rows[0].finalTranscriptThroughHub).toBe(false);
    expect(matrix.rows[0].gaps).toContain(
      'final transcript hub path not proven'
    );
  });

  it('rejects pasted run reports that claim hub routing without a receipt', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT requires network' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true
    });
    const pasted = {
      ...report,
      finalTranscriptPath: {
        ...report.finalTranscriptPath,
        submittedThroughHub: true
      }
    };

    const parsed = parseBrowserVoiceReportsJSON(JSON.stringify(pasted));

    expect(parsed.reports).toEqual([]);
    expect(parsed.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('rejects pasted run reports that carry stale receipts while unsubmitted', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT requires network' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true
    });
    const pasted = {
      ...report,
      finalTranscriptPath: {
        ...report.finalTranscriptPath,
        hubReceipt: hubReceiptAt()
      }
    };

    const parsed = parseBrowserVoiceReportsJSON(JSON.stringify(pasted));

    expect(parsed.reports).toEqual([]);
    expect(parsed.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('builds an auditable multi-run acceptance matrix', () => {
    const chromiumReport = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'started' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'JS heap', value: '30 MB' },
        { label: 'Offline behavior', value: 'browser STT requires network' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T15:20:00.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });

    const offlineReport = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
        secureContext: true,
        online: false,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '110 ms' },
        { label: 'TTS cold start', value: '40 ms' },
        { label: 'JS heap', value: 'unknown' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T15:21:00.000Z',
      transcriptCaptured: false
    });

    const matrix = browserVoiceRunMatrix(
      [chromiumReport, offlineReport],
      '2026-06-15T15:22:00.000Z'
    );

    expect(matrix.issue).toBe('JUT-6');
    expect(matrix.recommendation).toBe('experimental_display_local_only');
    expect(matrix.targetsCovered).toEqual([
      'desktop-chromium',
      'offline-display'
    ]);
    expect(matrix.missingTargets).toEqual(['desktop-safari', 'kiosk-pwa']);
    expect(matrix.acceptance.complete).toBe(false);
    expect(matrix.acceptance.problems).toContain(
      'matrix has missing required targets'
    );
    expect(matrix.rows[0]).toMatchObject({
      target: 'desktop-chromium',
      generatedAt: '2026-06-15T15:20:00.000Z',
      microphoneMeasured: true,
      browserSTTMeasured: true,
      ttsMeasured: true,
      hardwareMeasured: true,
      modelDownloadMeasured: true,
      cpuMemoryMeasured: true,
      offlineBehaviorMeasured: true,
      finalTranscriptThroughHub: true,
      finalTranscriptCaptured: true
    });
    expect(matrix.rows[1].gaps).toContain(
      'browser speech recognition cold-start not measured'
    );
    expect(matrix.rows[1].gaps).toContain(
      'final transcript capture not proven'
    );
    expect(matrix.acceptance.problems).toContain(
      'offline-display row does not include a captured final transcript'
    );
    expect(matrix.gaps).toContain('desktop-safari run not recorded');
    expect(browserVoiceRunMatrixJSON(matrix)).toContain(
      '"target": "desktop-chromium"'
    );
  });

  it('validates complete browser voice acceptance matrices', () => {
    const measurements = [
      { label: 'Microphone permission', value: '90 ms' },
      { label: 'Browser STT cold start', value: 'unsupported' },
      { label: 'TTS cold start', value: '25 ms' },
      { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
      { label: 'Model download size', value: '0 MB' },
      { label: 'CPU', value: '8 percent average' },
      { label: 'Offline behavior', value: 'browser STT unavailable offline' }
    ];
    const reports = [
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'Linux arm64',
        standalone: true,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
          secureContext: true,
          online: false,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      })
    ];

    const matrix = browserVoiceRunMatrix(reports, futureGeneratedAt());

    expect(matrix.missingTargets).toEqual([]);
    expect(matrix.gaps).toEqual([]);
    expect(matrix.acceptance).toEqual({ complete: true, problems: [] });
    expect(
      validateBrowserVoiceRunMatrix({
        issue: matrix.issue,
        recommendation: matrix.recommendation,
        generatedAt: matrix.generatedAt,
        targetsCovered: matrix.targetsCovered,
        missingTargets: matrix.missingTargets,
        rows: matrix.rows,
        gaps: matrix.gaps
      })
    ).toEqual({ complete: true, problems: [] });
  });

  it('parses copied run reports for a combined matrix', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'started' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'JS heap', value: '30 MB' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T15:30:00.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt('2026-06-15T15:30:00.000Z')
    });

    const parsedSingle = parseBrowserVoiceReportsJSON(JSON.stringify(report));
    const parsedPack = parseBrowserVoiceReportsJSON(
      JSON.stringify({ reports: [report] })
    );

    expect(parsedSingle).toMatchObject({
      reports: [report],
      problems: []
    });
    expect(parsedPack).toMatchObject({
      reports: [report],
      problems: []
    });
  });

  it('parses copied matrix JSON and recalculates acceptance', () => {
    const measurements = [
      { label: 'Microphone permission', value: '90 ms' },
      { label: 'Browser STT cold start', value: 'unsupported' },
      { label: 'TTS cold start', value: '25 ms' },
      { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
      { label: 'Model download size', value: '0 MB' },
      { label: 'CPU', value: '8 percent average' },
      { label: 'Offline behavior', value: 'browser STT unavailable offline' }
    ];
    const reports = [
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'Linux arm64',
        standalone: true,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
          secureContext: true,
          online: false,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      })
    ];
    const matrixGeneratedAt = futureGeneratedAt();
    const matrix = browserVoiceRunMatrix(reports, matrixGeneratedAt);
    const tampered = {
      ...matrix,
      acceptance: {
        complete: false,
        problems: ['stale copied value']
      }
    };

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(tampered));

    expect(parsed.problems).toEqual([]);
    expect(parsed.matrix?.acceptance).toEqual({
      complete: true,
      problems: []
    });
    expect(parsed.matrix?.generatedAt).toBe(matrixGeneratedAt);
  });

  it('does not trust copied matrix target summaries over row evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:19:59.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:15:00.000Z');
    const tampered = {
      ...matrix,
      targetsCovered: [
        'desktop-chromium',
        'desktop-safari',
        'kiosk-pwa',
        'offline-display'
      ],
      missingTargets: [],
      gaps: [],
      acceptance: {
        complete: true,
        problems: []
      }
    };

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(tampered));

    expect(parsed.problems).toEqual([]);
    expect(parsed.matrix?.acceptance.complete).toBe(false);
    expect(parsed.matrix?.acceptance.problems).toContain(
      'desktop-safari run not recorded'
    );
    expect(parsed.matrix?.acceptance.problems).toContain(
      'matrix targetsCovered does not match row targets'
    );
    expect(parsed.matrix?.acceptance.problems).toContain(
      'matrix missingTargets does not match row targets'
    );
  });

  it('rejects copied matrix rows whose target does not match row evidence', () => {
    const measurements = [
      { label: 'Microphone permission', value: '90 ms' },
      { label: 'Browser STT cold start', value: 'unsupported' },
      { label: 'TTS cold start', value: '25 ms' },
      { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
      { label: 'Model download size', value: '0 MB' },
      { label: 'CPU', value: '8 percent average' },
      { label: 'Offline behavior', value: 'browser STT unavailable offline' }
    ];
    const reports = [
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          secureContext: true,
          online: true,
          capabilities: []
        },
        measurements,
        platform: 'Linux arm64',
        standalone: true,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      }),
      browserVoiceReport({
        snapshot: {
          userAgent:
            'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
          secureContext: true,
          online: false,
          capabilities: []
        },
        measurements,
        platform: 'MacIntel',
        standalone: false,
        transcriptCaptured: true,
        submittedThroughHub: true,
        hubReceipt: hubReceiptAt()
      })
    ];
    const matrix = browserVoiceRunMatrix(reports, '2026-06-15T16:16:00.000Z');
    const tampered = {
      ...matrix,
      rows: matrix.rows.map((row) =>
        row.target === 'offline-display'
          ? { ...row, online: true }
          : row.target === 'desktop-safari'
            ? {
                ...row,
                browser:
                  'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36'
              }
            : row
      )
    };

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(tampered));

    expect(parsed.problems).toEqual([]);
    expect(parsed.matrix?.acceptance.complete).toBe(false);
    expect(parsed.matrix?.acceptance.problems).toContain(
      'offline-display row must be recorded while offline'
    );
    expect(parsed.matrix?.acceptance.problems).toContain(
      'desktop-safari row target does not match browser/display evidence (desktop-chromium)'
    );
  });

  it('does not accept unknown browser rows as JUT-6 target evidence', () => {
    const acceptance = validateBrowserVoiceRunMatrix({
      issue: 'JUT-6',
      recommendation: 'experimental_display_local_only',
      generatedAt: '2026-06-17T09:00:00.000Z',
      targetsCovered: ['unknown-browser'],
      missingTargets: [
        'desktop-chromium',
        'desktop-safari',
        'kiosk-pwa',
        'offline-display'
      ],
      rows: [
        {
          target: 'unknown-browser',
          generatedAt: '2026-06-17T09:00:00.000Z',
          browser: 'FixtureBrowser/1.0',
          platform: 'FixtureOS',
          displayMode: 'browser-tab',
          online: true,
          microphoneMeasured: true,
          browserSTTMeasured: true,
          ttsMeasured: true,
          hardwareMeasured: true,
          modelDownloadMeasured: true,
          cpuMemoryMeasured: true,
          offlineBehaviorMeasured: true,
          finalTranscriptThroughHub: true,
          finalTranscriptCaptured: true,
          recommendation: 'experimental_display_local_only',
          evidence: {
            microphone: 'Microphone permission: 90 ms',
            browserSTT: 'Browser STT cold start: unsupported',
            tts: 'TTS cold start: 25 ms',
            hardware: 'Hardware: Fixture hardware',
            modelDownload: 'Model download size: 0 MB',
            cpuMemory: 'CPU: 8 percent average',
            offlineBehavior: 'Offline behavior: browser STT unavailable offline'
          },
          gaps: []
        }
      ],
      gaps: []
    });

    expect(acceptance.complete).toBe(false);
    expect(acceptance.problems).toContain(
      'unknown-browser row cannot satisfy a required JUT-6 target'
    );
  });

  it('does not accept duplicate required target rows', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT requires network' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix(
      [report, report],
      '2026-06-17T09:00:00.000Z'
    );

    expect(matrix.acceptance.complete).toBe(false);
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium has duplicate browser run rows'
    );
  });

  it('summarizes copied matrix evidence from rows instead of target summaries', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:18:00.000Z');
    const tampered = {
      ...matrix,
      targetsCovered: [
        'desktop-chromium',
        'desktop-safari',
        'kiosk-pwa',
        'offline-display'
      ] as BrowserVoiceMatrixTarget[],
      missingTargets: []
    };

    const markdown = browserVoiceRunMatrixEvidenceMarkdown(tampered);

    expect(markdown).toContain('Targets covered: `desktop-chromium`');
    expect(markdown).not.toContain(
      'Targets covered: `desktop-chromium`, `desktop-safari`'
    );
    expect(markdown).toContain(
      'Missing targets: `desktop-safari`, `kiosk-pwa`, `offline-display`'
    );
    expect(markdown).toContain(
      'matrix targetsCovered does not match row targets'
    );
  });

  it('does not trust copied row measurement booleans without matching evidence text', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:18:30.000Z');
    const tampered = {
      ...matrix,
      rows: [
        {
          ...matrix.rows[0],
          microphoneMeasured: true,
          cpuMemoryMeasured: true,
          evidence: {
            ...matrix.rows[0].evidence,
            microphone: '',
            cpuMemory: 'Heap maybe later: unknown'
          }
        }
      ]
    };

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(tampered));

    expect(parsed.problems).toEqual([]);
    expect(parsed.matrix?.acceptance.complete).toBe(false);
    expect(parsed.matrix?.acceptance.problems).toContain(
      'desktop-chromium row is missing microphone permission evidence'
    );
    expect(parsed.matrix?.acceptance.problems).toContain(
      'desktop-chromium row is missing CPU or memory evidence'
    );
  });

  it('does not accept generic unsupported text for TTS or offline evidence', () => {
    const acceptance = validateBrowserVoiceRunMatrix({
      issue: 'JUT-6',
      recommendation: 'experimental_display_local_only',
      generatedAt: '2026-06-17T09:00:00.000Z',
      targetsCovered: ['desktop-chromium'],
      missingTargets: ['desktop-safari', 'kiosk-pwa', 'offline-display'],
      rows: [
        {
          target: 'desktop-chromium',
          generatedAt: '2026-06-17T09:00:00.000Z',
          browser:
            'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
          platform: 'MacIntel',
          displayMode: 'browser-tab',
          online: true,
          microphoneMeasured: true,
          browserSTTMeasured: true,
          ttsMeasured: true,
          hardwareMeasured: true,
          modelDownloadMeasured: true,
          cpuMemoryMeasured: true,
          offlineBehaviorMeasured: true,
          finalTranscriptThroughHub: true,
          finalTranscriptCaptured: true,
          hubTranscriptReceipt: hubReceiptAt(),
          recommendation: 'experimental_display_local_only',
          evidence: {
            microphone: 'Microphone permission: 90 ms',
            browserSTT: 'Browser STT cold start: unsupported',
            tts: 'TTS cold start: unsupported',
            hardware: 'Hardware: MacBook Pro M3, 18 GB RAM',
            modelDownload: 'Model download size: 0 MB',
            cpuMemory: 'CPU: 8 percent average',
            offlineBehavior: 'Offline behavior: unavailable'
          },
          gaps: []
        }
      ],
      gaps: []
    });

    expect(acceptance.complete).toBe(false);
    expect(acceptance.problems).toContain(
      'desktop-chromium row is missing speechSynthesis evidence'
    );
    expect(acceptance.problems).toContain(
      'desktop-chromium row is missing offline behavior evidence'
    );
    expect(acceptance.problems).not.toContain(
      'desktop-chromium row is missing browser STT evidence'
    );
  });

  it('requires captured final transcripts for acceptance', () => {
    const row = {
      target: 'desktop-chromium' as const,
      generatedAt: '2026-06-15T16:19:00.000Z',
      browser: 'FixtureBrowser/1.0',
      platform: 'MacIntel',
      displayMode: 'browser-tab' as const,
      online: true,
      microphoneMeasured: true,
      browserSTTMeasured: true,
      ttsMeasured: true,
      hardwareMeasured: true,
      modelDownloadMeasured: true,
      cpuMemoryMeasured: true,
      offlineBehaviorMeasured: true,
      finalTranscriptThroughHub: true,
      finalTranscriptCaptured: false,
      recommendation: 'experimental_display_local_only' as const,
      evidence: {
        microphone: 'Microphone permission: 90 ms',
        browserSTT: 'Browser STT cold start: unsupported',
        tts: 'TTS cold start: 25 ms',
        hardware: 'Hardware: Fixture hardware',
        modelDownload: 'Model download size: 0 MB',
        cpuMemory: 'CPU: 8 percent average',
        offlineBehavior: 'Offline behavior: browser STT unavailable offline'
      },
      gaps: []
    };

    const acceptance = validateBrowserVoiceRunMatrix({
      issue: 'JUT-6',
      recommendation: 'experimental_display_local_only',
      generatedAt: '2026-06-15T16:19:00.000Z',
      targetsCovered: ['desktop-chromium'],
      missingTargets: ['desktop-safari', 'kiosk-pwa', 'offline-display'],
      rows: [row],
      gaps: []
    });

    expect(acceptance.complete).toBe(false);
    expect(acceptance.problems).toContain(
      'desktop-chromium row does not include a captured final transcript'
    );
  });

  it('requires hub transcript receipt evidence when hub routing is claimed', () => {
    const row = {
      target: 'desktop-chromium' as const,
      generatedAt: '2026-06-15T16:19:05.000Z',
      browser: 'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
      platform: 'MacIntel',
      displayMode: 'browser-tab' as const,
      online: true,
      microphoneMeasured: true,
      browserSTTMeasured: true,
      ttsMeasured: true,
      hardwareMeasured: true,
      modelDownloadMeasured: true,
      cpuMemoryMeasured: true,
      offlineBehaviorMeasured: true,
      finalTranscriptThroughHub: true,
      finalTranscriptCaptured: true,
      recommendation: 'experimental_display_local_only' as const,
      evidence: {
        microphone: 'Microphone permission: 90 ms',
        browserSTT: 'Browser STT cold start: unsupported',
        tts: 'TTS cold start: 25 ms',
        hardware: 'Hardware: Fixture hardware',
        modelDownload: 'Model download size: 0 MB',
        cpuMemory: 'CPU: 8 percent average',
        offlineBehavior: 'Offline behavior: browser STT unavailable offline'
      },
      gaps: []
    };

    const acceptance = validateBrowserVoiceRunMatrix({
      issue: 'JUT-6',
      recommendation: 'experimental_display_local_only',
      generatedAt: '2026-06-15T16:19:05.000Z',
      targetsCovered: ['desktop-chromium'],
      missingTargets: ['desktop-safari', 'kiosk-pwa', 'offline-display'],
      rows: [row],
      gaps: []
    });

    expect(acceptance.complete).toBe(false);
    expect(acceptance.problems).toContain(
      'desktop-chromium row is missing hub transcript receipt evidence'
    );
  });

  it('rejects impossible hub transcript receipt counters', () => {
    const row = {
      target: 'desktop-chromium' as const,
      generatedAt: '2026-06-15T16:19:06.000Z',
      browser: 'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
      platform: 'MacIntel',
      displayMode: 'browser-tab' as const,
      online: true,
      microphoneMeasured: true,
      browserSTTMeasured: true,
      ttsMeasured: true,
      hardwareMeasured: true,
      modelDownloadMeasured: true,
      cpuMemoryMeasured: true,
      offlineBehaviorMeasured: true,
      finalTranscriptThroughHub: true,
      finalTranscriptCaptured: true,
      hubTranscriptReceipt: {
        submittedAt: '2026-06-15T16:19:06.000Z',
        followupActive: false,
        followupTurns: 6,
        followupMaxTurns: 5
      },
      recommendation: 'experimental_display_local_only' as const,
      evidence: {
        microphone: 'Microphone permission: 90 ms',
        browserSTT: 'Browser STT cold start: unsupported',
        tts: 'TTS cold start: 25 ms',
        hardware: 'Hardware: Fixture hardware',
        modelDownload: 'Model download size: 0 MB',
        cpuMemory: 'CPU: 8 percent average',
        offlineBehavior: 'Offline behavior: browser STT unavailable offline'
      },
      gaps: []
    };

    const acceptance = validateBrowserVoiceRunMatrix({
      issue: 'JUT-6',
      recommendation: 'experimental_display_local_only',
      generatedAt: '2026-06-15T16:19:06.000Z',
      targetsCovered: ['desktop-chromium'],
      missingTargets: ['desktop-safari', 'kiosk-pwa', 'offline-display'],
      rows: [row],
      gaps: []
    });

    expect(acceptance.complete).toBe(false);
    expect(acceptance.problems).toContain(
      'desktop-chromium row is missing hub transcript receipt evidence'
    );
  });

  it('rejects row evidence generated before the hub receipt it cites', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:19:08.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: {
        submittedAt: '2026-06-15T16:19:09.000Z',
        followupActive: false,
        followupTurns: 0,
        followupMaxTurns: 5
      }
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:10.000Z');

    expect(matrix.acceptance.complete).toBe(false);
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row generatedAt must not be before hub receipt submittedAt'
    );
  });

  it('rejects active follow-up receipts without a future expiry', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:19:07.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: {
        submittedAt: '2026-06-15T16:19:07.000Z',
        followupActive: true,
        followupTurns: 1,
        followupMaxTurns: 5,
        followupExpiresAt: '2026-06-15T16:19:06.000Z'
      }
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:08.000Z');

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(matrix));

    expect(parsed.matrix).toBeUndefined();
    expect(parsed.problems).toEqual([
      'saved matrix is not a JUT-6 browser voice matrix'
    ]);
  });

  it('does not accept unknown browser memory as CPU or memory evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'JS heap', value: 'unknown' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:30.000Z');

    expect(matrix.rows[0].cpuMemoryMeasured).toBe(false);
    expect(matrix.rows[0].gaps).toContain('CPU or memory note not measured');
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row is missing CPU or memory evidence'
    );
  });

  it('does not accept subjective CPU notes as rough CPU or memory numbers', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: 'low' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:30.000Z');

    expect(matrix.rows[0].cpuMemoryMeasured).toBe(true);
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row is missing CPU or memory evidence'
    );
  });

  it('does not accept missing device hardware as matrix evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'not provided' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:35.000Z');

    expect(report.gaps).toContain('device hardware not measured');
    expect(matrix.rows[0].hardwareMeasured).toBe(false);
    expect(matrix.rows[0].gaps).toContain('device hardware note not measured');
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row is missing device hardware evidence'
    );
  });

  it('does not accept placeholder model download size as evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: 'unknown' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:40.000Z');

    expect(report.gaps).toContain('WASM/model download size not measured');
    expect(matrix.rows[0].modelDownloadMeasured).toBe(false);
    expect(matrix.rows[0].gaps).toContain('model download size not measured');
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row is missing model download evidence'
    );
  });

  it('does not accept placeholder capture, STT, or TTS values as evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: 'not measured' },
        { label: 'Browser STT cold start', value: 'unknown' },
        { label: 'TTS cold start', value: 'not tested' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:42.000Z');

    expect(report.gaps).toEqual(
      expect.arrayContaining([
        'microphone permission and setup latency not measured',
        'browser speech recognition cold-start not measured',
        'speechSynthesis preview cold-start not measured'
      ])
    );
    expect(matrix.rows[0]).toMatchObject({
      microphoneMeasured: false,
      browserSTTMeasured: false,
      ttsMeasured: false
    });
    expect(matrix.rows[0].gaps).toEqual(
      expect.arrayContaining([
        'microphone permission not measured',
        'browser STT cold start not measured',
        'speechSynthesis cold start not measured'
      ])
    );
    expect(matrix.acceptance.problems).toEqual(
      expect.arrayContaining([
        'desktop-chromium row is missing microphone permission evidence',
        'desktop-chromium row is missing browser STT evidence',
        'desktop-chromium row is missing speechSynthesis evidence'
      ])
    );
  });

  it('does not accept placeholder offline behavior as evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'not tested' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:19:45.000Z');

    expect(report.gaps).toContain('offline behavior not measured in this run');
    expect(matrix.rows[0].offlineBehaviorMeasured).toBe(false);
    expect(matrix.rows[0].gaps).toContain('offline behavior note not measured');
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row is missing offline behavior evidence'
    );
  });

  it('summarizes browser voice matrix evidence for Linear comments', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:19:59.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:20:00.000Z');

    const markdown = browserVoiceRunMatrixEvidenceMarkdown(matrix);

    for (const want of [
      'Browser Voice Evidence: JUT-6',
      'Recommendation: `experimental_display_local_only`',
      'Targets covered: `desktop-chromium`',
      'Missing targets: `desktop-safari`, `kiosk-pwa`, `offline-display`',
      'Acceptance: gaps remain',
      'matrix has missing required targets',
      '`desktop-chromium`: generatedAt=`2026-06-15T16:19:59.000Z`, display=`browser-tab`, online=true',
      'mic=`Microphone permission: 90 ms`',
      'stt=`Browser STT cold start: unsupported`',
      'model=`Model download size: 0 MB`',
      'cpuMemory=`CPU: 8 percent average`',
      'offline=`Offline behavior: browser STT unavailable offline`'
    ]) {
      expect(markdown).toContain(want);
    }
  });

  it('redacts copied measurement URLs and paths from Linear evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        {
          label: 'Hardware',
          value: 'MacBook Pro M3 from /private/tmp/hardware.txt'
        },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        {
          label: 'Offline behavior',
          value: 'notes at https://internal.example.test/offline'
        }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const markdown = browserVoiceRunMatrixEvidenceMarkdown(
      browserVoiceRunMatrix([report], '2026-06-15T16:21:00.000Z')
    );

    expect(markdown).toContain('mic=`Microphone permission: 90 ms`');
    expect(markdown).toContain('Hardware: MacBook Pro M3 from [redacted-path]');
    expect(markdown).toContain('Offline behavior: notes at [redacted-url]');
    expect(markdown).not.toContain('/private/tmp');
    expect(markdown).not.toContain('https://internal.example.test');
  });

  it('does not accept credential-shaped measurement values as evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms token=secret' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:21:30.000Z');
    const markdown = browserVoiceRunMatrixEvidenceMarkdown(matrix);

    expect(report.gaps).toContain(
      'microphone permission and setup latency not measured'
    );
    expect(matrix.rows[0].microphoneMeasured).toBe(false);
    expect(matrix.rows[0].evidence.microphone).toBe('');
    expect(matrix.acceptance.problems).toContain(
      'desktop-chromium row is missing microphone permission evidence'
    );
    expect(markdown).not.toContain('token=secret');
  });

  it('builds and revalidates a browser voice closure bundle', () => {
    const measurements = [
      { label: 'Microphone permission', value: '90 ms' },
      { label: 'Browser STT cold start', value: 'unsupported' },
      { label: 'TTS cold start', value: '25 ms' },
      { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
      { label: 'Model download size', value: '0 MB' },
      { label: 'CPU', value: '8 percent average' },
      { label: 'Offline behavior', value: 'browser STT unavailable offline' }
    ];
    const matrixGeneratedAt = futureGeneratedAt();
    const bundleGeneratedAt = futureGeneratedAt(2000);
    const matrix = browserVoiceRunMatrix(
      [
        browserVoiceReport({
          snapshot: {
            userAgent:
              'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
            secureContext: true,
            online: true,
            capabilities: []
          },
          measurements,
          platform: 'MacIntel',
          standalone: false,
          transcriptCaptured: true,
          submittedThroughHub: true,
          hubReceipt: hubReceiptAt()
        }),
        browserVoiceReport({
          snapshot: {
            userAgent:
              'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
            secureContext: true,
            online: true,
            capabilities: []
          },
          measurements,
          platform: 'MacIntel',
          standalone: false,
          transcriptCaptured: true,
          submittedThroughHub: true,
          hubReceipt: hubReceiptAt()
        }),
        browserVoiceReport({
          snapshot: {
            userAgent:
              'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
            secureContext: true,
            online: true,
            capabilities: []
          },
          measurements,
          platform: 'Linux arm64',
          standalone: true,
          transcriptCaptured: true,
          submittedThroughHub: true,
          hubReceipt: hubReceiptAt()
        }),
        browserVoiceReport({
          snapshot: {
            userAgent:
              'Mozilla/5.0 AppleWebKit/605.1.15 Version/18.0 Safari/605.1.15',
            secureContext: true,
            online: false,
            capabilities: []
          },
          measurements,
          platform: 'MacIntel',
          standalone: false,
          transcriptCaptured: true,
          submittedThroughHub: true,
          hubReceipt: hubReceiptAt()
        })
      ],
      matrixGeneratedAt
    );
    const bundle = browserVoiceClosureBundle(matrix, bundleGeneratedAt);

    const parsed = parseBrowserVoiceClosureBundleJSON(
      browserVoiceClosureBundleJSON(bundle)
    );
    const markdown = browserVoiceClosureBundleEvidenceMarkdown(bundle);

    expect(parsed.problems).toEqual([]);
    expect(parsed.bundle?.matrix.acceptance.complete).toBe(true);
    expect(bundle.evidenceMarkdown).toBe(
      browserVoiceRunMatrixEvidenceMarkdown(matrix)
    );
    expect(markdown).toContain('Browser Voice Closure Bundle: JUT-6');
    expect(markdown).toContain('Evidence summary attached: `true`');
    expect(markdown).toContain('Acceptance complete: `true`');
  });

  it('rejects stale browser voice closure bundle summaries', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:42:00.000Z');
    const bundle = browserVoiceClosureBundle(matrix);

    const parsed = parseBrowserVoiceClosureBundleJSON(
      JSON.stringify({
        ...bundle,
        evidenceMarkdown: '### Browser Voice Evidence: JUT-6\n\n- stale=true'
      })
    );

    expect(parsed.bundle).toBeDefined();
    expect(parsed.problems).toContain(
      'closure bundle evidenceMarkdown does not match matrix'
    );
    expect(parsed.problems).toContain('desktop-safari run not recorded');
    expect(parsed.problems).toContain('matrix has missing required targets');
  });

  it('rejects closure bundles with placeholder generatedAt evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], 'replace-with-generatedAt');
    const bundle = browserVoiceClosureBundle(matrix, 'placeholder-generatedAt');

    const parsed = parseBrowserVoiceClosureBundleJSON(
      browserVoiceClosureBundleJSON(bundle)
    );
    const markdown = browserVoiceClosureBundleEvidenceMarkdown(bundle);

    expect(parsed.bundle).toBeDefined();
    expect(parsed.problems).toContain('matrix generatedAt must be RFC3339');
    expect(parsed.problems).toContain(
      'closure bundle generatedAt must be RFC3339'
    );
    expect(markdown).toContain('matrix generatedAt must be RFC3339');
    expect(markdown).toContain('closure bundle generatedAt must be RFC3339');
  });

  it('rejects closure bundles generated before their matrix', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:30:00.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:31:00.000Z');
    const bundle = browserVoiceClosureBundle(
      matrix,
      '2026-06-15T16:30:30.000Z'
    );

    const parsed = parseBrowserVoiceClosureBundleJSON(
      browserVoiceClosureBundleJSON(bundle)
    );

    expect(parsed.bundle).toBeDefined();
    expect(parsed.problems).toContain(
      'closure bundle generatedAt must not be before matrix generatedAt'
    );
  });

  it('rejects saved closure bundles with undeclared fields', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const bundle = browserVoiceClosureBundle(browserVoiceRunMatrix([report]));

    for (const withExtra of [
      { ...bundle, rawAudioPcm: 'secret' },
      { ...bundle, matrix: { ...bundle.matrix, rawTranscript: 'secret' } },
      {
        ...bundle,
        matrix: {
          ...bundle.matrix,
          rows: [{ ...bundle.matrix.rows[0], providerDebug: 'token=secret' }]
        }
      }
    ]) {
      const parsed = parseBrowserVoiceClosureBundleJSON(
        JSON.stringify(withExtra)
      );
      expect(parsed.bundle).toBeUndefined();
      expect(parsed.problems).toEqual([
        'saved closure bundle is not a JUT-6 browser voice bundle'
      ]);
    }
  });

  it('rejects invalid saved matrix JSON without throwing', () => {
    expect(parseBrowserVoiceRunMatrixJSON('')).toEqual({
      matrix: undefined,
      problems: []
    });

    const invalidJSON = parseBrowserVoiceRunMatrixJSON(
      '{"rawTranscript":"token=secret"'
    );
    expect(invalidJSON.matrix).toBeUndefined();
    expect(invalidJSON.problems).toEqual([
      'saved matrix JSON could not be parsed'
    ]);
    expect(invalidJSON.problems.join('\n')).not.toContain('token=secret');
    expect(invalidJSON.problems.join('\n')).not.toContain('rawTranscript');

    const wrongIssue = parseBrowserVoiceRunMatrixJSON(
      JSON.stringify({ issue: 'JUT-7' })
    );
    expect(wrongIssue.matrix).toBeUndefined();
    expect(wrongIssue.problems).toEqual([
      'saved matrix is not a JUT-6 browser voice matrix'
    ]);
  });

  it('rejects saved matrices with non-RFC3339 generatedAt evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], 'not-provided');

    const parsed = parseBrowserVoiceRunMatrixJSON(
      browserVoiceRunMatrixJSON(matrix)
    );

    expect(parsed.matrix).toBeDefined();
    expect(parsed.matrix?.acceptance.problems).toContain(
      'matrix generatedAt must be RFC3339'
    );
  });

  it('rejects saved matrices with non-RFC3339 row generatedAt evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:30:00.000Z');
    const tampered = {
      ...matrix,
      rows: [{ ...matrix.rows[0], generatedAt: 'replace-with-generatedAt' }]
    };

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(tampered));

    expect(parsed.matrix).toBeDefined();
    expect(parsed.matrix?.acceptance.problems).toContain(
      'desktop-chromium row generatedAt must be RFC3339'
    );
  });

  it('rejects saved matrices generated before a row run', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:31:00.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:30:00.000Z');

    const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(matrix));

    expect(parsed.matrix).toBeDefined();
    expect(parsed.matrix?.acceptance.problems).toContain(
      'desktop-chromium row generatedAt must not be after matrix generatedAt'
    );
  });

  it('rejects saved matrices whose generatedAt omits a timezone', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:40:00');

    const parsed = parseBrowserVoiceRunMatrixJSON(
      browserVoiceRunMatrixJSON(matrix)
    );

    expect(parsed.matrix).toBeDefined();
    expect(parsed.matrix?.acceptance.problems).toContain(
      'matrix generatedAt must be RFC3339'
    );
  });

  it('rejects invalid saved run JSON without throwing', () => {
    expect(parseBrowserVoiceReportsJSON('')).toEqual({
      reports: [],
      problems: []
    });

    const invalidJSON = parseBrowserVoiceReportsJSON(
      '{"transcript":"token=secret"'
    );
    expect(invalidJSON.reports).toEqual([]);
    expect(invalidJSON.problems).toEqual([
      'saved runs JSON could not be parsed'
    ]);
    expect(invalidJSON.problems.join('\n')).not.toContain('token=secret');
    expect(invalidJSON.problems.join('\n')).not.toContain('transcript');

    const wrongIssue = parseBrowserVoiceReportsJSON(
      JSON.stringify({ issue: 'JUT-7' })
    );
    expect(wrongIssue.reports).toEqual([]);
    expect(wrongIssue.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('rejects saved run reports with non-RFC3339 generatedAt evidence', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });

    const parsed = parseBrowserVoiceReportsJSON(
      JSON.stringify({ ...report, generatedAt: 'replace-with-generatedAt' })
    );

    expect(parsed.reports).toEqual([]);
    expect(parsed.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('rejects saved run reports whose generatedAt omits a timezone', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });

    const parsed = parseBrowserVoiceReportsJSON(
      JSON.stringify({ ...report, generatedAt: '2026-06-15T16:40:00' })
    );

    expect(parsed.reports).toEqual([]);
    expect(parsed.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('rejects saved run reports generated before the hub receipt they cite', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      generatedAt: '2026-06-15T16:40:00.000Z',
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt('2026-06-15T16:40:01.000Z')
    });

    const parsed = parseBrowserVoiceReportsJSON(JSON.stringify(report));

    expect(parsed.reports).toEqual([]);
    expect(parsed.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('rejects saved run reports with malformed nested fields', () => {
    const validReport = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: [
          {
            id: 'microphone',
            label: 'Microphone capture',
            available: true,
            detail: 'available'
          }
        ]
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });

    for (const malformed of [
      {
        ...validReport,
        measurements: [{ label: 'Microphone permission', value: 90 }]
      },
      {
        ...validReport,
        capabilities: [{ id: 'microphone', available: true }]
      },
      {
        ...validReport,
        finalTranscriptPath: {
          ...validReport.finalTranscriptPath,
          transcriptCaptured: 'yes'
        }
      },
      {
        ...validReport,
        gaps: ['ok', 42]
      }
    ]) {
      const parsed = parseBrowserVoiceReportsJSON(JSON.stringify(malformed));
      expect(parsed.reports).toEqual([]);
      expect(parsed.problems).toEqual([
        'saved run 1 is not a JUT-6 browser voice report'
      ]);
    }
  });

  it('rejects saved run reports with undeclared fields', () => {
    const validReport = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: [
          {
            id: 'microphone',
            label: 'Microphone capture',
            available: true,
            detail: 'available'
          }
        ]
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });

    for (const withExtra of [
      { ...validReport, rawAudioPcm: 'secret' },
      {
        ...validReport,
        browser: { ...validReport.browser, remoteDebugURL: 'http://secret' }
      },
      {
        ...validReport,
        finalTranscriptPath: {
          ...validReport.finalTranscriptPath,
          providerDebug: 'token=secret'
        }
      },
      {
        ...validReport,
        capabilities: [
          { ...validReport.capabilities[0], providerCredential: 'secret' }
        ]
      },
      {
        ...validReport,
        measurements: [{ ...validReport.measurements[0], rawSample: 'debug' }]
      }
    ]) {
      const parsed = parseBrowserVoiceReportsJSON(JSON.stringify(withExtra));
      expect(parsed.reports).toEqual([]);
      expect(parsed.problems).toEqual([
        'saved run 1 is not a JUT-6 browser voice report'
      ]);
    }
  });

  it('rejects saved run report bundles with undeclared wrapper fields', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });

    const parsed = parseBrowserVoiceReportsJSON(
      JSON.stringify({ reports: [report], rawAudioPcm: 'secret' })
    );

    expect(parsed.reports).toEqual([]);
    expect(parsed.problems).toEqual([
      'saved run 1 is not a JUT-6 browser voice report'
    ]);
  });

  it('rejects saved matrices with undeclared fields', () => {
    const report = browserVoiceReport({
      snapshot: {
        userAgent:
          'Mozilla/5.0 AppleWebKit/537.36 Chrome/126.0.0.0 Safari/537.36',
        secureContext: true,
        online: true,
        capabilities: []
      },
      measurements: [
        { label: 'Microphone permission', value: '90 ms' },
        { label: 'Browser STT cold start', value: 'unsupported' },
        { label: 'TTS cold start', value: '25 ms' },
        { label: 'Hardware', value: 'MacBook Pro M3, 18 GB RAM' },
        { label: 'Model download size', value: '0 MB' },
        { label: 'CPU', value: '8 percent average' },
        { label: 'Offline behavior', value: 'browser STT unavailable offline' }
      ],
      platform: 'MacIntel',
      standalone: false,
      transcriptCaptured: true,
      submittedThroughHub: true,
      hubReceipt: hubReceiptAt()
    });
    const matrix = browserVoiceRunMatrix([report], '2026-06-15T16:30:00.000Z');

    for (const withExtra of [
      { ...matrix, rawAudioPcm: 'secret' },
      {
        ...matrix,
        acceptance: {
          ...matrix.acceptance,
          providerDebug: 'token=secret'
        }
      },
      {
        ...matrix,
        rows: [{ ...matrix.rows[0], providerDebug: 'token=secret' }]
      }
    ]) {
      const parsed = parseBrowserVoiceRunMatrixJSON(JSON.stringify(withExtra));
      expect(parsed.matrix).toBeUndefined();
      expect(parsed.problems).toEqual([
        'saved matrix is not a JUT-6 browser voice matrix'
      ]);
    }
  });
});
