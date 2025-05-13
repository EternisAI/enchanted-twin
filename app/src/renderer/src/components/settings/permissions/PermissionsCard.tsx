'use client'

import { Card } from '@renderer/components/ui/card'
import NotificationStatusCard from './NotificationStatus'
import MediaStatus from './MediaStatus'
import AccessibilityStatus from './AccessibilityStatus'
import ScreenpipePanel from './ScreenpipeCard'

export default function PermissionsCard() {
  return (
    <Card className="p-6">
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 w-full">
        <MediaStatus />
        <AccessibilityStatus />
        <NotificationStatusCard />
        <ScreenpipePanel />
      </div>
    </Card>
  )
}
