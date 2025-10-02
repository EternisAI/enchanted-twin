import { createFileRoute } from '@tanstack/react-router'
import { FileBrowser } from '@renderer/components/data-sources/files/FileBrowser'

export const Route = createFileRoute('/files/')({
  component: RouteComponent
})

function RouteComponent() {
  return <FileBrowser />
}
