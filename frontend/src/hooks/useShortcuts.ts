import { useEffect } from 'react'

interface ShortcutMap {
  onCommandPalette?: () => void
  onAddTool?: () => void
  onRefresh?: () => void
  onSettings?: () => void
  onBack?: () => void
}

export function useShortcuts(handlers: ShortcutMap) {
  useEffect(() => {
    function handler(e: KeyboardEvent) {
      const ctrl = e.ctrlKey || e.metaKey
      if (!ctrl) return

      switch (e.key.toLowerCase()) {
        case 'k':
          e.preventDefault()
          handlers.onCommandPalette?.()
          break
        case 'n':
          e.preventDefault()
          handlers.onAddTool?.()
          break
        case 'r':
          e.preventDefault()
          handlers.onRefresh?.()
          break
        case 'e':
          e.preventDefault()
          handlers.onSettings?.()
          break
        case 'w':
          e.preventDefault()
          handlers.onBack?.()
          break
      }
    }

    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [handlers])
}
