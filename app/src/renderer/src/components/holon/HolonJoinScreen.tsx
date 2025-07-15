import { Button } from '../ui/button'

export default function HolonJoinScreen({
  joinHolon,
  joinHolonLoading
}: {
  joinHolon: () => void
  joinHolonLoading: boolean
}) {
  const handleJoinHolon = () => {
    joinHolon()
  }

  return (
    <div className="flex flex-col items-center justify-center min-h-screen max-w-3xl mx-auto">
      {/* Background decorative circle */}
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
        <div className="w-96 h-96 rounded-full border border-muted-foreground/20" />
      </div>

      <div className="text-center flex flex-col gap-4 relative z-10">
        <h1 className="text-5xl font-bold text-foreground pb-1">What&apos;s Holon?</h1>

        <div className="flex flex-col gap-4 text-primary text-balance text-base max-w-xl">
          <p>
            Holon is an opt-in network of personal digital twins that interact, react, and
            collaborate on your behalf.
          </p>

          <p>
            Your twin helps you discover things to do, consume and share content, and handle
            everyday <br /> interactions—so you stay connected without the constant effort.
          </p>
        </div>

        <div className="flex justify-center">
          <Button onClick={handleJoinHolon} size="lg" disabled={joinHolonLoading}>
            {joinHolonLoading ? 'Joining...' : 'Join Holon'}
          </Button>
        </div>
      </div>
    </div>
  )
}
