<script lang="ts">
  import { Check, MapPin, RotateCcw } from "lucide-svelte";
  import {
    WidgetActionButton,
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

  type MonthDay = {
    key: string;
    label: number;
    ariaLabel: string;
    currentMonth: boolean;
    today: boolean;
    hasEvents: boolean;
  };

  export let data: CalendarData = {};
  export let stale = false;
  export let dispatch: (
    action: string,
    args?: Record<string, unknown>,
  ) => Promise<unknown> = async () => {};

  const weekdayLabels = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  const today = new Date();

  $: events = [...(data.events ?? [])].sort(
    (a, b) => Date.parse(a.start) - Date.parse(b.start),
  );
  $: ringingAlerts = data.ringing ?? [];
  $: primaryAlert = ringingAlerts[0];
  $: nextEvent = data.nextEvent ?? events[0];
  $: monthLabel = today.toLocaleDateString([], {
    month: "long",
    year: "numeric",
  });
  $: monthDays = buildMonthDays(today, events);

  function formatEventTime(event: Event | undefined) {
    if (!event) return "";
    const start = new Date(event.start);
    const end = new Date(event.end);
    if (event.allDay) {
      return "All day";
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

  function buildMonthDays(anchor: Date, calendarEvents: Event[]): MonthDay[] {
    const monthStart = new Date(anchor.getFullYear(), anchor.getMonth(), 1);
    const gridStart = new Date(monthStart);
    gridStart.setDate(monthStart.getDate() - monthStart.getDay());
    const eventDays = new Set(
      calendarEvents.map((event) => dayKey(new Date(event.start))),
    );

    return Array.from({ length: 42 }, (_, index) => {
      const date = new Date(gridStart);
      date.setDate(gridStart.getDate() + index);
      return {
        key: dayKey(date),
        label: date.getDate(),
        ariaLabel: date.toLocaleDateString([], {
          weekday: "long",
          month: "long",
          day: "numeric",
        }),
        currentMonth: date.getMonth() === anchor.getMonth(),
        today: dayKey(date) === dayKey(anchor),
        hasEvents: eventDays.has(dayKey(date)),
      };
    });
  }

  function dayKey(date: Date) {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const day = String(date.getDate()).padStart(2, "0");
    return `${year}-${month}-${day}`;
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
</script>

<WidgetStack {stale} gap="tight">
  <section class:has-next={!!nextEvent} class="calendar-shell">
    <section class="month-calendar" aria-label={monthLabel}>
      <div class="month-heading">
        <strong>{monthLabel}</strong>
        <span
          >{today.toLocaleDateString([], {
            weekday: "long",
            day: "numeric",
          })}</span
        >
      </div>
      <div class="weekday-grid" aria-hidden="true">
        {#each weekdayLabels as weekday}
          <span>{weekday}</span>
        {/each}
      </div>
      <div class="month-grid">
        {#each monthDays as day (day.key)}
          <span
            class:current-month={day.currentMonth}
            class:today={day.today}
            class:has-events={day.hasEvents}
            aria-label={day.ariaLabel}
          >
            {day.label}
          </span>
        {/each}
      </div>
    </section>

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
</WidgetStack>

<style>
  .calendar-shell {
    display: grid;
    flex: 1;
    gap: clamp(8px, 2.2cqmin, 12px);
    height: 100%;
    min-height: 0;
  }

  .calendar-shell.has-next {
    grid-template-columns: minmax(0, 1fr) minmax(144px, 0.72fr);
    align-items: stretch;
  }

  .month-calendar {
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr);
    gap: clamp(5px, 1.5cqmin, 8px);
    height: 100%;
    min-height: 0;
  }

  .month-heading,
  .next-event {
    min-width: 0;
  }

  .month-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 10px;
  }

  .month-heading strong {
    overflow: hidden;
    color: var(--foreground);
    font-size: clamp(0.95rem, 4.8cqmin, 1.32rem);
    font-weight: 760;
    line-height: 1.1;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .month-heading span,
  .weekday-grid span {
    color: var(--muted);
    font-size: clamp(0.58rem, 2.8cqmin, 0.72rem);
    font-weight: 700;
    text-transform: uppercase;
  }

  .weekday-grid,
  .month-grid {
    display: grid;
    grid-template-columns: repeat(7, minmax(0, 1fr));
  }

  .weekday-grid {
    gap: 2px;
  }

  .weekday-grid span {
    text-align: center;
  }

  .month-grid {
    gap: clamp(2px, 0.9cqmin, 5px);
    grid-template-rows: repeat(6, minmax(0, 1fr));
    min-height: 0;
  }

  .month-grid span {
    position: relative;
    display: grid;
    place-items: center;
    min-width: 0;
    min-height: 0;
    border-radius: 6px;
    color: color-mix(in srgb, var(--muted) 54%, transparent);
    font-size: clamp(0.66rem, 3.4cqmin, 0.86rem);
    font-weight: 650;
    line-height: 1;
  }

  .month-grid span.current-month {
    color: var(--foreground);
  }

  .month-grid span.today {
    background: var(--surface-strong);
    color: var(--active);
    font-weight: 800;
    box-shadow: inset 0 0 0 1px var(--border-strong);
  }

  .month-grid span.has-events::after {
    position: absolute;
    bottom: clamp(1px, 0.6cqmin, 3px);
    width: 4px;
    height: 4px;
    border-radius: 999px;
    background: currentColor;
    content: "";
  }

  .next-event {
    display: grid;
    align-content: center;
    grid-template-columns: 1fr;
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

  .next-copy {
    display: grid;
    min-width: 0;
    gap: 3px;
  }

  .next-copy strong {
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

  @container (max-width: 360px) {
    .calendar-shell.has-next {
      grid-template-columns: 1fr;
    }

    .next-event {
      display: none;
    }

    .alert-actions {
      width: 100%;
    }

    .alert-actions :global(button) {
      flex: 1;
    }
  }
</style>
