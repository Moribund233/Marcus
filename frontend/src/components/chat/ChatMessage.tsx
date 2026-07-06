import { useI18n } from '@/hooks/useI18n'
import { cn } from '@/lib/utils'
import { User, Bot, Wrench, CheckCircle2 } from 'lucide-react'
import { model } from '../../../wailsjs/go/models'

interface ChatMessageProps {
  message: model.ConversationMessage
}

/**
 * 单条聊天消息组件。
 *
 * 根据消息角色渲染不同的头像与背景样式，并可选地展示工具调用（tool_calls）
 * 与工具执行结果（tool_results）。
 */
export function ChatMessage({ message }: ChatMessageProps) {
  const { t } = useI18n()
  const isUser = message.role === 'user'

  return (
    <div
      className={cn(
        'flex gap-3 px-6 py-4',
        isUser ? 'bg-background' : 'bg-muted/30',
      )}
    >
      <div
        className={cn(
          'flex h-8 w-8 shrink-0 items-center justify-center rounded-full',
          isUser ? 'bg-primary/10 text-primary' : 'bg-secondary text-secondary-foreground',
        )}
      >
        {isUser ? <User className="h-4 w-4" /> : <Bot className="h-4 w-4" />}
      </div>

      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-sm font-medium">
            {isUser ? t('chat.message.user') : t('chat.message.assistant')}
          </span>
        </div>

        {message.content && (
          <div className="text-sm whitespace-pre-wrap leading-relaxed text-foreground">
            {message.content}
          </div>
        )}

        {message.tool_calls && message.tool_calls.length > 0 && (
          <div className="mt-3 space-y-2">
            {message.tool_calls.map((tc) => (
              <div
                key={tc.id}
                className="rounded-md border border-border bg-background/60 px-3 py-2 text-xs"
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
          <div className="mt-3 space-y-2">
            {message.tool_results.map((tr, idx) => (
              <div
                key={`${tr.tool_call_id}-${idx}`}
                className="rounded-md border border-border bg-green-500/5 px-3 py-2 text-xs"
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
  )
}
