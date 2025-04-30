import { DataSourcesPanel } from './components/data-sources/DataSourcesPanel'
import MCPPanel from './components/oauth/MCPPanel'

function App(): React.JSX.Element {
  return (
    <div className="flex items-center flex-col gap-8 py-10 flex-1 text-foreground">
      <style>
        {`
          :root {
            --direction: 1;
          }
        `}
      </style>
      <div className="flex w-full max-w-4xl flex-col gap-8">
        <div>
          <h1 className="text-4xl">My Twin</h1>
        </div>
        <MCPPanel />
        <DataSourcesPanel />
      </div>
    </div>
  )
}

export default App
