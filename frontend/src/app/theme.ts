import { useEffect, useState } from 'react'

export type ThemeMode = 'system' | 'dark' | 'light'
export type ResolvedTheme = 'dark' | 'light'
const STORAGE_KEY = 'shuttle-theme'

function resolve(mode: ThemeMode): ResolvedTheme {
  if (mode !== 'system') return mode
  try {
    return matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

function storedMode(): ThemeMode {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    return value === 'dark' || value === 'light' ? value : 'system'
  } catch {
    return 'system'
  }
}

// Applies the persisted mode before React renders so the first paint already matches.
// color-scheme is pinned to the resolved theme: a "dark light" meta makes WebView2 paint
// a light backplate under dark surfaces, which reads as white/black flashing on redraws.
export function initTheme() {
  const effective = resolve(storedMode())
  document.documentElement.dataset.theme = effective
  document.documentElement.style.colorScheme = effective
}

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(storedMode)
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolve(storedMode()))

  useEffect(() => {
    const apply = () => {
      const effective = resolve(mode)
      document.documentElement.dataset.theme = effective
      document.documentElement.style.colorScheme = effective
      setResolved(effective)
    }
    apply()
    try { localStorage.setItem(STORAGE_KEY, mode) } catch { /* private mode: theme not persisted */ }
    const media = matchMedia('(prefers-color-scheme: light)')
    media.addEventListener('change', apply)
    return () => media.removeEventListener('change', apply)
  }, [mode])

  return { mode, resolved, setMode }
}
