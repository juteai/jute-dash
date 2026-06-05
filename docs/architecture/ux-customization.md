# UX Customization

## Experience Goal

Jute should feel like a calm home dashboard rather than a web admin tool. The default experience is close to an Echo Show: glanceable status, large touch targets, ambient mode, voice-ready interaction, and enough personalization for different rooms and households.

The canonical clean-slate display UX is specified in [Display UX](display-ux.md). The current `apps/web` dashboard is throwaway POC work and should not constrain the final visual system.

Runtime resilience, hub disconnects, stale data, no-agent states, and safe user-facing error copy are specified in [Resilience And Error UX](resilience-error-ux.md).

Full-display themes, background images, and widget transparency are specified in [Visual Customization](visual-customization.md).

## Display Surfaces

Supported surfaces:

- kitchen or hallway wall display;
- tablet on a stand;
- desktop browser;
- TV or large kiosk screen;
- phone browser for quick control;
- future headless voice node with no visual display.

The same hub can serve multiple surfaces, each with its own device profile.

## Responsive Layout

The dashboard is mobile-first and uses a single authored layout across all surfaces:

- a layout profile stores widget placement once on a **12-column base grid**;
- each surface renders a **proportional remap** of that base to a target column count chosen by viewport width (phone → 1–2, small tablet → 4–6, large tablet/desktop/wall → 12);
- the remap is deterministic and read-only — smaller surfaces never overwrite the stored base layout, so there are no per-device stored arrangements to maintain;
- because the phone layout is derived, on-phone editing is limited to reorder, configure, headless toggle, add, and remove; fine drag/resize placement happens on tablet and larger;
- this keeps one source of truth per profile while still adapting cleanly from a phone to a wall display.

Device profiles still carry per-surface preferences (density, interaction mode, preferred agent, ambient behavior); they do not carry separate widget arrangements.

## Layout Profiles

A layout profile defines:

- grid density;
- visible widgets;
- widget order and size;
- ambient mode behavior;
- preferred agent;
- interaction mode: `touch`, `keyboard`, `remote`, or `voice`;
- room or household scope.

Default profiles:

- `morning`: weather, calendar, commute, energy, reminders.
- `day`: rooms, devices, cameras, agent dock.
- `evening`: scenes, media, security, tomorrow summary.
- `ambient`: clock, weather, photos or minimal status.
- `headless`: no visual widgets, voice and automation context only.

## Conversation UI

Voice conversations use an Echo Show-style layer over the dashboard rather than replacing the dashboard.

Conversation UI requirements:

- bottom sheet on tablet and ordinary display layouts;
- side sheet on large wall displays when it preserves more dashboard context;
- large listening orb or ring for active listening states;
- transcript bubbles for user and assistant turns;
- compact task progress while the agent is thinking;
- always-visible mute and cancel controls while voice is active;
- clear distinction between wake listening and follow-up listening.
- clear speaking state when TTS playback is active;
- stop and mute controls for spoken responses;
- visual response fallback when TTS is disabled or fails.

After an assistant response, the UI enters follow-up listening for 8 seconds by default. During that window, the user can continue without repeating the wake word. Ambient mode shows listening/responding status by default, not full transcripts.

## Voice Provider Customization

Voice settings are configured per device profile through the hub API.

Users can choose:

- STT provider pack;
- TTS provider pack;
- microphone profile;
- language and locale;
- TTS voice;
- TTS speed and volume;
- cloud STT/TTS opt-in;
- command-provider enablement for trusted local wrappers;
- sensitive-output speech policy.

The settings UI should show provider health as `available`, `misconfigured`, `offline`, `degraded`, or `disabled`. Voice preview must use explicit user-confirmed text, not household data or recent transcripts.

## Themes

Theme settings include:

- Theme Pack ID;
- color mode: `system`, `light`, `dark`;
- density: `comfortable`, `compact`, `large-touch`;
- motion level: `full`, `reduced`, `none`;
- contrast level: `standard`, `high`;
- typography scale;
- background style;
- default widget chrome.

Use shadcn-svelte conventions for accessible UI primitives and stable Theme Pack tokens for theme application. Theme Packs are repo-contributed data records, similar to code editor themes, and must not contain executable code.

## Widget Customization

Users can:

- add and remove widgets;
- resize widgets to supported sizes;
- reorder widgets;
- configure widget settings;
- override widget chrome as `solid`, `clear`, `smoked`, `frosted`, or `auto`;
- grant or revoke widget permissions;
- decide whether a widget can expose public context to agents.

Widget layout is persisted per device profile, with optional shared household layouts.

Widget placement, edit mode, and `WidgetFrame` behavior are specified in [Display UX](display-ux.md).

## Persistence

The hub persists customization in SQLite:

- household defaults;
- user preferences when user profiles exist;
- device profile overrides;
- layout profiles;
- widget settings and permissions;
- selected default agent per surface.

Config precedence:

1. command-line flags and environment variables for server boot behavior;
2. optional YAML or JSON bootstrap config for empty-store initialization;
3. SQLite household settings;
4. SQLite device profile settings;
5. transient browser state for non-durable UI only.

The settings UI writes durable changes through the hub API.

Configuration storage and first-run setup details are specified in [Configuration And Persistence](configuration-persistence.md).

## Ambient Mode

Ambient mode activates after an idle timeout or by schedule. It shows reduced information with lower visual noise.

Ambient mode may show:

- time and date;
- weather;
- next calendar item;
- home security state;
- active alerts;
- selected photo source later;
- active task progress when an agent is working;
- voice listening or responding status.

Ambient mode should avoid sensitive data by default.

When hub data becomes stale or the display is reconnecting, ambient mode may continue showing the last in-memory time and dashboard shell, but it must clearly mark hub-backed data as stale. Ambient mode must not hide a hub disconnect as if the system were fully live.

## Accessibility

Jute must support:

- keyboard navigation;
- screen-reader labels;
- high-contrast mode;
- reduced motion;
- large touch targets;
- text that does not overflow compact widgets;
- predictable focus order.

All contributed widgets must support keyboard navigation, screen reader labels, reduced motion, and high contrast behavior.
