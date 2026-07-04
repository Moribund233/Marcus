import {
  Minus,
  Square,
  X,
} from 'lucide-react'
import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime'

import { Button } from '@/components/ui/button'

export function TitleBar() {
  return (
    <div
      className="flex h-10 items-center justify-between select-none"
      style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
    >
      <div className="flex items-center gap-2 ml-3">
        <div className="flex items-center gap-1.5">
          <div className="h-3 w-3 rounded-full bg-red-500" />
          <div className="h-3 w-3 rounded-full bg-yellow-500" />
          <div className="h-3 w-3 rounded-full bg-green-500" />
        </div>
        <span className="text-sm font-medium text-muted-foreground ml-2">
          Marcus
        </span>
      </div>

      <div className="flex" style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}>
        <Button
          variant="ghost"
          size="icon"
          className="h-10 w-10 rounded-none hover:bg-accent"
          onClick={WindowMinimise}
        >
          <Minus className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-10 w-10 rounded-none hover:bg-accent"
          onClick={WindowToggleMaximise}
        >
          <Square className="h-4 w-4" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-10 w-10 rounded-none hover:bg-destructive hover:text-destructive-foreground"
          onClick={Quit}
        >
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}
