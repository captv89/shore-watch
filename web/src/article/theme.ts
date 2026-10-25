import { useCallback, useEffect, useState } from 'react'

// Light by default, dark when the visitor's system asks for it, and a
// toggle that overrides both. index.html sets the attribute
// before the first paint, so the page never flashes the wrong theme.
export type Theme = 'light' | 'dark'
const KEY = 'shorewatch-theme'

const current = (): Theme => (document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')

export function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(current)
  useEffect(() => {
    // Follow the system while the visitor has not chosen.
    const m = window.matchMedia?.('(prefers-color-scheme: dark)')
    if (!m) return
    const f = () => {
      if (localStorage.getItem(KEY)) return
      document.documentElement.dataset.theme = m.matches ? 'dark' : 'light'
      setTheme(current())
    }
    m.addEventListener('change', f)
    return () => m.removeEventListener('change', f)
  }, [])
  const toggle = useCallback(() => {
    const next: Theme = current() === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = next
    try { localStorage.setItem(KEY, next) } catch { /* private mode: the choice lasts this visit */ }
    setTheme(next)
  }, [])
  return [theme, toggle]
}
