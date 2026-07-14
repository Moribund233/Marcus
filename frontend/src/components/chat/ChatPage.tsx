import { useEffect, useRef, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useI18n } from '@/hooks/useI18n'
import { ChatMessage } from './ChatMessage'
import { ChatInput } from './ChatInput'
import { model } from '../../../wailsjs/go/models'
import { AlertCircle, X, Bot, Wrench, Globe, Code2, Loader2, CheckCircle2, ArrowLeft } from 'lucide-react'
import { AppLogo } from '@/components/common/AppLogo'
import type { StreamPhase } from '@/hooks/useChat'

interface ChatPageProps {
  conversation: model.Conversation | null
  messages: model.ConversationMessage[]
  messagesLoading: boolean
  sending: boolean
  isStreaming: boolean
  streamingContent: string
  streamingPhase: StreamPhase | null
  sendError: string | null
  onSend: (conversationId: string, content: string) => void
  onLoadMessages: (conversationId: string) => void
  onClearError?: () => void
  onBack?: () => void
}

export function ChatPage({
  conversation,
  messages,
  messagesLoading,
  sending,
  isStreaming,
  streamingContent,
  streamingPhase,
  sendError,
  onSend,
  onLoadMessages,
  onClearError,
  onBack,
}: ChatPageProps) {
  const { t } = useI18n()
  const bottomRef = useRef<HTMLDivElement>(null)
  const scrollContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (conversation) {
      onLoadMessages(conversation.id)
    }
  }, [conversation?.id, onLoadMessages])

  // only auto-scroll when user is near the bottom (within 120px)
  const isNearBottom = useCallback(() => {
    const el = scrollContainerRef.current
    if (!el) return true
    return el.scrollHeight - el.scrollTop - el.clientHeight < 120
  }, [])

  useEffect(() => {
    if (isNearBottom()) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
    }
  }, [messages, streamingContent, streamingPhase, isNearBottom])

  const handleSend = useCallback(
    (content: string) => {
      if (!conversation) return
      onSend(conversation.id, content)
    },
    [conversation, onSend],
  )

  const handleRecall = useCallback(() => {
    if (conversation) {
      onLoadMessages(conversation.id)
    }
  }, [conversation, onLoadMessages])

  if (!conversation) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center text-muted-foreground">
        <span className="text-sm">{t('chat.empty')}</span>
      </div>
    )
  }

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <div className="flex items-center gap-3 border-b border-border px-4 py-3">
        {onBack && (
          <button
            onClick={onBack}
            className="flex items-center justify-center rounded-md p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
            title={t('chat.back')}
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
        )}
        <h2 className="flex-1 truncate text-sm font-medium">{conversation.title}</h2>
        {messagesLoading && (
          <span className="shrink-0 text-xs text-muted-foreground">{t('chat.loadingMessages')}</span>
        )}
      </div>

      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto">
        {messages.length === 0 && !messagesLoading && (
          <div className="flex h-full flex-col items-center justify-center px-6 text-center text-muted-foreground">
            <p className="text-sm">{t('chat.welcome')}</p>
          </div>
        )}
        {messages.map((msg) => (
          <ChatMessage key={msg.id} message={msg} conversationId={conversation.id} onRecall={handleRecall} />
        ))}

        {isStreaming && <StreamingArea phase={streamingPhase} content={streamingContent} t={t} />}

        {sending && !isStreaming && (
          <div className="flex gap-3 px-6 py-4 flex-row">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
              <AppLogo size={32} />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2 mb-1">
                <span className="text-xs font-medium text-muted-foreground">
                  {t('chat.message.assistant')}
                </span>
              </div>
              <div className="rounded-2xl rounded-tl-md bg-muted px-4 py-2.5 text-sm text-muted-foreground">
                <div className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span>{t('chat.thinking')}</span>
                </div>
              </div>
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

function StreamingArea({
  phase,
  content,
  t,
}: {
  phase: StreamPhase | null
  content?: string
  t: (key: string) => string
}) {
  const hasText = content && content.length > 0

  if (!phase) {
    if (hasText) {
      return <MarkdownBubble content={content!} t={t} />
    }
    return <ThinkingIndicator t={t} />
  }

  switch (phase.type) {
    case 'thinking':
      return <ThinkingIndicator t={t} />

    case 'tool_call':
      return <ToolCallCard phase={phase} t={t} />

    case 'tool_done':
      return <ToolDoneCard phase={phase} t={t} />

    case 'code':
    case 'text':
      if (hasText) {
        return <MarkdownBubble content={content!} t={t} />
      }
      return <ThinkingIndicator t={t} />

    case 'fetch':
      return <FetchCard phase={phase} t={t} />

    default:
      if (hasText) {
        return <MarkdownBubble content={content!} t={t} />
      }
      return <ThinkingIndicator t={t} />
  }
}

function ThinkingIndicator({ t }: { t: (key: string) => string }) {
  return (
    <div className="flex gap-3 px-6 py-4 flex-row">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
        <AppLogo size={32} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium text-muted-foreground">
            {t('chat.message.assistant')}
          </span>
        </div>
        <div className="rounded-2xl rounded-tl-md bg-muted px-4 py-3 text-sm text-muted-foreground">
          <div className="flex items-center gap-2">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span>{t('chat.phase.thinking')}</span>
          </div>
        </div>
      </div>
    </div>
  )
}

function ToolCallCard({ phase, t }: { phase: StreamPhase; t: (key: string) => string }) {
  const toolName = phase.metadata?.tool_name || 'tool'

  return (
    <div className="flex gap-3 px-6 py-3 flex-row">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
        <AppLogo size={32} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium text-muted-foreground">
            {t('chat.message.assistant')}
          </span>
        </div>
        <div className="rounded-xl border border-border bg-card px-3 py-2.5">
          <div className="flex items-center gap-2 text-sm">
            <Wrench className="h-4 w-4 text-primary" />
            <span className="font-medium text-foreground">{toolName}</span>
            <Loader2 className="ml-auto h-3.5 w-3.5 animate-spin text-muted-foreground" />
          </div>
          {phase.content && (
            <pre className="mt-2 overflow-x-auto rounded-md bg-muted/50 p-2 font-mono text-xs text-muted-foreground">
              {phase.content}
            </pre>
          )}
        </div>
      </div>
    </div>
  )
}

function ToolDoneCard({ phase, t }: { phase: StreamPhase; t: (key: string) => string }) {
  const toolName = phase.metadata?.tool_name || 'tool'
  const truncated = phase.content.length > 500
    ? phase.content.slice(0, 500) + '...'
    : phase.content

  return (
    <div className="flex gap-3 px-6 py-3 flex-row">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
        <AppLogo size={32} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium text-muted-foreground">
            {t('chat.message.assistant')}
          </span>
        </div>
        <div className="rounded-xl border border-border bg-green-500/5 px-3 py-2.5">
          <div className="flex items-center gap-2 text-sm">
            <CheckCircle2 className="h-4 w-4 text-green-500" />
            <span className="font-medium text-foreground">{toolName}</span>
            <span className="text-xs text-muted-foreground">{t('chat.phase.toolDone')}</span>
          </div>
          <pre className="mt-2 overflow-x-auto rounded-md bg-muted/50 p-2 font-mono text-xs text-muted-foreground max-h-40 overflow-y-auto">
            {truncated}
          </pre>
        </div>
      </div>
    </div>
  )
}

function FetchCard({ phase, t }: { phase: StreamPhase; t: (key: string) => string }) {
  const url = phase.metadata?.url || phase.content

  return (
    <div className="flex gap-3 px-6 py-3 flex-row">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
        <AppLogo size={32} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium text-muted-foreground">
            {t('chat.message.assistant')}
          </span>
        </div>
        <div className="rounded-xl border border-border bg-card px-3 py-2.5">
          <div className="flex items-center gap-2 text-sm">
            <Globe className="h-4 w-4 text-primary" />
            <span className="font-medium text-foreground">{t('chat.phase.fetch')}</span>
            <Loader2 className="ml-auto h-3.5 w-3.5 animate-spin text-muted-foreground" />
          </div>
          {url && (
            <div className="mt-1.5 font-mono text-xs text-muted-foreground truncate">{url}</div>
          )}
        </div>
      </div>
    </div>
  )
}

function MarkdownBubble({ content, t }: { content: string; t: (key: string) => string }) {
  return (
    <div className="flex gap-3 px-6 py-3 flex-row">
      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-secondary text-secondary-foreground">
        <AppLogo size={32} />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-xs font-medium text-muted-foreground">
            {t('chat.message.assistant')}
          </span>
        </div>
        <div className="rounded-2xl rounded-tl-md bg-muted px-4 py-2.5 text-sm leading-relaxed markdown-content">
          <ReactMarkdown
            remarkPlugins={[remarkGfm]}
            components={{
              code({ className, children, ...props }) {
                const match = /language-(\w+)/.exec(className || '')
                const isInline = !match && !className
                if (isInline) {
                  return (
                    <code className="rounded bg-background/80 px-1 py-0.5 font-mono text-xs text-foreground" {...props}>
                      {children}
                    </code>
                  )
                }
                return (
                  <div className="my-2 overflow-x-auto rounded-lg border border-border bg-background/80">
                    {match && (
                      <div className="border-b border-border px-3 py-1 text-xs text-muted-foreground font-mono">
                        {match[1]}
                      </div>
                    )}
                    <pre className="p-3 overflow-x-auto">
                      <code className={`font-mono text-xs leading-relaxed ${className || ''}`} {...props}>
                        {children}
                      </code>
                    </pre>
                  </div>
                )
              },
              pre({ children }) {
                return <>{children}</>
              },
            }}
          >
            {content}
          </ReactMarkdown>
          <span className="inline-block h-4 w-0.5 animate-pulse bg-primary ml-0.5" />
        </div>
      </div>
    </div>
  )
}
