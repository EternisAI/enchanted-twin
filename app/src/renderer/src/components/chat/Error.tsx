import { useLlamaCpp } from '@renderer/hooks/useLlamaCpp'
import { JSX } from 'react'
import { Button } from '../ui/button'

const usePrettifyError = () => {
  const { start: startLlamaCpp } = useLlamaCpp()

  const prettifyError = (error: string): JSX.Element => {
    if (error.includes('Anonymiser is not running')) {
      return (
        <div className="flex items-center gap-2">
          Anonymiser is not running. Please start it in the admin panel.
          <Button size="sm" variant="outline" onClick={startLlamaCpp}>
            Start Anonymiser
          </Button>
        </div>
      )
    }
    return <>{error}</>
  }

  return {
    prettifyError
  }
}

export default function Error({ error }: { error: string }) {
  const { prettifyError } = usePrettifyError()
  return (
    <div className="py-2 px-4 mt-2 rounded-md border border-red-500 bg-red-500/10 text-red-500">
      Error: {prettifyError(error)}
    </div>
  )
}
