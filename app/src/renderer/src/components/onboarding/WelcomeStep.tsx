import { useRef, useState } from 'react'
import { OnboardingLayout } from './OnboardingLayout'
import { Input } from '@renderer/components/ui/input'
import { useMutation } from '@apollo/client'
import { gql } from '@apollo/client'
import { toast } from 'sonner'
import { Button } from '@renderer/components/ui/button'
import { Loader2 } from 'lucide-react'

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

export function WelcomeStep({ name, onContinue }: { name: string; onContinue: () => void }) {
  const [updateProfile, { loading }] = useMutation(UPDATE_PROFILE)
  const [error, setError] = useState<string | null>(null)

  const nameInputRef = useRef<HTMLInputElement>(null)
  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const newName = nameInputRef.current?.value
    if (!newName?.trim()) {
      setError('Please enter your name')
      toast.error('Please enter your name')
      return
    }

    try {
      await updateProfile({
        variables: {
          input: {
            name: newName.trim()
          }
        }
      })
      toast.success('Profile updated successfully')
      onContinue()
    } catch (error) {
      console.error('Failed to update profile:', error)
      setError('Failed to update profile. Please try again.')
      toast.error('Failed to update profile. Please try again.')
    }
  }

  return (
    <OnboardingLayout
      title="Enchanted"
      subtitle="Let's get started by personalizing your experience"
      className="max-w-sm w-full self-center"
    >
      <div className="space-y-4">
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <label
              htmlFor="name"
              className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
            >
              What&apos;s your name?
            </label>
            <Input
              ref={nameInputRef}
              type="text"
              id="name"
              placeholder="Enter your name"
              defaultValue={name}
              className={error ? 'border-destructive' : ''}
              onChange={() => setError(null)}
            />
            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Updating...
              </>
            ) : (
              'Continue'
            )}
          </Button>
        </form>
      </div>
    </OnboardingLayout>
  )
}
