import { useState, useEffect, useCallback, useRef } from 'react'
import {
  GetTools,
  LaunchTool,
  StopTool,
  GetToolState,
  DeleteTool,
  AddManualTool,
  RefreshTools,
  GetToolLogs,
  UninstallTool,
} from '../../wailsjs/go/main/App'
import { model } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime'

export function useTools(category: string) {
  const [tools, setTools] = useState<model.ToolInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Cross-category cache: accumulates all seen tools so pinned tools
  // and the command palette can resolve any tool regardless of the
  // current category filter.
  const toolsById = useRef<Map<string, model.ToolInfo>>(new Map())

  // Merge incoming tools into the cross-category cache.
  const mergeIntoCache = useCallback((incoming: model.ToolInfo[]) => {
    for (const t of incoming) {
      toolsById.current.set(t.id, t)
    }
  }, [])

  // Evict entries that are no longer in the active tool set once the
  // cache exceeds a reasonable size.
  useEffect(() => {
    if (toolsById.current.size <= 300) return
    const activeIds = new Set(tools.map((t) => t.id))
    for (const id of toolsById.current.keys()) {
      if (!activeIds.has(id)) {
        toolsById.current.delete(id)
      }
    }
  }, [tools])

  const fetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const result = await GetTools(category)
      const list = result ?? []
      setTools(list)
      mergeIntoCache(list)
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [category, mergeIntoCache])

  useEffect(() => {
    fetch()
  }, [fetch])

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      const result = await RefreshTools()
      const list = result ?? []
      setTools(list)
      mergeIntoCache(list)
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [mergeIntoCache])

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
    toolsById.current.delete(id)
  }, [])

  const addManual = useCallback(async (name: string, command: string, argType: string) => {
    const tool = await AddManualTool(name, command, argType)
    setTools((prev) => [...prev, tool])
    toolsById.current.set(tool.id, tool)
    return tool
  }, [])

  const getLogs = useCallback(async (id: string, limit: number = 20) => {
    return await GetToolLogs(id, limit)
  }, [])

  const uninstall = useCallback(async (id: string) => {
    const result = await UninstallTool(id)
    if (result.success) {
      setTools((prev) => prev.filter((t) => t.id !== id))
      toolsById.current.delete(id)
    }
    return result
  }, [])

  // Fetch all tools (for the command palette) without changing the
  // current category view. Results are merged into the cache.
  const fetchAllTools = useCallback(async () => {
    try {
      const all = await GetTools('all')
      const list = all ?? []
      mergeIntoCache(list)
      return list
    } catch (e) {
      console.error('GetTools(all) failed', e)
      return []
    }
  }, [mergeIntoCache])

  // Look up a tool by ID from the cross-category cache.
  const getToolById = useCallback((id: string) => {
    return toolsById.current.get(id)
  }, [])

  // All known tools as an array (for the picker).
  const allKnownTools = useCallback(() => {
    return Array.from(toolsById.current.values())
  }, [])

  useEffect(() => {
    const unsub = EventsOn('tool-uninstalled', (toolID: string) => {
      setTools((prev) => prev.filter((t) => t.id !== toolID))
      toolsById.current.delete(toolID)
    })
    return () => unsub()
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
    uninstall,
    fetchAllTools,
    getToolById,
    allKnownTools,
  }
}
