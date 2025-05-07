'use client'

import { Card } from '@renderer/components/ui/card'
import NotificationStatusCard from './NotificationStatus'
import MediaStatus from './MediaStatus'

export default function PermissionsCard() {
  return (
    <Card className="p-6">
      <h3 className="text-xl font-semibold mb-4">Permissions</h3>
      <div className="flex flex-wrap gap-4 w-full">
        <MediaStatus />
        <NotificationStatusCard />
      </div>
    </Card>
  )
}
