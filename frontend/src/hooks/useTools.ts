import { useState, useEffect, useCallback } from 'react'
import {
  GetTools,
  LaunchTool,
  StopTool,
  GetToolState,
  DeleteTool,
  AddManualTool,
  RefreshTools,
  GetToolLogs,
} from '../../wailsjs/go/main/App'
import { model } from '../../wailsjs/go/models'

export function useTools(category: string) {
  const [tools, setTools] = useState<model.ToolInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await GetTools(category)
      setTools(result ?? [])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [category])

  useEffect(() => {
    fetch()
  }, [fetch])

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const result = await RefreshTools()
      setTools(result ?? [])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  const launch = useCallback(async (id: string, args: Record<string, string> = {}) => {
    return await LaunchTool(id, args)
  }, [])

  const stop = useCallback(async (id: string) => {
    await StopTool(id)
  }, [])

  const pollState = useCallback(async (id: string) => {
    return await GetToolState(id)
  }, [])

  const remove = useCallback(async (id: string) => {
    await DeleteTool(id)
    setTools((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const addManual = useCallback(async (name: string, command: string, argType: string) => {
    const tool = await AddManualTool(name, command, argType)
    setTools((prev) => [...prev, tool])
    return tool
  }, [])

  const getLogs = useCallback(async (id: string, limit: number = 20) => {
    return await GetToolLogs(id, limit)
  }, [])

  return {
    tools,
    loading,
    error,
    refetch: fetch,
    refresh,
    launch,
    stop,
    pollState,
    remove,
    addManual,
    getLogs,
  }
}
