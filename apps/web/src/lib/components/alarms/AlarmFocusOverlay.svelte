<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import {
    AlarmClock,
    CalendarDays,
    Check,
    RotateCcw,
    Timer
  } from 'lucide-svelte';
  import StardustCanvas from '$lib/components/chat/StardustCanvas.svelte';
  import Button from '$lib/components/ui/Button.svelte';
  import {
    alertFocusCommand,
    deriveAlertFocusState,
    formatAlertFocusTime,
    type AlertFocusItem
  } from '$lib/alerts/alertFocus';
  import { playNotificationSound } from '$lib/alerts/notificationSound';
  import { createDisplayWidgetDispatcher } from '$lib/widgetActions';
  import type { DashboardData } from '$lib/types';

  export let data: DashboardData;

  let now = Date.now();
  let timer: number | undefined;
  let playedSoundKey = '';

  $: alertState = deriveAlertFocusState(data, now);
  $: primary = alertState.primary;
  $: syncSound(primary);

  onMount(() => {
    timer = window.setInterval(() => {
      now = Date.now();
    }, 1000);
  });

  onDestroy(() => {
    if (timer) {
      window.clearInterval(timer);
    }
  });

  function syncSound(item: AlertFocusItem | undefined) {
    if (!item) {
      playedSoundKey = '';
      return;
    }
    if (item.playbackKey === playedSoundKey) return;
    playedSoundKey = item.playbackKey;
    playNotificationSound(item.sound);
  }

  async function invoke(item: AlertFocusItem, action: 'snooze' | 'dismiss') {
    const dispatch = createDisplayWidgetDispatcher(fetch, item.widgetId);
    const command = alertFocusCommand(item, action);
    await dispatch(command.actionId, command.arguments);
  }
</script>

{#if primary}
  <section class="alarm-focus" aria-live="assertive" aria-label="Alert due">
    <StardustCanvas state="streaming" />
    <div class="alarm-focus-content">
      <div class="alarm-kind">
        {#if primary.kind === 'timer'}
          <Timer size={26} />
          <span>{primary.kindLabel}</span>
        {:else if primary.kind === 'calendar-event'}
          <CalendarDays size={26} />
          <span>{primary.kindLabel}</span>
        {:else}
          <AlarmClock size={26} />
          <span>{primary.kindLabel}</span>
        {/if}
      </div>
      <div class="alarm-time">{formatAlertFocusTime(primary)}</div>
      <h2>{primary.label}</h2>
      {#if alertState.ringingCount > 1}
        <p>{alertState.ringingCount} active alerts</p>
      {/if}
      <div class="alarm-actions">
        <Button variant="secondary" on:click={() => invoke(primary, 'snooze')}>
          <RotateCcw size={20} />
          <span>Snooze</span>
        </Button>
        <Button variant="outline" on:click={() => invoke(primary, 'dismiss')}>
          <Check size={20} />
          <span>Dismiss</span>
        </Button>
      </div>
    </div>
  </section>
{/if}

<style>
  .alarm-focus {
    position: fixed;
    inset: 0;
    z-index: 70;
    display: grid;
    place-items: center;
    overflow: hidden;
    background:
      radial-gradient(
        circle at 50% 42%,
        rgba(255, 255, 255, 0.1),
        transparent 32%
      ),
      color-mix(in srgb, var(--background) 72%, black);
    color: var(--foreground);
  }

  .alarm-focus :global(.stardust-canvas) {
    opacity: 0.92;
  }

  .alarm-focus-content {
    position: relative;
    z-index: 2;
    display: grid;
    justify-items: center;
    gap: 18px;
    width: min(560px, calc(100vw - 32px));
    text-align: center;
  }

  .alarm-kind {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    color: var(--muted);
    font-size: 0.95rem;
    text-transform: uppercase;
    letter-spacing: 0;
  }

  .alarm-time {
    font-size: clamp(5rem, 20vw, 11rem);
    font-weight: 780;
    line-height: 0.9;
    font-variant-numeric: tabular-nums;
  }

  h2 {
    margin: 0;
    max-width: 100%;
    overflow-wrap: anywhere;
    font-size: clamp(1.5rem, 6vw, 3rem);
    line-height: 1.05;
  }

  p {
    margin: 0;
    color: var(--muted);
  }

  .alarm-actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 12px;
    margin-top: 10px;
  }

  .alarm-actions :global(button) {
    min-width: 150px;
    min-height: 52px;
  }
</style>
