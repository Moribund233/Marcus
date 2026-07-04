import { useState, useEffect, useCallback } from 'react'
import { GetRuntimeStatus } from '../../wailsjs/go/main/App'
import type { RuntimeInfo } from '@/components/renderer/types'

export function useRuntime() {
  const [status, setStatus] = useState<Record<string, RuntimeInfo>>({})
  const [loading, setLoading] = useState(false)

  const fetch = useCallback(async () => {
    setLoading(true)
    try {
      const result = await GetRuntimeStatus()
      setStatus(result)
    } catch {
      // silently fail
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetch()
  }, [fetch])

  return { status, loading, refresh: fetch }
}
