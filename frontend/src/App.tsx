import { useState, useEffect, useCallback } from 'react'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { TitleBar } from '@/components/title-bar'
import { StatusBar } from '@/components/status-bar'
import { Sidebar } from '@/components/sidebar'
import { RightSidebar } from '@/components/right-sidebar'
import { WelcomePage } from '@/components/WelcomePage'
import { ToolGrid } from '@/components/tool-grid'
import { ToolDetail } from '@/components/tools/ToolDetail'
import { ToolAdd } from '@/components/tools/ToolAdd'
import { ToolPickerModal } from '@/components/tools/ToolPickerModal'
import { Settings } from '@/components/settings/Settings'
import { ChatPage } from '@/components/chat/ChatPage'
import { useTools } from '@/hooks/useTools'
import { useRuntime } from '@/hooks/useRuntime'
import { usePinnedTools } from '@/hooks/usePinnedTools'
import { useShortcuts } from '@/hooks/useShortcuts'
import { useI18n } from '@/hooks/useI18n'
import { useAppState } from '@/hooks/useAppState'
import { useChat } from '@/hooks/useChat'
import { GetConfig, GetLLMConfig } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime'
import { model } from '../wailsjs/go/models'

function App() {
  const {
    category, setCategory,
    view, setView,
    selectedTool,
    runningTools, setRunningTools,
    sidebarExpanded,
    rightSidebarVisible,
    statusBarVisible,
    showToolPicker, setShowToolPicker,
    allTools, setAllTools,
    handleSelectTool,
    handleBack,
    handleGoBack,
    handleToolLaunch,
    handleToolStop,
    handleToggleSidebar,
    handleToggleRightSidebar,
    handleToggleStatusBar,
    handleEnterChat,
  } = useAppState()

  const { t, setLocale } = useI18n()
  const [statusMessage, setStatusMessage] = useState(t('statusBar.ready'))
  const [llmProvider, setLlmProvider] = useState<string>('')
  const [llmModel, setLlmModel] = useState<string>('')
  const { pinnedIds, togglePin, isPinned } = usePinnedTools()
  const {
    tools, loading, refresh, launch, stop, addManual, uninstall,
    fetchAllTools, getToolById,
  } = useTools(category)
  const { status: runtimeStatus, loading: runtimeLoading, refresh: refreshRuntime } = useRuntime()
  const {
    conversations,
    selectedId,
    currentConversation,
    messages,
    messagesLoading,
    fetchMessages,
    createConversation,
    deleteConversation,
    selectConversation,
    sendMessageStream,
    sending,
    isStreaming,
    streamingContent,
    streamingPhase,
    sendError,
    setSendError,
  } = useChat()

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

    GetLLMConfig().then((cfg) => {
      if (cfg) {
        setLlmProvider(cfg.provider)
        setLlmModel(cfg.model)
      }
    }).catch((err) => console.error('GetLLMConfig failed', err))
  }, [setLocale])

  // Derive pinned tool objects from the cross-category cache.
  const pinnedToolObjects = pinnedIds
    .map((id) => getToolById(id))
    .filter((t): t is model.ToolInfo => !!t)
    .filter((t) => !runningTools.find((r) => r.id === t.id))

  const showStatus = useCallback((msg: string) => {
    setStatusMessage(msg)
    const timer = setTimeout(() => setStatusMessage(t('statusBar.ready')), 5000)
    return () => clearTimeout(timer)
  }, [t])

  const handleRefresh = useCallback(() => {
    showStatus(t('statusBar.scanning'))
    refresh()
  }, [refresh, showStatus])

  const handleAddToolSubmit = useCallback((name: string, command: string, argType: string) => {
    addManual(name, command, argType)
    showStatus(t('statusBar.toolAdded', { name }))
    setView('grid')
  }, [addManual, showStatus, setView])

  const handleShowToolPicker = useCallback(async () => {
    setAllTools([])
    const list = await fetchAllTools()
    setAllTools(list)
    setShowToolPicker(true)
  }, [fetchAllTools, setAllTools, setShowToolPicker])

  const handleCreateConversation = useCallback(async () => {
    await createConversation()
    handleEnterChat()
  }, [createConversation, handleEnterChat])

  const handleSelectConv = useCallback(async (id: string) => {
    await selectConversation(id)
    handleEnterChat()
  }, [selectConversation, handleEnterChat])

  const handleDeleteConv = useCallback(async (id: string) => {
    await deleteConversation(id)
    if (selectedId === id) {
      setView('grid')
    }
  }, [deleteConversation, selectedId, setView])

  useShortcuts({
    onCommandPalette: handleShowToolPicker,
    onAddTool: () => setView('manual'),
    onRefresh: handleRefresh,
    onSettings: () => setView('settings'),
    onBack: handleGoBack,
  })

  // listen for state-change events pushed from backend
  useEffect(() => {
    const unsub = EventsOn('tool:state-changed', (data: { tool_id: string; status: string }) => {
      if (data.status === 'launching' || data.status === 'running') {
        const cached = getToolById(data.tool_id)
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
  }, [getToolById, setRunningTools])

  return (
    <div className="flex h-screen flex-col bg-background">
      <div className="pointer-events-none fixed inset-0 bg-[radial-gradient(ellipse_80%_50%_at_50%_-20%,hsl(var(--primary)/0.08),transparent)]" />
      <TitleBar
        sidebarExpanded={sidebarExpanded}
        rightSidebarVisible={rightSidebarVisible}
        statusBarVisible={statusBarVisible}
        onToggleSidebar={handleToggleSidebar}
        onToggleRightSidebar={handleToggleRightSidebar}
        onToggleStatusBar={handleToggleStatusBar}
      />
      <div className="relative z-10 flex flex-1 overflow-hidden">
        <Sidebar
          activeView={view === 'chat' ? 'chat' : view === 'welcome' ? 'welcome' : 'tools'}
          activeCategory={category}
          activeConversationId={selectedId ?? undefined}
          conversations={conversations}
          expanded={sidebarExpanded}
          onSelectCategory={(cat) => { setCategory(cat); setView('grid') }}
          onSelectConversation={handleSelectConv}
          onNewConversation={handleCreateConversation}
          onDeleteConversation={handleDeleteConv}
          onAddTool={() => setView('manual')}
          onSettings={() => setView('settings')}
          onRefresh={handleRefresh}
        />

        <ErrorBoundary>
          {view === 'welcome' && (
            <WelcomePage
              conversations={conversations.slice(0, 5)}
              onSelectConversation={handleSelectConv}
              onSelectTool={handleSelectTool}
              onNewConversation={handleCreateConversation}
              onDeleteConversation={handleDeleteConv}
            />
          )}

          {view === 'chat' && (
            <ChatPage
              conversation={currentConversation}
              messages={messages}
              messagesLoading={messagesLoading}
              sending={sending}
              isStreaming={isStreaming}
              streamingContent={streamingContent}
              streamingPhase={streamingPhase}
              sendError={sendError}
              onSend={sendMessageStream}
              onLoadMessages={fetchMessages}
              onClearError={() => setSendError(null)}
              onBack={handleBack}
            />
          )}

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
            <ToolAdd onAdd={handleAddToolSubmit} onCancel={handleBack} onRefresh={handleRefresh} />
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
      {statusBarVisible && <StatusBar runtimeStatus={runtimeStatus} runtimeLoading={runtimeLoading} llmProvider={llmProvider} llmModel={llmModel} />}
    </div>
  )
}

export default App
