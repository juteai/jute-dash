import { describe, expect, it } from 'vitest';
import {
  alertFocusCommand,
  collectAlertFocusItems,
  deriveAlertFocusState,
  formatAlertFocusTime
} from './alertFocus';
import type { DashboardData, WidgetInstance } from '$lib/types';

const nowMs = Date.parse('2026-06-15T09:00:00Z');

describe('alertFocus', () => {
  it('normalizes due timers and suppresses duplicate ringing/active items', () => {
    const item = {
      id: 'timer-1',
      kind: 'timer',
      label: 'Tea',
      status: 'active',
      dueAt: '2026-06-15T08:59:00Z',
      sound: 'bell'
    };
    const state = deriveAlertFocusState(
      dashboard([
        widget('timers-alarms', {
          data: {
            active: [item],
            ringing: [item],
            defaultSnoozeMins: 5,
            notificationSound: 'chime'
          }
        })
      ]),
      nowMs
    );

    expect(state.ringingCount).toBe(1);
    expect(state.primary).toMatchObject({
      id: 'timer-1',
      kind: 'timer',
      kindLabel: 'Timer',
      label: 'Tea',
      sound: 'bell',
      defaultSnoozeMins: 5,
      snoozeActionId: 'snooze',
      dismissActionId: 'dismiss'
    });
  });

  it('normalizes alarms and keeps alarm display time as HH:MM', () => {
    const [alarm] = collectAlertFocusItems(
      [
        widget('timers-alarms', {
          data: {
            active: [
              {
                id: 'alarm-1',
                kind: 'alarm',
                label: 'School',
                status: 'active',
                dueAt: '2026-06-15T09:00:00Z',
                time: '09:00'
              }
            ],
            notificationSound: 'soft'
          }
        })
      ],
      nowMs
    );

    expect(alarm.kind).toBe('alarm');
    expect(formatAlertFocusTime(alarm, 'en-GB')).toBe('09:00');
  });

  it('normalizes calendar alerts and maps widget action ids', () => {
    const [event] = collectAlertFocusItems(
      [
        widget('calendar', {
          data: {
            alerts: [
              {
                id: 'calendar:event-1',
                kind: 'calendar-event',
                label: 'Dentist',
                status: 'active',
                dueAt: '2026-06-15T08:55:00Z',
                eventStart: '2026-06-15T09:05:00Z',
                sound: 'pulse',
                defaultSnoozeMins: 12
              }
            ]
          }
        })
      ],
      nowMs
    );

    expect(event).toMatchObject({
      kind: 'calendar-event',
      kindLabel: 'Event',
      sound: 'pulse',
      defaultSnoozeMins: 12
    });
    expect(alertFocusCommand(event, 'snooze')).toEqual({
      actionId: 'snooze_event',
      arguments: { id: 'calendar:event-1', minutes: 12 }
    });
    expect(alertFocusCommand(event, 'dismiss')).toEqual({
      actionId: 'dismiss_event',
      arguments: { id: 'calendar:event-1' }
    });
    expect(formatAlertFocusTime(event, 'en-GB', { timeZone: 'UTC' })).toBe(
      '09:05'
    );
  });

  it('ignores hidden widgets and future due times', () => {
    const items = collectAlertFocusItems(
      [
        widget('timers-alarms', {
          visible: false,
          data: {
            active: [
              {
                id: 'timer-hidden',
                kind: 'timer',
                label: 'Hidden',
                status: 'active',
                dueAt: '2026-06-15T08:59:00Z'
              }
            ]
          }
        }),
        widget('calendar', {
          data: {
            alerts: [
              {
                id: 'calendar:future',
                status: 'active',
                dueAt: '2026-06-15T09:01:00Z',
                eventStart: '2026-06-15T09:10:00Z'
              }
            ]
          }
        })
      ],
      nowMs
    );

    expect(items).toEqual([]);
  });
});

function dashboard(widgets: WidgetInstance[]): DashboardData {
  return {
    layout: { profileId: 'default', widgets }
  } as unknown as DashboardData;
}

function widget(
  kind: string,
  options: { visible?: boolean; data: unknown }
): WidgetInstance {
  return {
    id: `${kind}-1`,
    kind,
    title: kind,
    x: 0,
    y: 0,
    w: 1,
    h: 1,
    minW: 1,
    minH: 1,
    size: 'medium',
    settings: {},
    visible: options.visible ?? true,
    data: { status: 'ok', data: options.data }
  };
}
