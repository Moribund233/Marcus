import { useState, useEffect } from 'react'
import { useI18n } from '@/hooks/useI18n'
import { GetRecentTools, ListConversations } from '../../wailsjs/go/main/App'
import { model } from '../../wailsjs/go/models'
import { MessageSquare, Clock, Sparkles } from 'lucide-react'

interface WelcomePageProps {
  onSelectConversation: (id: string) => void
  onSelectTool: (tool: model.ToolInfo) => void
  onNewConversation: () => void
}

export function WelcomePage({ onSelectConversation, onSelectTool, onNewConversation }: WelcomePageProps) {
  const { t } = useI18n()
  const [conversations, setConversations] = useState<model.Conversation[]>([])
  const [recentTools, setRecentTools] = useState<model.ToolInfo[]>([])

  const hour = new Date().getHours()
  let greetingKey: string
  if (hour < 12) {
    greetingKey = 'welcome.greeting.morning'
  } else if (hour < 18) {
    greetingKey = 'welcome.greeting.afternoon'
  } else {
    greetingKey = 'welcome.greeting.evening'
  }

  useEffect(() => {
    ListConversations(5).then((list) => setConversations(list ?? [])).catch(() => {})
    GetRecentTools(5).then((list) => setRecentTools(list ?? [])).catch(() => {})
  }, [])

  return (
    <div className="flex flex-1 flex-col items-center justify-center overflow-y-auto p-8">
      <div className="flex max-w-lg flex-col items-center gap-8">
        <div className="flex flex-col items-center gap-2 text-center">
          <div className="rounded-full bg-primary/10 p-4">
            <Sparkles className="h-8 w-8 text-primary" />
          </div>
          <h1 className="text-2xl font-semibold text-foreground">{t(greetingKey)}</h1>
          <p className="text-sm text-muted-foreground">{t('welcome.subtitle')}</p>
        </div>

        {conversations.length > 0 && (
          <div className="w-full">
            <h2 className="mb-3 flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              <MessageSquare className="h-3.5 w-3.5" />
              {t('welcome.recentConversations')}
            </h2>
            <div className="flex flex-col gap-1">
              {conversations.map((conv) => (
                <button
                  key={conv.id}
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-left text-sm text-foreground transition-colors hover:bg-accent"
                  onClick={() => onSelectConversation(conv.id)}
                >
                  <MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <span className="truncate">{conv.title}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {conversations.length === 0 && (
          <button
            className="inline-flex items-center gap-2 rounded-lg border border-border bg-background px-4 py-2 text-sm text-foreground transition-colors hover:bg-accent"
            onClick={onNewConversation}
          >
            <MessageSquare className="h-4 w-4" />
            {t('sidebar.conversations.new')}
          </button>
        )}

        {recentTools.length > 0 && (
          <div className="w-full">
            <h2 className="mb-3 flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">
              <Clock className="h-3.5 w-3.5" />
              {t('welcome.recentTools')}
            </h2>
            <div className="flex flex-col gap-1">
              {recentTools.map((tool) => (
                <button
                  key={tool.id}
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-left text-sm text-foreground transition-colors hover:bg-accent"
                  onClick={() => onSelectTool(tool)}
                >
                  <div className="flex h-6 w-6 items-center justify-center rounded bg-primary/10 text-xs font-medium text-primary">
                    {tool.display_name.charAt(0)}
                  </div>
                  <span className="truncate">{tool.display_name}</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {conversations.length === 0 && recentTools.length === 0 && (
          <p className="text-xs text-muted-foreground">
            {t('welcome.noConversations')}
          </p>
        )}
      </div>
    </div>
  )
}
