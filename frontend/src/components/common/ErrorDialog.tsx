import { AlertCircle, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useI18n } from '@/hooks/useI18n'

interface ErrorDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  message: string
  details?: string
}

export function ErrorDialog({ open, onOpenChange, title, message, details }: ErrorDialogProps) {
  const { t } = useI18n()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader className="text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="h-6 w-6 text-red-500" />
          </div>
          <DialogTitle className="text-base font-semibold">{title}</DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground">
            {message}
          </DialogDescription>
        </DialogHeader>
        
        {details && (
          <div className="mt-4 rounded-md border border-border bg-muted/50 p-3 text-xs text-muted-foreground">
            <div className="mb-1 font-medium">{t('errorDialog.details')}</div>
            <div className="break-all">{details}</div>
          </div>
        )}
        
        <DialogFooter className="justify-center">
          <Button onClick={() => onOpenChange(false)}>
            <X className="mr-1.5 h-4 w-4" />
            {t('errorDialog.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}