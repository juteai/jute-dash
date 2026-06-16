# Calendar Widget

The Calendar widget starts as a blank local calendar and exposes event alerts to the display and Widget Skills. A Calendar Account connection can be added later to sync upcoming events from an iCalendar feed.

The dashboard tile is calendar-first: it renders the current month even when no source is configured. Alert lead time and notification sound are settings, not always-visible tile controls; snooze and dismiss appear in alert states.

## Sync

Calendar sync v1 optionally supports private iCalendar feed URLs. Feeds may be public bearer-style URLs or Basic Auth-protected URLs using the connection `username` field and secret `password` field.

Plain IMAP email is not a live calendar sync source. Email invite parsing can be added later as an import path. CalDAV and provider API sync are future work rather than part of the v1 account interface.

## Settings

- `timezone`: household durable display timezone for parsing floating and all-day event dates.
- `notificationSound`: household durable local sound used by event alerts.
- `lookaheadDays`: household durable fetch window, defaulting to 14 days.
- `alertLeadMinutes`: household durable minutes before event start when an alert becomes due.
- `defaultSnoozeMins`: household durable snooze interval for event alerts.
- `dismissedAlerts`: household durable dismissed event occurrence IDs.
- `snoozedAlerts`: household durable snoozed event occurrence IDs and snooze-until timestamps.

## Connection

The widget can run without a connection. To sync external events, link an optional `calendar-account` connection in the `account` slot:

- `feed_url`: optional private `.ics` or provider calendar export URL.
- `username`: optional Basic Auth username.
- `password`: optional Basic Auth password or app password stored as a secret.
- `calendar_name`: display name applied to events from the feed.

## Skill

The widget exposes `jute.calendar.events`.

Agent actions:

- `snooze_event`: snooze a due event alert by ID.
- `dismiss_event`: dismiss an event alert occurrence by ID.
- `set_event_alert_lead`: change the event alert lead time in minutes.
- `set_event_notification_sound`: change the local sound used by event alerts.

Skill context includes upcoming events, the next event, currently ringing event alerts, the configured alert lead minutes, and the configured notification sound. Secrets and raw credentials are never exposed in skill context.
