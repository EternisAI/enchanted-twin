import LocalFilesTab from '@renderer/components/data-sources/LocalFilesTab'
import LocalFolderSync from '@renderer/components/data-sources/LocalFolderSync'
import { ScrollArea } from '@renderer/components/ui/scroll-area'

// dual pane interface?
export function FileBrowser() {
  return (
    <ScrollArea className="flex flex-col h-full w-full">
      <div className="flex flex-col w-full h-full gap-10 max-w-4xl mx-auto justify-center p-6">
        <LocalFilesTab />
        <LocalFolderSync />
      </div>
    </ScrollArea>
  )
}
