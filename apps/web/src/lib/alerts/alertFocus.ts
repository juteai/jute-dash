import type {
  DashboardData,
  WidgetInstance,
  WidgetRuntimePayload
} from '$lib/types';
import {
  normalizeNotificationSound,
  type NotificationSound
} from './notificationSound';

const DEFAULT_SNOOZE_MINS = 9;

type TimerAlarmItem = {
  id?: unknown;
  kind?: unknown;
  label?: unknown;
  status?: unknown;
  dueAt?: unknown;
  time?: unknown;
  sound?: unknown;
};

type CalendarAlert = {
  id?: unknown;
  kind?: unknown;
  label?: unknown;
  status?: unknown;
  dueAt?: unknown;
  eventStart?: unknown;
  eventEnd?: unknown;
  sound?: unknown;
  defaultSnoozeMins?: unknown;
};

type TimersPayload = {
  active?: unknown[];
  ringing?: unknown[];
  defaultSnoozeMins?: unknown;
  notificationSound?: unknown;
};

type CalendarPayload = {
  alerts?: unknown[];
  ringing?: unknown[];
  defaultSnoozeMins?: unknown;
  notificationSound?: unknown;
};

export type AlertFocusKind = 'timer' | 'alarm' | 'calendar-event';

export type AlertFocusItem = {
  id: string;
  widgetId: string;
  kind: AlertFocusKind;
  kindLabel: string;
  label: string;
  dueAt: string;
  displayAt: string;
  sound: NotificationSound;
  defaultSnoozeMins: number;
  snoozeActionId: string;
  dismissActionId: string;
  playbackKey: string;
};

export type AlertFocusState = {
  primary?: AlertFocusItem;
  items: AlertFocusItem[];
  ringingCount: number;
};

export type AlertFocusCommand = {
  actionId: string;
  arguments: Record<string, string | number>;
};

export function deriveAlertFocusState(
  data: DashboardData,
  nowMs: number
): AlertFocusState {
  const items = collectAlertFocusItems(data.layout.widgets, nowMs);
  return {
    primary: items[0],
    items,
    ringingCount: items.length
  };
}

export function alertFocusCommand(
  item: AlertFocusItem,
  action: 'snooze' | 'dismiss'
): AlertFocusCommand {
  if (action === 'snooze') {
    return {
      actionId: item.snoozeActionId,
      arguments: { id: item.id, minutes: item.defaultSnoozeMins }
    };
  }
  return {
    actionId: item.dismissActionId,
    arguments: { id: item.id }
  };
}

export function formatAlertFocusTime(
  item: AlertFocusItem,
  locales?: Intl.LocalesArgument,
  options: Intl.DateTimeFormatOptions = {}
): string {
  if (item.kind === 'alarm' && /^\d{2}:\d{2}$/.test(item.displayAt)) {
    return item.displayAt;
  }
  const timestamp = Date.parse(item.displayAt);
  if (Number.isNaN(timestamp)) {
    return '00:00';
  }
  return new Date(timestamp).toLocaleTimeString(locales, {
    hour: '2-digit',
    minute: '2-digit',
    ...options
  });
}

export function collectAlertFocusItems(
  widgets: WidgetInstance[],
  nowMs: number
): AlertFocusItem[] {
  const items: AlertFocusItem[] = [];
  for (const widget of widgets) {
    if (!widget.visible) continue;

    const payload = widgetPayload(widget) ?? {};
    if (widget.kind === 'timers-alarms' && isTimersPayload(payload)) {
      items.push(...timerAlarmAlerts(widget, payload, nowMs));
    }
    if (widget.kind === 'calendar' && isCalendarPayload(payload)) {
      items.push(...calendarAlerts(widget, payload, nowMs));
    }
  }
  return items;
}

function timerAlarmAlerts(
  widget: WidgetInstance,
  payload: TimersPayload,
  nowMs: number
): AlertFocusItem[] {
  const items = uniqueById([
    ...(Array.isArray(payload.ringing) ? payload.ringing : []),
    ...(Array.isArray(payload.active) ? payload.active : [])
  ] as TimerAlarmItem[]);
  const fallbackSound = normalizeNotificationSound(payload.notificationSound);
  const defaultSnoozeMins = positiveNumber(
    payload.defaultSnoozeMins,
    DEFAULT_SNOOZE_MINS
  );

  return items.flatMap((item) => {
    const id = stringValue(item.id);
    const kind =
      item.kind === 'alarm'
        ? 'alarm'
        : item.kind === 'timer'
          ? 'timer'
          : undefined;
    const dueAt = stringValue(item.dueAt);
    const dueMs = Date.parse(dueAt);
    const active = item.status === 'active' || item.status === 'snoozed';
    if (!id || !kind || !active || Number.isNaN(dueMs) || dueMs > nowMs) {
      return [];
    }

    const sound = normalizeNotificationSound(item.sound, fallbackSound);
    const displayAt =
      kind === 'alarm' ? stringValue(item.time) || dueAt : dueAt;
    return [
      {
        id,
        widgetId: widget.id,
        kind,
        kindLabel: kind === 'timer' ? 'Timer' : 'Alarm',
        label:
          stringValue(item.label) || (kind === 'timer' ? 'Timer' : 'Alarm'),
        dueAt,
        displayAt,
        sound,
        defaultSnoozeMins,
        snoozeActionId: 'snooze',
        dismissActionId: 'dismiss',
        playbackKey: `${widget.id}:${id}:${dueAt}:${sound}`
      }
    ];
  });
}

function calendarAlerts(
  widget: WidgetInstance,
  payload: CalendarPayload,
  nowMs: number
): AlertFocusItem[] {
  const alerts = uniqueById([
    ...(Array.isArray(payload.ringing) ? payload.ringing : []),
    ...(Array.isArray(payload.alerts) ? payload.alerts : [])
  ] as CalendarAlert[]);
  const fallbackSound = normalizeNotificationSound(payload.notificationSound);
  const payloadSnooze = positiveNumber(
    payload.defaultSnoozeMins,
    DEFAULT_SNOOZE_MINS
  );

  return alerts.flatMap((alert) => {
    const id = stringValue(alert.id);
    const dueAt = stringValue(alert.dueAt);
    const dueMs = Date.parse(dueAt);
    if (
      !id ||
      alert.status !== 'active' ||
      Number.isNaN(dueMs) ||
      dueMs > nowMs
    ) {
      return [];
    }

    const eventStart = stringValue(alert.eventStart) || dueAt;
    const sound = normalizeNotificationSound(alert.sound, fallbackSound);
    return [
      {
        id,
        widgetId: widget.id,
        kind: 'calendar-event',
        kindLabel: 'Event',
        label: stringValue(alert.label) || 'Calendar event',
        dueAt,
        displayAt: eventStart,
        sound,
        defaultSnoozeMins: positiveNumber(
          alert.defaultSnoozeMins,
          payloadSnooze
        ),
        snoozeActionId: 'snooze_event',
        dismissActionId: 'dismiss_event',
        playbackKey: `${widget.id}:${id}:${dueAt}:${sound}`
      }
    ];
  });
}

function widgetPayload(widget: WidgetInstance): unknown {
  const payload = widget.data as WidgetRuntimePayload | undefined;
  if (payload && typeof payload === 'object' && 'data' in payload) {
    return payload.data;
  }
  return widget.data;
}

function isTimersPayload(value: unknown): value is TimersPayload {
  return Boolean(value && typeof value === 'object');
}

function isCalendarPayload(value: unknown): value is CalendarPayload {
  return Boolean(value && typeof value === 'object');
}

function uniqueById<T extends { id?: unknown }>(items: T[]): T[] {
  const seen = new Set<string>();
  const out: T[] = [];
  for (const item of items) {
    const id = stringValue(item.id);
    if (!id || seen.has(id)) continue;
    seen.add(id);
    out.push(item);
  }
  return out;
}

function positiveNumber(value: unknown, fallback: number): number {
  const numeric = Number(value);
  return Number.isFinite(numeric) && numeric > 0 ? numeric : fallback;
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}
