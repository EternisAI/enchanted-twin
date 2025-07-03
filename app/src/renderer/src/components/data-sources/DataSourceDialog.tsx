import { Clock, ExternalLink, FileUp, Settings, Upload } from 'lucide-react'
import { Button } from '../ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '../ui/dialog'
import { DataSource, PendingDataSource } from './types'
import { truncatePath } from './utils'
import { EXPORT_INSTRUCTIONS } from './export-instructions'
import { ReactNode, useState } from 'react'
import { TabsContent } from '../ui/tabs'
import { TabsList, TabsTrigger } from '../ui/tabs'
import { Tabs } from '../ui/tabs'
import { cn } from '@renderer/lib/utils'

export const DataSourceDialog = ({
  selectedSource,
  onClose,
  pendingDataSources,
  onFileSelect,
  onAddSource,
  onFileDrop,
  customComponent
}: {
  selectedSource: DataSource | null
  onClose: () => void
  pendingDataSources: Record<string, PendingDataSource>
  onFileSelect: () => void
  onAddSource: () => void
  onFileDrop?: (files: File[], sourceName: string) => Promise<void>
  customComponent?: {
    name: string
    component: ReactNode
  }
}) => {
  const [activeTab, setActiveTab] = useState<string>(
    customComponent ? customComponent.name : 'file'
  )

  if (!selectedSource) return null

  return (
    <Dialog open={!!selectedSource} onOpenChange={onClose}>
      <DialogContent className="max-w-3xl">
        <DialogHeader>
          <DialogTitle>Add {selectedSource.label} Data</DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground flex items-center gap-2">
            <Clock className="h-4 w-4 text-muted-foreground/50" />
            {EXPORT_INSTRUCTIONS[selectedSource.name]?.timeEstimate}
          </DialogDescription>
        </DialogHeader>

        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <TabsList className={`grid ${customComponent ? 'grid-cols-2' : 'grid-cols-1'} mb-6`}>
            <TabsTrigger value="file" className="flex py-2 items-center gap-2">
              <FileUp className="h-4 w-4" />
              File Upload
            </TabsTrigger>

            {customComponent && (
              <TabsTrigger value={customComponent.name} className="flex items-center gap-2">
                <Settings className="h-4 w-4" />
                {customComponent.name}
              </TabsTrigger>
            )}
          </TabsList>

          <TabsContent value="file">
            <StandardFileUpload
              selectedSource={selectedSource}
              pendingDataSources={pendingDataSources}
              onFileSelect={onFileSelect}
              onClose={onClose}
              onAddSource={onAddSource}
              onFileDrop={onFileDrop}
            />
          </TabsContent>

          {customComponent && (
            <TabsContent value="QR Code">{customComponent.component}</TabsContent>
          )}
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}

const StandardFileUpload = ({
  selectedSource,
  pendingDataSources,
  onFileSelect,
  onClose,
  onAddSource,
  onFileDrop
}: {
  selectedSource: DataSource
  pendingDataSources: Record<string, PendingDataSource>
  onFileSelect: () => void
  onClose: () => void
  onAddSource: () => void
  onFileDrop?: (files: File[], sourceName: string) => Promise<void>
}) => {
  const [isDragOver, setIsDragOver] = useState(false)

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(true)
  }

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    // Only set isDragOver to false if we're leaving the drop zone entirely
    if (!e.currentTarget.contains(e.relatedTarget as Node)) {
      setIsDragOver(false)
    }
  }

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)

    const files = Array.from(e.dataTransfer.files)
    if (files.length === 0) return

    // If we have a file drop handler, use it
    if (onFileDrop) {
      try {
        await onFileDrop(files, selectedSource.name)
      } catch (error) {
        console.error('Error handling dropped files:', error)
        // Fall back to file browser if drag and drop fails
        onFileSelect()
      }
    } else {
      // Fallback to file browser
      onFileSelect()
    }
    onClose()
  }

  return (
    <>
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

        <div
          className={cn(
            'flex flex-col gap-2 p-4 rounded-lg border-2 border-dashed transition-all duration-200 cursor-pointer',
            isDragOver
              ? 'border-primary bg-primary/5 scale-[1.02]'
              : 'border-border bg-card dark:bg-muted'
          )}
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          onClick={onFileSelect}
        >
          <div className="flex flex-col items-center gap-2 text-center">
            <div
              className={cn(
                'p-4 rounded-full transition-colors',
                isDragOver ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
              )}
            >
              <Upload className="w-5 h-5" />
            </div>

            <div className="space-y-2">
              <p
                className={cn(
                  'text-md font-medium transition-colors',
                  isDragOver ? 'text-primary' : 'text-foreground'
                )}
              >
                {isDragOver ? 'Drop your file here' : 'Drag & drop your file here'}
              </p>
              <p className="text-xs text-muted-foreground">
                {pendingDataSources[selectedSource.name]?.path
                  ? truncatePath(pendingDataSources[selectedSource.name]?.path)
                  : selectedSource.fileRequirement}
              </p>
            </div>

            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <div className="h-px bg-border flex-1"></div>
              <span>or</span>
              <div className="h-px bg-border flex-1"></div>
            </div>

            <Button
              variant="outline"
              onClick={(e) => {
                e.stopPropagation()
                onFileSelect()
              }}
            >
              Browse Files
            </Button>
          </div>
        </div>
      </div>
      <DialogFooter className="pt-4">
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button onClick={onAddSource} disabled={!pendingDataSources[selectedSource.name]}>
          Add Source
        </Button>
      </DialogFooter>
    </>
  )
}
