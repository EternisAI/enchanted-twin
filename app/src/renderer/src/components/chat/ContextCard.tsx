import { gql, useQuery, useMutation } from '@apollo/client'
import { PencilIcon } from 'lucide-react'
import { useState, useEffect } from 'react'
import { Button } from '../ui/button'
import {
  Sheet,
  SheetTrigger,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription
} from '../ui/sheet'
import { Textarea } from '../ui/textarea'
import { toast } from 'sonner'
import { ScrollArea } from '../ui/scroll-area'

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
  const [context, setContext] = useState('')
  const [open, setOpen] = useState(false)

  useEffect(() => {
    if (userData?.profile?.bio) {
      setContext(userData.profile.bio)
    }
  }, [userData?.profile?.bio])

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

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      handleSubmit(e as unknown as React.FormEvent)
    }
  }

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <div className="relative p-4 border border-border rounded-lg flex items-center justify-between">
        {!loading && (
          <div className="line-clamp-6 text-sm">{userData?.profile?.bio || 'Add context'}</div>
        )}
        <SheetTrigger asChild>
          <Button variant="ghost" size="icon" className="">
            <PencilIcon className="text-muted-foreground size-4" />
          </Button>
        </SheetTrigger>
      </div>
      <SheetContent className="flex flex-col h-full">
        <SheetHeader>
          <SheetTitle>Add Context</SheetTitle>
        </SheetHeader>
        <div className="flex-1 overflow-hidden">
          <ScrollArea className="h-full">
            <SheetDescription className="px-4">
              Share information about yourself, your preferences, or any other context that might
              help your twin understand you better.
            </SheetDescription>
            <form className="flex flex-col gap-4 p-4" onSubmit={handleSubmit}>
              <Textarea
                placeholder="Enter any information that might help your twin understand you better..."
                value={context}
                onChange={(e) => setContext(e.target.value)}
                onKeyDown={handleKeyDown}
                className="min-h-[200px]"
              />
            </form>
          </ScrollArea>
        </div>
        <div className="p-4 border-t">
          <Button disabled={updateLoading} className="w-full" onClick={handleSubmit}>
            Save Context
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
