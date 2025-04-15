import { useQuery } from '@apollo/client'
import Versions from './components/Versions'
import { GetConditionsDocument } from './graphql/generated/graphql'
import { Button } from './components/ui/button'

function App(): React.JSX.Element {
  const ipcHandle = (): void => window.electron.ipcRenderer.send('ping')
  const { data, loading, error, refetch } = useQuery(GetConditionsDocument)

  if (loading) return <div>Loading...</div>
  if (error) return <div>Error: {error.message}</div>

  console.log('data', data)
  return (
    <div className="flex  items-center flex-col gap-8 py-10">
      <style>
        {`
          :root {
            --direction: 1;
          }
        `}
      </style>
      <div className="flex w-full max-w-2xl flex-col gap-4 ">
        <div>
          <h1>GraphQL</h1>
          <ul>
            {data?.conditions.map((condition) => <li key={condition.id}>{condition.name}</li>)}
          </ul>
          <Button onClick={() => refetch()}>Refetch</Button>
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
