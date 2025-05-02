import { DataSourcesPanel } from './components/data-sources/DataSourcesPanel'
import MCPPanel from './components/oauth/MCPPanel'
import { ScrollArea } from './components/ui/scroll-area'

function App(): React.JSX.Element {
  return (
    <ScrollArea className="h-full w-full">
      <div className="flex flex-col gap-8 flex-1 text-foreground w-full h-full justify-center">
        <div className="flex flex-col gap-8 p-4 max-w-4xl self-center justify-center mx-auto">
          <MCPPanel />
          <DataSourcesPanel showStatus={true} />
        </div>
      </div>
    </ScrollArea>
  )
}

export default App
