import { gql, useQuery, useMutation } from '@apollo/client'
import { PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'
import { Card, CardHeader, CardTitle, CardContent } from '../ui/card'
import {
  Sheet,
  SheetTrigger,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
  SheetFooter
} from '../ui/sheet'
import { Textarea } from '../ui/textarea'
import { toast } from 'sonner'

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

const GET_PROFILE = gql`
  query GetProfile {
    profile {
      bio
    }
  }
`

export function ContextCard() {
  const { data: userData, loading, refetch } = useQuery(GET_PROFILE)
  const [updateProfile, { loading: updateLoading }] = useMutation(UPDATE_PROFILE)
  const [context, setContext] = useState(userData?.profile?.bio || '')
  const [open, setOpen] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const { data } = await updateProfile({ variables: { input: { bio: context } } })
    if (data?.updateProfile) {
      await refetch()
      setOpen(false)
      toast.success('Context updated successfully')
    } else {
      toast.error('Failed to update context')
    }
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Card className="w-full">
          <CardHeader>
            <CardTitle className="flex items-center justify-between gap-2">
              <span className="text-muted-foreground">Context</span>
              <PlusIcon className="size-4" />
            </CardTitle>
          </CardHeader>
          {!loading && <CardContent>{userData?.profile?.bio || ''}</CardContent>}
        </Card>
      </SheetTrigger>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Add Context</SheetTitle>
          <SheetDescription>
            Share information about yourself, your preferences, or any other context that might help
            your twin understand you better.
          </SheetDescription>
        </SheetHeader>
        <form className="space-y-4 px-4" onSubmit={handleSubmit}>
          <Textarea
            placeholder="Enter any information that might help your twin understand you better..."
            value={context}
            onChange={(e) => setContext(e.target.value)}
            className="min-h-[200px]"
          />
          <SheetFooter>
            <Button disabled={updateLoading}>Save Context</Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}
