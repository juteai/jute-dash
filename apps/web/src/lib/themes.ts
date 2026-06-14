import type { DisplayConfig, WidgetChrome, WidgetInstance } from '$lib/types';

export type ResolvedThemeMode = 'light' | 'dark';

type ThemeTokenName =
  | 'background'
  | 'foreground'
  | 'surface'
  | 'surfaceMuted'
  | 'surfaceStrong'
  | 'border'
  | 'borderStrong'
  | 'muted'
  | 'mutedStrong'
  | 'inverse'
  | 'accent'
  | 'danger'
  | 'warning'
  | 'success'
  | 'active'
  | 'focus'
  | 'shadow';

type ThemeTokens = Record<ThemeTokenName, string>;

type ThemeManifest = {
  id: string;
  name: string;
  supportedModes?: ResolvedThemeMode[];
  modes: Partial<Record<ResolvedThemeMode, Partial<ThemeTokens>>>;
};

const juteMonoFallback: Record<ResolvedThemeMode, ThemeTokens> = {
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
    focus: '#000000',
    shadow: 'rgba(0, 0, 0, 0.12)'
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
    focus: '#ffffff',
    shadow: 'rgba(255, 255, 255, 0.12)'
  }
};

const themeModules = import.meta.glob('../../../../themes/*/theme.json', {
  eager: true
}) as Record<string, { default?: ThemeManifest } | ThemeManifest>;

const themeManifests = Object.values(themeModules)
  .map((module) => ('default' in module ? module.default : module))
  .filter(isThemeManifest);

const themeRegistry = buildThemeRegistry(themeManifests);

export const themeOptions = Object.values(themeRegistry)
  .map((theme) => ({ id: theme.id, name: theme.name }))
  .sort((a, b) => {
    if (a.id === 'jute-mono') return -1;
    if (b.id === 'jute-mono') return 1;
    return a.name.localeCompare(b.name);
  });

export function resolveColorMode(
  display: DisplayConfig,
  systemPrefersDark: boolean
): ResolvedThemeMode {
  const colorMode = display.colorMode || display.theme || 'system';
  if (colorMode === 'dark') {
    return 'dark';
  }
  if (colorMode === 'light') {
    return 'light';
  }
  return systemPrefersDark ? 'dark' : 'light';
}

export function displayThemeStyle(
  display: DisplayConfig,
  mode: ResolvedThemeMode,
  resolvedImageURL = ''
): string {
  const tokens = resolveThemeTokens(display.themeId, mode);
  const background = display.background ?? {
    kind: 'theme',
    value: '',
    fit: 'cover',
    position: 'center',
    overlay: 'none'
  };
  const backgroundColor =
    background.kind === 'color' && background.value
      ? background.value
      : tokens.background;
  const imageURL =
    resolvedImageURL ||
    (background.kind === 'asset' && background.value ? background.value : '');
  const image = imageURL ? `url("${cssEscapeURL(imageURL)}")` : 'none';
  const repeat = background.fit === 'tile' ? 'repeat' : 'no-repeat';
  const size =
    background.fit === 'tile'
      ? 'auto'
      : background.fit === 'contain'
        ? 'contain'
        : 'cover';

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
    cssVar(
      'display-background-overlay',
      overlayColor(background.overlay, mode)
    ),
    cssVar(
      'smoked-opacity',
      String(display.widgetChrome?.smokedOpacity ?? 0.6)
    ),
    cssVar(
      'smoked-opacity-percent',
      `${Math.round((display.widgetChrome?.smokedOpacity ?? 0.6) * 100)}%`
    ),
    cssVar(
      'frosted-opacity',
      String(display.widgetChrome?.frostedOpacity ?? 0.3)
    ),
    cssVar(
      'frosted-opacity-percent',
      `${Math.round((display.widgetChrome?.frostedOpacity ?? 0.3) * 100)}%`
    )
  ].join(' ');
}

export function resolveWidgetChrome(
  widget: WidgetInstance,
  display: DisplayConfig
): Exclude<WidgetChrome, 'auto'> {
  const widgetChrome = String(widget.settings?.chrome ?? '').trim();
  const requested = widgetChrome || display.widgetChrome?.default || 'solid';
  if (
    requested === 'clear' ||
    requested === 'smoked' ||
    requested === 'frosted' ||
    requested === 'solid'
  ) {
    return requested;
  }
  const hasBackground =
    display.background?.kind === 'asset' ||
    display.background?.kind === 'file' ||
    display.background?.kind === 'slideshow' ||
    display.background?.kind === 'dynamic';
  return hasBackground ? 'smoked' : 'solid';
}

function buildThemeRegistry(manifests: ThemeManifest[]) {
  const registry: Record<
    string,
    { id: string; name: string; modes: Record<ResolvedThemeMode, ThemeTokens> }
  > = {};

  for (const manifest of manifests) {
    registry[manifest.id] = {
      id: manifest.id,
      name: manifest.name,
      modes: {
        light: normalizeTokens(manifest.modes.light, 'light'),
        dark: normalizeTokens(manifest.modes.dark, 'dark')
      }
    };
  }

  if (!registry['jute-mono']) {
    registry['jute-mono'] = {
      id: 'jute-mono',
      name: 'Jute Mono',
      modes: juteMonoFallback
    };
  }

  return registry;
}

function resolveThemeTokens(id: string, mode: ResolvedThemeMode) {
  return (
    themeRegistry[id]?.modes[mode] ?? themeRegistry['jute-mono'].modes[mode]
  );
}

function normalizeTokens(
  tokens: Partial<ThemeTokens> | undefined,
  mode: ResolvedThemeMode
): ThemeTokens {
  return {
    ...juteMonoFallback[mode],
    ...(tokens ?? {}),
    shadow: tokens?.shadow ?? juteMonoFallback[mode].shadow
  };
}

function isThemeManifest(value: unknown): value is ThemeManifest {
  if (!value || typeof value !== 'object') {
    return false;
  }
  const candidate = value as Partial<ThemeManifest>;
  return (
    typeof candidate.id === 'string' &&
    typeof candidate.name === 'string' &&
    !!candidate.modes &&
    typeof candidate.modes === 'object'
  );
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
      return mode === 'dark'
        ? 'rgba(0, 0, 0, 0.42)'
        : 'rgba(255, 255, 255, 0.28)';
    case 'smoked':
      return mode === 'dark'
        ? 'rgba(0, 0, 0, 0.62)'
        : 'rgba(255, 255, 255, 0.58)';
    case 'frosted':
      return mode === 'dark'
        ? 'rgba(0, 0, 0, 0.48)'
        : 'rgba(255, 255, 255, 0.42)';
    default:
      return 'transparent';
  }
}
