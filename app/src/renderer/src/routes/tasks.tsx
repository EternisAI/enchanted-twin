import { createFileRoute } from '@tanstack/react-router'
import AgentTasks from '@renderer/components/tasks/AgentTasks'

export const Route = createFileRoute('/tasks')({
  component: Tasks
})

export default function Tasks() {
  return (
    <div className="flex flex-col gap-6 h-full w-full mt-10 mx-auto overflow-y-auto">
      <AgentTasks />
    </div>
  )
}
