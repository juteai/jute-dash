<script lang="ts">
  export let state:
    | 'idle'
    | 'listening'
    | 'thinking'
    | 'speaking'
    | 'followup' = 'idle';
</script>

<div
  class="orb-container {state}"
  aria-label="Voice conversation orb - {state} mode"
>
  {#if state !== 'idle'}
    <div class="orb-glow"></div>
  {/if}

  <div class="orb-inner">
    {#if state === 'thinking'}
      <div class="ring ring-1"></div>
      <div class="ring ring-2"></div>
    {:else if state === 'listening' || state === 'followup'}
      <div class="pulse-ring pulse-1"></div>
      <div class="pulse-ring pulse-2"></div>
    {:else if state === 'speaking'}
      <div class="wave-ring wave-1"></div>
      <div class="wave-ring wave-2"></div>
      <div class="wave-ring wave-3"></div>
    {/if}
    <div class="orb-core"></div>
  </div>
</div>

<style>
  .orb-container {
    position: relative;
    width: 96px;
    height: 96px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .orb-glow {
    position: absolute;
    width: 140%;
    height: 140%;
    border-radius: 50%;
    filter: blur(24px);
    opacity: 0.6;
    z-index: 0;
    transition: all 0.8s ease;
  }

  .orb-inner {
    position: relative;
    width: 100%;
    height: 100%;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1;
  }

  .orb-core {
    position: absolute;
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background: radial-gradient(
      circle at 30% 30%,
      var(--core-color-1, #ffffff),
      var(--core-color-2, #888888)
    );
    box-shadow:
      inset 0 2px 4px rgba(255, 255, 255, 0.4),
      0 4px 12px rgba(0, 0, 0, 0.3);
    transition: all 0.5s ease;
    z-index: 5;
  }

  .orb-container.idle {
    transform: scale(0.6);
    opacity: 0.3;
  }
  .orb-container.idle .orb-core {
    --core-color-1: rgba(120, 120, 120, 0.2);
    --core-color-2: rgba(30, 30, 30, 0.4);
    box-shadow: none;
    border: 1px solid rgba(255, 255, 255, 0.1);
  }

  .orb-container.listening {
    transform: scale(1.1);
  }
  .orb-container.listening .orb-glow {
    background: radial-gradient(
      circle,
      rgba(6, 182, 212, 0.4) 0%,
      transparent 70%
    );
  }
  .orb-container.listening .orb-core {
    --core-color-1: #e0f7fa;
    --core-color-2: #00acc1;
    box-shadow:
      0 0 20px rgba(6, 182, 212, 0.8),
      inset 0 2px 4px rgba(255, 255, 255, 0.6);
  }

  .orb-container.followup {
    transform: scale(1.1);
  }
  .orb-container.followup .orb-glow {
    background: radial-gradient(
      circle,
      rgba(234, 179, 8, 0.4) 0%,
      transparent 70%
    );
  }
  .orb-container.followup .orb-core {
    --core-color-1: #fef9c3;
    --core-color-2: #ca8a04;
    box-shadow:
      0 0 20px rgba(234, 179, 8, 0.8),
      inset 0 2px 4px rgba(255, 255, 255, 0.6);
  }

  .orb-container.thinking {
    transform: scale(1.15);
  }
  .orb-container.thinking .orb-glow {
    background: radial-gradient(
      circle,
      rgba(168, 85, 247, 0.4) 0%,
      rgba(6, 182, 212, 0.2) 50%,
      transparent 70%
    );
  }
  .orb-container.thinking .orb-core {
    --core-color-1: #f3e8ff;
    --core-color-2: #7e22ce;
    box-shadow:
      0 0 25px rgba(168, 85, 247, 0.8),
      inset 0 2px 4px rgba(255, 255, 255, 0.6);
  }

  .orb-container.speaking {
    transform: scale(1.1);
  }
  .orb-container.speaking .orb-glow {
    background: radial-gradient(
      circle,
      rgba(16, 185, 129, 0.4) 0%,
      transparent 70%
    );
  }
  .orb-container.speaking .orb-core {
    --core-color-1: #d1fae5;
    --core-color-2: #059669;
    box-shadow:
      0 0 20px rgba(16, 185, 129, 0.8),
      inset 0 2px 4px rgba(255, 255, 255, 0.6);
  }

  .pulse-ring {
    position: absolute;
    border-radius: 50%;
    border: 2px solid currentColor;
    opacity: 0.8;
    z-index: 2;
  }
  .listening .pulse-ring {
    color: rgba(6, 182, 212, 0.6);
  }
  .followup .pulse-ring {
    color: rgba(234, 179, 8, 0.6);
  }
  .pulse-1 {
    width: 64px;
    height: 64px;
    animation: orb-pulse 2.2s cubic-bezier(0.16, 1, 0.3, 1) infinite;
  }
  .pulse-2 {
    width: 80px;
    height: 80px;
    animation: orb-pulse 2.2s cubic-bezier(0.16, 1, 0.3, 1) infinite;
    animation-delay: 0.7s;
  }

  .ring {
    position: absolute;
    border-radius: 50%;
    border: 3px solid transparent;
    z-index: 3;
    backdrop-filter: blur(2px);
  }
  .ring-1 {
    width: 72px;
    height: 72px;
    border-top-color: #8b5cf6;
    border-bottom-color: #06b6d4;
    animation: spin-clockwise 2s linear infinite;
  }
  .ring-2 {
    width: 88px;
    height: 88px;
    border-left-color: #3b82f6;
    border-right-color: #ec4899;
    animation: spin-counter 3s linear infinite;
  }

  .wave-ring {
    position: absolute;
    border-radius: 50%;
    border: 1px solid rgba(16, 185, 129, 0.4);
    background: radial-gradient(
      circle,
      rgba(16, 185, 129, 0.05) 0%,
      transparent 80%
    );
    z-index: 2;
  }
  .wave-1 {
    width: 60px;
    height: 60px;
    animation: voice-wave 1.5s ease-in-out infinite alternate;
  }
  .wave-2 {
    width: 76px;
    height: 76px;
    animation: voice-wave 1.8s ease-in-out infinite alternate;
    animation-delay: 0.3s;
  }
  .wave-3 {
    width: 92px;
    height: 92px;
    animation: voice-wave 1.2s ease-in-out infinite alternate;
    animation-delay: 0.6s;
  }

  @keyframes orb-pulse {
    0% {
      transform: scale(0.9);
      opacity: 0.8;
    }
    100% {
      transform: scale(1.6);
      opacity: 0;
    }
  }

  @keyframes spin-clockwise {
    0% {
      transform: rotate(0deg);
    }
    100% {
      transform: rotate(360deg);
    }
  }

  @keyframes spin-counter {
    0% {
      transform: rotate(360deg);
    }
    100% {
      transform: rotate(0deg);
    }
  }

  @keyframes voice-wave {
    0% {
      transform: scale(0.95);
      border-color: rgba(16, 185, 129, 0.2);
    }
    100% {
      transform: scale(1.15);
      border-color: rgba(16, 185, 129, 0.7);
      box-shadow: 0 0 10px rgba(16, 185, 129, 0.2);
    }
  }
</style>
