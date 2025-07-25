import { useMutation, useQuery } from '@apollo/client'
import { AlarmClockCheckIcon, Bell, BellOff, MessageCircle, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@radix-ui/react-tooltip'
import { Badge } from '@renderer/components/ui/badge'
import { Button } from '@renderer/components/ui/button'
import {
  AgentTask,
  DeleteAgentTaskDocument,
  GetAgentTasksDocument,
  UpdateAgentTaskDocument
} from '@renderer/graphql/generated/graphql'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'

export default function AgentTasks() {
  const { openOmnibar } = useOmnibarStore()
  const { data, loading, error, refetch } = useQuery(GetAgentTasksDocument, {
    fetchPolicy: 'network-only'
  })
  const [deleteAgentTask] = useMutation(DeleteAgentTaskDocument, {
    onCompleted: () => {
      toast.success('Task deleted')
      refetch()
    },
    onError: (error: Error) => {
      toast.error(`Error deleting agent task: ${error.message}`)
    }
  })

  const [updateAgentTask] = useMutation(UpdateAgentTaskDocument, {
    onCompleted: () => {
      toast.success('Task updated')
      refetch()
    },
    onError: (error: Error) => {
      toast.error(`Error updating agent task: ${error.message}`)
    }
  })

  const agentTasks = [...(data?.getAgentTasks || [])]

  return (
    <div className="p-4 w-full overflow-y-auto">
      {loading && <div className="py-4 text-center">Loading tasks...</div>}
      {error && <div className="p-4 text-center text-red-500">Error: {error.message}</div>}

      <div className="flex flex-col gap-4 pb-6">
        <div className="flex items-center justify-between gap-2">
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <AlarmClockCheckIcon className="w-6 h-6" />
            Tasks
          </h1>
          <Button
            onClick={() => openOmnibar('Create a task to automate recurring activities')}
            variant="default"
            size="sm"
          >
            <Plus className="w-4 h-4" />
            Create task
          </Button>
        </div>
        {agentTasks.length === 0 ? (
          <EmptyTasksState />
        ) : (
          agentTasks.map((task) => (
            <AgentTaskRow
              key={task.id}
              task={task}
              onDelete={(id) => deleteAgentTask({ variables: { id } })}
              onUpdate={(id, notify) => updateAgentTask({ variables: { id, notify } })}
            />
          ))
        )}
      </div>
    </div>
  )
}

function EmptyTasksState() {
  const { setQuery, openOmnibar } = useOmnibarStore()

  const handleSuggestionClick = (suggestion: string) => {
    setQuery(suggestion)
    openOmnibar('Create a task to automate recurring activities')
  }

  const examples = [
    'Summarize the daily AI news every Monday morning',
    'Check BTC price every minute and send me a message if it breaks $123,000',
    'Remind me to move the bins to the street every Wednesday at 8pm'
  ]

  return (
    <div className="flex flex-col items-center justify-center py-20 px-8 gap-4">
      <div className="text-center items-center max-w-md flex flex-col gap-4">
        <AlarmClockCheckIcon className="w-10 h-10 text-muted-foreground" />
        <h3 className="text-2xl font-semibold mb-8 text-balance max-w-sm">
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
          <Button
            variant="outline"
            size="lg"
            className="w-full"
            onClick={() => handleSuggestionClick('')}
          >
            <Plus className="w-4 h-4" />
            Create your own
          </Button>
        </div>
      </div>
    </div>
  )
}

type Props = {
  task: Pick<AgentTask, 'id' | 'name' | 'schedule' | 'plan' | 'createdAt' | 'output' | 'notify'>
  onDelete?: (id: string) => void
  onUpdate?: (id: string, notify: boolean) => void
}

function AgentTaskRow({ task, onDelete, onUpdate }: Props) {
  return (
    <div className="flex flex-col gap-2 rounded-xl border border-border p-4 w-full">
      <div className="flex justify-between items-start gap-4">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-4">
            <p className="text-lg font-medium">{task.name}</p>
            <Badge variant="outline">{task.schedule}</Badge>
          </div>
          {task.plan && <p className="text-sm text-muted-foreground">{task.plan}</p>}
        </div>

        <div className="flex flex-row gap-2 items-center shrink-0">
          {onUpdate && (
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-black hover:bg-black/10"
                    onClick={() => onUpdate(task.id, !task.notify)}
                  >
                    {task.notify ? (
                      <Bell className="w-4 h-4 text-primary" />
                    ) : (
                      <BellOff className="w-4 h-4 text-primary" />
                    )}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  <div className="flex items-center gap-2 bg-popover p-2 rounded-md border shadow-md">
                    <p>{task.notify ? 'Disable notifications' : 'Enable notifications'}</p>
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
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
                    <p>Delete task</p>
                  </div>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          )}
        </div>
      </div>
    </div>
  )
}
