<script lang="ts">
  import {
    Bell,
    CalendarDays,
    Check,
    Clock3,
    MapPin,
    RotateCcw,
    Volume2,
  } from "lucide-svelte";
  import {
    WidgetActionButton,
    WidgetBadge,
    WidgetEmptyState,
    WidgetList,
    WidgetListItem,
    WidgetMeta,
    WidgetStack,
  } from "$lib/components/widget-content";

  type Event = {
    id: string;
    title: string;
    calendar: string;
    start: string;
    end: string;
    allDay?: boolean;
    location?: string;
  };

  type EventAlert = {
    id: string;
    label: string;
    dueAt: string;
    eventStart: string;
    eventEnd: string;
    calendar: string;
    ringing?: boolean;
    defaultSnoozeMins?: number;
    event?: Event;
  };

  type CalendarData = {
    events?: Event[];
    nextEvent?: Event | null;
    alerts?: EventAlert[];
    ringing?: EventAlert[];
    alertLeadMinutes?: number;
    defaultSnoozeMins?: number;
    notificationSound?: string;
    supportedSounds?: string[];
    source?: string;
  };

  export let data: CalendarData = {};
  export let stale = false;
  export let dispatch: (
    action: string,
    args?: Record<string, unknown>,
  ) => Promise<unknown> = async () => {};

  const leadOptions = [0, 5, 10, 30];

  $: events = [...(data.events ?? [])].sort(
    (a, b) => Date.parse(a.start) - Date.parse(b.start),
  );
  $: ringingAlerts = data.ringing ?? [];
  $: primaryAlert = ringingAlerts[0];
  $: nextEvent = data.nextEvent ?? events[0];
  $: alertLead = Number(data.alertLeadMinutes ?? 10);
  $: sound = data.notificationSound ?? "chime";
  $: sounds = data.supportedSounds ?? [
    "chime",
    "bell",
    "pulse",
    "soft",
    "none",
  ];

  function formatEventTime(event: Event | undefined) {
    if (!event) return "";
    const start = new Date(event.start);
    const end = new Date(event.end);
    if (event.allDay) {
      return start.toLocaleDateString([], {
        weekday: "short",
        month: "short",
        day: "numeric",
      });
    }
    return `${start.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    })} - ${end.toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    })}`;
  }

  function formatRelativeDay(event: Event | undefined) {
    if (!event) return "";
    const start = new Date(event.start);
    const today = new Date();
    const startDay = new Date(
      start.getFullYear(),
      start.getMonth(),
      start.getDate(),
    ).getTime();
    const todayDay = new Date(
      today.getFullYear(),
      today.getMonth(),
      today.getDate(),
    ).getTime();
    const diffDays = Math.round((startDay - todayDay) / 86400000);
    if (diffDays === 0) return "Today";
    if (diffDays === 1) return "Tomorrow";
    return start.toLocaleDateString([], {
      weekday: "short",
      month: "short",
      day: "numeric",
    });
  }

  function snooze(alert: EventAlert) {
    return dispatch("snooze_event", {
      id: alert.id,
      minutes: alert.defaultSnoozeMins ?? data.defaultSnoozeMins ?? 9,
    });
  }

  function dismiss(alert: EventAlert) {
    return dispatch("dismiss_event", { id: alert.id });
  }

  function setSound(nextSound: string) {
    return dispatch("set_event_notification_sound", { sound: nextSound });
  }
</script>

<WidgetStack {stale} gap="loose">
  <section class="calendar-head">
    <div class="calendar-section-header">
      <div class="calendar-section-title">
        <CalendarDays size={14} />
        <h3>Calendar</h3>
      </div>
      {#if ringingAlerts.length > 0}
        <WidgetBadge tone="warning" pulse>
          {ringingAlerts.length} due
        </WidgetBadge>
      {:else}
        <WidgetBadge>{alertLead}m alerts</WidgetBadge>
      {/if}
    </div>

    {#if nextEvent}
      <article class:ringing={!!primaryAlert} class="next-event">
        <div class="date-pill">
          <span>{formatRelativeDay(nextEvent)}</span>
          <strong>{formatEventTime(nextEvent)}</strong>
        </div>
        <div class="next-copy">
          <strong>{primaryAlert?.label ?? nextEvent.title}</strong>
          <WidgetMeta>
            <span>{nextEvent.calendar}</span>
            {#if nextEvent.location}
              <span class="meta-with-icon">
                <MapPin size={12} />
                {nextEvent.location}
              </span>
            {/if}
          </WidgetMeta>
        </div>
        {#if primaryAlert}
          <div class="alert-actions">
            <WidgetActionButton
              label="Snooze event"
              on:click={() => snooze(primaryAlert)}
            >
              <RotateCcw size={15} />
            </WidgetActionButton>
            <WidgetActionButton
              label="Dismiss event"
              on:click={() => dismiss(primaryAlert)}
            >
              <Check size={15} />
            </WidgetActionButton>
          </div>
        {/if}
      </article>
    {/if}
  </section>

  <div class="lead-row" aria-label="Event alert lead time">
    <Bell size={14} />
    {#each leadOptions as minutes}
      <button
        type="button"
        class:active={alertLead === minutes}
        on:click={() => dispatch("set_event_alert_lead", { minutes })}
      >
        {minutes === 0 ? "At time" : `${minutes}m`}
      </button>
    {/each}
  </div>

  <label class="sound-row">
    <Volume2 size={14} />
    <select
      aria-label="Event notification sound"
      value={sound}
      on:change={(event) => setSound(event.currentTarget.value)}
    >
      {#each sounds as option}
        <option value={option}>{option}</option>
      {/each}
    </select>
  </label>

  {#if events.length === 0}
    <WidgetEmptyState message="No upcoming events">
      <CalendarDays slot="icon" size={32} />
    </WidgetEmptyState>
  {:else}
    <WidgetList gap="tight">
      {#each events.slice(0, 5) as event (event.id)}
        <WidgetListItem
          direction="row"
          class={event.id === nextEvent?.id ? "current" : ""}
        >
          <div class="event-time">
            <Clock3 size={14} />
            <span>{formatEventTime(event)}</span>
          </div>
          <div class="event-copy">
            <strong>{event.title}</strong>
            <WidgetMeta>
              <span>{formatRelativeDay(event)}</span>
              {#if event.location}
                <span>{event.location}</span>
              {/if}
            </WidgetMeta>
          </div>
        </WidgetListItem>
      {/each}
    </WidgetList>
  {/if}
</WidgetStack>

<style>
  .calendar-head {
    display: grid;
    gap: clamp(6px, 2cqmin, 10px);
  }

  .calendar-section-header,
  .calendar-section-title {
    display: flex;
    align-items: center;
  }

  .calendar-section-header {
    justify-content: space-between;
    gap: 8px;
    padding-bottom: 2px;
  }

  .calendar-section-title {
    min-width: 0;
    gap: 8px;
  }

  .calendar-section-title :global(svg) {
    flex: 0 0 auto;
    color: var(--muted);
  }

  h3 {
    margin: 0;
    overflow: hidden;
    color: var(--muted);
    font-size: var(--widget-label-size, 0.75rem);
    font-weight: 740;
    line-height: 1.1;
    text-overflow: ellipsis;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .next-event {
    display: grid;
    grid-template-columns: minmax(74px, auto) minmax(0, 1fr) auto;
    align-items: center;
    gap: clamp(8px, 2.2cqmin, 12px);
    padding: clamp(8px, 2.7cqmin, 13px);
    border: 1px solid var(--border);
    border-radius: 8px;
    background: color-mix(in srgb, var(--surface-muted) 48%, transparent);
  }

  .next-event.ringing {
    border-color: var(--warning);
    background: color-mix(in srgb, var(--warning) 13%, transparent);
  }

  .date-pill {
    display: grid;
    gap: 3px;
    min-width: 0;
    color: var(--muted);
    font-size: clamp(0.62rem, 3.5cqmin, 0.78rem);
    text-transform: uppercase;
  }

  .date-pill strong {
    color: var(--foreground);
    font-size: clamp(0.78rem, 4.6cqmin, 1.05rem);
    font-weight: 760;
    text-transform: none;
    white-space: nowrap;
  }

  .next-copy,
  .event-copy {
    display: grid;
    min-width: 0;
    gap: 3px;
  }

  .next-copy strong,
  .event-copy strong {
    overflow: hidden;
    color: var(--foreground);
    font-size: clamp(0.82rem, 4.6cqmin, 1rem);
    font-weight: 720;
    line-height: 1.18;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .meta-with-icon {
    display: inline-flex;
    align-items: center;
    gap: 3px;
  }

  .alert-actions {
    display: flex;
    gap: 5px;
  }

  .lead-row {
    display: grid;
    grid-template-columns: auto repeat(4, minmax(0, 1fr));
    align-items: center;
    gap: clamp(4px, 1cqmin, 7px);
    color: var(--muted);
  }

  .lead-row button {
    min-width: 0;
    min-height: clamp(28px, 5cqmin, 36px);
    border: 1px solid var(--border);
    border-radius: 6px;
    background: transparent;
    color: var(--foreground);
    font: inherit;
    font-size: clamp(0.64rem, 3.5cqmin, 0.78rem);
    cursor: pointer;
  }

  .lead-row button.active {
    border-color: var(--border-strong);
    background: var(--surface-strong);
    color: var(--active);
  }

  .sound-row {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: center;
    gap: clamp(4px, 1cqmin, 8px);
    color: var(--muted);
  }

  select {
    width: 100%;
    min-width: 0;
    min-height: clamp(30px, 5.2cqmin, 40px);
    border: 1px solid var(--border);
    border-radius: 6px;
    background: color-mix(in srgb, var(--surface-muted) 36%, transparent);
    color: var(--foreground);
    padding: 0 clamp(6px, 1.6cqmin, 10px);
    font: inherit;
    font-size: clamp(0.72rem, 4cqmin, 0.92rem);
  }

  .event-time {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    min-width: clamp(70px, 24cqmin, 96px);
    color: var(--muted);
    font-size: clamp(0.66rem, 3.7cqmin, 0.82rem);
    white-space: nowrap;
  }

  .event-copy {
    flex: 1;
  }

  :global(.widget-list-item.current) {
    border-color: var(--border-strong);
  }

  @container (max-width: 260px) {
    .next-event {
      grid-template-columns: 1fr;
    }

    .alert-actions {
      width: 100%;
    }

    .alert-actions :global(button) {
      flex: 1;
    }
  }
</style>
