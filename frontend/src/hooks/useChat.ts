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

export type StreamPhaseType = 'thinking' | 'tool_call' | 'tool_done' | 'code' | 'fetch' | 'text'

export interface StreamPhase {
  type: StreamPhaseType
  content: string
  metadata?: Record<string, string>
}

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
  const [streamingPhase, setStreamingPhase] = useState<StreamPhase | null>(null)
  const streamingIdRef = useRef<string | null>(null)

  // debounce buffer for streaming content
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const bufferRef = useRef('')
  const DEBOUNCE_MS = 150

  const flushBuffer = useCallback(() => {
    if (bufferRef.current) {
      setStreamingContent((prev) => prev + bufferRef.current)
      bufferRef.current = ''
    }
  }, [])

  const debouncedAppend = useCallback((chunk: string) => {
    bufferRef.current += chunk
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current)
    }
    debounceTimerRef.current = setTimeout(flushBuffer, DEBOUNCE_MS)
  }, [flushBuffer])

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

  const selectConversation = useCallback(async (conversationId: string) => {
    setSelectedId(conversationId)
    const msgs = await fetchMessages(conversationId)
    if (!msgs || msgs.length === 0) {
      setEmptyConversationId(conversationId)
    }
  }, [fetchMessages])

  const sendMessage = useCallback(async (conversationId: string, content: string) => {
    if (sendingRef.current) return
    sendingRef.current = true
    setSending(true)
    setSendError(null)
    try {
      const response = await SendMessage(conversationId, content)
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

  const sendMessageStream = useCallback(async (conversationId: string, content: string) => {
    if (sendingRef.current || isStreaming) return
    sendingRef.current = true
    setSending(true)
    setSendError(null)

    // optimistically show user message immediately
    const optimisticMsg: model.ConversationMessage = {
      id: `temp-${Date.now()}`,
      conversation_id: conversationId,
      role: 'user' as const,
      content,
      created_at: new Date().toISOString(),
    } as model.ConversationMessage
    setMessages((prev) => [...prev, optimisticMsg])

    try {
      await SendMessageStream(conversationId, content)
      // Stream start/end events handle the state transitions:
      //   start → setIsStreaming(true), setSending(false)
      //   end   → setIsStreaming(false), sendingRef.current = false
      // Don't fetchMessages here — the stream end handler does it.
    } catch (e) {
      const msg = String(e)
      setSendError(msg)
      setSending(false)
      sendingRef.current = false
      console.error('send stream failed', e)
      throw e
    }
  }, [isStreaming])

  useEffect(() => {
    const unsubStart = EventsOn('chat:stream:start', (data: { conversation_id: string }) => {
      streamingIdRef.current = data.conversation_id
      setIsStreaming(true)
      setSending(false) // transition from "sending" to "streaming"
      setStreamingContent('')
      setStreamingPhase(null)
      bufferRef.current = ''
    })

    const unsubChunk = EventsOn('chat:stream:chunk', (data: { conversation_id: string; content: string }) => {
      if (data.conversation_id === streamingIdRef.current) {
        debouncedAppend(data.content)
      }
    })

    const unsubPhase = EventsOn('chat:stream:phase', (data: { conversation_id: string; type: string; content: string; tool_name?: string; url?: string; language?: string }) => {
      if (data.conversation_id === streamingIdRef.current) {
        const phase: StreamPhase = {
          type: data.type as StreamPhaseType,
          content: data.content,
          metadata: {},
        }
        if (data.tool_name) phase.metadata!.tool_name = data.tool_name
        if (data.url) phase.metadata!.url = data.url
        if (data.language) phase.metadata!.language = data.language
        setStreamingPhase(phase)
      }
    })

    const unsubEnd = EventsOn('chat:stream:end', (data: { conversation_id: string }) => {
      if (data.conversation_id !== streamingIdRef.current) return
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }
      flushBuffer()
      setIsStreaming(false)
      setStreamingContent('')
      setStreamingPhase(null)
      streamingIdRef.current = null
      sendingRef.current = false
      fetchMessages(data.conversation_id)
    })

    const unsubError = EventsOn('chat:stream:error', (data: { conversation_id: string; error: string }) => {
      if (data.conversation_id !== streamingIdRef.current) return
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }
      setIsStreaming(false)
      setStreamingContent('')
      setStreamingPhase(null)
      streamingIdRef.current = null
      sendingRef.current = false
      setSendError(data.error)
      fetchMessages(data.conversation_id)
    })

    return () => {
      unsubStart()
      unsubChunk()
      unsubPhase()
      unsubEnd()
      unsubError()
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }
    }
  }, [fetchMessages, debouncedAppend, flushBuffer])

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
    streamingPhase,
    sendError,
    setSendError,
  }
}
