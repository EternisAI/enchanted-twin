import Versions from './components/Versions'
import DragNDrop from './components/onboarding/DragNDrop'
import { Button } from './components/ui/button'

function App(): React.JSX.Element {
  const ipcHandle = (): void => window.electron.ipcRenderer.send('ping')
  return (
    <div className="flex  items-center flex-col gap-8 py-10 h-full flex-1 text-foreground">
      <style>
        {`
          :root {
            --direction: 1;
          }
        `}
      </style>
      <div className="flex w-full max-w-2xl flex-col gap-4 ">
        <div>
          <h1>My Twin</h1>
        </div>
        <div className="actions">
          <DragNDrop />
          <Button variant="secondary" onClick={ipcHandle}>
            Send IPC
          </Button>
          {process.env.NODE_ENV === 'development' && <Versions></Versions>}
        </div>
      </div>
    </div>
  )
}

export default App
