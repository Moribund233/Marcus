import { useState, useCallback } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { useI18n } from '@/hooks/useI18n'
import { cn } from '@/lib/utils'
import { User, Wrench, CheckCircle2, Send } from 'lucide-react'
import { AppLogo } from '@/components/common/AppLogo'
import { ChatContextMenu } from './ChatContextMenu'
import { EditMessage, DeleteMessage, RecallMessages } from '../../../wailsjs/go/main/App'
import { model } from '../../../wailsjs/go/models'

interface ChatMessageProps {
  message: model.ConversationMessage
  conversationId: string
  onRecall?: () => void
}

export function ChatMessage({ message, conversationId, onRecall }: ChatMessageProps) {
  const { t } = useI18n()
  const isUser = message.role === 'user'

  const [contextMenu, setContextMenu] = useState<{ x: number; y: number } | null>(null)
  const [editing, setEditing] = useState(false)
  const [editValue, setEditValue] = useState(message.content || '')

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY })
  }, [])

  const handleRecall = useCallback(async () => {
    try {
      await RecallMessages(message.id)
      onRecall?.()
    } catch (err) {
      console.error('recall failed', err)
    }
  }, [message.id, onRecall])

  const handleDelete = useCallback(async () => {
    try {
      await DeleteMessage(message.id)
      onRecall?.()
    } catch (err) {
      console.error('delete failed', err)
    }
  }, [message.id, onRecall])

  const handleEditSave = useCallback(async () => {
    const trimmed = editValue.trim()
    if (!trimmed || trimmed === message.content) {
      setEditing(false)
      return
    }
    try {
      await EditMessage(message.id, trimmed)
      setEditing(false)
    } catch (err) {
      console.error('edit failed', err)
    }
  }, [message.id, editValue, message.content])

  const handleEditCancel = useCallback(() => {
    setEditValue(message.content || '')
    setEditing(false)
  }, [message.content])

  const openEdit = useCallback(() => {
    setEditValue(message.content || '')
    setEditing(true)
  }, [message.content])

  return (
    <>
      <div
        className={cn(
          'flex gap-3 px-6 py-4 group',
          isUser ? 'flex-row-reverse' : 'flex-row',
        )}
      >
        <div
          className={cn(
            'flex h-8 w-8 shrink-0 items-center justify-center rounded-full',
            isUser ? 'bg-primary/10 text-primary' : 'bg-secondary text-secondary-foreground',
          )}
        >
          {isUser ? <User className="h-4 w-4" /> : <AppLogo size={32} />}
        </div>

        <div className={cn('flex max-w-[80%] flex-col', isUser ? 'items-end' : 'items-start')}>
          <div className="flex items-center gap-2 mb-1">
            <span className="text-xs font-medium text-muted-foreground">
              {isUser ? t('chat.message.user') : t('chat.message.assistant')}
            </span>
          </div>

          <div
            className={cn(
              'rounded-2xl px-4 py-2.5 text-sm leading-relaxed break-words',
              isUser && 'bg-primary text-primary-foreground rounded-tr-md',
              !isUser && 'bg-muted text-foreground rounded-tl-md markdown-content',
            )}
            onContextMenu={handleContextMenu}
          >
            {editing ? (
              <div className="flex flex-col gap-2 min-w-[240px]">
                <textarea
                  value={editValue}
                  onChange={(e) => setEditValue(e.target.value)}
                  className="w-full resize-none rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  rows={3}
                  autoFocus
                />
                <div className="flex justify-end gap-2">
                  <button
                    onClick={handleEditCancel}
                    className="rounded-md px-3 py-1 text-xs text-muted-foreground hover:bg-background/50 transition-colors"
                  >
                    {t('chat.edit.cancel')}
                  </button>
                  <button
                    onClick={handleEditSave}
                    className="flex items-center gap-1 rounded-md bg-primary px-3 py-1 text-xs text-primary-foreground hover:bg-primary/90 transition-colors"
                  >
                    <Send className="h-3 w-3" />
                    {t('chat.edit.save')}
                  </button>
                </div>
              </div>
            ) : isUser ? (
              <span className="whitespace-pre-wrap">{message.content}</span>
            ) : (
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
                {message.content}
              </ReactMarkdown>
            )}
          </div>

          {message.tool_calls && message.tool_calls.length > 0 && (
            <div className="mt-2 space-y-1.5 w-full">
              {message.tool_calls.map((tc) => (
                <div
                  key={tc.id}
                  className="rounded-lg border border-border bg-background/60 px-3 py-2 text-xs"
                >
                  <div className="flex items-center gap-1.5 text-muted-foreground mb-1">
                    <Wrench className="h-3 w-3" />
                    <span className="font-medium">{tc.function?.name}</span>
                  </div>
                  <pre className="overflow-x-auto font-mono text-muted-foreground">
                    {tc.function?.arguments}
                  </pre>
                </div>
              ))}
            </div>
          )}

          {message.tool_results && message.tool_results.length > 0 && (
            <div className="mt-2 space-y-1.5 w-full">
              {message.tool_results.map((tr, idx) => (
                <div
                  key={`${tr.tool_call_id}-${idx}`}
                  className="rounded-lg border border-border bg-green-500/5 px-3 py-2 text-xs"
                >
                  <div className="flex items-center gap-1.5 text-green-600 dark:text-green-400 mb-1">
                    <CheckCircle2 className="h-3 w-3" />
                    <span className="font-medium">{tr.name}</span>
                  </div>
                  <pre className="overflow-x-auto font-mono text-muted-foreground">
                    {tr.content}
                  </pre>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {contextMenu && (
        <ChatContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          isUser={isUser}
          onRecall={handleRecall}
          onEdit={openEdit}
          onDelete={handleDelete}
          onClose={() => setContextMenu(null)}
        />
      )}
    </>
  )
}
