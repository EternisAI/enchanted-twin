'use client'

import NotificationStatusCard from './NotificationStatus'
import MediaStatus from './MediaStatus'
import AccessibilityStatus from './AccessibilityStatus'
import ScreenpipePanel from './ScreenpipeCard'
import TelemetryToggle from '../telemetry/TelemetryToggle'
import { Card } from '@renderer/components/ui/card'

export default function PermissionsCard() {
  return (
    <Card className="flex flex-col gap-5 p-4">
      <AccessibilityStatus />
      <MediaStatus />
      <NotificationStatusCard />
      <TelemetryToggle />
      <ScreenpipePanel />
    </Card>
  )
}
