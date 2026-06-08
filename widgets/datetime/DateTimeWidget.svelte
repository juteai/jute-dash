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
