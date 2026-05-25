<script lang="ts">
  import { onMount } from 'svelte';
  import { CalendarDays, Clock3 } from 'lucide-svelte';
  import type { HomeConfig } from '$lib/types';

  export let home: HomeConfig;
  export let stale = false;

  let now = new Date();

  onMount(() => {
    const timer = window.setInterval(() => {
      now = new Date();
    }, 1000);

    return () => window.clearInterval(timer);
  });

  $: time = new Intl.DateTimeFormat(home.locale, {
    hour: '2-digit',
    minute: '2-digit',
    timeZone: home.timezone
  }).format(now);

  $: date = new Intl.DateTimeFormat(home.locale, {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
    timeZone: home.timezone
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
  <div class="date-time-zone">{home.timezone}</div>
  {#if stale}
    <div class="widget-state-note">Showing last hub state</div>
  {/if}
</div>
