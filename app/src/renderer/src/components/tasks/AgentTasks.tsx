import { useMutation, useQuery } from '@apollo/client'
import { MessageCircle, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@radix-ui/react-tooltip'
import { Badge } from '@renderer/components/ui/badge'
import { Button } from '@renderer/components/ui/button'
import { Card } from '@renderer/components/ui/card'
import {
  AgentTask,
  DeleteAgentTaskDocument,
  GetAgentTasksDocument
} from '@renderer/graphql/generated/graphql'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'

export default function AgentTasks() {
  const { data, loading, error, refetch } = useQuery(GetAgentTasksDocument, {
    fetchPolicy: 'network-only'
  })
  const [deleteAgentTask] = useMutation(DeleteAgentTaskDocument, {
    onCompleted: () => {
      toast.success('Agent task deleted')
      refetch()
    },
    onError: (error: Error) => {
      toast.error(`Error deleting agent task: ${error.message}`)
    }
  })

  const agentTasks = [...(data?.getAgentTasks || [])]
    .filter((task) => !task.terminatedAt)
    .sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime())

  console.log(data?.getAgentTasks, agentTasks)

  return (
    <Card className="p-4 w-full overflow-y-auto">
      {loading && <div className="py-4 text-center">Loading tasks...</div>}
      {error && <div className="p-4 text-center text-red-500">Error: {error.message}</div>}

      <div className="flex flex-col gap-4 pb-6">
        {agentTasks.length === 0 ? (
          <EmptyTasksState />
        ) : (
          agentTasks.map((task) => (
            <AgentTaskRow
              key={task.id}
              task={task}
              onDelete={(id) => deleteAgentTask({ variables: { id } })}
            />
          ))
        )}
      </div>
    </Card>
  )
}

function EmptyTasksState() {
  const { setQuery, openOmnibar } = useOmnibarStore()

  const handleSuggestionClick = (suggestion: string) => {
    setQuery(suggestion)
    openOmnibar()
  }

  const examples = [
    'Send me a joke on Telegram every day at 9am',
    'Remind me to move the bins to the street every Wednesday at 8pm',
    'Summarize the latest crypto news every Monday morning'
  ]

  return (
    <div className="flex flex-col items-center justify-center py-20 px-8 gap-4">
      <div className="text-center max-w-md mb-12 flex flex-col gap-4">
        <h3 className="text-xl font-medium mb-8 text-balance max-w-sm">
          Create a task to automate recurring activities
        </h3>
      </div>

      <div className="w-full max-w-lg">
        <div className="flex flex-col gap-4">
          {examples.map((example, index) => (
            <Button
              key={index}
              variant="outline"
              size="lg"
              onClick={() => handleSuggestionClick(example)}
              className="w-full group text-left whitespace-normal flex items-start h-fit p-4"
            >
              <div className="flex justify-between items-center w-full">
                <span className="flex-1 pr-4">{example}</span>
                <span className="text-xs text-primary opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                  <MessageCircle className="w-4 h-4" />
                </span>
              </div>
            </Button>
          ))}
        </div>
      </div>
    </div>
  )
}

type Props = {
  task: AgentTask
  onDelete?: (id: string) => void
}

function AgentTaskRow({ task, onDelete }: Props) {
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-gray-300 p-4 w-full">
      <div className="flex justify-between items-start gap-4">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-4">
            <p className="text-lg font-medium">{task.name}</p>
            <Badge variant="outline">{task.schedule}</Badge>
          </div>
          {/* {task.plan && <p className="text-sm text-muted-foreground">{task.plan}</p>} */}
        </div>

        {onDelete && (
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-destructive hover:bg-destructive/10"
                  onClick={() => onDelete(task.id)}
                >
                  <Trash2 className="w-4 h-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                <div className="flex items-center gap-2 bg-popover p-2 rounded-md border shadow-md">
                  <p>Delete agent task</p>
                </div>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        )}
      </div>
    </div>
  )
}
