import { Eye, EyeClosed } from 'lucide-react'
import { Button } from '../ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'

interface AnonToggleButtonProps {
  isAnonymized: boolean
  setIsAnonymized: (isAnonymized: boolean) => void
}

export function AnonToggleButton({ isAnonymized, setIsAnonymized }: AnonToggleButtonProps) {
  return (
    <div className="absolute top-4 right-4 flex justify-end mb-2 z-40">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            onClick={() => setIsAnonymized(!isAnonymized)}
            className="p-2 rounded-md no-drag backdrop-blur-sm"
            variant="outline"
          >
            {isAnonymized ? <EyeClosed className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          <p>{isAnonymized ? 'Show original messages' : 'Anonymize messages'}</p>
        </TooltipContent>
      </Tooltip>
    </div>
  )
}
