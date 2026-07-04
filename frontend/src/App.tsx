import { useState, useEffect, useCallback, useRef } from 'react'
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
  const { tools, loading, refresh, launch, stop, pollState, addManual } = useTools(category)
  const { status: runtimeStatus, refresh: refreshRuntime } = useRuntime()
  const { pinnedIds, togglePin, isPinned } = usePinnedTools()

  // load theme & language on startup
  useEffect(() => {
    GetConfig().then((cfg) => {
      if (cfg) {
        document.documentElement.classList.toggle('dark', cfg.theme !== 'light')
        if (cfg.language) setLocale(cfg.language)
      }
    }).catch(() => {})
  }, [setLocale])

  const toolCache = useRef<Record<string, model.ToolInfo>>({})

  // keep tool cache in sync
  useEffect(() => {
    for (const tool of tools) {
      toolCache.current[tool.id] = tool
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
    } catch {
      // ignore
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

  // global polling: detect exited processes even when ToolDetail is not mounted
  useEffect(() => {
    if (runningTools.length === 0) return
    const id = setInterval(async () => {
      const currentIds = [...runningTools]
      for (const tool of currentIds) {
        try {
          const state = await pollState(tool.id)
          if (state?.status === 'exited' || state?.status === 'crashed') {
            handleToolStop(tool.id)
          }
        } catch {
          // ignore
        }
      }
    }, 2000)
    return () => clearInterval(id)
  }, [runningTools, pollState, handleToolStop])

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
            onToolLaunch={handleToolLaunch}
            onToolStop={handleToolStop}
            pollState={pollState}
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
      {statusBarVisible && <StatusBar message={statusMessage} runtimeStatus={runtimeStatus} />}
    </div>
  )
}

export default App
