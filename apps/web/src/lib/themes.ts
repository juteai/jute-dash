import type { DisplayConfig, WidgetChrome, WidgetInstance } from '$lib/types';

export type ResolvedThemeMode = 'light' | 'dark';

type ThemeTokens = Record<string, string>;

export const themeOptions = [
  { id: 'jute-mono', name: 'Jute Mono' },
  { id: 'solarized', name: 'Solarized' },
  { id: 'ayu', name: 'Ayu' },
  { id: 'one-dark', name: 'One Dark' },
  { id: 'gruvbox', name: 'Gruvbox' },
  { id: 'dracula', name: 'Dracula' },
  { id: 'catppuccin', name: 'Catppuccin' },
  { id: 'nord', name: 'Nord' },
  { id: 'tokyo-night', name: 'Tokyo Night' },
  { id: 'kanagawa', name: 'Kanagawa' },
  { id: 'monokai', name: 'Monokai' },
  { id: 'material', name: 'Material' },
  { id: 'github', name: 'GitHub' },
  { id: 'everforest', name: 'Everforest' }
] as const;

const themeTokens: Record<string, Record<ResolvedThemeMode, ThemeTokens>> = {
  'jute-mono': {
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
  },
  solarized: {
    light: {
      background: '#fdf6e3',
      foreground: '#073642',
      surface: '#fdf6e3',
      surfaceMuted: '#eee8d5',
      surfaceStrong: '#e4dcc4',
      border: '#d8cfb7',
      borderStrong: '#586e75',
      muted: '#657b83',
      mutedStrong: '#586e75',
      inverse: '#fdf6e3',
      accent: '#268bd2',
      danger: '#dc322f',
      warning: '#b58900',
      success: '#859900',
      active: '#268bd2',
      shadow: 'rgba(88, 110, 117, 0.18)',
      focus: '#268bd2'
    },
    dark: {
      background: '#002b36',
      foreground: '#eee8d5',
      surface: '#073642',
      surfaceMuted: '#002b36',
      surfaceStrong: '#0b3f4f',
      border: '#164b5c',
      borderStrong: '#93a1a1',
      muted: '#93a1a1',
      mutedStrong: '#eee8d5',
      inverse: '#002b36',
      accent: '#2aa198',
      danger: '#dc322f',
      warning: '#b58900',
      success: '#859900',
      active: '#268bd2',
      shadow: 'rgba(0, 0, 0, 0.28)',
      focus: '#2aa198'
    }
  },
  ayu: {
    light: {
      background: '#fafafa',
      foreground: '#5c6773',
      surface: '#ffffff',
      surfaceMuted: '#f3f4f5',
      surfaceStrong: '#eceff1',
      border: '#d8dce0',
      borderStrong: '#5c6773',
      muted: '#828c99',
      mutedStrong: '#5c6773',
      inverse: '#ffffff',
      accent: '#ff9940',
      danger: '#f07178',
      warning: '#ffb454',
      success: '#86b300',
      active: '#399ee6',
      shadow: 'rgba(92, 103, 115, 0.16)',
      focus: '#ff9940'
    },
    dark: {
      background: '#0f1419',
      foreground: '#e6e1cf',
      surface: '#14191f',
      surfaceMuted: '#1f2430',
      surfaceStrong: '#232936',
      border: '#2d3640',
      borderStrong: '#e6e1cf',
      muted: '#95a2b3',
      mutedStrong: '#d2cbb7',
      inverse: '#0f1419',
      accent: '#ffb454',
      danger: '#f07178',
      warning: '#ffb454',
      success: '#aad94c',
      active: '#59c2ff',
      shadow: 'rgba(0, 0, 0, 0.35)',
      focus: '#ffb454'
    }
  },
  'one-dark': {
    light: {
      background: '#fafafa',
      foreground: '#383a42',
      surface: '#ffffff',
      surfaceMuted: '#f0f0f1',
      surfaceStrong: '#e5e5e6',
      border: '#d4d4d5',
      borderStrong: '#383a42',
      muted: '#696c77',
      mutedStrong: '#4f535d',
      inverse: '#ffffff',
      accent: '#4078f2',
      danger: '#e45649',
      warning: '#c18401',
      success: '#50a14f',
      active: '#4078f2',
      shadow: 'rgba(56, 58, 66, 0.16)',
      focus: '#4078f2'
    },
    dark: {
      background: '#282c34',
      foreground: '#abb2bf',
      surface: '#21252b',
      surfaceMuted: '#2c313a',
      surfaceStrong: '#353b45',
      border: '#3e4451',
      borderStrong: '#abb2bf',
      muted: '#8b95a7',
      mutedStrong: '#c8ccd4',
      inverse: '#282c34',
      accent: '#61afef',
      danger: '#e06c75',
      warning: '#e5c07b',
      success: '#98c379',
      active: '#61afef',
      shadow: 'rgba(0, 0, 0, 0.32)',
      focus: '#61afef'
    }
  },
  gruvbox: {
    light: {
      background: '#fbf1c7',
      foreground: '#3c3836',
      surface: '#f9f5d7',
      surfaceMuted: '#ebdbb2',
      surfaceStrong: '#d5c4a1',
      border: '#d5c4a1',
      borderStrong: '#504945',
      muted: '#7c6f64',
      mutedStrong: '#504945',
      inverse: '#fbf1c7',
      accent: '#076678',
      danger: '#cc241d',
      warning: '#b57614',
      success: '#79740e',
      active: '#458588',
      shadow: 'rgba(60, 56, 54, 0.18)',
      focus: '#af3a03'
    },
    dark: {
      background: '#282828',
      foreground: '#ebdbb2',
      surface: '#32302f',
      surfaceMuted: '#3c3836',
      surfaceStrong: '#504945',
      border: '#504945',
      borderStrong: '#ebdbb2',
      muted: '#a89984',
      mutedStrong: '#d5c4a1',
      inverse: '#282828',
      accent: '#83a598',
      danger: '#fb4934',
      warning: '#fabd2f',
      success: '#b8bb26',
      active: '#83a598',
      shadow: 'rgba(0, 0, 0, 0.34)',
      focus: '#fe8019'
    }
  },
  dracula: {
    light: {
      background: '#f8f8f2',
      foreground: '#282a36',
      surface: '#ffffff',
      surfaceMuted: '#eeeef4',
      surfaceStrong: '#e3e4ee',
      border: '#d8d9e6',
      borderStrong: '#44475a',
      muted: '#62677d',
      mutedStrong: '#44475a',
      inverse: '#f8f8f2',
      accent: '#6272a4',
      danger: '#ff5555',
      warning: '#ffb86c',
      success: '#50fa7b',
      active: '#6272a4',
      shadow: 'rgba(40, 42, 54, 0.16)',
      focus: '#bd93f9'
    },
    dark: {
      background: '#282a36',
      foreground: '#f8f8f2',
      surface: '#1f2130',
      surfaceMuted: '#343746',
      surfaceStrong: '#44475a',
      border: '#44475a',
      borderStrong: '#f8f8f2',
      muted: '#b6b8c8',
      mutedStrong: '#f8f8f2',
      inverse: '#282a36',
      accent: '#bd93f9',
      danger: '#ff5555',
      warning: '#ffb86c',
      success: '#50fa7b',
      active: '#8be9fd',
      shadow: 'rgba(0, 0, 0, 0.34)',
      focus: '#ff79c6'
    }
  },
  catppuccin: {
    light: {
      background: '#eff1f5',
      foreground: '#4c4f69',
      surface: '#ffffff',
      surfaceMuted: '#e6e9ef',
      surfaceStrong: '#dce0e8',
      border: '#ccd0da',
      borderStrong: '#4c4f69',
      muted: '#6c6f85',
      mutedStrong: '#5c5f77',
      inverse: '#eff1f5',
      accent: '#8839ef',
      danger: '#d20f39',
      warning: '#df8e1d',
      success: '#40a02b',
      active: '#1e66f5',
      shadow: 'rgba(76, 79, 105, 0.16)',
      focus: '#8839ef'
    },
    dark: {
      background: '#1e1e2e',
      foreground: '#cdd6f4',
      surface: '#181825',
      surfaceMuted: '#313244',
      surfaceStrong: '#45475a',
      border: '#585b70',
      borderStrong: '#cdd6f4',
      muted: '#a6adc8',
      mutedStrong: '#bac2de',
      inverse: '#1e1e2e',
      accent: '#cba6f7',
      danger: '#f38ba8',
      warning: '#f9e2af',
      success: '#a6e3a1',
      active: '#89b4fa',
      shadow: 'rgba(0, 0, 0, 0.34)',
      focus: '#cba6f7'
    }
  },
  nord: {
    light: {
      background: '#eceff4',
      foreground: '#2e3440',
      surface: '#ffffff',
      surfaceMuted: '#e5e9f0',
      surfaceStrong: '#d8dee9',
      border: '#c8d0dd',
      borderStrong: '#3b4252',
      muted: '#5e6778',
      mutedStrong: '#4c566a',
      inverse: '#eceff4',
      accent: '#5e81ac',
      danger: '#bf616a',
      warning: '#d08770',
      success: '#a3be8c',
      active: '#5e81ac',
      shadow: 'rgba(46, 52, 64, 0.16)',
      focus: '#88c0d0'
    },
    dark: {
      background: '#2e3440',
      foreground: '#eceff4',
      surface: '#3b4252',
      surfaceMuted: '#343a46',
      surfaceStrong: '#434c5e',
      border: '#4c566a',
      borderStrong: '#eceff4',
      muted: '#d8dee9',
      mutedStrong: '#e5e9f0',
      inverse: '#2e3440',
      accent: '#88c0d0',
      danger: '#bf616a',
      warning: '#ebcb8b',
      success: '#a3be8c',
      active: '#81a1c1',
      shadow: 'rgba(0, 0, 0, 0.32)',
      focus: '#88c0d0'
    }
  },
  'tokyo-night': {
    light: {
      background: '#d5d6db',
      foreground: '#343b59',
      surface: '#ffffff',
      surfaceMuted: '#e1e2e7',
      surfaceStrong: '#cbccd4',
      border: '#b8bac6',
      borderStrong: '#343b59',
      muted: '#5a6182',
      mutedStrong: '#454b6b',
      inverse: '#d5d6db',
      accent: '#34548a',
      danger: '#8c4351',
      warning: '#8f5e15',
      success: '#485e30',
      active: '#34548a',
      shadow: 'rgba(52, 59, 89, 0.16)',
      focus: '#5a4a78'
    },
    dark: {
      background: '#1a1b26',
      foreground: '#c0caf5',
      surface: '#16161e',
      surfaceMuted: '#24283b',
      surfaceStrong: '#292e42',
      border: '#3b4261',
      borderStrong: '#c0caf5',
      muted: '#9aa5ce',
      mutedStrong: '#a9b1d6',
      inverse: '#1a1b26',
      accent: '#7aa2f7',
      danger: '#f7768e',
      warning: '#e0af68',
      success: '#9ece6a',
      active: '#7aa2f7',
      shadow: 'rgba(0, 0, 0, 0.36)',
      focus: '#bb9af7'
    }
  },
  kanagawa: {
    light: {
      background: '#f2ecbc',
      foreground: '#545464',
      surface: '#f7f4d1',
      surfaceMuted: '#e6ddb2',
      surfaceStrong: '#d9cfa0',
      border: '#c8bc8f',
      borderStrong: '#545464',
      muted: '#6f6f83',
      mutedStrong: '#545464',
      inverse: '#f2ecbc',
      accent: '#4b699b',
      danger: '#c84053',
      warning: '#b35c00',
      success: '#6f894e',
      active: '#4b699b',
      shadow: 'rgba(84, 84, 100, 0.17)',
      focus: '#b35c00'
    },
    dark: {
      background: '#1f1f28',
      foreground: '#dcd7ba',
      surface: '#16161d',
      surfaceMuted: '#2a2a37',
      surfaceStrong: '#363646',
      border: '#54546d',
      borderStrong: '#dcd7ba',
      muted: '#a6a69c',
      mutedStrong: '#c8c093',
      inverse: '#1f1f28',
      accent: '#7e9cd8',
      danger: '#e46876',
      warning: '#ffa066',
      success: '#98bb6c',
      active: '#7fb4ca',
      shadow: 'rgba(0, 0, 0, 0.36)',
      focus: '#ffa066'
    }
  },
  monokai: {
    light: {
      background: '#f8f8f2',
      foreground: '#272822',
      surface: '#ffffff',
      surfaceMuted: '#eeeeea',
      surfaceStrong: '#dfdfd8',
      border: '#d2d2c8',
      borderStrong: '#49483e',
      muted: '#6f6f64',
      mutedStrong: '#49483e',
      inverse: '#f8f8f2',
      accent: '#66d9ef',
      danger: '#f92672',
      warning: '#fd971f',
      success: '#a6e22e',
      active: '#66d9ef',
      shadow: 'rgba(39, 40, 34, 0.17)',
      focus: '#ae81ff'
    },
    dark: {
      background: '#272822',
      foreground: '#f8f8f2',
      surface: '#1f201b',
      surfaceMuted: '#3a3b32',
      surfaceStrong: '#49483e',
      border: '#5b5a4d',
      borderStrong: '#f8f8f2',
      muted: '#cfcfc2',
      mutedStrong: '#f8f8f2',
      inverse: '#272822',
      accent: '#66d9ef',
      danger: '#f92672',
      warning: '#fd971f',
      success: '#a6e22e',
      active: '#66d9ef',
      shadow: 'rgba(0, 0, 0, 0.34)',
      focus: '#ae81ff'
    }
  },
  material: {
    light: {
      background: '#fafafa',
      foreground: '#263238',
      surface: '#ffffff',
      surfaceMuted: '#eceff1',
      surfaceStrong: '#dfe5e8',
      border: '#cfd8dc',
      borderStrong: '#37474f',
      muted: '#607d8b',
      mutedStrong: '#455a64',
      inverse: '#fafafa',
      accent: '#009688',
      danger: '#e53935',
      warning: '#ff9800',
      success: '#43a047',
      active: '#2196f3',
      shadow: 'rgba(38, 50, 56, 0.16)',
      focus: '#00acc1'
    },
    dark: {
      background: '#263238',
      foreground: '#eeffff',
      surface: '#1e272c',
      surfaceMuted: '#2f3f46',
      surfaceStrong: '#37474f',
      border: '#455a64',
      borderStrong: '#eeffff',
      muted: '#b0bec5',
      mutedStrong: '#d9f1f1',
      inverse: '#263238',
      accent: '#80cbc4',
      danger: '#ff5370',
      warning: '#ffcb6b',
      success: '#c3e88d',
      active: '#82aaff',
      shadow: 'rgba(0, 0, 0, 0.34)',
      focus: '#89ddff'
    }
  },
  github: {
    light: {
      background: '#ffffff',
      foreground: '#24292f',
      surface: '#ffffff',
      surfaceMuted: '#f6f8fa',
      surfaceStrong: '#eaeef2',
      border: '#d0d7de',
      borderStrong: '#24292f',
      muted: '#57606a',
      mutedStrong: '#3d444d',
      inverse: '#ffffff',
      accent: '#0969da',
      danger: '#cf222e',
      warning: '#9a6700',
      success: '#1a7f37',
      active: '#0969da',
      shadow: 'rgba(31, 35, 40, 0.15)',
      focus: '#0969da'
    },
    dark: {
      background: '#0d1117',
      foreground: '#e6edf3',
      surface: '#161b22',
      surfaceMuted: '#21262d',
      surfaceStrong: '#30363d',
      border: '#30363d',
      borderStrong: '#e6edf3',
      muted: '#8b949e',
      mutedStrong: '#c9d1d9',
      inverse: '#0d1117',
      accent: '#2f81f7',
      danger: '#f85149',
      warning: '#d29922',
      success: '#3fb950',
      active: '#2f81f7',
      shadow: 'rgba(0, 0, 0, 0.35)',
      focus: '#58a6ff'
    }
  },
  everforest: {
    light: {
      background: '#fdf6e3',
      foreground: '#5c6a72',
      surface: '#fffbea',
      surfaceMuted: '#f4f0d9',
      surfaceStrong: '#e6e0c2',
      border: '#d8d0ad',
      borderStrong: '#5c6a72',
      muted: '#829181',
      mutedStrong: '#708089',
      inverse: '#fdf6e3',
      accent: '#3a94c5',
      danger: '#f85552',
      warning: '#dfa000',
      success: '#8da101',
      active: '#35a77c',
      shadow: 'rgba(92, 106, 114, 0.16)',
      focus: '#35a77c'
    },
    dark: {
      background: '#2b3339',
      foreground: '#d3c6aa',
      surface: '#232a2e',
      surfaceMuted: '#323c41',
      surfaceStrong: '#3f4b50',
      border: '#4f5b58',
      borderStrong: '#d3c6aa',
      muted: '#a7c080',
      mutedStrong: '#d3c6aa',
      inverse: '#2b3339',
      accent: '#7fbbb3',
      danger: '#e67e80',
      warning: '#dbbc7f',
      success: '#a7c080',
      active: '#83c092',
      shadow: 'rgba(0, 0, 0, 0.34)',
      focus: '#83c092'
    }
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
  const tokens = themeTokens[display.themeId]?.[mode] ?? themeTokens['jute-mono'][mode];
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
