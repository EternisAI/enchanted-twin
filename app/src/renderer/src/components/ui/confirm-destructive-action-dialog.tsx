import { Button } from './button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from './dialog'

export function ConfirmDestructiveAction({
  title,
  description,
  onConfirm,
  onCancel = () => {},
  confirmText
}: {
  title: string
  description: string
  onConfirm: () => void
  onCancel?: () => void
  confirmText?: string
}) {
  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="destructive">{title}</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onCancel}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={onConfirm}>
            {confirmText || 'Confirm'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
