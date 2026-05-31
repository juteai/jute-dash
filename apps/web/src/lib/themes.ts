import type { DisplayConfig, WidgetChrome, WidgetInstance } from '$lib/types';

export type ResolvedThemeMode = 'light' | 'dark';

type ThemeTokens = Record<string, string>;

const juteMono: Record<ResolvedThemeMode, ThemeTokens> = {
  light: {
    background: '#ffffff',
    foreground: '#000000',
    surface: '#ffffff',
    surfaceMuted: '#f7f7f7',
    surfaceStrong: '#eeeeee',
    border: '#d8d8d8',
    borderStrong: '#000000',
    muted: '#5f5f5f',
    mutedStrong: '#2d2d2d',
    inverse: '#ffffff',
    accent: '#111111',
    danger: '#b42318',
    warning: '#8a5a00',
    success: '#147a3d',
    active: '#155eef',
    shadow: 'rgba(0, 0, 0, 0.12)',
    focus: '#000000'
  },
  dark: {
    background: '#000000',
    foreground: '#ffffff',
    surface: '#000000',
    surfaceMuted: '#111111',
    surfaceStrong: '#1f1f1f',
    border: '#333333',
    borderStrong: '#ffffff',
    muted: '#a6a6a6',
    mutedStrong: '#dddddd',
    inverse: '#000000',
    accent: '#ffffff',
    danger: '#ffb4ab',
    warning: '#ffd28a',
    success: '#8de6ad',
    active: '#adc6ff',
    shadow: 'rgba(255, 255, 255, 0.12)',
    focus: '#ffffff'
  }
};

export function resolveColorMode(display: DisplayConfig, systemPrefersDark: boolean): ResolvedThemeMode {
  const colorMode = display.colorMode || display.theme || 'system';
  if (colorMode === 'dark') {
    return 'dark';
  }
  if (colorMode === 'light') {
    return 'light';
  }
  return systemPrefersDark ? 'dark' : 'light';
}

export function displayThemeStyle(display: DisplayConfig, mode: ResolvedThemeMode): string {
  const tokens = juteMono[mode];
  const background = display.background ?? {
    kind: 'theme',
    value: '',
    fit: 'cover',
    position: 'center',
    overlay: 'none'
  };
  const backgroundColor = background.kind === 'color' && background.value ? background.value : tokens.background;
  const image = background.kind === 'asset' && background.value ? `url("${cssEscapeURL(background.value)}")` : 'none';
  const repeat = background.fit === 'tile' ? 'repeat' : 'no-repeat';
  const size = background.fit === 'tile' ? 'auto' : background.fit === 'contain' ? 'contain' : 'cover';

  return [
    cssVar('background', backgroundColor),
    cssVar('foreground', tokens.foreground),
    cssVar('surface', tokens.surface),
    cssVar('surface-muted', tokens.surfaceMuted),
    cssVar('surface-strong', tokens.surfaceStrong),
    cssVar('border', tokens.border),
    cssVar('border-strong', tokens.borderStrong),
    cssVar('muted', tokens.muted),
    cssVar('muted-strong', tokens.mutedStrong),
    cssVar('inverse', tokens.inverse),
    cssVar('accent', tokens.accent),
    cssVar('danger', tokens.danger),
    cssVar('warning', tokens.warning),
    cssVar('success', tokens.success),
    cssVar('active', tokens.active),
    cssVar('shadow', tokens.shadow),
    cssVar('focus', tokens.focus),
    cssVar('display-background-image', image),
    cssVar('display-background-size', size),
    cssVar('display-background-repeat', repeat),
    cssVar('display-background-position', background.position || 'center'),
    cssVar('display-background-overlay', overlayColor(background.overlay, mode))
  ].join(' ');
}

export function resolveWidgetChrome(
  widget: WidgetInstance,
  display: DisplayConfig
): Exclude<WidgetChrome, 'auto'> {
  const widgetChrome = String(widget.settings?.chrome ?? '').trim();
  const requested = widgetChrome || display.widgetChrome?.default || 'solid';
  if (requested === 'clear' || requested === 'smoked' || requested === 'frosted' || requested === 'solid') {
    return requested;
  }
  const hasBackground = display.background?.kind === 'asset' || display.background?.kind === 'file';
  return hasBackground ? 'smoked' : 'solid';
}

function cssVar(name: string, value: string) {
  return `--${name}: ${value};`;
}

function cssEscapeURL(value: string) {
  return value.replaceAll('\\', '\\\\').replaceAll('"', '\\"');
}

function overlayColor(overlay: string, mode: ResolvedThemeMode) {
  switch (overlay) {
    case 'dim':
      return mode === 'dark' ? 'rgba(0, 0, 0, 0.42)' : 'rgba(255, 255, 255, 0.28)';
    case 'smoked':
      return mode === 'dark' ? 'rgba(0, 0, 0, 0.62)' : 'rgba(255, 255, 255, 0.58)';
    case 'frosted':
      return mode === 'dark' ? 'rgba(0, 0, 0, 0.48)' : 'rgba(255, 255, 255, 0.42)';
    default:
      return 'transparent';
  }
}
