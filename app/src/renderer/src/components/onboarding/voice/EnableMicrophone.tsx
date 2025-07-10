import { Mic } from 'lucide-react'
import { Button } from '@renderer/components/ui/button'
import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'

interface EnableMicrophoneProps {
  onGrantPermission?: () => void
  onSkip?: () => void
}

export default function EnableMicrophone({ onGrantPermission, onSkip }: EnableMicrophoneProps) {
  const { microphoneStatus, requestMicrophoneAccess } = useMicrophonePermission()

  const handleGrantPermission = async () => {
    await requestMicrophoneAccess()
    if (microphoneStatus === 'granted') {
      onGrantPermission?.()
    }
  }

  const handleSkip = () => {
    onSkip?.()
  }

  return (
    <div className="w-full max-w-3xl flex flex-col gap-6 p-10 bg-white/5 rounded-xl border border-white/50">
      <div className="flex items-center gap-2">
        <Mic className="w-12 h-12 text-white" />
        <div className="flex flex-col gap-1.5">
          <h2 className="text-lg text-white font-normal">Enable Microphone</h2>
          <p className="text-sm text-white/75">
            To talk to Enchanted, you&apos;ll need to allow microphone access.
          </p>
        </div>
      </div>

      <div className="flex gap-12">
        <div className="flex flex-col gap-2">
          <p className="text-white font-bold text-sm">How to Grant Access</p>
          <ul className="text-white/80 text-sm">
            <li>
              <p>1. Click button or visit System Settings</p>
            </li>
            <li>
              <p>2. Navigate to Privacy & Security</p>
            </li>
            <li>
              <p>3. Enable Enchanted in Microphone section</p>
            </li>
          </ul>
        </div>
        <div className="flex flex-col gap-2">
          <p className="text-white font-bold text-sm">Why provice access:</p>
          <ul className="text-white/80 text-sm list-disc pl-4">
            <li>
              <p>Voice to text transcription</p>
            </li>
            <li>
              <p>Sending voice messages to AI</p>
            </li>
            <li>
              <p>In-app voice commands</p>
            </li>
          </ul>
        </div>
      </div>

      <div>
        <div className="w-full h-34 bg-gray-300 flex items-center justify-center rounded-xl">
          <span className="text-gray-600">Image Placeholder</span>
        </div>
      </div>

      <div className="flex justify-center gap-6 items-center">
        <Button
          onClick={handleGrantPermission}
          variant="default"
          className="bg-gray-100 text-black hover:bg-gray-200 !px-4"
        >
          Grant Microphone Permission
        </Button>
        <Button variant="outline" onClick={handleSkip} className="text-white">
          Stick to Text-Chat
        </Button>
      </div>
    </div>
  )
}
