import { useState, useCallback } from 'react'
import { Upload, File, Folder, Plus } from 'lucide-react'
import { Button } from '../ui/button'
import { cn } from '../../lib/utils'

export default function LocalFilesTab() {
  const [isDragging, setIsDragging] = useState(false)
  const [selectedFiles, setSelectedFiles] = useState<string[]>([])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)
  }, [])

  const handleDrop = useCallback(async (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragging(false)

    const files = Array.from(e.dataTransfer.files)
    if (files.length > 0) {
      try {
        const filePaths = files.map((file) =>
          (window.api.getPathForFile as unknown as (file: File) => string)(file)
        )
        const copiedPaths = await window.api.copyDroppedFiles(filePaths)
        if (copiedPaths && copiedPaths.length > 0) {
          setSelectedFiles((prev) => [...prev, ...copiedPaths])
        }
      } catch (error) {
        console.error('Error handling dropped files:', error)
      }
    }
  }, [])

  const handleSelectFiles = useCallback(async () => {
    try {
      const result = await window.api.selectFiles({
        filters: [
          { name: 'Documents', extensions: ['txt', 'pdf', 'doc', 'docx'] },
          { name: 'Spreadsheets', extensions: ['xls', 'xlsx', 'csv'] },
          { name: 'All Files', extensions: ['*'] }
        ]
      })

      if (!result.canceled && result.filePaths.length > 0) {
        setSelectedFiles((prev) => [...prev, ...result.filePaths])
      }
    } catch (error) {
      console.error('Error selecting files:', error)
    }
  }, [])

  return (
    <div className="flex flex-col h-full">
      {/* Drag and Drop Container */}
      <div
        className={cn(
          'relative rounded-lg border-2 border-dashed transition-all duration-200 p-8 mb-6',
          isDragging ? 'border-primary bg-primary/5' : 'border-border hover:border-muted-foreground'
        )}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        <div className="flex flex-col items-center justify-center space-y-4">
          <div
            className={cn(
              'rounded-full p-4 transition-colors',
              isDragging ? 'bg-primary/10' : 'bg-muted'
            )}
          >
            <Upload
              className={cn(
                'h-8 w-8 transition-colors',
                isDragging ? 'text-primary' : 'text-muted-foreground'
              )}
            />
          </div>

          <div className="text-center space-y-2">
            <p className="text-sm font-medium">Drag and drop files here</p>
            <p className="text-xs text-muted-foreground">or click to select files</p>
          </div>

          <Button onClick={handleSelectFiles} variant="outline" size="sm" className="gap-2">
            <Plus className="h-4 w-4" />
            Select Files
          </Button>
        </div>
      </div>

      {/* File List Placeholder */}
      <div className="flex-1 rounded-lg border bg-card">
        <div className="border-b px-4 py-3">
          <h3 className="text-sm font-medium">Local Files</h3>
        </div>

        <div className="p-4">
          {selectedFiles.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
              <Folder className="h-12 w-12 mb-4 opacity-20" />
              <p className="text-sm">No files selected</p>
              <p className="text-xs mt-1">Add files to get started</p>
            </div>
          ) : (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground mb-4">
                File list implementation coming soon...
              </p>
              {/* Temporary display of selected files */}
              {selectedFiles.map((filePath, index) => (
                <div key={index} className="flex items-center gap-2 p-2 rounded-md bg-muted/50">
                  <File className="h-4 w-4 text-muted-foreground" />
                  <span className="text-sm truncate flex-1">
                    {filePath.split('/').pop() || filePath}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
