<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';
  import { AlertTriangle, Mic, MicOff, Radio, X } from 'lucide-svelte';
  import ConversationOrb from '$lib/components/chat/ConversationOrb.svelte';
  import type { VoiceConversationMessage, VoiceStatus } from '$lib/types';

  export let voice: VoiceStatus;
  export let voiceOrbState:
    | 'listening'
    | 'followup'
    | 'thinking'
    | 'speaking'
    | 'idle'
    | 'error';
  export let voiceMessages: VoiceConversationMessage[] = [];
  export let voiceTranscript = '';
  export let assistantSpeech = '';
  export let voiceError = '';
  export let followupExpiresAt = '';
  export let showConversationText = false;

  const dispatch = createEventDispatcher<{
    toggleMute: void;
    cancel: void;
  }>();

  $: activeMessages = showConversationText
    ? voiceMessages.filter((message) => message.text.trim())
    : [];
  $: partialTranscript =
    showConversationText &&
    voiceTranscript &&
    !activeMessages.some(
      (message) => message.role === 'user' && message.text === voiceTranscript
    )
      ? voiceTranscript
      : '';
  $: fallbackAssistant =
    showConversationText &&
    assistantSpeech &&
    !activeMessages.some(
      (message) =>
        message.role === 'assistant' && message.text === assistantSpeech
    )
      ? assistantSpeech
      : '';
  $: safeVoiceError = voiceError ? safeInlineVoiceError(voiceError) : '';
  $: safeDeviceProfileLabel = safeInlineVoiceMetadata(
    voice.deviceProfileId,
    'default display'
  );
  $: safeSTTProviderLabel = safeInlineVoiceMetadata(
    voice.sttProviderId,
    'No STT provider'
  );
  $: headline = voice.muted
    ? 'Voice muted'
    : voiceOrbState === 'followup'
      ? 'Follow-up listening'
      : voiceOrbState === 'thinking'
        ? 'Agent thinking'
        : voiceOrbState === 'speaking'
          ? 'Speaking'
          : voiceOrbState === 'error'
            ? 'Voice needs attention'
            : voiceOrbState === 'listening'
              ? 'Listening'
              : 'Voice';
  $: statusMeta = safeVoiceError
    ? 'Recoverable error'
    : followupExpiresAt && voiceOrbState === 'followup'
      ? 'Follow-up window active'
      : voice.serviceStatus === 'ready'
        ? voice.state.replace(/_/g, ' ')
        : voice.serviceStatus.replace(/_/g, ' ');

  function safeInlineVoiceError(error: string) {
    const trimmed = error.trim();
    if (!trimmed) {
      return '';
    }
    if (
      /\b(token|secret|credential|api[_ -]?key|bearer)\b/i.test(trimmed) ||
      /\b(?:https?|tcp|ws|wss):\/\//i.test(trimmed) ||
      /\bdial tcp\b/i.test(trimmed)
    ) {
      return 'Voice needs attention. Check provider settings.';
    }
    return trimmed;
  }

  function safeInlineVoiceMetadata(value: string, fallback: string) {
    const trimmed = value.trim();
    if (!trimmed) {
      return fallback;
    }
    if (containsSensitiveVoiceDetail(trimmed)) {
      return fallback;
    }
    return trimmed;
  }

  function containsSensitiveVoiceDetail(value: string) {
    return (
      /\b(token|secret|credential|api[_ -]?key|bearer)\b/i.test(value) ||
      /\b(?:https?|tcp|ws|wss):\/\//i.test(value) ||
      /\bdial tcp\b/i.test(value)
    );
  }
</script>

<div class="voice-overlay-container" transition:fade={{ duration: 300 }}>
  <section
    class:voice-card--error={voiceOrbState === 'error'}
    class="voice-card"
  >
    <div class="voice-header">
      <ConversationOrb
        state={voiceOrbState === 'error' ? 'idle' : voiceOrbState}
      />
      <div class="voice-title">
        <strong>{headline}</strong>
        <span>{statusMeta}</span>
      </div>
      <span class="voice-live-badge">
        <Radio size={14} />
        {voice.enabled ? 'Voice' : 'Disabled'}
      </span>
    </div>

    <div class="voice-content" aria-live="polite">
      {#if safeVoiceError}
        <div class="voice-error">
          <AlertTriangle size={18} />
          <span>{safeVoiceError}</span>
        </div>
      {/if}

      {#if activeMessages.length > 0}
        <div class="message-stack">
          {#each activeMessages as message (message.id)}
            <div
              class:assistant-bubble={message.role === 'assistant'}
              class:user-bubble={message.role === 'user'}
              class:system-bubble={message.role === 'system'}
              class="bubble"
            >
              <span class="bubble-label">
                {message.role === 'assistant'
                  ? 'Assistant'
                  : message.role === 'user'
                    ? 'You'
                    : 'Status'}
              </span>
              <p class="bubble-text">{message.text}</p>
            </div>
          {/each}
        </div>
      {/if}

      {#if partialTranscript}
        <div class="bubble user-bubble bubble--partial">
          <span class="bubble-label">You</span>
          <p class="bubble-text">{partialTranscript}</p>
        </div>
      {/if}

      {#if fallbackAssistant}
        <div class="bubble assistant-bubble">
          <span class="bubble-label">Assistant</span>
          <p class="bubble-text">{fallbackAssistant}</p>
        </div>
      {/if}

      {#if activeMessages.length === 0 && !partialTranscript && !fallbackAssistant && !safeVoiceError}
        <div class="status-tip">
          {#if voiceOrbState === 'listening'}
            <span class="status-pulse-dot cyan"></span> Listening...
          {:else if voiceOrbState === 'followup'}
            <span class="status-pulse-dot yellow"></span> Follow-up listening...
          {:else if voiceOrbState === 'thinking'}
            <span class="status-pulse-dot purple"></span> Thinking...
          {:else if voiceOrbState === 'speaking'}
            <span class="status-pulse-dot green"></span> Speaking...
          {/if}
        </div>
      {/if}
    </div>

    <div class="voice-footer">
      <div class="voice-service">
        <span>{safeDeviceProfileLabel}</span>
        <strong>{safeSTTProviderLabel}</strong>
      </div>

      <div class="voice-controls">
        <button
          type="button"
          class="control-btn mute-btn {voice.muted ? 'muted' : ''}"
          on:click={() => dispatch('toggleMute')}
          aria-label={voice.muted ? 'Unmute Microphone' : 'Mute Microphone'}
        >
          {#if voice.muted}
            <MicOff size={18} />
          {:else}
            <Mic size={18} />
          {/if}
        </button>

        <button
          type="button"
          class="control-btn cancel-btn"
          on:click={() => dispatch('cancel')}
          aria-label="Cancel Voice Session"
        >
          <X size={18} />
        </button>
      </div>
    </div>
  </section>
</div>

<style>
  .voice-overlay-container {
    position: fixed;
    bottom: 24px;
    left: 50%;
    transform: translateX(-50%);
    width: min(92vw, 680px);
    z-index: 100;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
  }

  .voice-card {
    background: color-mix(in srgb, var(--surface) 97%, #000 3%);
    backdrop-filter: blur(16px) saturate(180%);
    -webkit-backdrop-filter: blur(16px) saturate(180%);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 16px;
    box-shadow:
      0 20px 60px rgba(0, 0, 0, 0.28),
      0 0 0 1px color-mix(in srgb, var(--border) 70%, transparent);
    display: flex;
    flex-direction: column;
    gap: 14px;
    color: var(--text);
  }

  .voice-card--error {
    border-color: color-mix(in srgb, var(--danger) 60%, var(--border));
  }

  .voice-header {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 12px;
  }

  .voice-title {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .voice-title strong {
    font-size: 1rem;
    line-height: 1.2;
  }

  .voice-title span,
  .voice-service span {
    color: var(--muted);
    font-size: 0.78rem;
    overflow-wrap: anywhere;
  }

  .voice-live-badge {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    min-height: 32px;
    padding: 0 10px;
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--muted-strong);
    font-size: 0.78rem;
    white-space: nowrap;
  }

  .voice-content {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 72px;
    max-height: min(44vh, 340px);
    overflow-y: auto;
    justify-content: flex-end;
  }

  .message-stack {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .bubble {
    display: flex;
    flex-direction: column;
    padding: 10px 12px;
    border-radius: 8px;
    font-size: 14px;
    line-height: 1.5;
    max-width: min(100%, 560px);
    animation: fade-in-up 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  .user-bubble {
    background: color-mix(in srgb, var(--accent) 13%, var(--surface-muted));
    border-left: 3px solid var(--accent);
    align-self: flex-start;
  }

  .assistant-bubble {
    background: var(--surface-muted);
    border-left: 3px solid var(--success);
    align-self: flex-end;
  }

  .system-bubble {
    background: var(--surface-muted);
    border-left: 3px solid var(--warning);
    align-self: center;
  }

  .bubble--partial {
    opacity: 0.76;
  }

  .bubble-label {
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    opacity: 0.6;
    margin-bottom: 4px;
  }

  .bubble-text {
    margin: 0;
    font-weight: 500;
    color: var(--text);
    overflow-wrap: anywhere;
  }

  .status-tip {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    font-size: 13px;
    color: var(--muted);
    font-weight: 500;
  }

  .voice-error {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    border-radius: 8px;
    background: color-mix(in srgb, var(--danger) 12%, var(--surface));
    color: var(--danger);
    font-size: 0.9rem;
    overflow-wrap: anywhere;
  }

  .status-pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    animation: dot-pulse 1.5s ease-in-out infinite;
  }

  .status-pulse-dot.cyan {
    background-color: #06b6d4;
    box-shadow: 0 0 8px #06b6d4;
  }

  .status-pulse-dot.yellow {
    background-color: #eab308;
    box-shadow: 0 0 8px #eab308;
  }

  .status-pulse-dot.purple {
    background-color: #a855f7;
    box-shadow: 0 0 8px #a855f7;
  }

  .status-pulse-dot.green {
    background-color: #10b981;
    box-shadow: 0 0 8px #10b981;
  }

  .voice-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border-top: 1px solid var(--border);
    padding-top: 12px;
  }

  .voice-service {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .voice-service strong {
    font-size: 0.85rem;
    overflow-wrap: anywhere;
  }

  .voice-controls {
    display: flex;
    gap: 8px;
  }

  .control-btn {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    border: 1px solid var(--border);
    background: var(--surface-muted);
    color: var(--text);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .control-btn:hover {
    background: var(--surface);
    transform: scale(1.05);
  }

  .mute-btn.muted {
    background: color-mix(in srgb, var(--danger) 16%, var(--surface));
    border-color: color-mix(in srgb, var(--danger) 52%, var(--border));
    color: var(--danger);
  }

  .mute-btn.muted:hover {
    background: color-mix(in srgb, var(--danger) 22%, var(--surface));
  }

  @media (min-width: 1100px) {
    .voice-overlay-container {
      right: 24px;
      left: auto;
      bottom: 24px;
      transform: none;
      width: min(34vw, 520px);
    }
  }

  @media (max-width: 640px) {
    .voice-overlay-container {
      left: 12px;
      right: 12px;
      bottom: 12px;
      width: auto;
      transform: none;
    }

    .voice-card {
      padding: 14px;
    }

    .voice-header {
      grid-template-columns: auto minmax(0, 1fr);
    }

    .voice-live-badge {
      grid-column: 2;
      justify-self: start;
    }

    .voice-footer {
      align-items: flex-end;
    }
  }

  @keyframes fade-in-up {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  @keyframes dot-pulse {
    0%,
    100% {
      opacity: 1;
      transform: scale(1);
    }
    50% {
      opacity: 0.4;
      transform: scale(0.85);
    }
  }
</style>
