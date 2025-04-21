// import React, { useCallback, useState } from 'react'
// import { cn } from '@renderer/lib/utils'

// export default function DragNDrop() {
//   const [pathToCopiedFiles, setPathToCopiedFiles] = useState<string[]>([])
//   const [isDragging, setIsDragging] = useState(false)

//   const handleDrop = async (e: React.DragEvent) => {
//     // e.preventDefault()
//     // const files = Array.from(e.dataTransfer.files)
//     // const paths = files.map((f) => window.api.getPathForFile(f))
//     // console.log('[Drop] Files:', files)
//     // console.log('[Drop] File paths:', paths)
//     // const pathsToCopiedFiles = await window.api.copyDroppedFiles(paths)
//     // console.log('[Copied] Files:', pathsToCopiedFiles)
//     // setPathToCopiedFiles((state) => [...state, ...pathsToCopiedFiles])
//   }

//   const handleDragEnter = useCallback((e: React.DragEvent<HTMLDivElement>) => {
//     e.preventDefault()
//     e.stopPropagation()
//     setIsDragging(true)
//   }, [])

//   const handleDragOver = useCallback((e: React.DragEvent<HTMLDivElement>) => {
//     e.preventDefault()
//     e.stopPropagation()
//   }, [])

//   const handleDragLeave = useCallback((e: React.DragEvent<HTMLDivElement>) => {
//     e.preventDefault()
//     e.stopPropagation()
//     if (e.currentTarget === e.target) {
//       setIsDragging(false)
//     }
//   }, [])

//   return (
//     <div className="flex flex-col gap-4">
//       <div
//         onDragEnter={handleDragEnter}
//         onDragLeave={handleDragLeave}
//         onDragOver={handleDragOver}
//         onDrop={handleDrop}
//         className={cn('border-2 rounded-md p-4 text-center transition-all', {
//           'border-border': !isDragging,
//           'border-dashed': !isDragging,
//           'border-primary bg-accent/30': isDragging
//         })}
//       >
//         Drag and drop files here
//       </div>
//       {pathToCopiedFiles.length > 0 && (
//         <div className="flex flex-col gap-2">
//           <p>Files copied to: </p>
//           {pathToCopiedFiles.map((file) => (
//             <div key={file}>{file}</div>
//           ))}
//         </div>
//       )}
//     </div>
//   )
// }
