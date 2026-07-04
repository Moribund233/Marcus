import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Grid3X3,
  Settings,
  Package,
  Wrench,
} from 'lucide-react'

const categories = [
  { id: 'all', label: '所有工具', icon: Grid3X3 },
  { id: 'text', label: '文本处理', icon: Wrench },
  { id: 'image', label: '图像处理', icon: Wrench },
  { id: 'network', label: '网络工具', icon: Wrench },
  { id: 'dev', label: '开发工具', icon: Package },
  { id: 'file', label: '文件工具', icon: Wrench },
] as const

interface SidebarProps {
  active: string
  onSelect: (id: string) => void
}

export function Sidebar({ active, onSelect }: SidebarProps) {
  return (
    <aside className="flex w-56 flex-col border-r border-border bg-card">
      <div className="flex flex-col gap-1 p-3">
        <span className="px-3 text-xs font-medium text-muted-foreground uppercase tracking-wider">
          分类
        </span>
        <div className="mt-2 flex flex-col gap-0.5">
          {categories.map((cat) => (
            <Button
              key={cat.id}
              variant="ghost"
              className={cn(
                'justify-start gap-3 px-3 font-normal',
                active === cat.id && 'bg-accent text-accent-foreground',
              )}
              onClick={() => onSelect(cat.id)}
            >
              <cat.icon className="h-4 w-4" />
              {cat.label}
            </Button>
          ))}
        </div>
      </div>

      <div className="mt-auto border-t border-border p-3">
        <Button
          variant="ghost"
          className="justify-start gap-3 px-3 w-full font-normal"
          onClick={() => onSelect('settings')}
        >
          <Settings className="h-4 w-4" />
          设置
        </Button>
      </div>
    </aside>
  )
}
