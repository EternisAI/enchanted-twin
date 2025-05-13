import useAppVersion from '@renderer/hooks/useAppVersion'
import { useState } from 'react'
import { Card } from './ui/card'

function Versions(): React.JSX.Element {
  const [versions] = useState(window.electron.process.versions)
  const { version } = useAppVersion()

  return (
    <Card className="p-6 w-full">
      <p className="text-base">Version {version}</p>
      <ul className="versions text-sm text-muted-foreground">
        <li className="electron-version">Electron v{versions.electron}</li>
        <li className="chrome-version">Chromium v{versions.chrome}</li>
        <li className="node-version">Node v{versions.node}</li>
      </ul>
    </Card>
  )
}

export default Versions
