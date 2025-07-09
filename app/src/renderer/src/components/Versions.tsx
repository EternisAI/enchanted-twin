import useAppVersion from '@renderer/hooks/useAppVersion'
import { useState } from 'react'

function Versions(): React.JSX.Element {
  const [versions] = useState(window.electron.process.versions)
  const { version } = useAppVersion()

  return (
    <div className=" w-full  border-none flex flex-col gap-2 items-center text-center">
      <h2 className="text-2xl font-semibold">Version {version}</h2>
      <ul className="versions text-sm text-muted-foreground">
        <li className="electron-version">Electron v{versions.electron}</li>
        <li className="chrome-version">Chromium v{versions.chrome}</li>
        <li className="node-version">Node v{versions.node}</li>
      </ul>
    </div>
  )
}

export default Versions
