import { createFileRoute } from '@tanstack/react-router'
import AgentTasks from '@renderer/components/tasks/AgentTasks'

export const Route = createFileRoute('/tasks')({
  component: Tasks
})

export default function Tasks() {
  return (
    <div className="p-6 flex flex-col gap-6 max-w-4xl mx-auto">
      <h2 className="text-4xl mb-6">Agent Tasks</h2>
      <AgentTasks />
    </div>
  )
}
