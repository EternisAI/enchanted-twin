import { useState } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { OnboardingLayout } from './OnboardingLayout'
import { Input } from '@renderer/components/ui/input'

export function WelcomeStep({ onContinue }: { onContinue: () => void }) {
  const { userName, setUserName } = useOnboardingStore()
  const [name, setName] = useState(userName)

  const handleNameChange = (value: string) => {
    setName(value)
    setUserName(value.trim())
  }

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    if (name.trim()) {
      e.preventDefault()
      onContinue()
    }
  }

  return (
    <OnboardingLayout
      title="Enchanted"
      subtitle="Let's get started by personalizing your experience"
      className="max-w-sm w-full self-center"
    >
      <div className="space-y-4">
        <form className="space-y-2" onSubmit={handleSubmit}>
          <label
            htmlFor="name"
            className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
          >
            What&apos;s your name?
          </label>
          <Input
            type="text"
            id="name"
            placeholder="Enter your name"
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
          />
        </form>
      </div>
    </OnboardingLayout>
  )
}
