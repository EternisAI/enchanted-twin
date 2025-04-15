import { useQuery } from '@apollo/client'
import { GetConditionsDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'

export default function ChatContainer() {
  const { data, loading, error, refetch } = useQuery(GetConditionsDocument)

  if (loading) return <div>Loading...</div>
  if (error) return <div>Error: {error.message}</div>

  console.log('data', data)

  return (
    <div>
      <h1>Conditions</h1>
      <div>
        <ul className="bg-transparent">
          {data?.conditions.map((condition) => <li key={condition.id}>{condition.name}</li>)}
        </ul>
      </div>
      <Button onClick={() => refetch()}>Refetch</Button>
    </div>
  )
}
