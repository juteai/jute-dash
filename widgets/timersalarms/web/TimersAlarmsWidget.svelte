<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import {
    AlarmClock,
    Bell,
    Check,
    Clock3,
    MoreVertical,
    Plus,
    RotateCcw,
    TimerOff,
    Volume2,
  } from "lucide-svelte";
  import WidgetEmptyState from "$lib/components/widget-content/WidgetEmptyState.svelte";
  import WidgetBadge from "$lib/components/widget-content/WidgetBadge.svelte";

  type Item = {
    id: string;
    kind: "timer" | "alarm";
    label: string;
    status: string;
    createdAt?: string;
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

  type Weekday = {
    short: string;
    long: string;
    value: number;
  };

  export let data: TimersData = {};
  export let stale = false;
  export let dispatch: (
    action: string,
    args?: Record<string, unknown>,
  ) => Promise<unknown> = async () => {};

  const sounds = ["chime", "bell", "pulse", "soft", "none"];
  const timerPresets = [5, 10, 15, 30];
  const weekdays: Weekday[] = [
    { short: "Sun", long: "Sunday", value: 0 },
    { short: "Mon", long: "Monday", value: 1 },
    { short: "Tue", long: "Tuesday", value: 2 },
    { short: "Wed", long: "Wednesday", value: 3 },
    { short: "Thu", long: "Thursday", value: 4 },
    { short: "Fri", long: "Friday", value: 5 },
    { short: "Sat", long: "Saturday", value: 6 },
  ];

  let menuOpen = false;
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
  let menuButton: HTMLButtonElement | undefined;
  let menuTop = 0;
  let menuRight = 0;

  $: sound = sound || data.notificationSound || "chime";
  $: activeItems = (data.active ?? []).filter(
    (item) => item.status !== "dismissed" && item.status !== "cancelled",
  );
  $: sortedItems = [...activeItems].sort((a, b) => dueMs(a) - dueMs(b));
  $: ringingItems = sortedItems.filter((item) => isRinging(item));
  $: primaryItem = ringingItems[0] ?? sortedItems[0];
  $: visibleItems = sortedItems.slice(0, 3);
  $: hiddenItemCount = Math.max(0, sortedItems.length - visibleItems.length);
  $: heroProgress = progressFor(primaryItem);

  onMount(() => {
    const closeMenu = () => {
      menuOpen = false;
    };
    const repositionMenu = () => {
      if (menuOpen) positionMenu();
    };

    tick = window.setInterval(() => {
      now = Date.now();
    }, 1000);
    window.addEventListener("pointerdown", closeMenu);
    window.addEventListener("resize", repositionMenu);

    return () => {
      if (tick) {
        window.clearInterval(tick);
      }
      window.removeEventListener("pointerdown", closeMenu);
      window.removeEventListener("resize", repositionMenu);
    };
  });

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

  function formatPrimary(item: Item | undefined) {
    if (!item) return "--:--";
    if (item.kind === "alarm" && item.time) return item.time;
    return formatRemaining(remaining(item));
  }

  function formatDetail(item: Item) {
    if (isRinging(item)) return "due now";
    if (item.kind === "alarm") {
      return weekdayLabel(item.weekdays ?? []);
    }
    return "remaining";
  }

  function formatItemTime(item: Item) {
    if (item.kind === "alarm" && item.time) return item.time;
    return formatRemaining(remaining(item));
  }

  function formatKind(item: Item | undefined) {
    if (!item) return "Timer";
    return item.kind === "alarm" ? "Alarm" : "Timer";
  }

  function heroCaption(item: Item | undefined) {
    if (!item) return "Tap options to add one";
    if (isRinging(item)) return "Due now";
    if (item.kind === "alarm") return weekdayLabel(item.weekdays ?? []);
    return "Remaining";
  }

  function progressFor(item: Item | undefined) {
    if (!item || item.kind !== "timer" || !item.durationSeconds) return 0;
    const seconds = remaining(item);
    return Math.max(0, Math.min(1, seconds / item.durationSeconds));
  }

  function choosePreset(minutes: number) {
    timerMinutes = minutes;
  }

  function weekdayLabel(days: number[]) {
    if (days.length === 0) return "once";
    if (days.length === 7) return "daily";
    return days
      .map(
        (day) =>
          weekdays.find((weekday) => weekday.value === day)?.short ?? `${day}`,
      )
      .join(" ");
  }

  function toggleWeekday(day: number) {
    selectedWeekdays = selectedWeekdays.includes(day)
      ? selectedWeekdays.filter((value) => value !== day)
      : [...selectedWeekdays, day].sort();
  }

  function setMode(nextMode: "timer" | "alarm") {
    mode = nextMode;
    error = "";
    positionMenu();
  }

  function positionMenu() {
    if (!menuButton || typeof window === "undefined") return;
    const rect = menuButton.getBoundingClientRect();
    const estimatedHeight = mode === "alarm" ? 300 : 230;
    menuTop = Math.max(
      8,
      Math.min(rect.bottom + 6, window.innerHeight - estimatedHeight),
    );
    menuRight = Math.max(12, window.innerWidth - rect.right);
  }

  function toggleMenu() {
    if (menuOpen) {
      menuOpen = false;
      return;
    }
    positionMenu();
    menuOpen = true;
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
      menuOpen = false;
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
  <header class="widget-topline">
    <div class="widget-title">
      <Clock3 size={15} />
      <span>Timers</span>
    </div>
    <div class="topline-actions">
      {#if ringingItems.length > 0}
        <WidgetBadge tone="warning" pulse>{ringingItems.length} due</WidgetBadge
        >
      {:else if sortedItems.length > 0}
        <WidgetBadge>{sortedItems.length} active</WidgetBadge>
      {:else}
        <WidgetBadge>ready</WidgetBadge>
      {/if}
      <div class="menu-wrap">
        <button
          bind:this={menuButton}
          type="button"
          class="menu-trigger"
          aria-label="Timer and alarm options"
          aria-haspopup="dialog"
          aria-expanded={menuOpen}
          on:pointerdown|stopPropagation
          on:click|stopPropagation={toggleMenu}
        >
          <MoreVertical size={18} />
        </button>
        {#if menuOpen}
          <div
            class="timer-menu"
            role="dialog"
            tabindex="-1"
            aria-label="Add timer or alarm"
            style={`top: ${menuTop}px; right: ${menuRight}px;`}
            on:pointerdown|stopPropagation
          >
            <div class="menu-heading">
              <strong>{mode === "timer" ? "New timer" : "New alarm"}</strong>
              <span>{sound} sound</span>
            </div>

            <div class="menu-mode" aria-label="Timer or alarm">
              <button
                type="button"
                class:active={mode === "timer"}
                on:click={() => setMode("timer")}
              >
                <Clock3 size={15} />
                <span>Timer</span>
              </button>
              <button
                type="button"
                class:active={mode === "alarm"}
                on:click={() => setMode("alarm")}
              >
                <AlarmClock size={15} />
                <span>Alarm</span>
              </button>
            </div>

            {#if mode === "timer"}
              <div class="preset-row" aria-label="Timer duration presets">
                {#each timerPresets as minutes}
                  <button
                    type="button"
                    class:active={timerMinutes === minutes}
                    on:click={() => choosePreset(minutes)}
                  >
                    {minutes}m
                  </button>
                {/each}
              </div>
              <div class="menu-fields">
                <input
                  aria-label="Timer label"
                  placeholder="Tea, laundry, oven"
                  bind:value={timerLabel}
                />
                <label class="number-field">
                  <span>min</span>
                  <input
                    aria-label="Timer minutes"
                    type="number"
                    min="1"
                    step="1"
                    bind:value={timerMinutes}
                  />
                </label>
              </div>
            {:else}
              <div class="menu-fields">
                <input
                  aria-label="Alarm label"
                  placeholder="Wake up, school run"
                  bind:value={alarmLabel}
                />
                <input
                  aria-label="Alarm time"
                  type="time"
                  bind:value={alarmTime}
                />
              </div>
              <div class="weekday-row" aria-label="Recurring days">
                {#each weekdays as weekday}
                  <button
                    type="button"
                    aria-label={weekday.long}
                    class:active={selectedWeekdays.includes(weekday.value)}
                    on:click={() => toggleWeekday(weekday.value)}
                  >
                    {weekday.short}
                  </button>
                {/each}
              </div>
            {/if}

            <label class="sound-row">
              <Volume2 size={14} />
              <span>Sound</span>
              <select
                aria-label="Notification sound"
                value={sound}
                on:change={(event) => setSound(event.currentTarget.value)}
              >
                {#each sounds as option}
                  <option value={option}>{option}</option>
                {/each}
              </select>
            </label>

            {#if error}
              <div class="inline-error">{error}</div>
            {/if}

            <button
              type="button"
              class="add-action"
              on:click={submit}
              disabled={busy}
            >
              <Plus size={16} />
              <span
                >{busy
                  ? "Adding"
                  : mode === "timer"
                    ? "Add timer"
                    : "Add alarm"}</span
              >
            </button>
          </div>
        {/if}
      </div>
    </div>
  </header>

  {#if primaryItem}
    <article
      class:ringing={isRinging(primaryItem)}
      class="hero-card"
      style={`--timer-progress: ${heroProgress * 360}deg;`}
    >
      <div class="time-orbit" aria-hidden="true">
        {#if primaryItem.kind === "alarm"}
          <AlarmClock size={24} />
        {:else}
          <Clock3 size={24} />
        {/if}
      </div>
      <div class="hero-body">
        <span>{formatKind(primaryItem)}</span>
        <strong>{formatPrimary(primaryItem)}</strong>
        <small>{primaryItem.label} · {heroCaption(primaryItem)}</small>
      </div>
      {#if isRinging(primaryItem)}
        <div class="hero-actions">
          <button type="button" on:click={() => snooze(primaryItem)}>
            <RotateCcw size={15} />
            <span>Snooze</span>
          </button>
          <button
            type="button"
            class="primary"
            on:click={() => dispatch("dismiss", { id: primaryItem.id })}
          >
            <Check size={15} />
            <span>Dismiss</span>
          </button>
        </div>
      {:else}
        <button
          type="button"
          class="quiet-cancel"
          aria-label={`Cancel ${primaryItem.label}`}
          on:click={() => dispatch("cancel", { id: primaryItem.id })}
        >
          <TimerOff size={15} />
        </button>
      {/if}
    </article>
  {:else}
    <div class="empty-panel">
      <WidgetEmptyState message="No timers or alarms">
        <Clock3 slot="icon" size={34} />
      </WidgetEmptyState>
    </div>
  {/if}

  {#if sortedItems.length > 1}
    <div class="item-list" aria-live="polite">
      {#each visibleItems.filter((item) => item.id !== primaryItem?.id) as item (item.id)}
        <article class:ringing={isRinging(item)} class="timer-item">
          <div class="item-main">
            <span class="item-kind">
              {#if item.kind === "timer"}
                <Clock3 size={13} />
                Timer
              {:else}
                <Bell size={13} />
                Alarm
              {/if}
            </span>
            <strong>{item.label}</strong>
          </div>
          <div class="item-side">
            <strong>{formatItemTime(item)}</strong>
            <span>{formatDetail(item)}</span>
          </div>
          <div class="item-actions">
            {#if isRinging(item)}
              <button
                type="button"
                aria-label="Snooze"
                on:click={() => snooze(item)}
              >
                <RotateCcw size={14} />
              </button>
              <button
                type="button"
                aria-label="Dismiss"
                on:click={() => dispatch("dismiss", { id: item.id })}
              >
                <Check size={14} />
              </button>
            {:else}
              <button
                type="button"
                aria-label={`Cancel ${item.label}`}
                on:click={() => dispatch("cancel", { id: item.id })}
              >
                <TimerOff size={14} />
              </button>
            {/if}
          </div>
        </article>
      {/each}
      {#if hiddenItemCount > 0}
        <div class="more-row">+{hiddenItemCount} more</div>
      {/if}
    </div>
  {:else}
    <div class="quiet-footer">
      <span>{data.notificationSound ?? "chime"} sound</span>
      <span>{data.defaultSnoozeMins ?? 9}m snooze</span>
    </div>
  {/if}
</section>

<style>
  .timers-widget {
    container-type: size;
    position: relative;
    display: grid;
    grid-template-rows: auto minmax(0, 1fr) auto;
    gap: clamp(8px, 2.2cqmin, 13px);
    min-height: 100%;
    color: var(--foreground);
  }

  .timers-widget--stale {
    opacity: 0.72;
  }

  .widget-topline,
  .widget-title,
  .topline-actions {
    display: flex;
    align-items: center;
  }

  .widget-topline {
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
  }

  .widget-title {
    min-width: 0;
    gap: 8px;
    color: var(--muted);
    font-size: var(--widget-label-size, 0.75rem);
    font-weight: 740;
    line-height: 1.1;
    text-transform: uppercase;
  }

  .widget-title span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .topline-actions {
    gap: 6px;
  }

  .menu-wrap {
    position: relative;
    align-self: start;
  }

  .menu-trigger,
  .quiet-cancel,
  .item-actions button {
    display: inline-grid;
    place-items: center;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
    color: var(--foreground);
    cursor: pointer;
    transition:
      background-color 0.18s ease,
      border-color 0.18s ease,
      color 0.18s ease,
      transform 0.18s ease;
  }

  .menu-trigger {
    width: 32px;
    height: 32px;
  }

  .menu-trigger:hover,
  .menu-trigger[aria-expanded="true"],
  .quiet-cancel:hover,
  .item-actions button:hover {
    border-color: var(--border);
    background: var(--surface-strong);
  }

  .menu-trigger:active,
  .quiet-cancel:active,
  .item-actions button:active,
  .hero-actions button:active,
  .add-action:active {
    transform: scale(0.97);
  }

  .timer-menu {
    position: fixed;
    z-index: 80;
    display: grid;
    width: min(342px, calc(100vw - 24px));
    gap: 10px;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: color-mix(in srgb, var(--surface) 94%, var(--background));
    box-shadow: 0 18px 42px rgba(0, 0, 0, 0.28);
  }

  .menu-heading {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
  }

  .menu-heading strong {
    font-size: 0.92rem;
    line-height: 1.1;
  }

  .menu-heading span,
  .sound-row span,
  .quiet-footer,
  .more-row,
  .hero-body span,
  .hero-body small,
  .item-kind,
  .item-side span {
    color: var(--muted);
    font-size: clamp(0.62rem, 3.2cqmin, 0.76rem);
  }

  .menu-mode,
  .weekday-row,
  .preset-row {
    display: flex;
    gap: 5px;
  }

  .menu-mode button,
  .weekday-row button,
  .preset-row button,
  .add-action {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-height: 32px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: transparent;
    color: var(--foreground);
    font: inherit;
    font-size: 0.78rem;
    cursor: pointer;
  }

  .menu-mode button,
  .preset-row button {
    flex: 1;
  }

  .menu-mode button.active,
  .weekday-row button.active,
  .preset-row button.active {
    border-color: var(--border-strong);
    background: var(--surface-strong);
    color: var(--active);
  }

  .menu-fields {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(82px, 0.45fr);
    gap: 6px;
  }

  input {
    width: 100%;
    min-width: 0;
    min-height: 34px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: color-mix(in srgb, var(--surface-muted) 42%, transparent);
    color: var(--foreground);
    padding: 0 9px;
    font: inherit;
    font-size: 0.82rem;
  }

  .number-field {
    position: relative;
    display: block;
  }

  .number-field span {
    position: absolute;
    top: 50%;
    right: 8px;
    color: var(--muted);
    font-size: 0.7rem;
    transform: translateY(-50%);
    pointer-events: none;
  }

  .number-field input {
    padding-right: 30px;
  }

  .weekday-row button {
    flex: 1;
    min-width: 0;
    font-size: 0.68rem;
  }

  .sound-row {
    display: grid;
    grid-template-columns: auto auto minmax(0, 1fr);
    align-items: center;
    gap: 7px;
    min-height: 34px;
  }

  select {
    width: 100%;
    min-width: 0;
    min-height: 34px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: color-mix(in srgb, var(--surface-muted) 42%, transparent);
    color: var(--foreground);
    padding: 0 9px;
    font: inherit;
    font-size: 0.82rem;
  }

  .add-action {
    width: 100%;
    min-height: 36px;
    border-color: var(--border-strong);
    background: var(--surface-strong);
    color: var(--foreground);
    font-weight: 720;
  }

  .add-action:disabled {
    cursor: default;
    opacity: 0.55;
  }

  .hero-card {
    position: relative;
    display: grid;
    grid-template-columns: minmax(74px, 0.44fr) minmax(0, 1fr) auto;
    align-items: center;
    gap: clamp(10px, 3cqmin, 16px);
    min-height: 0;
    min-height: 100%;
    padding: clamp(12px, 4cqmin, 18px);
    border: 1px solid color-mix(in srgb, var(--border) 56%, transparent);
    border-radius: 8px;
    background:
      linear-gradient(
        145deg,
        color-mix(in srgb, var(--surface-muted) 48%, transparent),
        transparent 62%
      ),
      color-mix(in srgb, var(--surface-muted) 18%, transparent);
    overflow: hidden;
  }

  .hero-card.ringing {
    border-color: color-mix(in srgb, var(--warning) 70%, var(--border));
    background:
      radial-gradient(
        circle at 16% 18%,
        color-mix(in srgb, var(--warning) 20%, transparent),
        transparent 42%
      ),
      color-mix(in srgb, var(--warning) 9%, transparent);
  }

  .time-orbit {
    display: grid;
    place-items: center;
    width: clamp(70px, 24cqmin, 116px);
    aspect-ratio: 1;
    border-radius: 999px;
    background:
      radial-gradient(
        circle,
        color-mix(in srgb, var(--surface) 86%, transparent) 55%,
        transparent 57%
      ),
      conic-gradient(
        var(--active) var(--timer-progress),
        color-mix(in srgb, var(--border) 58%, transparent) 0deg
      );
    color: var(--foreground);
  }

  .hero-card.ringing .time-orbit {
    background:
      radial-gradient(
        circle,
        color-mix(in srgb, var(--surface) 86%, transparent) 55%,
        transparent 57%
      ),
      conic-gradient(
        var(--warning) 360deg,
        color-mix(in srgb, var(--warning) 16%, transparent) 0deg
      );
  }

  .hero-body {
    display: grid;
    min-width: 0;
    gap: 3px;
  }

  .hero-body span,
  .item-kind {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-weight: 740;
    text-transform: uppercase;
  }

  .hero-body strong {
    overflow: hidden;
    font-size: clamp(2.25rem, 18cqmin, 4.9rem);
    font-weight: 815;
    font-variant-numeric: tabular-nums;
    line-height: 0.93;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .hero-body small {
    overflow: hidden;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .quiet-cancel {
    align-self: start;
    width: 34px;
    height: 34px;
    color: var(--muted);
  }

  .hero-actions {
    display: flex;
    flex-direction: column;
    gap: 6px;
    align-self: center;
  }

  .hero-actions button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    min-width: 94px;
    min-height: 34px;
    border: 1px solid var(--border);
    border-radius: 8px;
    background: var(--surface-muted);
    color: var(--foreground);
    font: inherit;
    font-size: 0.78rem;
    font-weight: 720;
    cursor: pointer;
    transition:
      background-color 0.18s ease,
      border-color 0.18s ease,
      transform 0.18s ease;
  }

  .hero-actions button.primary {
    border-color: var(--border-strong);
    background: var(--surface-strong);
  }

  .empty-panel {
    min-height: 100%;
    border: 1px dashed var(--border);
    border-radius: 8px;
    background: color-mix(in srgb, var(--surface-muted) 20%, transparent);
  }

  .item-list {
    display: grid;
    align-content: start;
    gap: clamp(5px, 1.2cqmin, 8px);
    overflow: auto;
    min-height: 0;
    padding-right: 2px;
  }

  .timer-item {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    align-items: center;
    gap: clamp(6px, 1.4cqmin, 10px);
    min-height: clamp(38px, 9cqmin, 50px);
    padding: clamp(5px, 1.4cqmin, 8px) 2px;
    border-top: 1px solid var(--border);
  }

  .timer-item.ringing {
    border-color: color-mix(in srgb, var(--warning) 58%, var(--border));
  }

  .item-main {
    display: grid;
    min-width: 0;
    gap: 2px;
  }

  .item-main strong {
    overflow: hidden;
    font-size: clamp(0.78rem, 4cqmin, 0.95rem);
    line-height: 1.05;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .item-side {
    display: grid;
    justify-items: end;
    gap: 2px;
    min-width: max-content;
  }

  .item-side strong {
    font-size: clamp(0.84rem, 4.4cqmin, 1.04rem);
    font-weight: 780;
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .item-actions {
    display: flex;
    gap: 4px;
  }

  .item-actions button {
    width: clamp(28px, 6cqmin, 34px);
    height: clamp(28px, 6cqmin, 34px);
    min-width: 28px;
    min-height: 28px;
  }

  .more-row {
    padding: 2px 4px;
    text-align: center;
  }

  .quiet-footer {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    padding: 0 2px;
    text-transform: uppercase;
  }

  .inline-error {
    color: var(--danger);
    font-size: 0.76rem;
  }

  button:focus-visible,
  input:focus-visible,
  select:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 1px;
  }

  @container (max-width: 250px) {
    .hero-card {
      grid-template-columns: 1fr auto;
    }

    .time-orbit {
      display: none;
    }

    .hero-actions {
      grid-column: 1 / -1;
      flex-direction: row;
    }

    .hero-actions button {
      flex: 1;
      min-width: 0;
    }

    .timer-item {
      grid-template-columns: minmax(0, 1fr) auto;
    }

    .item-actions {
      grid-column: 1 / -1;
      justify-content: stretch;
    }

    .item-actions button {
      flex: 1;
    }
  }

  @container (max-height: 180px) {
    .timers-widget {
      gap: clamp(6px, 1.8cqmin, 9px);
    }

    .item-list,
    .quiet-footer {
      display: none;
    }

    .hero-card {
      padding: clamp(9px, 3cqmin, 13px);
    }

    .time-orbit {
      width: clamp(58px, 21cqmin, 86px);
    }

    .hero-body strong {
      font-size: clamp(2rem, 16cqmin, 3.5rem);
    }
  }

  @container (max-height: 120px) {
    .timers-widget {
      gap: 5px;
    }

    .widget-title {
      font-size: 0.68rem;
    }

    .menu-trigger {
      width: 28px;
      height: 28px;
    }

    .hero-card {
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 8px;
      padding: 5px 8px;
    }

    .time-orbit {
      display: none;
    }

    .hero-body {
      gap: 1px;
    }

    .hero-body span {
      font-size: 0.58rem;
    }

    .hero-body strong {
      font-size: clamp(1.7rem, 14cqmin, 2.25rem);
    }

    .hero-body small {
      display: none;
    }

    .quiet-cancel {
      width: 30px;
      height: 30px;
    }
  }
</style>
