import { Component, type ReactNode } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import zh from '@/locales/zh-CN'
import en from '@/locales/en-US'

const LOCALE_MAP: Record<string, Record<string, string>> = { 'zh-CN': zh, 'en-US': en }

function t(key: string): string {
  try {
    const locale = localStorage.getItem('marcus_language') || 'zh-CN'
    return LOCALE_MAP[locale]?.[key] ?? key
  } catch {
    return LOCALE_MAP['zh-CN']?.[key] ?? key
  }
}

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info)
  }

  handleRetry = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8">
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-destructive/10">
            <AlertTriangle className="h-7 w-7 text-destructive" />
          </div>
          <h2 className="text-lg font-semibold">{t('errorBoundary.title')}</h2>
          <p className="max-w-md text-center text-sm text-muted-foreground">
            {this.state.error?.message ?? t('errorBoundary.unknownError')}
          </p>
          <button
            onClick={this.handleRetry}
            className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
          >
            <RefreshCw className="h-4 w-4" />
            {t('errorBoundary.retry')}
          </button>
        </div>
      )
    }

    return this.props.children
  }
}
