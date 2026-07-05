import { useState, useEffect, useCallback, useRef } from 'react'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { TitleBar } from '@/components/title-bar'
import { StatusBar } from '@/components/status-bar'
import { Sidebar } from '@/components/sidebar'
import { RightSidebar } from '@/components/right-sidebar'
import { ToolGrid } from '@/components/tool-grid'
import { ToolDetail } from '@/components/tools/ToolDetail'
import { ToolAdd } from '@/components/tools/ToolAdd'
import { ToolPickerModal } from '@/components/tools/ToolPickerModal'
import { Settings } from '@/components/settings/Settings'
import { useTools } from '@/hooks/useTools'
import { useRuntime } from '@/hooks/useRuntime'
import { usePinnedTools } from '@/hooks/usePinnedTools'
import { useShortcuts } from '@/hooks/useShortcuts'
import { useI18n } from '@/hooks/useI18n'
import { GetTools, GetConfig } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime'
import { model } from '../wailsjs/go/models'

type View = 'grid' | 'detail' | 'manual' | 'settings'

function App() {
  const [category, setCategory] = useState('all')
  const [view, setView] = useState<View>('grid')
  const [selectedTool, setSelectedTool] = useState<model.ToolInfo | null>(null)
  const [runningTools, setRunningTools] = useState<model.ToolInfo[]>([])
  const [sidebarExpanded, setSidebarExpanded] = useState(true)
  const [rightSidebarVisible, setRightSidebarVisible] = useState(true)
  const [statusBarVisible, setStatusBarVisible] = useState(true)
  const { t, setLocale } = useI18n()
  const [statusMessage, setStatusMessage] = useState(t('statusBar.ready'))
  const [showToolPicker, setShowToolPicker] = useState(false)
  const [allTools, setAllTools] = useState<model.ToolInfo[]>([])
  const { tools, loading, refresh, launch, stop, addManual, uninstall } = useTools(category)
  const { status: runtimeStatus, loading: runtimeLoading, refresh: refreshRuntime } = useRuntime()
  const { pinnedIds, togglePin, isPinned } = usePinnedTools()

  // load theme & language on startup
  useEffect(() => {
    GetConfig().then((cfg) => {
      if (cfg) {
        document.documentElement.classList.remove('dark', 'theme-marcus')
        if (cfg.theme === 'dark') document.documentElement.classList.add('dark')
        if (cfg.theme === 'marcus') document.documentElement.classList.add('theme-marcus')
        if (cfg.language) setLocale(cfg.language)
      }
    }).catch((err) => console.error('GetConfig failed', err))
  }, [setLocale])

  const toolCache = useRef<Record<string, model.ToolInfo>>({})
  const MAX_CACHE = 200

  // keep tool cache in sync, evict stale entries beyond MAX_CACHE
  useEffect(() => {
    const currentIds = new Set(tools.map((t) => t.id))
    for (const tool of tools) {
      toolCache.current[tool.id] = tool
    }
    // evict entries that are no longer in the current tool set
    const ids = Object.keys(toolCache.current)
    if (ids.length > MAX_CACHE) {
      for (const id of ids) {
        if (!currentIds.has(id)) {
          delete toolCache.current[id]
        }
      }
    }
  }, [tools])

  const pinnedToolObjects = pinnedIds
    .map((id) => toolCache.current[id])
    .filter(Boolean)
    .filter((t) => !runningTools.find((r) => r.id === t.id)) as model.ToolInfo[]

  const showStatus = useCallback((msg: string) => {
    setStatusMessage(msg)
    const timer = setTimeout(() => setStatusMessage(t('statusBar.ready')), 5000)
    return () => clearTimeout(timer)
  }, [t])

  const handleSelectTool = useCallback((tool: model.ToolInfo) => {
    toolCache.current[tool.id] = tool
    setSelectedTool(tool)
    setView('detail')
  }, [])

  const handleBack = useCallback(() => {
    setView('grid')
    setSelectedTool(null)
  }, [])

  const handleAddTool = useCallback((name: string, command: string, argType: string) => {
    addManual(name, command, argType)
    showStatus(t('statusBar.toolAdded', { name }))
    setView('grid')
  }, [addManual, showStatus])

  const handleRefresh = useCallback(() => {
    showStatus(t('statusBar.scanning'))
    refresh()
  }, [refresh, showStatus])

  const handleToolLaunch = useCallback((tool: model.ToolInfo) => {
    toolCache.current[tool.id] = tool
    setRunningTools((prev) => {
      if (prev.find((x) => x.id === tool.id)) return prev
      return [...prev, tool]
    })
    showStatus(t('statusBar.launching', { name: tool.display_name }))
  }, [showStatus])

  const handleShowToolPicker = useCallback(async () => {
    setAllTools([])
    try {
      const all = await GetTools('all')
      setAllTools(all ?? [])
    } catch (err) {
      console.error('GetTools(all) failed', err)
    }
    setShowToolPicker(true)
  }, [])

  const handleGoBack = useCallback(() => {
    if (view !== 'grid') {
      handleBack()
    }
  }, [view, handleBack])

  useShortcuts({
    onCommandPalette: handleShowToolPicker,
    onAddTool: () => setView('manual'),
    onRefresh: handleRefresh,
    onSettings: () => setView('settings'),
    onBack: handleGoBack,
  })

  const handleToolStop = useCallback((toolId: string) => {
    setRunningTools((prev) => prev.filter((x) => x.id !== toolId))
    showStatus(t('statusBar.ready'))
  }, [showStatus])

  // listen for state-change events pushed from backend (replaces polling).
  useEffect(() => {
    const unsub = EventsOn('tool:state-changed', (data: { tool_id: string; status: string }) => {
      if (data.status === 'launching' || data.status === 'running') {
        const cached = toolCache.current[data.tool_id]
        if (cached) {
          setRunningTools((prev) =>
            prev.find((x) => x.id === data.tool_id) ? prev : [...prev, cached],
          )
        }
      } else if (data.status === 'exited' || data.status === 'crashed') {
        setRunningTools((prev) => prev.filter((x) => x.id !== data.tool_id))
      }
    })
    return () => unsub()
  }, [])

  return (
    <div className="flex h-screen flex-col bg-background">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(ellipse_80%_50%_at_50%_-20%,hsl(var(--primary)/0.08),transparent)]" />
      <TitleBar
        sidebarExpanded={sidebarExpanded}
        rightSidebarVisible={rightSidebarVisible}
        statusBarVisible={statusBarVisible}
        onToggleSidebar={() => setSidebarExpanded((v) => !v)}
        onToggleRightSidebar={() => setRightSidebarVisible((v) => !v)}
        onToggleStatusBar={() => setStatusBarVisible((v) => !v)}
      />
      <div className="relative z-10 flex flex-1 overflow-hidden">
        <Sidebar
          active={category}
          onSelectCategory={(cat) => { setCategory(cat); setView('grid') }}
          onAddTool={() => setView('manual')}
          onSettings={() => setView('settings')}
          onRefresh={handleRefresh}
          expanded={sidebarExpanded}
        />

        <ErrorBoundary>
          {view === 'grid' && (
            <ToolGrid
              category={category}
              tools={tools}
              loading={loading}
              onSelectTool={handleSelectTool}
              onTogglePin={togglePin}
              isPinned={isPinned}
            />
          )}

          {view === 'detail' && selectedTool && (
            <ToolDetail
              key={selectedTool.id}
              tool={selectedTool}
              onBack={handleBack}
              onLaunch={launch}
              onStop={stop}
              onUninstall={uninstall}
              onToolLaunch={handleToolLaunch}
              onToolStop={handleToolStop}
            />
          )}

          {view === 'manual' && (
            <ToolAdd onAdd={handleAddTool} onCancel={handleBack} onRefresh={handleRefresh} />
          )}

          {view === 'settings' && (
            <Settings
              runtimeStatus={runtimeStatus}
              runtimeLoading={loading}
              onRefreshRuntime={refreshRuntime}
              onClose={handleBack}
            />
          )}
        </ErrorBoundary>

        {rightSidebarVisible && (
          <RightSidebar
            runningTools={runningTools}
            pinnedTools={pinnedToolObjects}
            onSelectTool={handleSelectTool}
            onTogglePin={togglePin}
            onShowToolPicker={handleShowToolPicker}
          />
        )}

        <ToolPickerModal
          open={showToolPicker}
          tools={allTools}
          pinnedIds={pinnedIds}
          onClose={() => setShowToolPicker(false)}
          onSelect={handleSelectTool}
          onTogglePin={togglePin}
        />
      </div>
      {statusBarVisible && <StatusBar runtimeStatus={runtimeStatus} runtimeLoading={runtimeLoading} />}
    </div>
  )
}

export default App
