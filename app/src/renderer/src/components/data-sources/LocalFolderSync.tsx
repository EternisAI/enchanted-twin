import { useState } from 'react'
import { FolderOpen, Upload, Unplug } from 'lucide-react'

import { useMutation, useQuery } from '@apollo/client'
import {
  AddTrackedFolderDocument,
  DeleteTrackedFolderDocument,
  GetTrackedFoldersDocument
} from '@renderer/graphql/generated/graphql'

import { Button } from '../ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card'
import { cn } from '@renderer/lib/utils'

export default function LocalFolderSync() {
  const { data, refetch } = useQuery(GetTrackedFoldersDocument)
  const [addTrackedFolder] = useMutation(AddTrackedFolderDocument)
  const [deleteTrackedFolder] = useMutation(DeleteTrackedFolderDocument)
  const [isDragOver, setIsDragOver] = useState(false)

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(true)
  }

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!e.currentTarget.contains(e.relatedTarget as Node)) {
      setIsDragOver(false)
    }
  }

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setIsDragOver(false)

    const items = Array.from(e.dataTransfer.items)
    const folderPaths: string[] = []

    const existingPaths = data?.getTrackedFolders?.map((folder) => folder.path) || []

    for (const item of items) {
      if (item.kind === 'file') {
        if ('webkitGetAsEntry' in item) {
          const entry = item.webkitGetAsEntry?.()
          if (entry && entry.isDirectory) {
            const fullPath = entry.fullPath
            const normalizedPath = fullPath.startsWith('/') ? fullPath.slice(1) : fullPath
            if (!existingPaths.includes(normalizedPath)) {
              folderPaths.push(normalizedPath)
            }
          }
        }
      }
    }

    if (folderPaths.length === 0) {
      console.log('No new folders to add')
      return
    }

    console.log('Adding folders:', folderPaths)

    for (const path of folderPaths) {
      try {
        await addTrackedFolder({
          variables: {
            input: {
              path,
              name: path.split('/').pop() || 'Untitled Folder'
            }
          }
        })
        console.log(`Successfully added folder: ${path}`)
      } catch (error) {
        console.error('Error adding tracked folder:', error)
      }
    }

    refetch()
  }

  const handleDeleteFolder = async (id: string) => {
    try {
      await deleteTrackedFolder({
        variables: { id }
      })
      refetch()
    } catch (error) {
      console.error('Error deleting tracked folder:', error)
    }
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric'
    })
  }

  const truncatePath = (path: string, maxLength: number = 50) => {
    if (path.length <= maxLength) return path
    return '...' + path.slice(-maxLength + 3)
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FolderOpen className="h-5 w-5" />
            Add Local Folders
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div
            className={cn(
              'flex flex-col gap-2 p-6 rounded-lg border-2 border-dashed transition-all duration-200 cursor-pointer',
              isDragOver
                ? 'border-primary bg-primary/5 scale-[1.02]'
                : 'border-border bg-card dark:bg-muted'
            )}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            <div className="flex flex-col items-center gap-2 text-center">
              <div
                className={cn(
                  'p-4 rounded-full transition-colors',
                  isDragOver ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
                )}
              >
                <Upload className="w-6 h-6" />
              </div>

              <div className="space-y-2">
                <p
                  className={cn(
                    'text-lg font-medium transition-colors',
                    isDragOver ? 'text-primary' : 'text-foreground'
                  )}
                >
                  {isDragOver ? 'Drop your folder here' : 'Drag & drop folders here'}
                </p>
                <p className="text-sm text-muted-foreground">
                  Drop folders to automatically sync and track changes
                </p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="flex flex-col gap-4">
        <h2 className="text-lg font-bold">Connected</h2>
        {data?.getTrackedFolders && data.getTrackedFolders.length > 0 ? (
          <div className="space-y-3">
            {data.getTrackedFolders.map((folder) => (
              <div
                key={folder.id}
                className="flex items-center gap-4 justify-between p-4 rounded-lg bg-card hover:bg-accent/80 transition-colors"
              >
                <div className="flex items-center bg-gray-200 rounded-mg p-2.25">
                  <FolderOpen className="h-6 w-6 text-muted-foreground flex-shrink-0" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 mb-2">
                    <h3 className="font-medium truncate">
                      {folder.name || folder.path.split('/').pop() || 'Untitled Folder'}
                    </h3>
                    {/* <Badge variant={folder.isEnabled ? 'default' : 'secondary'}>
                        {folder.isEnabled ? 'Active' : 'Inactive'}
                      </Badge> */}
                  </div>
                  <div className="flex items-center gap-2">
                    <p className="text-sm text-muted-foreground mb-2">
                      /{truncatePath(folder.path)}
                    </p>
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      width="2"
                      height="2"
                      viewBox="0 0 2 2"
                      fill="none"
                    >
                      <circle cx="1" cy="1" r="1" fill="#71717A" />
                    </svg>
                    <span className="text-xs text-muted-foreground">
                      Connected: {formatDate(folder.createdAt)}
                    </span>
                  </div>

                  {/* <div className="flex items-center gap-4 text-xs text-muted-foreground">
                      <div className="flex items-center gap-1">
                        <Calendar className="h-3 w-3" />
                      </div>
                      <div className="flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        <span>Updated: {formatDate(folder.updatedAt)}</span>
                      </div>
                    </div> */}
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeleteFolder(folder.id)}
                  className="ml-4 flex-shrink-0"
                >
                  <Unplug className="h-4 w-4" />
                  Disconnect
                </Button>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-8 text-muted-foreground">
            <FolderOpen className="h-12 w-12 mx-auto mb-4 opacity-50" />
            <p>No folders connected yet</p>
            <p className="text-sm">Drag and drop folders above to get started</p>
          </div>
        )}
      </div>
    </div>
  )
}
