import { Clock, ExternalLink } from 'lucide-react'
import { Button } from '../ui/button'
import {
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '../ui/dialog'
import { Dialog } from '../ui/dialog'
import { DataSource, PendingDataSource } from './types'
import { truncatePath } from './utils'
import { EXPORT_INSTRUCTIONS } from './export-instructions'

export const DataSourceDialog = ({
  selectedSource,
  onClose,
  pendingDataSources,
  onFileSelect,
  onAddSource
}: {
  selectedSource: DataSource | null
  onClose: () => void
  pendingDataSources: Record<string, PendingDataSource>
  onFileSelect: () => void
  onAddSource: () => void
}) => {
  if (!selectedSource) return null

  return (
    <Dialog open={!!selectedSource} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Add {selectedSource.name} Data</DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground flex items-center gap-2">
            <Clock className="h-4 w-4 text-muted-foreground/50" />
            {EXPORT_INSTRUCTIONS[selectedSource.name]?.timeEstimate}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-8">
          <div className="space-y-4 py-4">
            <div className="rounded-lg">
              <ol className="space-y-4 flex flex-col gap-3">
                {EXPORT_INSTRUCTIONS[selectedSource.name]?.steps.map((step, index) => (
                  <li key={index} className="flex gap-3">
                    <div className="flex-shrink-0 w-6 h-6 rounded-full bg-accent flex items-center justify-center text-primary font-medium text-sm">
                      {index + 1}
                    </div>
                    <div className="flex-1">
                      <p className="text-sm">{step}</p>
                    </div>
                  </li>
                ))}
              </ol>
              {EXPORT_INSTRUCTIONS[selectedSource.name]?.link && (
                <Button variant="link" className="mt-4 p-0 h-auto text-primary" asChild>
                  <a
                    href={EXPORT_INSTRUCTIONS[selectedSource.name].link}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Open {selectedSource.name} Export Page <ExternalLink className="h-4 w-4" />
                  </a>
                </Button>
              )}
            </div>
          </div>

          <div className="flex flex-col gap-2 bg-card p-4 rounded-lg dark text-white">
            <div className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <div className="flex-1 h-9 px-3 py-1 rounded-md border bg-background text-sm">
                  {pendingDataSources[selectedSource.name]?.path
                    ? truncatePath(pendingDataSources[selectedSource.name]?.path)
                    : selectedSource.fileRequirement}
                </div>
                <Button onClick={onFileSelect}>Browse</Button>
              </div>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={onAddSource} disabled={!pendingDataSources[selectedSource.name]}>
            Add Source
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
