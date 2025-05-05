import { DataSourcesPanel } from './components/data-sources/DataSourcesPanel'
import MCPPanel from './components/oauth/MCPPanel'
import { ScrollArea } from './components/ui/scroll-area'
import { useEffect, useState } from 'react'

export default function App(): React.JSX.Element {
  const [version, setVersion] = useState<string>('')

  useEffect(() => {
    const getVersion = async () => {
      const appVersion = await window.api.getAppVersion()
      setVersion(appVersion)
    }
    getVersion()
  }, [])

  return (
    <ScrollArea className="h-full w-full">
      <div className="flex flex-col gap-8 flex-1 text-foreground w-full h-full justify-center">
        <div className="flex flex-col gap-8 p-4 max-w-4xl self-center justify-center mx-auto">
          <MCPPanel />
          <DataSourcesPanel showStatus={true} />
          <div className="text-md text-muted-foreground text-center">Version {version}</div>
        </div>
      </div>
    </ScrollArea>
  )
}
