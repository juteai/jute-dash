# Timers & Alarms Widget

Kind: `timers-alarms`

The Timers & Alarms widget provides local countdown timers, one-off alarms, recurring weekday alarms, notification sound configuration, snooze, dismiss, and cancel controls.

## Settings

- `notificationSound`: one of `chime`, `bell`, `pulse`, `soft`, or `none`.
- `defaultSnoozeMins`: default snooze interval for UI and agent actions.
- `timezone`: default IANA timezone used when creating alarms.
- `items`: hub-managed timer/alarm records persisted with the widget instance settings.

## Agent Skill

Skill ID: `jute.timers_alarms.control`

Actions:

- `create_timer`
- `create_alarm`
- `snooze`
- `dismiss`
- `cancel`
- `set_notification_sound`

All actions route through the hub widget action dispatcher. Successful mutating actions persist updated widget settings through the layout store, so UI-created and MCP-created timers share the same state.
