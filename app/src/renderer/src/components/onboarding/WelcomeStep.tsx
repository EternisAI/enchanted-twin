import { useState } from 'react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { OnboardingLayout } from './OnboardingLayout'

export function WelcomeStep() {
  const { userName, setUserName } = useOnboardingStore()
  const [name, setName] = useState(userName)

  const handleNameChange = (value: string) => {
    setName(value)
    setUserName(value.trim())
  }

  return (
    <OnboardingLayout
      title="Welcome to Enchanted Twin"
      subtitle="Let's get started by personalizing your experience"
    >
      <div className="space-y-4">
        <div className="space-y-2">
          <label
            htmlFor="name"
            className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
          >
            What&apos;s your name?
          </label>
          <input
            type="text"
            id="name"
            className="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
            placeholder="Enter your name"
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
          />
        </div>
      </div>
    </OnboardingLayout>
  )
}
