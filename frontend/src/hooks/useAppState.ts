import { useState, useCallback } from 'react'
import { model } from '../../wailsjs/go/models'

export type View = 'welcome' | 'grid' | 'detail' | 'manual' | 'settings' | 'chat'

/**
 * 应用级状态钩子，管理视图、选中工具、运行中工具以及侧边栏布局状态。
 */
export function useAppState() {
  const [category, setCategory] = useState('all')
  const [view, setView] = useState<View>('welcome')
  const [selectedTool, setSelectedTool] = useState<model.ToolInfo | null>(null)
  const [runningTools, setRunningTools] = useState<model.ToolInfo[]>([])
  const [sidebarExpanded, setSidebarExpanded] = useState(true)
  const [rightSidebarVisible, setRightSidebarVisible] = useState(true)
  const [statusBarVisible, setStatusBarVisible] = useState(true)
  const [showToolPicker, setShowToolPicker] = useState(false)
  const [allTools, setAllTools] = useState<model.ToolInfo[]>([])

  const handleSelectTool = useCallback((tool: model.ToolInfo) => {
    setSelectedTool(tool)
    setView('detail')
  }, [])

  const handleBack = useCallback(() => {
    setView('welcome')
    setSelectedTool(null)
  }, [])

  const handleAddTool = useCallback((_name: string) => {
    setView('grid')
  }, [])

  const handleGoBack = useCallback(() => {
    if (view !== 'grid') {
      handleBack()
    }
  }, [view, handleBack])

  const handleToolLaunch = useCallback((tool: model.ToolInfo) => {
    setRunningTools((prev) => {
      if (prev.find((x) => x.id === tool.id)) return prev
      return [...prev, tool]
    })
  }, [])

  const handleToolStop = useCallback((_toolId: string) => {
    setRunningTools((prev) => prev.filter((x) => x.id !== _toolId))
  }, [])

  const handleToggleSidebar = useCallback(() => {
    setSidebarExpanded((v) => !v)
  }, [])

  const handleToggleRightSidebar = useCallback(() => {
    setRightSidebarVisible((v) => !v)
  }, [])

  const handleToggleStatusBar = useCallback(() => {
    setStatusBarVisible((v) => !v)
  }, [])

  const handleEnterChat = useCallback(() => {
    setView('chat')
  }, [])

  return {
    category,
    setCategory,
    view,
    setView,
    selectedTool,
    setSelectedTool,
    runningTools,
    setRunningTools,
    sidebarExpanded,
    rightSidebarVisible,
    statusBarVisible,
    showToolPicker,
    setShowToolPicker,
    allTools,
    setAllTools,
    handleSelectTool,
    handleBack,
    handleAddTool,
    handleGoBack,
    handleToolLaunch,
    handleToolStop,
    handleToggleSidebar,
    handleToggleRightSidebar,
    handleToggleStatusBar,
    handleEnterChat,
  }
}
