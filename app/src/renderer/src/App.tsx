import { DataSourcesPanel } from './components/data-sources/DataSourcesPanel'
import MCPPanel from './components/oauth/MCPPanel'
import { ScrollArea } from './components/ui/scroll-area'

function App(): React.JSX.Element {
  return (
    <div className="flex flex-col gap-8 flex-1 text-foreground w-full h-full ">
      <ScrollArea className="h-full">
        <div className="flex flex-col gap-8 p-4 max-w-4xl self-center justify-center">
          <div>
            <h1 className="text-4xl">Home</h1>
          </div>
          <MCPPanel />
          <DataSourcesPanel showStatus={true} />
        </div>
      </ScrollArea>
    </div>
  )
}

export default App
