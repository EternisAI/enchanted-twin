'use client'

import NotificationStatusCard from './NotificationStatus'
import MediaStatus from './MediaStatus'
import AccessibilityStatus from './AccessibilityStatus'
import ScreenpipePanel from './ScreenpipeCard'
import TelemetryToggle from '../telemetry/TelemetryToggle'

export default function PermissionsCard() {
  return (
    <div className="flex flex-col gap-5 ">
      <AccessibilityStatus />
      <MediaStatus />
      <NotificationStatusCard />
      <TelemetryToggle />
      <ScreenpipePanel />
    </div>
  )
}
