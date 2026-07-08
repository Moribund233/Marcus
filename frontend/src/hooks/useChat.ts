import { useState, useEffect, useCallback, useRef } from 'react'
import {
  CreateConversation,
  ListConversations,
  GetConversationMessages,
  DeleteConversation,
  SendMessage,
  SendMessageStream,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'
import { model } from '../../wailsjs/go/models'

/**
 * Chat 状态钩子，封装会话与消息的增删改查，并支持流式消息推送。
 *
 * 该钩子负责与 Wails 后端进行会话（Conversation）和消息（Message）的交互，
 * 并在前端维护当前选中的会话、消息列表以及发送消息的加载状态。
 */
export function useChat() {
  const [conversations, setConversations] = useState<model.Conversation[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [messages, setMessages] = useState<model.ConversationMessage[]>([])
  const [messagesLoading, setMessagesLoading] = useState(false)
  const [emptyConversationId, setEmptyConversationId] = useState<string | null>(null)

  const sendingRef = useRef(false)
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState<string | null>(null)

  const [isStreaming, setIsStreaming] = useState(false)
  const [streamingContent, setStreamingContent] = useState('')
  const streamingIdRef = useRef<string | null>(null)

  /** 获取会话列表，默认按更新时间倒序排列。 */
  const fetchConversations = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await ListConversations(100)
      setConversations(list ?? [])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  /** 根据当前选中的会话 ID 拉取消息记录。返回消息列表。 */
  const fetchMessages = useCallback(async (conversationId: string): Promise<model.ConversationMessage[]> => {
    setMessagesLoading(true)
    try {
      const list = await GetConversationMessages(conversationId)
      const msgs = list ?? []
      setMessages(msgs)
      if (msgs.length > 0) {
        setEmptyConversationId((prev) => prev === conversationId ? null : prev)
      }
      return msgs
    } catch (e) {
      console.error('fetch messages failed', e)
      setMessages([])
      return []
    } finally {
      setMessagesLoading(false)
    }
  }, [])

  /** 创建新会话并自动选中。若已有空会话则复用，不重复创建。 */
  const createConversation = useCallback(async (title?: string) => {
    if (emptyConversationId) {
      const existing = conversations.find((c) => c.id === emptyConversationId)
      if (existing) {
        setSelectedId(existing.id)
        setMessages([])
        return existing
      }
      setEmptyConversationId(null)
    }
    try {
      const conv = await CreateConversation(title || '新会话')
      setConversations((prev) => [conv, ...prev])
      setSelectedId(conv.id)
      setMessages([])
      setEmptyConversationId(conv.id)
      return conv
    } catch (e) {
      setError(String(e))
      throw e
    }
  }, [emptyConversationId, conversations])

  /** 删除指定会话，若删除的是当前选中会话则清空消息。 */
  const deleteConversation = useCallback(async (conversationId: string) => {
    try {
      await DeleteConversation(conversationId)
      setConversations((prev) => prev.filter((c) => c.id !== conversationId))
      setEmptyConversationId((prev) => prev === conversationId ? null : prev)
      if (selectedId === conversationId) {
        setSelectedId(null)
        setMessages([])
      }
    } catch (e) {
      setError(String(e))
      throw e
    }
  }, [selectedId])

  /** 选中会话并加载其历史消息。若消息为空则标记为空会话。 */
  const selectConversation = useCallback(async (conversationId: string) => {
    setSelectedId(conversationId)
    const msgs = await fetchMessages(conversationId)
    if (!msgs || msgs.length === 0) {
      setEmptyConversationId(conversationId)
    }
  }, [fetchMessages])

  /** 向当前会话发送用户消息并触发 Agent 处理（非流式）。 */
  const sendMessage = useCallback(async (conversationId: string, content: string) => {
    if (sendingRef.current) return
    sendingRef.current = true
    setSending(true)
    setSendError(null)
    try {
      const response = await SendMessage(conversationId, content)
      // 发送成功后刷新消息列表以包含用户消息和模型回复。
      await fetchMessages(conversationId)
      return response
    } catch (e) {
      const msg = String(e)
      setSendError(msg)
      console.error('send message failed', e)
      throw e
    } finally {
      setSending(false)
      sendingRef.current = false
    }
  }, [fetchMessages])

  /** 以流式方式发送用户消息。 */
  const sendMessageStream = useCallback(async (conversationId: string, content: string) => {
    if (sendingRef.current || isStreaming) return
    sendingRef.current = true
    setSending(true)
    setSendError(null)
    try {
      // 先触发后端流式处理，后端会保存用户消息与最终助手消息。
      await SendMessageStream(conversationId, content)
      // 立即刷新一次以显示用户消息。
      await fetchMessages(conversationId)
    } catch (e) {
      const msg = String(e)
      setSendError(msg)
      console.error('send stream failed', e)
      throw e
    } finally {
      // 流式状态由事件控制结束；这里只释放发送锁。
      sendingRef.current = false
      setSending(false)
    }
  }, [fetchMessages, isStreaming])

  /** 订阅后端流式消息事件。 */
  useEffect(() => {
    const unsubStart = EventsOn('chat:stream:start', (data: { conversation_id: string }) => {
      streamingIdRef.current = data.conversation_id
      setIsStreaming(true)
      setStreamingContent('')
    })
    const unsubChunk = EventsOn('chat:stream:chunk', (data: { conversation_id: string; content: string }) => {
      if (data.conversation_id === streamingIdRef.current) {
        setStreamingContent((prev) => prev + data.content)
      }
    })
    const unsubEnd = EventsOn('chat:stream:end', (data: { conversation_id: string }) => {
      if (data.conversation_id !== streamingIdRef.current) return
      setIsStreaming(false)
      setStreamingContent('')
      streamingIdRef.current = null
      fetchMessages(data.conversation_id)
    })
    const unsubError = EventsOn('chat:stream:error', (data: { conversation_id: string; error: string }) => {
      if (data.conversation_id !== streamingIdRef.current) return
      setIsStreaming(false)
      setStreamingContent('')
      streamingIdRef.current = null
      setSendError(data.error)
      fetchMessages(data.conversation_id)
    })

    return () => {
      unsubStart()
      unsubChunk()
      unsubEnd()
      unsubError()
    }
  }, [fetchMessages])

  useEffect(() => {
    fetchConversations()
  }, [fetchConversations])

  const currentConversation = conversations.find((c) => c.id === selectedId) || null

  return {
    conversations,
    loading,
    error,
    fetchConversations,
    createConversation,
    deleteConversation,
    selectConversation,
    selectedId,
    currentConversation,
    messages,
    messagesLoading,
    fetchMessages,
    sendMessage,
    sendMessageStream,
    sending,
    isStreaming,
    streamingContent,
    sendError,
    setSendError,
  }
}
