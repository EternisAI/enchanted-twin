import React, { useCallback, useState } from 'react'

export default function DragNDrop() {
  const [pathToCopiedFiles, setPathToCopiedFiles] = useState<string[]>([])

  const handleDrop = async (e: React.DragEvent) => {
    e.preventDefault()

    const files = Array.from(e.dataTransfer.files)
    const paths = files.map((f) => window.api.getPathForFile(f))

    console.log('[Drop] Files:', files)
    console.log('[Drop] File paths:', paths)

    const pathsToCopiedFiles = await window.api.copyDroppedFiles(paths)
    console.log('[Copied] Files:', pathsToCopiedFiles)

    setPathToCopiedFiles((state) => [...state, ...pathsToCopiedFiles])
  }

  const handleDragOver = useCallback((e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    e.stopPropagation()
  }, [])

  return (
    <div className="flex flex-col gap-4">
      <div
        onDragOver={handleDragOver}
        onDrop={handleDrop}
        className="border-2 border-border rounded-md p-4 text-center border-dashed"
      >
        Drag and drop files here
      </div>
      {pathToCopiedFiles.length > 0 && (
        <div className="flex flex-col gap-2">
          <p>Files copied to: </p>
          {pathToCopiedFiles.map((file) => (
            <div key={file}>{file}</div>
          ))}
        </div>
      )}
    </div>
  )
}
