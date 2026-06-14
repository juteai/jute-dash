import { describe, expect, it } from 'vitest';
import {
  displayThemeStyle,
  resolveColorMode,
  resolveWidgetChrome,
  themeOptions
} from './themes';
import type { DisplayConfig, WidgetInstance } from './types';

const display: DisplayConfig = {
  theme: 'system',
  colorMode: 'system',
  themeId: 'jute-mono',
  density: 'comfortable',
  motion: 'none',
  background: {
    kind: 'theme',
    value: '',
    fit: 'cover',
    position: 'center',
    overlay: 'none'
  },
  widgetChrome: {
    default: 'solid',
    smokedOpacity: 0.6,
    frostedOpacity: 0.3
  },
  accentColor: '',
  idleMode: ''
};

const widget: WidgetInstance = {
  id: 'weather',
  kind: 'weather',
  title: 'Weather',
  x: 0,
  y: 0,
  w: 3,
  h: 2,
  minW: 2,
  minH: 1,
  size: 'medium',
  settings: {},
  visible: true
};

describe('theme style module', () => {
  it('loads Theme Pack options from theme manifests', () => {
    expect(themeOptions.map((option) => option.id)).toContain('jute-mono');
    expect(themeOptions.map((option) => option.id)).toContain('solarized');
  });

  it('resolves color mode from config and system preference', () => {
    expect(resolveColorMode({ ...display, colorMode: 'dark' }, false)).toBe(
      'dark'
    );
    expect(resolveColorMode({ ...display, colorMode: 'system' }, true)).toBe(
      'dark'
    );
    expect(resolveColorMode({ ...display, colorMode: 'system' }, false)).toBe(
      'light'
    );
  });

  it('falls back to jute-mono tokens for unknown themes', () => {
    const style = displayThemeStyle(
      { ...display, themeId: 'missing-theme' },
      'light'
    );
    expect(style).toContain('--background: #ffffff;');
    expect(style).toContain('--shadow: rgba(0, 0, 0, 0.12);');
  });

  it('resolves auto chrome based on background presence', () => {
    expect(resolveWidgetChrome(widget, display)).toBe('solid');
    expect(
      resolveWidgetChrome(widget, {
        ...display,
        widgetChrome: { default: 'auto' },
        background: {
          ...display.background,
          kind: 'file',
          value: 'kitchen.jpg'
        }
      })
    ).toBe('smoked');
  });

  it('prefers per-widget chrome over the display default', () => {
    expect(
      resolveWidgetChrome(
        { ...widget, settings: { chrome: 'clear' } },
        { ...display, widgetChrome: { default: 'solid' } }
      )
    ).toBe('clear');
  });
});
