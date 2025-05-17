import { useRef, useState } from 'react'
import { OnboardingLayout } from './OnboardingLayout'
import { Input } from '@renderer/components/ui/input'
import { useMutation, useQuery } from '@apollo/client'
import { gql } from '@apollo/client'
import { toast } from 'sonner'
import { Button } from '@renderer/components/ui/button'
import { Loader2 } from 'lucide-react'
import { cn } from '@renderer/lib/utils'

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

const GET_PROFILE = gql`
  query GetProfile {
    profile {
      name
    }
  }
`

export function WelcomeStep({ onContinue }: { onContinue: () => void }) {
  const [updateProfile, { loading }] = useMutation(UPDATE_PROFILE)
  const { data } = useQuery(GET_PROFILE)
  const [error, setError] = useState<string | null>(null)

  const nameInputRef = useRef<HTMLInputElement>(null)
  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const newName = nameInputRef.current?.value
    if (newName === data?.profile?.name) {
      onContinue()
      return
    }
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
      title="Welcome"
      subtitle="What should we call you?"
      className="max-w-sm w-full self-center text-center"
    >
      <form className="flex flex-col gap-6" onSubmit={handleSubmit}>
        <Input
          ref={nameInputRef}
          type="text"
          id="name"
          placeholder="Enter your name"
          defaultValue={data?.profile?.name}
          className={cn('py-4 !h-fit w-full !text-2xl text-center', error && 'border-destructive')}
          onChange={() => setError(null)}
        />
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button size="lg" type="submit" className="w-full" disabled={loading}>
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
      <p
        className="text-sm text-muted-foreground cursor-pointer"
        onClick={() => window.api.openLogsFolder()}
      >
        Debug Logs
      </p>
    </OnboardingLayout>
  )
}
