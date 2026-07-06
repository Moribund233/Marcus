import { useEffect, useRef, useCallback } from 'react'
import { useI18n } from '@/hooks/useI18n'
import { ChatMessage } from './ChatMessage'
import { ChatInput } from './ChatInput'
import { model } from '../../../wailsjs/go/models'
import { AlertCircle, X, Bot } from 'lucide-react'

interface ChatPageProps {
  conversation: model.Conversation | null
  messages: model.ConversationMessage[]
  messagesLoading: boolean
  sending: boolean
  isStreaming?: boolean
  streamingContent?: string
  sendError?: string | null
  onSend: (conversationId: string, content: string) => void
  onLoadMessages: (conversationId: string) => void
  onClearError?: () => void
}

/**
 * 聊天主页面组件。
 *
 * 展示当前会话标题、历史消息列表与输入框；当切换会话时自动滚动到底部。
 */
export function ChatPage({
  conversation,
  messages,
  messagesLoading,
  sending,
  isStreaming,
  streamingContent,
  sendError,
  onSend,
  onLoadMessages,
  onClearError,
}: ChatPageProps) {
  const { t } = useI18n()
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (conversation) {
      onLoadMessages(conversation.id)
    }
  }, [conversation?.id, onLoadMessages])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, sending])

  const handleSend = useCallback(
    (content: string) => {
      if (!conversation) return
      onSend(conversation.id, content)
    },
    [conversation, onSend],
  )

  if (!conversation) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center text-muted-foreground">
        <span className="text-sm">{t('chat.empty')}</span>
      </div>
    )
  }

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <div className="flex items-center justify-between border-b border-border px-6 py-3">
        <h2 className="text-sm font-medium truncate">{conversation.title}</h2>
        {messagesLoading && (
          <span className="text-xs text-muted-foreground">{t('chat.loadingMessages')}</span>
        )}
      </div>

      <div className="flex-1 overflow-y-auto">
        {messages.length === 0 && !messagesLoading && (
          <div className="flex h-full flex-col items-center justify-center px-6 text-center text-muted-foreground">
            <p className="text-sm">{t('chat.welcome')}</p>
          </div>
        )}
        {messages.map((msg) => (
          <ChatMessage key={msg.id} message={msg} />
        ))}
        {isStreaming && (
          <div className="flex gap-3 px-6 py-4 bg-muted/30">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
              <Bot className="h-4 w-4" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium mb-1">{t('chat.message.assistant')}</div>
              <div className="text-sm whitespace-pre-wrap leading-relaxed text-foreground">
                {streamingContent}
                <span className="inline-block h-4 w-0.5 animate-pulse bg-primary ml-0.5" />
              </div>
            </div>
          </div>
        )}
        {sending && (
          <div className="flex gap-3 px-6 py-4 bg-muted/30">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
              <span className="h-4 w-4 animate-pulse">•••</span>
            </div>
            <div className="flex items-center text-sm text-muted-foreground">
              {t('chat.thinking')}
            </div>
          </div>
        )}
        <div ref={bottomRef} />
      </div>
      {sendError && (
        <div className="flex items-start gap-2 border-t border-destructive/20 bg-destructive/5 px-4 py-2 text-xs text-destructive">
          <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <span className="flex-1 break-all">{sendError}</span>
          {onClearError && (
            <button
              onClick={onClearError}
              className="shrink-0 text-destructive hover:text-destructive/80"
              title={t('chat.clearError')}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      )}
      <ChatInput onSend={handleSend} loading={sending} />
    </div>
  )
}
