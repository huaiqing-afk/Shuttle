// Palette data: ANSI order is black, red, green, yellow, blue, magenta, cyan, white,
// followed by the eight bright colors. Based on the named projects' terminal palettes.
import type { ITheme } from '@xterm/xterm'

const ansiKeys = [
  'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white',
  'brightBlack', 'brightRed', 'brightGreen', 'brightYellow', 'brightBlue', 'brightMagenta', 'brightCyan', 'brightWhite',
] as const

function palette(id: string, name: string, kind: 'dark' | 'light', background: string,
  foreground: string, cursor: string, selectionBackground: string, colors: string[]) {
  const theme: ITheme = { background, foreground, cursor, selectionBackground }
  ansiKeys.forEach((key, index) => { theme[key] = colors[index] })
  return { id, name, kind, theme }
}

export const terminalPalettes = [
  palette('tokyo-night', 'Tokyo Night', 'dark', '#16161e', '#c0caf5', '#7dcfff', '#33415e', [
    '#15161e', '#f7768e', '#9ece6a', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#a9b1d6',
    '#414868', '#f7768e', '#9ece6a', '#e0af68', '#7aa2f7', '#bb9af7', '#7dcfff', '#c0caf5',
  ]),
  palette('tokyo-day', 'Tokyo Day', 'light', '#f4f5f9', '#343b58', '#3760bf', '#c9d2ee', [
    '#b4b5b9', '#f52a65', '#587539', '#8c6c3e', '#2e7de9', '#9854f1', '#007197', '#6172b0',
    '#a1a6c5', '#ff4774', '#5c8524', '#a27629', '#358aff', '#a463ff', '#007ea8', '#3760bf',
  ]),
  palette('catppuccin-mocha', 'Catppuccin Mocha', 'dark', '#1e1e2e', '#cdd6f4', '#f5e0dc', '#45475a', [
    '#45475a', '#f38ba8', '#a6e3a1', '#f9e2af', '#89b4fa', '#f5c2e7', '#94e2d5', '#bac2de',
    '#585b70', '#f38ba8', '#a6e3a1', '#f9e2af', '#89b4fa', '#f5c2e7', '#94e2d5', '#a6adc8',
  ]),
  palette('catppuccin-latte', 'Catppuccin Latte', 'light', '#eff1f5', '#4c4f69', '#dc8a78', '#ccd0da', [
    '#5c5f77', '#d20f39', '#40a02b', '#df8e1d', '#1e66f5', '#ea76cb', '#179299', '#acb0be',
    '#6c6f85', '#d20f39', '#40a02b', '#df8e1d', '#1e66f5', '#ea76cb', '#179299', '#bcc0cc',
  ]),
  palette('dracula', 'Dracula', 'dark', '#282a36', '#f8f8f2', '#f8f8f2', '#44475a', [
    '#21222c', '#ff5555', '#50fa7b', '#f1fa8c', '#bd93f9', '#ff79c6', '#8be9fd', '#f8f8f2',
    '#6272a4', '#ff6e6e', '#69ff94', '#ffffa5', '#d6acff', '#ff92df', '#a4ffff', '#ffffff',
  ]),
  palette('nord', 'Nord', 'dark', '#2e3440', '#d8dee9', '#d8dee9', '#434c5e', [
    '#3b4252', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#88c0d0', '#e5e9f0',
    '#4c566a', '#bf616a', '#a3be8c', '#ebcb8b', '#81a1c1', '#b48ead', '#8fbcbb', '#eceff4',
  ]),
  palette('solarized-dark', 'Solarized Dark', 'dark', '#002b36', '#839496', '#93a1a1', '#073642', [
    '#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5',
    '#002b36', '#cb4b16', '#586e75', '#657b83', '#839496', '#6c71c4', '#93a1a1', '#fdf6e3',
  ]),
  palette('solarized-light', 'Solarized Light', 'light', '#fdf6e3', '#657b83', '#586e75', '#eee8d5', [
    '#073642', '#dc322f', '#859900', '#b58900', '#268bd2', '#d33682', '#2aa198', '#eee8d5',
    '#002b36', '#cb4b16', '#586e75', '#657b83', '#839496', '#6c71c4', '#93a1a1', '#fdf6e3',
  ]),
]

export const terminalPaletteKey = 'shuttle-terminal-palette'

export function normalizePalette(value: string | null): string {
  return terminalPalettes.some(palette => palette.id === value) ? value! : 'auto'
}

export function resolvePalette(choice: string, mode: 'dark' | 'light') {
  return terminalPalettes.find(palette => palette.id === choice) ?? terminalPalettes[mode === 'dark' ? 0 : 1]
}
