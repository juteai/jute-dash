# Theme Developer Guidelines

## Overview

Jute themes are repo-contributed Theme Packs. They work like code editor themes: a theme supplies visual tokens, while Jute supplies layout, components, widget behavior, permissions, and agent context.

Themes are data-only. Do not add JavaScript, HTML, runtime hooks, remote imports, or behavior changes to a theme contribution.

Architecture details live in [Visual Customization](../architecture/visual-customization.md).

Bundled starter themes are `jute-mono`, `solarized`, `ayu`, `one-dark`, `gruvbox`, `dracula`, `catppuccin`, `nord`, `tokyo-night`, `kanagawa`, `monokai`, `material`, `github`, and `everforest`. The Display reads Theme Pack token data from `themes/[theme-id]/theme.json`; new bundled themes must also be allowed by the hub display config validation.

## Folder Structure

Theme Packs use this shape:

```text
themes/
  jute-mono/
    theme.json
    README.md
    screenshots/
      dashboard-light.png
      dashboard-dark.png
```

`theme.json` is the source of truth for Display tokens. `README.md` explains the intent, accessibility notes, and supported modes.

## Manifest Requirements

Each `theme.json` must declare:

- `id`: stable lowercase ID;
- `name`: user-facing name;
- `version`: semantic version;
- `description`;
- `author`;
- `supportedModes`: `light`, `dark`, or both;
- `accessibility`: contrast and motion notes;
- `modes`: `light` and/or `dark` token maps using the stable Display token names.

Required token groups are defined in [Visual Customization](../architecture/visual-customization.md).

## Token Rules

Themes must provide tokens for:

- app shell backgrounds, foregrounds, surfaces, borders, shadow, and focus;
- primary, secondary, muted, and inverse text;
- danger, warning, success, active, and info states;
- chat bubbles, system rows, and streaming indicators;
- widget frames and widget chrome modes;
- sheets, modals, scrims, smoked overlays, and frosted overlays.

Use stable token names. Do not create one-off component-specific tokens unless the visual customization architecture is updated first.

## Widget Chrome Support

Every contributed theme must keep widgets readable in:

- `solid`;
- `clear`;
- `smoked`;
- `frosted`;
- `auto`.

If a visual treatment is not recommended, document why in the theme README. The host may still fall back to `solid` when contrast is not safe.

## Backgrounds

Themes may define a default background token or packaged local asset.

Rules:

- no remote image URLs;
- no user-specific files in committed themes;
- no image metadata that includes private household data;
- background images must preserve readability with `solid` and `smoked` widget chrome.

## Accessibility Checklist

Before contributing a theme, verify:

- primary text meets WCAG AA contrast on app and widget surfaces;
- secondary text remains readable in compact widgets;
- focus rings are obvious in light and dark modes;
- semantic states are not communicated by color alone;
- reduced-motion mode removes decorative motion;
- frosted or transparent surfaces remain legible over supported backgrounds.

## Contribution Checklist

1. Add `themes/[theme-id]/theme.json`.
2. Add a theme README with intent, supported modes, and accessibility notes.
3. Include dashboard, settings, chat, and widget screenshots or documented smoke results, including `solid` and `smoked` widget chrome.
4. Update examples only if the theme should be demonstrated by a harness.
5. Run `make check` and `cd apps/web && npm run test:browser` when Display theme behavior changes.

Theme submissions should not modify widget behavior, hub behavior, A2A behavior, MCP tools, or voice behavior.
