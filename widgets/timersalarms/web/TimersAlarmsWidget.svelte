<script lang="ts">
  import { onDestroy } from "svelte";
  import {
    AlarmClock,
    Bell,
    Check,
    Clock3,
    Plus,
    RotateCcw,
    TimerOff,
    Volume2,
  } from "lucide-svelte";
  import WidgetActionButton from "$lib/components/widget-content/WidgetActionButton.svelte";
  import WidgetBadge from "$lib/components/widget-content/WidgetBadge.svelte";
  import WidgetEmptyState from "$lib/components/widget-content/WidgetEmptyState.svelte";

  type Item = {
    id: string;
    kind: "timer" | "alarm";
    label: string;
    status: string;
    dueAt?: string;
    durationSeconds?: number;
    time?: string;
    timezone?: string;
    weekdays?: number[];
    sound?: string;
    snoozeCount?: number;
    ringing?: boolean;
    remainingSeconds?: number;
    recurring?: boolean;
  };

  type TimersData = {
    active?: Item[];
    ringing?: Item[];
    notificationSound?: string;
    defaultSnoozeMins?: number;
    timezone?: string;
  };

  export let data: TimersData = {};
  export let stale = false;
  export let dispatch: (
    action: string,
    args?: Record<string, unknown>,
  ) => Promise<unknown> = async () => {};

  const sounds = ["chime", "bell", "pulse", "soft", "none"];
  const weekdays = [
    ["S", 0],
    ["M", 1],
    ["T", 2],
    ["W", 3],
    ["T", 4],
    ["F", 5],
    ["S", 6],
  ] as const;

  let mode: "timer" | "alarm" = "timer";
  let timerMinutes = 5;
  let timerLabel = "";
  let alarmTime = "07:00";
  let alarmLabel = "";
  let selectedWeekdays: number[] = [];
  let sound = "";
  let busy = false;
  let error = "";
  let now = Date.now();
  let tick: number | undefined;

  $: sound = sound || data.notificationSound || "chime";
  $: activeItems = (data.active ?? []).filter(
    (item) => item.status !== "dismissed" && item.status !== "cancelled",
  );
  $: sortedItems = [...activeItems].sort((a, b) => dueMs(a) - dueMs(b));
  $: ringingItems = sortedItems.filter((item) => isRinging(item));

  if (typeof window !== "undefined") {
    tick = window.setInterval(() => {
      now = Date.now();
    }, 1000);
  }

  onDestroy(() => {
    if (tick) {
      window.clearInterval(tick);
    }
  });

  function dueMs(item: Item) {
    return item.dueAt
      ? new Date(item.dueAt).getTime()
      : Number.MAX_SAFE_INTEGER;
  }

  function isRinging(item: Item) {
    return (
      (item.status === "active" || item.status === "snoozed") &&
      dueMs(item) <= now
    );
  }

  function remaining(item: Item) {
    return Math.max(0, Math.ceil((dueMs(item) - now) / 1000));
  }

  function formatRemaining(seconds: number) {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    if (mins >= 60) {
      const hours = Math.floor(mins / 60);
      const remMins = mins % 60;
      return `${hours}h ${remMins}m`;
    }
    return `${mins}:${secs.toString().padStart(2, "0")}`;
  }

  function formatDue(item: Item) {
    if (item.kind === "alarm" && item.time) {
      return item.recurring || (item.weekdays?.length ?? 0) > 0
        ? `${item.time} · ${weekdayLabel(item.weekdays ?? [])}`
        : item.time;
    }
    return formatRemaining(remaining(item));
  }

  function weekdayLabel(days: number[]) {
    if (days.length === 0) return "once";
    if (days.length === 7) return "daily";
    return days
      .map((day) => ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][day])
      .join(" ");
  }

  function toggleWeekday(day: number) {
    selectedWeekdays = selectedWeekdays.includes(day)
      ? selectedWeekdays.filter((value) => value !== day)
      : [...selectedWeekdays, day].sort();
  }

  async function submit() {
    busy = true;
    error = "";
    try {
      if (mode === "timer") {
        await dispatch("create_timer", {
          durationSeconds: Math.max(1, Math.round(timerMinutes * 60)),
          label: timerLabel || "Timer",
          sound,
        });
        timerLabel = "";
      } else {
        await dispatch("create_alarm", {
          time: alarmTime,
          label: alarmLabel || "Alarm",
          weekdays: selectedWeekdays,
          sound,
        });
        alarmLabel = "";
      }
    } catch {
      error = "Action failed";
    } finally {
      busy = false;
    }
  }

  async function setSound(nextSound: string) {
    sound = nextSound;
    await dispatch("set_notification_sound", { sound: nextSound });
  }

  function snooze(item: Item) {
    return dispatch("snooze", {
      id: item.id,
      minutes: data.defaultSnoozeMins ?? 9,
    });
  }
</script>

<section class="timers-widget" class:timers-widget--stale={stale}>
  <div class="composer">
    <div class="mode-tabs" aria-label="Timer or alarm">
      <button
        type="button"
        class:active={mode === "timer"}
        on:click={() => (mode = "timer")}
      >
        <Clock3 size={15} />
        <span>Timer</span>
      </button>
      <button
        type="button"
        class:active={mode === "alarm"}
        on:click={() => (mode = "alarm")}
      >
        <AlarmClock size={15} />
        <span>Alarm</span>
      </button>
    </div>

    {#if mode === "timer"}
      <div class="entry-row">
        <input
          aria-label="Timer label"
          placeholder="Label"
          bind:value={timerLabel}
        />
        <input
          aria-label="Timer minutes"
          type="number"
          min="1"
          step="1"
          bind:value={timerMinutes}
        />
        <WidgetActionButton
          label={busy ? "Adding" : "Add"}
          on:click={submit}
          disabled={busy}
        >
          <Plus size={15} />
        </WidgetActionButton>
      </div>
    {:else}
      <div class="entry-row">
        <input
          aria-label="Alarm label"
          placeholder="Label"
          bind:value={alarmLabel}
        />
        <input aria-label="Alarm time" type="time" bind:value={alarmTime} />
        <WidgetActionButton
          label={busy ? "Adding" : "Add"}
          on:click={submit}
          disabled={busy}
        >
          <Plus size={15} />
        </WidgetActionButton>
      </div>
      <div class="weekday-row" aria-label="Recurring days">
        {#each weekdays as [label, day]}
          <button
            type="button"
            class:active={selectedWeekdays.includes(day)}
            on:click={() => toggleWeekday(day)}
          >
            {label}
          </button>
        {/each}
      </div>
    {/if}

    <label class="sound-row">
      <Volume2 size={14} />
      <select
        aria-label="Notification sound"
        bind:value={sound}
        on:change={(event) => setSound(event.currentTarget.value)}
      >
        {#each sounds as option}
          <option value={option}>{option}</option>
        {/each}
      </select>
    </label>
  </div>

  {#if error}
    <div class="inline-error">{error}</div>
  {/if}

  {#if sortedItems.length === 0}
    <WidgetEmptyState message="No timers or alarms" />
  {:else}
    <div class="item-list" aria-live="polite">
      {#each sortedItems as item (item.id)}
        <article class:ringing={isRinging(item)} class="timer-item">
          <div class="item-icon">
            {#if item.kind === "timer"}
              <Clock3 size={18} />
            {:else}
              <Bell size={18} />
            {/if}
          </div>
          <div class="item-main">
            <strong>{item.label}</strong>
            <span>{formatDue(item)}</span>
          </div>
          <div class="item-state">
            {#if isRinging(item)}
              <WidgetBadge tone="warning">due</WidgetBadge>
            {:else if item.status === "snoozed"}
              <WidgetBadge>snoozed</WidgetBadge>
            {:else if item.kind === "alarm" && (item.weekdays?.length ?? 0) > 0}
              <WidgetBadge>{weekdayLabel(item.weekdays ?? [])}</WidgetBadge>
            {/if}
          </div>
          <div class="item-actions">
            <button type="button" title="Snooze" on:click={() => snooze(item)}>
              <RotateCcw size={15} />
            </button>
            <button
              type="button"
              title="Dismiss"
              on:click={() => dispatch("dismiss", { id: item.id })}
            >
              <Check size={15} />
            </button>
            <button
              type="button"
              title="Cancel"
              on:click={() => dispatch("cancel", { id: item.id })}
            >
              <TimerOff size={15} />
            </button>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>

<style>
  .timers-widget {
    container-type: size;
    display: grid;
    gap: clamp(6px, 2.2cqmin, 12px);
    min-height: 100%;
    color: var(--foreground);
  }

  .timers-widget--stale {
    opacity: 0.72;
  }

  .composer {
    display: grid;
    gap: clamp(5px, 1.7cqmin, 10px);
  }

  .mode-tabs,
  .weekday-row {
    display: flex;
    gap: clamp(4px, 1cqmin, 8px);
    min-height: clamp(28px, 4.8cqmin, 38px);
  }

  .mode-tabs button,
  .weekday-row button,
  .item-actions button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: clamp(4px, 0.8cqmin, 7px);
    border: 1px solid var(--border);
    background: transparent;
    color: var(--foreground);
    min-height: clamp(28px, 4.8cqmin, 38px);
    border-radius: 6px;
    transition:
      background 140ms ease,
      border-color 140ms ease,
      transform 140ms ease;
  }

  .mode-tabs button {
    flex: 1;
    font-size: clamp(0.72rem, 4.2cqmin, 0.9rem);
  }

  .mode-tabs button.active,
  .weekday-row button.active {
    background: var(--surface-strong);
    border-color: var(--border-strong);
  }

  .entry-row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(52px, 0.42fr) auto;
    gap: clamp(4px, 1cqmin, 8px);
    align-items: center;
  }

  input,
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

  .weekday-row button {
    flex: 1;
    font-size: clamp(0.68rem, 3.8cqmin, 0.82rem);
  }

  .sound-row {
    display: grid;
    grid-template-columns: auto 1fr;
    align-items: center;
    gap: clamp(4px, 1cqmin, 8px);
    color: var(--muted);
  }

  .item-list {
    display: grid;
    gap: clamp(5px, 1.2cqmin, 8px);
    overflow: auto;
    min-height: 0;
  }

  .timer-item {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: clamp(6px, 1.3cqmin, 10px);
    border: 1px solid var(--border);
    border-radius: 7px;
    padding: clamp(5px, 1.3cqmin, 9px);
    background: color-mix(in srgb, var(--surface-muted) 42%, transparent);
  }

  .timer-item.ringing {
    border-color: var(--warning);
    background: color-mix(in srgb, var(--warning) 13%, transparent);
  }

  .item-icon {
    display: grid;
    place-items: center;
    width: clamp(24px, 4.6cqmin, 34px);
    height: clamp(24px, 4.6cqmin, 34px);
    border-radius: 50%;
    color: var(--foreground);
    background: var(--surface-strong);
  }

  .item-main {
    display: grid;
    min-width: 0;
    gap: 0.2cqmin;
  }

  .item-main strong,
  .item-main span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-main strong {
    font-size: clamp(0.76rem, 4.3cqmin, 0.96rem);
  }

  .item-main span {
    color: var(--muted);
    font-size: clamp(0.68rem, 3.7cqmin, 0.84rem);
  }

  .item-state {
    grid-column: 2 / 3;
    min-width: 0;
  }

  .item-actions {
    display: flex;
    grid-row: 1 / span 2;
    grid-column: 3;
    gap: clamp(3px, 0.6cqmin, 6px);
  }

  .item-actions button {
    width: clamp(30px, 4.7cqmin, 38px);
    min-width: 34px;
    min-height: 34px;
    cursor: pointer;
  }

  .inline-error {
    color: var(--danger);
    font-size: clamp(0.72rem, 4cqmin, 0.86rem);
  }

  @container (max-width: 220px) {
    .entry-row,
    .timer-item {
      grid-template-columns: 1fr;
    }

    .item-state {
      grid-column: auto;
    }

    .item-actions {
      grid-row: auto;
      grid-column: auto;
      justify-content: stretch;
    }

    .item-actions button {
      flex: 1;
    }
  }
</style>
