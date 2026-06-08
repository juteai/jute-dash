<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fade } from 'svelte/transition';
  import { Mic, MicOff, X } from 'lucide-svelte';
  import ConversationOrb from '$lib/components/chat/ConversationOrb.svelte';
  import type { VoiceStatus } from '$lib/types';

  export let voice: VoiceStatus;
  export let voiceOrbState:
    | 'listening'
    | 'followup'
    | 'thinking'
    | 'speaking'
    | 'idle';
  export let voiceTranscript = '';
  export let assistantSpeech = '';

  const dispatch = createEventDispatcher<{
    toggleMute: void;
    cancel: void;
  }>();
</script>

<div class="voice-overlay-container" transition:fade={{ duration: 300 }}>
  <div class="voice-card">
    <div class="voice-content">
      {#if voiceTranscript}
        <div class="bubble user-bubble">
          <span class="bubble-label">You</span>
          <p class="bubble-text">{voiceTranscript}</p>
        </div>
      {/if}

      {#if assistantSpeech}
        <div class="bubble assistant-bubble">
          <span class="bubble-label">Assistant</span>
          <p class="bubble-text">{assistantSpeech}</p>
        </div>
      {/if}

      {#if !voiceTranscript && !assistantSpeech}
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
      <ConversationOrb state={voiceOrbState} />

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
  </div>
</div>

<style>
  .voice-overlay-container {
    position: fixed;
    bottom: 24px;
    left: 50%;
    transform: translateX(-50%);
    width: 90%;
    max-width: 480px;
    z-index: 100;
    font-family: 'Outfit', 'Inter', system-ui, sans-serif;
  }

  .voice-card {
    background: rgba(18, 18, 18, 0.75);
    backdrop-filter: blur(16px) saturate(180%);
    -webkit-backdrop-filter: blur(16px) saturate(180%);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 24px;
    padding: 20px;
    box-shadow:
      0 12px 40px rgba(0, 0, 0, 0.5),
      0 0 0 1px rgba(255, 255, 255, 0.05);
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  :global([data-theme='light']) .voice-card {
    background: rgba(255, 255, 255, 0.75);
    border: 1px solid rgba(0, 0, 0, 0.08);
    box-shadow:
      0 12px 40px rgba(0, 0, 0, 0.15),
      0 0 0 1px rgba(0, 0, 0, 0.03);
  }

  .voice-content {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-height: 50px;
    justify-content: center;
  }

  .bubble {
    display: flex;
    flex-direction: column;
    padding: 12px 16px;
    border-radius: 16px;
    font-size: 14px;
    line-height: 1.5;
    max-width: 100%;
    animation: fade-in-up 0.3s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  }

  .user-bubble {
    background: rgba(6, 182, 212, 0.12);
    border-left: 3px solid #06b6d4;
    align-self: flex-start;
  }

  :global([data-theme='light']) .user-bubble {
    background: rgba(6, 182, 212, 0.08);
  }

  .assistant-bubble {
    background: rgba(255, 255, 255, 0.06);
    border-left: 3px solid #a855f7;
    align-self: flex-start;
  }

  :global([data-theme='light']) .assistant-bubble {
    background: rgba(0, 0, 0, 0.03);
    border-left: 3px solid #7e22ce;
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
    color: #ffffff;
  }

  :global([data-theme='light']) .bubble-text {
    color: #111111;
  }

  .status-tip {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    font-size: 13px;
    color: rgba(255, 255, 255, 0.7);
    font-weight: 500;
  }

  :global([data-theme='light']) .status-tip {
    color: rgba(0, 0, 0, 0.7);
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
    border-top: 1px solid rgba(255, 255, 255, 0.06);
    padding-top: 12px;
  }

  :global([data-theme='light']) .voice-footer {
    border-top: 1px solid rgba(0, 0, 0, 0.06);
  }

  .voice-controls {
    display: flex;
    gap: 8px;
  }

  .control-btn {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    border: 1px solid rgba(255, 255, 255, 0.08);
    background: rgba(255, 255, 255, 0.05);
    color: rgba(255, 255, 255, 0.8);
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  :global([data-theme='light']) .control-btn {
    border: 1px solid rgba(0, 0, 0, 0.08);
    background: rgba(0, 0, 0, 0.03);
    color: rgba(0, 0, 0, 0.8);
  }

  .control-btn:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #ffffff;
    transform: scale(1.05);
  }

  :global([data-theme='light']) .control-btn:hover {
    background: rgba(0, 0, 0, 0.06);
    color: #000000;
  }

  .mute-btn.muted {
    background: rgba(239, 68, 68, 0.2);
    border-color: rgba(239, 68, 68, 0.4);
    color: #ef4444;
  }

  .mute-btn.muted:hover {
    background: rgba(239, 68, 68, 0.3);
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
