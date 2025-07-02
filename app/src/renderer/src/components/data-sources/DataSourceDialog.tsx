import { Clock, ExternalLink, FileUp, Settings, Upload, CheckCircle } from 'lucide-react'
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
import { Card } from '../ui/card'
import { cn } from '@renderer/lib/utils'
import { ScrollArea } from '../ui/scroll-area'

export const DataSourceDialog = ({
  selectedSource,
  onClose,
  pendingDataSources,
  onFileSelect,
  onAddSource,
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
      <DialogContent
        className="max-w-2xl h-[calc(100vh-2rem)] sm:h-auto sm:max-h-[90vh] p-0 flex flex-col overflow-hidden"
        style={{ display: 'flex', flexDirection: 'column' }}
      >
        <DialogHeader className="p-4 sm:p-6 border-b flex-shrink-0">
          <div className="flex items-start gap-3 sm:gap-4">
            <div className="p-2 sm:p-3 bg-primary/10 rounded-lg flex-shrink-0">
              {selectedSource.icon}
            </div>
            <div className="flex-1 min-w-0">
              <DialogTitle className="text-lg sm:text-xl">
                Import {selectedSource.label}
              </DialogTitle>
              <DialogDescription className="mt-1">
                <span className="block sm:inline">{selectedSource.description}</span>
                {EXPORT_INSTRUCTIONS[selectedSource.name]?.timeEstimate && (
                  <span className="flex items-center gap-2 mt-1 text-xs sm:text-sm">
                    <Clock className="h-3 w-3 sm:h-4 sm:w-4 text-muted-foreground/50" />
                    {EXPORT_INSTRUCTIONS[selectedSource.name]?.timeEstimate}
                  </span>
                )}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <ScrollArea className="flex-1 min-h-0 overflow-y-auto">
          <div className="px-4 sm:px-6 py-3 sm:py-4 pb-6">
            {customComponent ? (
              <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
                <TabsList className="grid grid-cols-2 p-1 bg-muted h-auto mb-3 sm:mb-6">
                  <TabsTrigger
                    value="file"
                    className="data-[state=active]:bg-background data-[state=active]:shadow-sm text-xs sm:text-sm"
                  >
                    <FileUp className="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
                    <span className="hidden xs:inline">File Upload</span>
                    <span className="xs:hidden">File</span>
                  </TabsTrigger>
                  <TabsTrigger
                    value={customComponent.name}
                    className="data-[state=active]:bg-background data-[state=active]:shadow-sm text-xs sm:text-sm"
                  >
                    <Settings className="h-3 w-3 sm:h-4 sm:w-4 mr-1 sm:mr-2" />
                    {customComponent.name}
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="file" className="mt-0">
                  <StandardFileUpload
                    selectedSource={selectedSource}
                    pendingDataSources={pendingDataSources}
                    onFileSelect={onFileSelect}
                  />
                </TabsContent>

                <TabsContent value={customComponent.name} className="mt-0">
                  {customComponent.component}
                </TabsContent>
              </Tabs>
            ) : (
              <StandardFileUpload
                selectedSource={selectedSource}
                pendingDataSources={pendingDataSources}
                onFileSelect={onFileSelect}
              />
            )}
          </div>
        </ScrollArea>

        <DialogFooter className="px-4 sm:px-6 py-3 sm:py-4 border-t flex-shrink-0">
          <Button variant="outline" onClick={onClose} size="sm" className="sm:size-default">
            Cancel
          </Button>
          <Button
            onClick={onAddSource}
            disabled={
              !pendingDataSources[selectedSource.name] ||
              !pendingDataSources[selectedSource.name]?.path
            }
            size="sm"
            className="sm:size-default"
          >
            Add Data Source
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

const StandardFileUpload = ({
  selectedSource,
  pendingDataSources,
  onFileSelect,
  onFileDrop
}: {
  selectedSource: DataSource
  pendingDataSources: Record<string, PendingDataSource>
  onFileSelect: () => void
  onFileDrop?: (files: File[], sourceName: string) => Promise<void>
}) => {
  const [isDragOver, setIsDragOver] = useState(false)
  const hasSelectedFile = !!pendingDataSources[selectedSource.name]?.path
  const instructions = EXPORT_INSTRUCTIONS[selectedSource.name]

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
  }

  return (
    <>
      <div className="flex flex-col gap-4 sm:gap-6">
        {/* Export Instructions */}
        <details className="group" open>
          <summary className="cursor-pointer flex items-center justify-between text-sm font-semibold mb-3 hover:text-primary">
            <span>How to export your data</span>
            <span className="text-xs text-muted-foreground ml-2 group-open:hidden">Show steps</span>
          </summary>
          <div className="flex flex-col gap-3 sm:gap-4 py-2 sm:py-4">
            {instructions?.steps.map((step, index) => (
              <div key={index} className="flex gap-2">
                <div className="flex-shrink-0 w-5 h-5 sm:w-6 sm:h-6 rounded-full bg-accent flex items-center justify-center">
                  <span className="text-xs sm:text-sm font-medium text-accent-foreground">
                    {index + 1}
                  </span>
                </div>
                <p className="text-xs sm:text-sm text-foreground pt-0.5">{step}</p>
              </div>
            ))}
          </div>

          {instructions?.link && (
            <Button variant="outline" size="sm" className="mt-2 w-full sm:w-auto" asChild>
              <a
                href={instructions.link}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center justify-center gap-2"
              >
                Open {selectedSource.label} Export Page
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            </Button>
          )}
        </details>

        {/* File Selection */}
        <div className="space-y-4">
          <h3 className="text-sm font-semibold">Select your export file</h3>
          <Card
            className={cn(
              'p-4 sm:p-6 border-2 border-dashed cursor-pointer transition-colors',
              isDragOver
                ? 'border-primary bg-primary/5 scale-[1.02]'
                : hasSelectedFile
                  ? 'border-primary bg-primary/5'
                  : 'hover:border-primary/50'
            )}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onClick={onFileSelect}
          >
            <div className="flex flex-col items-center gap-2 sm:gap-3 text-center">
              {hasSelectedFile ? (
                <>
                  <CheckCircle className="h-8 w-8 sm:h-10 sm:w-10 text-primary" />
                  <div className="space-y-1">
                    <p className="text-xs sm:text-sm font-medium">File selected</p>
                    <p className="text-xs text-muted-foreground max-w-[200px] sm:max-w-none truncate">
                      {truncatePath(pendingDataSources[selectedSource.name].path)}
                    </p>
                  </div>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      onFileSelect()
                    }}
                    className="text-xs sm:text-sm"
                  >
                    Change file
                  </Button>
                </>
              ) : (
                <>
                  <Upload
                    className={cn(
                      'h-8 w-8 sm:h-10 sm:w-10 transition-colors',
                      isDragOver ? 'text-primary' : 'text-muted-foreground'
                    )}
                  />
                  <div className="space-y-1">
                    <p className="text-xs sm:text-sm font-medium">
                      {isDragOver ? 'Drop your file here' : 'Drag & drop your file here'}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {selectedSource.fileRequirement}
                    </p>
                  </div>
                  <div className="flex items-center gap-2 sm:gap-3 text-xs text-muted-foreground w-full max-w-[200px]">
                    <div className="h-px bg-border flex-1"></div>
                    <span>or</span>
                    <div className="h-px bg-border flex-1"></div>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      onFileSelect()
                    }}
                    className="text-xs sm:text-sm"
                  >
                    Browse Files
                  </Button>
                </>
              )}
            </div>
          </Card>
        </div>
      </div>
    </>
  )
}
