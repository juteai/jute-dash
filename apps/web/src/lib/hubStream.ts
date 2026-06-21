import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import {
  eventsURL,
  muteVoice,
  unmuteVoice,
  cancelVoice,
  getDashboard,
  submitVoiceAudio
} from '$lib/hubClient';
import { logger } from '$lib/logger';
import { navigationStore } from '$lib/navigationStore';
import type {
  DashboardData,
  DisplayNotification,
  DisplayFocusWidget,
  UserFacingIssue,
  WidgetLayout,
  VoiceConversationMessage,
  VoiceStatus
} from '$lib/types';

export type VoiceOrbState =
  | 'idle'
  | 'listening'
  | 'thinking'
  | 'speaking'
  | 'followup'
  | 'error';

export interface HubStreamState {
  dashboard: DashboardData;
  displayNotifications: DisplayNotification[];
  focusedWidgetId: string;
  voiceOrbState: VoiceOrbState;
  voiceAgentId: string;
  voiceConversationId: string;
  voiceMessages: VoiceConversationMessage[];
  voiceTranscript: string;
  assistantSpeech: string;
  voiceError: string;
  voiceFollowupExpiresAt: string;
  retrying: boolean;
}

const initialStub: DashboardData = {
  config: {
    home: { name: 'Jute Home' },
    display: {
      theme: 'jute-mono',
      colorMode: 'system',
      themeId: 'jute-mono',
      density: 'comfortable',
      motion: 'full',
      background: {
        kind: 'theme',
        value: '',
        fit: 'cover',
        position: 'center',
        overlay: 'none'
      },
      widgetChrome: { default: 'auto' },
      accentColor: '',
      idleMode: 'none'
    },
    agents: [],
    rooms: [],
    tiles: []
  },
  home: {
    generatedAt: '',
    home: { name: 'Jute Home' },
    rooms: [],
    tiles: []
  },
  agents: [],
  layout: { profileId: '', widgets: [] },
  voice: {
    enabled: false,
    muted: false,
    state: 'idle',
    serviceStatus: 'not_configured',
    deviceProfileId: '',
    wakeWordModelId: '',
    wakeWordPhrase: '',
    wakeSensitivity: 0.5,
    sttProviderId: '',
    ttsProviderId: '',
    sttModelId: '',
    ttsModelId: '',
    ttsVoiceId: '',
    ttsEnabled: false,
    ttsLocale: 'en',
    ttsSpeed: 1,
    ttsVolume: 1,
    preferredAgentId: '',
    cloudOptIn: false,
    commandProvidersEnabled: false,
    followupWindowSeconds: 8,
    microphoneProfile: '',
    updatedAt: ''
  },
  status: {
    status: 'ok',
    version: '0.0.1',
    startedAt: '',
    setup: { complete: true, missing: [] },
    config: { hasBootstrapConfig: true, writableYaml: false },
    eventStream: { available: true },
    mcp: {
      enabled: false,
      serviceStatus: 'disabled',
      transport: '',
      listenAddress: '',
      path: '',
      authMode: '',
      allowLan: false
    },
    agents: {
      total: 0,
      enabled: 0,
      disabled: 0,
      available: 0,
      unavailable: 0,
      dashboardContextSupported: 0,
      mcpScoped: 0
    },
    voice: { enabled: false, serviceStatus: 'not_configured', state: 'idle' }
  },
  connectionState: 'starting',
  stale: false,
  hubUrl: '',
  loadedAt: ''
};

const initialState: HubStreamState = {
  dashboard: initialStub,
  displayNotifications: [],
  focusedWidgetId: '',
  voiceOrbState: 'idle',
  voiceAgentId: '',
  voiceConversationId: '',
  voiceMessages: [],
  voiceTranscript: '',
  assistantSpeech: '',
  voiceError: '',
  voiceFollowupExpiresAt: '',
  retrying: false
};

function eventConversationID(event: { conversationId?: string }): string {
  return typeof event.conversationId === 'string' ? event.conversationId : '';
}

function safeEventTextLength(text: unknown): string | undefined {
  return typeof text === 'string' ? `chars=${text.length}` : undefined;
}

function voiceMessageID(prefix: string, event: { id?: string }): string {
  return typeof event.id === 'string' && event.id
    ? `${prefix}-${event.id}`
    : `${prefix}-${Date.now()}`;
}

function voiceConversationMessageID(
  prefix: string,
  event: {
    id?: string;
    conversationId?: string;
    payload?: { taskId?: string } & Record<string, unknown>;
  }
): string {
  const suffix =
    event.payload?.taskId ||
    event.conversationId ||
    (typeof event.id === 'string' ? event.id : '');
  return suffix ? `${prefix}-${suffix}` : voiceMessageID(prefix, event);
}

function appendOrReplaceVoiceMessage(
  messages: VoiceConversationMessage[],
  message: VoiceConversationMessage,
  append = false
): VoiceConversationMessage[] {
  const existing = messages.find((item) => item.id === message.id);
  const nextMessage =
    existing && append
      ? { ...message, text: `${existing.text}${message.text}` }
      : message;
  const next = messages.filter((item) => item.id !== message.id);
  return [...next, nextMessage].slice(-8);
}

function safeVoiceError(reason: unknown): string {
  switch (reason) {
    case 'agent_failure':
      return 'The agent could not complete that voice turn.';
    case 'stt_failure':
    case 'transcription_failed':
      return "I didn't catch that. Try again when listening resumes.";
    case 'tts_failure':
    case 'synthesis_failed':
      return 'Speech playback is unavailable. The visual response is still available.';
    case 'sensitive_output_visual_only':
    case 'sensitive_output_requires_confirmation':
      return 'Sensitive response is visual-only.';
    case 'provider_failure':
    case 'provider_unavailable':
      return 'The selected voice provider is unavailable.';
    case 'followup_expired':
      return 'The follow-up window expired.';
    case 'canceled':
    case 'followup_limit_reached':
      return '';
    default:
      return 'Voice session ended.';
  }
}

function createHubStreamStore() {
  const { subscribe, update } = writable<HubStreamState>(initialState);

  let eventSource: EventSource | undefined;
  let pollingTimer: number | undefined;
  let focusTimer: number | undefined;
  let notificationTimers: number[] = [];
  let voiceEndedTimeout: number | undefined;
  let hasConnected = false;
  let isMounted = false;

  function markConnected() {
    hasConnected = true;
    update((s) => ({
      ...s,
      dashboard: {
        ...s.dashboard,
        connectionState: 'connected',
        stale: false,
        issue: undefined
      }
    }));
  }

  function markIssue(
    connectionState: DashboardData['connectionState'],
    issue: UserFacingIssue
  ) {
    update((s) => ({
      ...s,
      dashboard: {
        ...s.dashboard,
        connectionState,
        stale: true,
        issue
      }
    }));
  }

  function parseDisplayEvent<T>(dataStr: string): T | undefined {
    try {
      return JSON.parse(dataStr) as T;
    } catch {
      return undefined;
    }
  }

  async function pollStatus(fetcher: typeof fetch = window.fetch) {
    try {
      const fresh = await getDashboard(fetcher);
      update((s) => {
        const conn = s.dashboard.connectionState;
        const nextConn =
          conn === 'offline' || conn === 'reconnecting' ? 'connected' : conn;
        return {
          ...s,
          dashboard: {
            ...fresh,
            connectionState: nextConn,
            stale: false,
            issue: undefined
          }
        };
      });
      hasConnected = true;
    } catch {
      const state = hasConnected ? 'reconnecting' : 'offline';
      markIssue(state, {
        code: 'hub_unreachable',
        severity: 'error',
        title: 'Hub not reachable',
        message: `Jute Dash cannot connect to the local hub.`,
        action: {
          label: 'Retry',
          target: 'retry'
        }
      });
    }
  }

  function addDisplayNotification(notification: DisplayNotification) {
    update((s) => {
      const updated = [
        notification,
        ...s.displayNotifications.filter((item) => item.id !== notification.id)
      ].slice(0, 3);

      const expiry = Date.parse(notification.expiresAt);
      const delay = Number.isFinite(expiry)
        ? Math.max(2500, expiry - Date.now())
        : 6000;

      const timer = window.setTimeout(() => {
        update((s2) => ({
          ...s2,
          displayNotifications: s2.displayNotifications.filter(
            (item) => item.id !== notification.id
          )
        }));
      }, delay);

      notificationTimers.push(timer);
      return { ...s, displayNotifications: updated };
    });
  }

  function focusWidget(focus: DisplayFocusWidget) {
    navigationStore.closeChat();

    if (focusTimer) {
      window.clearTimeout(focusTimer);
    }

    update((s) => ({ ...s, focusedWidgetId: focus.widgetInstanceId }));

    focusTimer = window.setTimeout(() => {
      update((s) => ({ ...s, focusedWidgetId: '' }));
      focusTimer = undefined;
    }, 4500);

    window.setTimeout(() => {
      const escaped =
        typeof CSS !== 'undefined' && CSS.escape
          ? CSS.escape(focus.widgetInstanceId)
          : focus.widgetInstanceId.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
      document.querySelector(`[data-widget-id="${escaped}"]`)?.scrollIntoView({
        block: 'center',
        behavior: 'smooth'
      });
    }, 0);
  }

  return {
    subscribe,
    init: (data: DashboardData) => {
      hasConnected = data.connectionState === 'connected';
      update((s) => ({
        ...s,
        dashboard: data
      }));
    },
    connect: (fetcher: typeof fetch = window.fetch) => {
      if (!browser || eventSource) return;
      isMounted = true;

      eventSource = new EventSource(eventsURL());
      eventSource.addEventListener('open', async () => {
        logger.sse('Connected');
        if (hasConnected) {
          try {
            const fresh = await getDashboard(fetcher);
            update((s) => ({
              ...s,
              dashboard: { ...fresh, connectionState: 'connected' }
            }));
          } catch {
            // ignore
          }
          markConnected();
        }
      });

      eventSource.addEventListener('error', async () => {
        logger.sseError('Event stream connection lost or failed');
        if (isMounted && hasConnected) {
          try {
            const fresh = await getDashboard(fetcher);
            update((s) => ({
              ...s,
              dashboard: {
                ...fresh,
                connectionState: 'degraded',
                stale: false,
                issue: {
                  code: 'event_stream_disconnected',
                  severity: 'warning',
                  title: 'Event stream disconnected',
                  message:
                    'Jute lost the live display event stream. Dashboard data may be stale.'
                }
              }
            }));
          } catch {
            markIssue('reconnecting', {
              code: 'hub_unreachable',
              severity: 'error',
              title: 'Hub not reachable',
              message: `Jute Dash cannot connect to the local hub.`,
              action: {
                label: 'Retry',
                target: 'retry'
              }
            });
          }
        }
      });

      eventSource.addEventListener('display.notification', (event) => {
        logger.sse(event.type);
        const notification = parseDisplayEvent<DisplayNotification>(
          (event as MessageEvent).data
        );
        if (notification) addDisplayNotification(notification);
      });

      eventSource.addEventListener('display.focus_widget', (event) => {
        logger.sse(event.type);
        const focus = parseDisplayEvent<DisplayFocusWidget>(
          (event as MessageEvent).data
        );
        if (focus) focusWidget(focus);
      });

      eventSource.addEventListener('voice.state_changed', (event) => {
        const e = parseDisplayEvent<{ payload?: Partial<VoiceStatus> }>(
          (event as MessageEvent).data
        );
        logger.sse(
          event.type,
          e?.payload ? `state=${e.payload.state}` : undefined
        );
        if (e?.payload) {
          const payload = e.payload;
          update((s) => ({
            ...s,
            dashboard: {
              ...s.dashboard,
              voice: {
                ...s.dashboard.voice,
                enabled: Boolean(payload.enabled),
                muted: Boolean(payload.muted),
                state:
                  typeof payload.state === 'string'
                    ? payload.state
                    : s.dashboard.voice.state,
                serviceStatus:
                  typeof payload.serviceStatus === 'string'
                    ? payload.serviceStatus
                    : s.dashboard.voice.serviceStatus
              }
            }
          }));
        }
      });

      eventSource.addEventListener('voice.wake_detected', (event) => {
        const e = parseDisplayEvent<{ conversationId?: string }>(
          (event as MessageEvent).data
        );
        logger.sse(event.type);
        if (voiceEndedTimeout) {
          window.clearTimeout(voiceEndedTimeout);
          voiceEndedTimeout = undefined;
        }
        update((s) => ({
          ...s,
          voiceConversationId: eventConversationID(e ?? {}),
          voiceAgentId: '',
          voiceMessages: [],
          voiceOrbState: 'listening',
          voiceTranscript: '',
          assistantSpeech: '',
          voiceError: '',
          voiceFollowupExpiresAt: ''
        }));
      });

      eventSource.addEventListener('voice.transcript.partial', (event) => {
        const e = parseDisplayEvent<{
          id?: string;
          conversationId?: string;
          createdAt?: string;
          payload?: { text?: string };
        }>((event as MessageEvent).data);
        const text = e?.payload?.text;
        logger.sse(event.type, safeEventTextLength(text));
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceTranscript: typeof text === 'string' ? text : s.voiceTranscript,
          voiceMessages:
            typeof text === 'string' && text
              ? appendOrReplaceVoiceMessage(s.voiceMessages, {
                  id: voiceConversationMessageID('user', e ?? {}),
                  role: 'user',
                  text,
                  createdAt:
                    typeof e?.createdAt === 'string'
                      ? e.createdAt
                      : new Date().toISOString(),
                  status: 'partial'
                })
              : s.voiceMessages,
          voiceOrbState: 'listening',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('voice.transcript.final', (event) => {
        const e = parseDisplayEvent<{
          id?: string;
          conversationId?: string;
          createdAt?: string;
          payload?: { text?: string };
        }>((event as MessageEvent).data);
        const text = e?.payload?.text;
        logger.sse(event.type, safeEventTextLength(text));
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceTranscript: typeof text === 'string' ? text : s.voiceTranscript,
          voiceMessages:
            typeof text === 'string' && text
              ? appendOrReplaceVoiceMessage(s.voiceMessages, {
                  id: voiceConversationMessageID('user', e ?? {}),
                  role: 'user',
                  text,
                  createdAt:
                    typeof e?.createdAt === 'string'
                      ? e.createdAt
                      : new Date().toISOString(),
                  status: 'final'
                })
              : s.voiceMessages,
          voiceOrbState: 'listening',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('conversation.started', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { agentId?: string };
        }>((event as MessageEvent).data);
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceAgentId:
            typeof e?.payload?.agentId === 'string'
              ? e.payload.agentId
              : s.voiceAgentId,
          voiceError: ''
        }));
      });

      eventSource.addEventListener('conversation.turn_started', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { agentId?: string };
        }>((event as MessageEvent).data);
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceAgentId:
            typeof e?.payload?.agentId === 'string'
              ? e.payload.agentId
              : s.voiceAgentId,
          voiceOrbState: 'thinking',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('conversation.turn_completed', (event) => {
        const e = parseDisplayEvent<{
          id?: string;
          conversationId?: string;
          createdAt?: string;
          payload?: {
            agentId?: string;
            speech?: string;
            text?: string;
            status?: string;
          };
        }>((event as MessageEvent).data);
        const speech = e?.payload?.speech;
        const text = e?.payload?.text;
        const assistantText =
          typeof speech === 'string'
            ? speech
            : typeof text === 'string'
              ? text
              : '';
        logger.sse(event.type, safeEventTextLength(assistantText));
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceAgentId:
            typeof e?.payload?.agentId === 'string'
              ? e.payload.agentId
              : s.voiceAgentId,
          assistantSpeech: assistantText || s.assistantSpeech,
          voiceMessages: assistantText
            ? appendOrReplaceVoiceMessage(s.voiceMessages, {
                id: voiceConversationMessageID('assistant', e ?? {}),
                role: 'assistant',
                text: assistantText,
                createdAt:
                  typeof e?.createdAt === 'string'
                    ? e.createdAt
                    : new Date().toISOString(),
                status: 'speaking'
              })
            : s.voiceMessages,
          voiceOrbState: 'speaking',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('conversation.assistant_delta', (event) => {
        const e = parseDisplayEvent<{
          id?: string;
          conversationId?: string;
          createdAt?: string;
          payload?: {
            agentId?: string;
            taskId?: string;
            text?: string;
            append?: boolean;
          };
        }>((event as MessageEvent).data);
        const text = e?.payload?.text;
        logger.sse(event.type, safeEventTextLength(text));
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceAgentId:
            typeof e?.payload?.agentId === 'string'
              ? e.payload.agentId
              : s.voiceAgentId,
          assistantSpeech:
            typeof text === 'string'
              ? e?.payload?.append
                ? `${s.assistantSpeech}${text}`
                : text
              : s.assistantSpeech,
          voiceMessages:
            typeof text === 'string' && text
              ? appendOrReplaceVoiceMessage(
                  s.voiceMessages,
                  {
                    id: voiceConversationMessageID('assistant', e ?? {}),
                    role: 'assistant',
                    text,
                    createdAt:
                      typeof e?.createdAt === 'string'
                        ? e.createdAt
                        : new Date().toISOString(),
                    status: 'speaking'
                  },
                  Boolean(e?.payload?.append)
                )
              : s.voiceMessages,
          voiceOrbState: 'speaking',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('conversation.followup_started', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { expiresAt?: string };
        }>((event as MessageEvent).data);
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceOrbState: 'followup',
          voiceTranscript: '',
          voiceFollowupExpiresAt:
            typeof e?.payload?.expiresAt === 'string'
              ? e.payload.expiresAt
              : s.voiceFollowupExpiresAt,
          voiceError: ''
        }));
      });

      eventSource.addEventListener('tts.started', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { state?: string };
        }>((event as MessageEvent).data);
        logger.sse(event.type, e?.payload?.state);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceOrbState: 'speaking',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('tts.completed', (event) => {
        const e = parseDisplayEvent<{ conversationId?: string }>(
          (event as MessageEvent).data
        );
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceOrbState: 'followup',
          voiceError: ''
        }));
      });

      eventSource.addEventListener('tts.stopped', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { reason?: string; state?: string };
        }>((event as MessageEvent).data);
        const reason = e?.payload?.reason;
        const policyMessage =
          reason === 'sensitive_output_visual_only' ||
          reason === 'sensitive_output_requires_confirmation'
            ? safeVoiceError(reason)
            : '';
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceOrbState: policyMessage ? 'followup' : 'listening',
          voiceError:
            reason === 'barge_in'
              ? 'Speech stopped. Listening for your follow-up.'
              : policyMessage
        }));
      });

      eventSource.addEventListener('tts.failed', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { reason?: string };
        }>((event as MessageEvent).data);
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceOrbState: 'error',
          voiceError: safeVoiceError(e?.payload?.reason || 'tts_failure')
        }));
      });

      eventSource.addEventListener('conversation.ended', (event) => {
        const e = parseDisplayEvent<{
          conversationId?: string;
          payload?: { reason?: string };
        }>((event as MessageEvent).data);
        const error = safeVoiceError(e?.payload?.reason);
        logger.sse(event.type);
        update((s) => ({
          ...s,
          voiceConversationId:
            eventConversationID(e ?? {}) || s.voiceConversationId,
          voiceOrbState: error ? 'error' : 'idle',
          voiceError: error
        }));
        if (voiceEndedTimeout) {
          window.clearTimeout(voiceEndedTimeout);
        }
        voiceEndedTimeout = window.setTimeout(() => {
          update((s) => {
            if (s.voiceOrbState === 'idle' || s.voiceOrbState === 'error') {
              return {
                ...s,
                voiceConversationId: '',
                voiceAgentId: '',
                voiceMessages: [],
                voiceTranscript: '',
                assistantSpeech: '',
                voiceError: '',
                voiceFollowupExpiresAt: ''
              };
            }
            return s;
          });
        }, 4000);
      });

      eventSource.addEventListener('hub.connected', (event) => {
        logger.sse(event.type);
        if (hasConnected) {
          markConnected();
        }
      });

      // Start polling
      if (!pollingTimer) {
        pollingTimer = window.setInterval(async () => {
          await pollStatus(fetcher);
        }, 10000);
      }
    },
    disconnect: () => {
      isMounted = false;
      eventSource?.close();
      eventSource = undefined;

      if (pollingTimer) {
        window.clearInterval(pollingTimer);
        pollingTimer = undefined;
      }

      if (focusTimer) {
        window.clearTimeout(focusTimer);
        focusTimer = undefined;
      }

      for (const timer of notificationTimers) {
        window.clearTimeout(timer);
      }
      notificationTimers = [];

      if (voiceEndedTimeout) {
        window.clearTimeout(voiceEndedTimeout);
        voiceEndedTimeout = undefined;
      }
    },
    retryDashboard: async (fetcher: typeof fetch = window.fetch) => {
      let isRetrying = false;
      update((s) => {
        if (s.retrying) return s;
        isRetrying = true;
        return { ...s, retrying: true };
      });
      if (!isRetrying) return;

      try {
        const fresh = await getDashboard(fetcher);
        update((s) => ({
          ...s,
          dashboard: fresh,
          retrying: false
        }));
        markConnected();
        return fresh;
      } catch (err) {
        update((s) => ({ ...s, retrying: false }));
        const state = hasConnected ? 'reconnecting' : 'offline';
        markIssue(state, {
          code: 'hub_unreachable',
          severity: 'error',
          title: 'Hub not reachable',
          message: `Jute Dash cannot connect to the local hub.`,
          action: {
            label: 'Retry',
            target: 'retry'
          }
        });
        throw err;
      }
    },
    toggleVoiceMute: async (fetcher: typeof fetch = window.fetch) => {
      let currentMuted = false;
      let serviceStatus = 'not_configured';
      update((s) => {
        currentMuted = s.dashboard.voice.muted;
        serviceStatus = s.dashboard.voice.serviceStatus;
        return s;
      });

      if (serviceStatus !== 'ready') {
        throw new Error(
          'Voice is not configured yet. Add an STT provider before using microphone controls.'
        );
      }

      try {
        const voice = currentMuted
          ? await unmuteVoice(fetcher)
          : await muteVoice(fetcher);
        update((s) => ({
          ...s,
          dashboard: {
            ...s.dashboard,
            voice,
            connectionState: 'connected',
            stale: false,
            issue: undefined
          }
        }));
      } catch (err) {
        throw new Error(
          'Voice state could not be updated. Check that the hub is running, then try again.',
          { cause: err }
        );
      }
    },
    beginBrowserVoiceCapture: () => {
      update((s) => ({
        ...s,
        voiceOrbState: 'listening',
        voiceTranscript: '',
        voiceError: ''
      }));
    },
    submitBrowserVoiceAudio: async (
      recording: { audio: Blob; sampleRate: number; channels: number },
      fetcher: typeof fetch = window.fetch
    ) => {
      let voice: VoiceStatus = initialStub.voice;
      let conversationId = '';
      let agentId = '';
      update((s) => {
        voice = s.dashboard.voice;
        conversationId = s.voiceConversationId;
        agentId =
          s.voiceAgentId ||
          voice.preferredAgentId ||
          s.dashboard.agents.find((agent) => agent.enabled)?.id ||
          '';
        return {
          ...s,
          voiceAgentId: agentId,
          voiceOrbState: 'thinking',
          voiceTranscript: '',
          voiceError: ''
        };
      });
      await submitVoiceAudio(fetcher, recording.audio, {
        sampleRate: recording.sampleRate,
        channels: recording.channels,
        deviceProfileId: voice.deviceProfileId,
        deviceId: voice.deviceProfileId,
        conversationId,
        agentId
      });
    },
    submitBrowserWakeAudio: async (
      recording: { audio: Blob; sampleRate: number; channels: number },
      fetcher: typeof fetch = window.fetch
    ) => {
      let voice: VoiceStatus = initialStub.voice;
      update((s) => {
        voice = s.dashboard.voice;
        return s;
      });
      await submitVoiceAudio(fetcher, recording.audio, {
        sampleRate: recording.sampleRate,
        channels: recording.channels,
        deviceProfileId: voice.deviceProfileId,
        deviceId: voice.deviceProfileId,
        agentId: voice.preferredAgentId,
        requireWake: true
      });
    },
    failBrowserVoiceCapture: (reason: string) => {
      update((s) => ({
        ...s,
        voiceOrbState: 'error',
        voiceError: reason
      }));
    },
    cancelVoiceSession: async (fetcher: typeof fetch = window.fetch) => {
      try {
        await cancelVoice(fetcher);
        update((s) => ({
          ...s,
          voiceOrbState: 'idle',
          voiceConversationId: '',
          voiceAgentId: '',
          voiceMessages: [],
          voiceTranscript: '',
          assistantSpeech: '',
          voiceError: '',
          voiceFollowupExpiresAt: ''
        }));
      } catch (err) {
        console.error('Failed to cancel voice session:', err);
        throw err;
      }
    },
    refreshAfterMutation: async (fetcher: typeof fetch = window.fetch) => {
      try {
        const fresh = await getDashboard(fetcher);
        update((s) => ({
          ...s,
          dashboard: fresh
        }));
        markConnected();
        return fresh;
      } catch (err) {
        // Mutation succeeded but refresh failed — not critical.
        // Dashboard will catch up on next poll or SSE event.
        console.error('Failed to refresh dashboard after mutation:', err);
      }
    },
    updateDashboard: (fresh: DashboardData) => {
      update((s) => ({
        ...s,
        dashboard: fresh
      }));
    },
    updateLayout: (layout: WidgetLayout) => {
      update((s) => ({
        ...s,
        dashboard: {
          ...s.dashboard,
          layout
        }
      }));
    }
  };
}

export const hubStream = createHubStreamStore();
