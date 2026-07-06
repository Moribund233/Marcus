import { useState, useRef, useCallback, KeyboardEvent } from 'react'
import { Button } from '@/components/ui/button'
import { useI18n } from '@/hooks/useI18n'
import { Send, Loader2 } from 'lucide-react'

interface ChatInputProps {
  disabled?: boolean
  loading?: boolean
  onSend: (content: string) => void
}

/**
 * 聊天输入框组件。
 *
 * 提供多行文本输入与发送按钮，支持 Shift+Enter 换行、Enter 发送。
 */
export function ChatInput({ disabled, loading, onSend }: ChatInputProps) {
  const { t } = useI18n()
  const [value, setValue] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const handleSend = useCallback(() => {
    const trimmed = value.trim()
    if (!trimmed || disabled || loading) return
    onSend(trimmed)
    setValue('')
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
    }
  }, [value, disabled, loading, onSend])

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        handleSend()
      }
    },
    [handleSend],
  )

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value)
    const el = e.target
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`
  }, [])

  return (
    <div className="flex items-end gap-2 border-t border-border bg-background p-4">
      <textarea
        ref={textareaRef}
        rows={1}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        disabled={disabled || loading}
        placeholder={t('chat.inputPlaceholder')}
        className="flex-1 resize-none rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50 max-h-40 min-h-[40px]"
      />
      <Button
        size="icon"
        disabled={!value.trim() || disabled || loading}
        onClick={handleSend}
      >
        {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
      </Button>
    </div>
  )
}
