import Versions from './components/Versions'
import { Button } from './components/ui/button'

function App(): React.JSX.Element {
  const ipcHandle = (): void => window.electron.ipcRenderer.send('ping')
  return (
    <div
      className="flex  items-center flex-col gap-8 py-10"
      style={{
        viewTransitionName: 'page-content'
      }}
    >
      <style>
        {`
          :root {
            --direction: 1;
          }
        `}
      </style>
      <div className="flex w-full max-w-2xl flex-col gap-4 ">
        <div>
          <h1>Home Page</h1>
        </div>
        <div className="actions">
          <Button variant="secondary" onClick={ipcHandle}>
            Send IPC
          </Button>
          <Versions></Versions>
        </div>
      </div>
    </div>
  )
}

export default App
