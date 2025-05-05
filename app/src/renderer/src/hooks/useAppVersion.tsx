import { useEffect, useState } from 'react'

export default function useAppVersion() {
  const [version, setVersion] = useState<string>('')

  useEffect(() => {
    const getVersion = async () => {
      const appVersion = await window.api.getAppVersion()
      setVersion(appVersion)
    }
    getVersion()
  }, [])

  return { version }
}
