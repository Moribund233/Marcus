import { cn } from '@/lib/utils'

interface ToolItem {
  id: string
  name: string
  description: string
  icon: string
  category: string
}

// placeholder — will come from Go backend later
const PLACEHOLDER_TOOLS: ToolItem[] = [
  { id: '1', name: 'JSON 格式化', description: '格式化与验证 JSON 数据', icon: '{ }', category: 'text' },
  { id: '2', name: 'Base64 编解码', description: 'Base64 编码与解码', icon: 'B64', category: 'text' },
  { id: '3', name: '时间戳转换', description: 'Unix 时间戳与日期互转', icon: '🕐', category: 'text' },
  { id: '4', name: '图片识别 (OCR)', description: '基于 PaddleOCR 的图片文字识别', icon: '👁', category: 'image' },
  { id: '5', name: '图片压缩', description: '压缩 PNG/JPEG/WebP 图片', icon: '📦', category: 'image' },
  { id: '6', name: 'JSON → Excel', description: '将 JSON 数据导出为 Excel', icon: '📊', category: 'text' },
]

interface ToolGridProps {
  category: string
}

export function ToolGrid({ category }: ToolGridProps) {
  const tools = category === 'all'
    ? PLACEHOLDER_TOOLS
    : PLACEHOLDER_TOOLS.filter(t => t.category === category)

  return (
    <div className="flex-1 p-6 overflow-y-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">工具箱</h1>
        <p className="text-sm text-muted-foreground mt-1">
          选择工具开始使用
        </p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {tools.map((tool) => (
          <button
            key={tool.id}
            className="flex flex-col items-start gap-3 rounded-lg border border-border bg-card p-4 text-left transition-colors hover:bg-accent hover:text-accent-foreground cursor-pointer"
          >
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-primary/10 text-primary text-sm font-mono">
              {tool.icon}
            </div>
            <div>
              <div className="font-medium text-sm">{tool.name}</div>
              <div className="text-xs text-muted-foreground mt-0.5 line-clamp-2">
                {tool.description}
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
