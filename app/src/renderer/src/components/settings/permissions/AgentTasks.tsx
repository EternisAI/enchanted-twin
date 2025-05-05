import { useMutation, useQuery } from '@apollo/client'
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
import { formatDistanceToNow } from 'date-fns'
import { Trash2 } from 'lucide-react'
import { toast } from 'sonner'

const MOCK_AGENT_TASKS: AgentTask[] = [
  {
    id: '1',
    name: 'Send a mock list every day at 9am',
    schedule: 'RRULE:FREQ=DAILY;BYHOUR=9;BYMINUTE=0;BYSECOND=0',
    plan: 'Come up with a mock list to send user',
    createdAt: new Date(Date.now() - 24 * 60 * 60 * 1000),
    updatedAt: new Date()
  },
  {
    id: '2',
    name: 'Send a mock joke every month on the last day of the month at 6pm',
    schedule: 'RRULE:FREQ=DAILY;BYDAY=MO,TU,WE,TH,FR;BYHOUR=8;BYMINUTE=0;BYSECOND=0',
    plan: 'Think of the best jokes and send them to the user',
    createdAt: new Date(),
    updatedAt: new Date()
  },
  {
    id: '3',
    name: 'Send reminders',
    schedule: 'RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR;UNTIL=20251231T235959Z',
    plan: 'Send reminders every week on Mon, Wed, Fri until December 31, 2025',
    createdAt: new Date(),
    updatedAt: new Date()
  }
]

export default function AgentTasks() {
  const { data, loading } = useQuery(GetAgentTasksDocument)
  const [deleteAgentTask] = useMutation(DeleteAgentTaskDocument, {
    onCompleted: () => {
      toast.success('Agent task deleted')
    },
    onError: (error: Error) => {
      toast.error(`Error deleting agent task: ${error.message}`)
    }
  })

  const agentTasks = data?.getAgentTasks || MOCK_AGENT_TASKS

  return (
    <Card className="p-6 w-full">
      <h3 className="text-xl font-semibold">Agent Tasks</h3>
      <p className="text-sm text-muted-foreground">
        Manage the agent tasks that are running on the server.
      </p>

      {loading && <div className="py-4 text-center">Loading agent tasks...</div>}
      {/* {error && <div className="p-4 text-center text-red-500">Error: {error.message}</div>} */}

      <div className="flex flex-col gap-4">
        {agentTasks.map((task) => (
          <AgentTaskRow
            key={task.id}
            task={task}
            onDelete={(id) => deleteAgentTask({ variables: { id } })}
          />
        ))}
      </div>
    </Card>
  )
}

type Props = {
  task: AgentTask
  onDelete?: (id: string) => void
}

export function AgentTaskRow({ task, onDelete }: Props) {
  const humanSchedule = formatRRuleToText(task.schedule)

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-gray-300 p-4 w-full">
      <div className="flex justify-between items-start gap-4">
        <div className="flex flex-col gap-2">
          <div className="flex items-center gap-4">
            <p className="text-lg font-medium">{task.name}</p>
            <Badge variant="outline">Runs {humanSchedule}</Badge>
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
              <div className="flex items-center gap-2 bg-gray/50 p-2 rounded-md">
                <p>Delete agent task</p>
              </div>
            </TooltipContent>
          </Tooltip>
        )}
      </div>

      <div className="flex gap-4 text-xs text-muted-foreground">
        <span>Created {formatDistanceToNow(new Date(task.createdAt))} ago</span>
        <span>Updated {formatDistanceToNow(new Date(task.updatedAt))} ago</span>
      </div>
    </div>
  )
}
