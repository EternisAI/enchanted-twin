import { Button } from '@renderer/components/ui/button'
import { useEffect, useMemo, useState } from 'react'
import { motion } from 'framer-motion'

import { CycleText } from '@renderer/components/ui/CycleText'
import { useQuery } from '@apollo/client'
import { GetSetupProgressDocument } from '@renderer/graphql/generated/graphql'

const CYCLE_TEXT_WORDS = [
  'Enchanted can talk',
  'Workflows on Autopilot',
  'Tasks - Auto Mode',
  'Your Digital Twin',
  'Learns and Grows'
]

export default function LaunchScreen() {
  const { data, loading, error } = useQuery(GetSetupProgressDocument)
  const [progress, setProgress] = useState(20)

  console.log('data', data)

  const handleComplete = async () => {
    await window.api.launch.complete()
  }

  const setupData = useMemo(() => data?.getSetupProgress || [], [data])

  useEffect(() => {
    console.log('loading', loading)
    console.error('error', error)
    if (loading || error) return

    const allRequiredComplete = setupData
      .filter((item) => item.required)
      .every((item) => item.status === 'complete')
    if (allRequiredComplete) {
      console.log('allRequiredComplete')
      handleComplete()
    }
  }, [setupData, loading, error])

  useEffect(() => {
    const remove = window.api.launch.onProgress((data) => {
      console.log('data from onProgress', data)
      setProgress(data.progress)
      // if (data.status) setStatus(data.status)
    })
    return remove
  }, [])

  return (
    <div className="flex h-screen w-screen items-center justify-center bg-background font-sans">
      <div className="flex flex-col gap-8 items-center w-full max-w-xl">
        <div className="w-full gap-4 max-w-lg h-2 rounded-full bg-primary/20 overflow-hidden mb-10">
          <div
            className="h-full rounded-full bg-primary transition-all duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>
        {progress < 100 ? (
          <div className="text-foreground text-md text-center">
            <CycleText words={CYCLE_TEXT_WORDS} />
          </div>
        ) : (
          <motion.div
            initial={{ opacity: 0, y: 15 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
          >
            <Button
              className="mt-6 rounded-md bg-primary px-8 py-2 text-primary-foreground disabled:opacity-50"
              onClick={handleComplete}
              size="lg"
            >
              Ready to Roll
            </Button>
          </motion.div>
        )}
      </div>
    </div>
  )
}
