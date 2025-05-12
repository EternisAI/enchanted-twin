'use client'

import { Card } from '@renderer/components/ui/card'
import NotificationStatusCard from './NotificationStatus'
import MediaStatus from './MediaStatus'
import AccessibilityStatus from './AccessibilityStatus'

export default function PermissionsCard() {
  return (
    <Card className="flex flex-col gap-4 p-6">
      <h3 className="text-xl font-semibold mb-2">Permissions</h3>
      <div className="flex flex-col gap-6">
        <MediaStatus />
        <AccessibilityStatus />
        <NotificationStatusCard />
      </div>
    </Card>
  )
}
