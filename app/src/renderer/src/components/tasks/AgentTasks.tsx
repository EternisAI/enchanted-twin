import { useMutation, useQuery } from '@apollo/client'
import { formatDistanceToNow } from 'date-fns'
import { Trash2 } from 'lucide-react'
import { toast } from 'sonner'

import { Tooltip, TooltipContent, TooltipTrigger } from '@radix-ui/react-tooltip'
import { Badge } from '@renderer/components/ui/badge'
import { Button } from '@renderer/components/ui/button'
import { Card } from '@renderer/components/ui/card'
import {
  AgentTask,
  DeleteAgentTaskDocument,
  GetAgentTasksDocument
} from '@renderer/graphql/generated/graphql'
import { formatRRuleToText } from '@renderer/lib/utils'

export default function AgentTasks() {
  const { data, loading } = useQuery(GetAgentTasksDocument, {
    fetchPolicy: 'network-only'
  })
  const [deleteAgentTask] = useMutation(DeleteAgentTaskDocument, {
    onCompleted: () => {
      toast.success('Agent task deleted')
    },
    onError: (error: Error) => {
      toast.error(`Error deleting agent task: ${error.message}`)
    }
  })

  const agentTasks = data?.getAgentTasks || []
  // data?.getAgentTasks.filter((task) => {
  //   if (!task.endedAt) return true
  //   return new Date(task.endedAt) > new Date()
  // }) || []

  console.log(data?.getAgentTasks, agentTasks)

  return (
    <Card className="p-6 w-full">
      <p className="text-md font-semibold">
        Manage the agent tasks that are running on the server.
      </p>

      {loading && <div className="py-4 text-center">Loading agent tasks...</div>}
      {/* {error && <div className="p-4 text-center text-red-500">Error: {error.message}</div>} */}

      <div className="flex flex-col gap-4">
        {agentTasks.length === 0 ? (
          <p className="text-md text-muted-foreground">No agent tasks found</p>
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

type Props = {
  task: AgentTask
  onDelete?: (id: string) => void
}

function AgentTaskRow({ task, onDelete }: Props) {
  const humanSchedule = formatRRuleToText(task.schedule)

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-gray-300 p-4 w-full">
      <div className="flex justify-between items-start gap-4">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-4">
            <p className="text-lg font-medium">{task.name}</p>
            {humanSchedule && <Badge variant="outline">Runs {humanSchedule}</Badge>}
          </div>
          {task.plan && <p className="text-sm text-muted-foreground">{task.plan}</p>}
        </div>

        {onDelete && (
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
        )}
      </div>

      <div className="flex gap-4 text-xs text-muted-foreground">
        {task.createdAt && <span>Created {formatDistanceToNow(new Date(task.createdAt))} ago</span>}
        {task.updatedAt && <span>Updated {formatDistanceToNow(new Date(task.updatedAt))} ago</span>}
        {task.endedAt && <span>Ended {formatDistanceToNow(new Date(task.endedAt))} ago</span>}
        {task.completedAt && (
          <span>Completed {formatDistanceToNow(new Date(task.completedAt))} ago</span>
        )}
      </div>
      <div className="flex flex-col gap-2 text-xs text-muted-foreground">
        {task.output && (
          <div>
            <strong>Agent Output:</strong> {task.output}
          </div>
        )}
      </div>
    </div>
  )
}
