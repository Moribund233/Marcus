import { useEffect, useRef } from 'react'
import { useI18n } from '@/hooks/useI18n'
import { cn } from '@/lib/utils'

interface ChatContextMenuProps {
  x: number
  y: number
  isUser: boolean
  onRecall: () => void
  onEdit: () => void
  onDelete: () => void
  onClose: () => void
}

export function ChatContextMenu({
  x,
  y,
  isUser,
  onRecall,
  onEdit,
  onDelete,
  onClose,
}: ChatContextMenuProps) {
  const { t } = useI18n()
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose()
      }
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleEscape)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [onClose])

  const items = isUser
    ? [
        { label: t('chat.contextMenu.edit'), action: onEdit, danger: false },
        { label: t('chat.contextMenu.delete'), action: onDelete, danger: true },
      ]
    : [
        { label: t('chat.contextMenu.recall'), action: onRecall, danger: false },
        { label: t('chat.contextMenu.delete'), action: onDelete, danger: true },
      ]

  const adjustedX = Math.min(x, window.innerWidth - 160)
  const adjustedY = Math.min(y, window.innerHeight - 160)

  return (
    <div
      ref={menuRef}
      className="fixed z-50 min-w-[140px] rounded-lg border border-border bg-popover py-1 shadow-lg"
      style={{ left: adjustedX, top: adjustedY }}
    >
      {items.map((item) => (
        <button
          key={item.label}
          className={cn(
            'w-full px-3 py-1.5 text-left text-sm transition-colors hover:bg-accent',
            item.danger ? 'text-destructive hover:text-destructive' : 'text-popover-foreground',
          )}
          onClick={() => {
            item.action()
            onClose()
          }}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}
