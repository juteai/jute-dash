<script lang="ts">
  import { onMount } from 'svelte';
  import { CalendarDays, Clock3 } from 'lucide-svelte';

  export let settings: { timezone: string; locale: string } = {
    timezone: 'UTC',
    locale: 'en'
  };
  export let stale = false;

  let now = new Date();

  onMount(() => {
    const timer = window.setInterval(() => {
      now = new Date();
    }, 1000);

    return () => window.clearInterval(timer);
  });

  $: time = new Intl.DateTimeFormat(settings.locale || 'en', {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: settings.timezone || 'UTC'
  }).format(now);

  $: date = new Intl.DateTimeFormat(settings.locale || 'en', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    timeZone: settings.timezone || 'UTC'
  }).format(now);
</script>

<div class:widget-stale={stale} class="date-time-widget">
  <div class="date-time-clock">
    <Clock3 size={24} aria-hidden="true" />
    <span>{time}</span>
  </div>
  <div class="date-time-date">
    <CalendarDays size={18} aria-hidden="true" />
    <span>{date}</span>
  </div>
  <div class="date-time-zone">{settings.timezone || 'UTC'}</div>
  {#if stale}
    <div class="widget-state-note">Showing last hub state</div>
  {/if}
</div>

<style>
  .date-time-widget {
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
    min-height: 100%;
    gap: 8px;
  }

  .date-time-clock {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: var(--widget-display-size);
    font-weight: 780;
    line-height: 1;
  }

  .date-time-clock :global(svg) {
    width: clamp(14px, 8cqmin, 40px);
    height: auto;
  }

  .date-time-date {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--muted-strong);
    font-size: var(--widget-value-size);
    font-weight: 680;
  }

  .date-time-zone {
    color: var(--muted);
    font-size: var(--widget-label-size);
    font-weight: 650;
  }

  :global(.widget-frame--wide) .date-time-widget {
    justify-content: center;
  }

  :global(.widget-frame--wide) .date-time-date,
  :global(.widget-frame--wide) .date-time-zone {
    display: none;
  }

  .widget-stale {
    opacity: 0.72;
  }

  .widget-state-note {
    color: var(--warning);
    font-size: 0.78rem;
    font-weight: 720;
  }

  @media (max-width: 640px) {
    .date-time-clock :global(svg) {
      width: 28px;
    }
  }
</style>
