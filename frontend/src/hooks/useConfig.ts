import { useState, useEffect, useCallback } from 'react'
import { GetConfig, SaveConfig } from '../../wailsjs/go/main/App'
import { config } from '../../wailsjs/go/models'

export function useConfig() {
  const [cfg, setCfg] = useState<config.Config | null>(null)
  const [loading, setLoading] = useState(false)

  const fetch = useCallback(async () => {
    setLoading(true)
    try {
      const c = await GetConfig()
      setCfg(c)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetch()
  }, [fetch])

  const save = useCallback(async (patch: Partial<config.Config>) => {
    if (!cfg) return
    const next = config.Config.createFrom({ ...cfg, ...patch })
    try {
      await SaveConfig(next)
      setCfg(next)
    } catch {
      // ignore
    }
  }, [cfg])

  return { config: cfg, loading, fetch, save }
}
