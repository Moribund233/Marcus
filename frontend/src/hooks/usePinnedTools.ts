import { useState, useCallback } from 'react'

const STORAGE_KEY = 'marcus_pinned_tools'

function loadPinned(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function savePinned(ids: string[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(ids))
}

export function usePinnedTools() {
  const [pinnedIds, setPinnedIds] = useState<string[]>(loadPinned)

  const togglePin = useCallback((id: string) => {
    setPinnedIds((prev) => {
      const next = prev.includes(id)
        ? prev.filter((x) => x !== id)
        : [...prev, id]
      savePinned(next)
      return next
    })
  }, [])

  const isPinned = useCallback((id: string) => {
    return pinnedIds.includes(id)
  }, [pinnedIds])

  return { pinnedIds, togglePin, isPinned }
}
