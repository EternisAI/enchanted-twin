import { createFileRoute } from '@tanstack/react-router'
import AdminPanel from '@renderer/components/admin/AdminPanel'

export const Route = createFileRoute('/admin')({
  component: AdminPage
})

function AdminPage() {
  return (
    <div className="p-6 flex flex-col gap-6 max-w-2xl mx-auto">
      <h2 className="text-4xl mb-6">Admin</h2>
      <AdminPanel />
    </div>
  )
}
