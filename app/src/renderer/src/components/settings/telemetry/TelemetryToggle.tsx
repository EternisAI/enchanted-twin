import { useEffect, useState } from 'react'
import { DetailCard } from '../permissions/DetailCard'
import { Telescope } from 'lucide-react'

const getStatusConfig = (status: boolean) => {
  return {
    label: status ? 'Enabled' : 'Disabled',
    color: status ? 'text-green-500' : 'text-red-500',
    icon: status ? Telescope : Telescope
  }
}

export default function TelemetryToggle() {
  const [enabled, setEnabled] = useState(true)

  useEffect(() => {
    window.api.analytics.getEnabled().then(setEnabled)
  }, [])

  const handleToggle = async () => {
    const newState = !enabled
    await window.api.analytics.setEnabled(newState)
    setEnabled(newState)
  }

  const { color, icon, label } = getStatusConfig(enabled)

  return (
    <DetailCard
      title="Telemetry"
      IconComponent={Telescope}
      statusInfo={{ color, icon, label }}
      buttonLabel="Enable"
      onButtonClick={handleToggle}
      isLoading={false}
      grantedIcon="Disable"
      explanation="Share anonymised app activity to improve the Enchanted. No personal information or memories will be shared."
    />
  )
}
