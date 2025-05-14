import { createFileRoute } from '@tanstack/react-router'
import AgentTasks from '@renderer/components/tasks/AgentTasks'

export const Route = createFileRoute('/tasks')({
  component: Tasks
})

export default function Tasks() {
  return (
    <div className="p-6 flex flex-col gap-6 w-full md:w-4xl mx-auto">
      <AgentTasks />
    </div>
  )
}
