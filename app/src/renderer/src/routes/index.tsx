import { ChatCard } from '@renderer/components/chat/ChatCard'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { createFileRoute, redirect, useRouterState } from '@tanstack/react-router'
import { client } from '@renderer/graphql/lib'
import { Chat, GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '@renderer/components/ui/button'
import { Plus } from 'lucide-react'
import { useOmnibarStore } from '@renderer/lib/stores/omnibar'
import { ContextCard } from '@renderer/components/chat/ContextCard'
import { useQuery, useMutation, gql } from '@apollo/client'
import { GetProfileDocument } from '@renderer/graphql/generated/graphql'
import { Input } from '@renderer/components/ui/input'
import { useState } from 'react'
import { toast } from 'sonner'

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

function IndexComponent() {
  const { data, error, success } = Route.useLoaderData()
  const chats: Chat[] = data?.getChats || []
  const { location } = useRouterState()
  const { openOmnibar } = useOmnibarStore()
  const { data: profile, refetch: refetchProfile } = useQuery(GetProfileDocument)
  const [updateProfile] = useMutation(UPDATE_PROFILE)
  const [isEditingName, setIsEditingName] = useState(false)
  const [editedName, setEditedName] = useState('')

  const handleNameUpdate = async () => {
    if (!editedName.trim()) {
      toast.error('Name cannot be empty')
      return
    }

    try {
      await updateProfile({
        variables: {
          input: {
            name: editedName.trim()
          }
        }
      })
      await refetchProfile()
      setIsEditingName(false)
      toast.success('Name updated successfully')
    } catch (error) {
      console.error('Failed to update name:', error)
      toast.error('Failed to update name')
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleNameUpdate()
    } else if (e.key === 'Escape') {
      setIsEditingName(false)
      setEditedName('')
    }
  }

  const twinName = profile?.profile?.name || 'Your Twin'

  return (
    <div className="flex flex-col h-full">
      <div className="flex flex-col items-center p-6 border-b border-border">
        <div className="w-full max-w-4xl">
          <div className="flex flex-col items-center gap-4">
            {isEditingName ? (
              <div className="flex items-center gap-2">
                <Input
                  value={editedName}
                  onChange={(e) => setEditedName(e.target.value)}
                  onKeyDown={handleKeyDown}
                  onBlur={handleNameUpdate}
                  autoFocus
                  className="!text-3xl font-bold text-center"
                />
              </div>
            ) : (
              <h1
                className="text-3xl font-bold text-center cursor-pointer hover:text-gray-600 transition-colors"
                onClick={() => {
                  setEditedName(twinName)
                  setIsEditingName(true)
                }}
              >
                {twinName}
              </h1>
            )}
            <div className="w-full max-w-lg">
              <ContextCard />
            </div>
          </div>
        </div>
      </div>

      <div className="flex flex-col p-6 flex-1 overflow-hidden gap-4 w-full">
        <div className="flex w-full items-center justify-between mb-6">
          <Button className="w-full" variant="outline" onClick={openOmnibar}>
            <Plus className="w-4 h-4" />
            New topic
          </Button>
        </div>

        {!success && (
          <div className="w-full flex justify-center items-center py-10">
            <div className="p-4 m-4 w-xl border border-red-300 bg-red-50 text-red-700 rounded-md">
              <h3 className="font-medium">Error loading chats</h3>
              <p className="text-sm">
                {error instanceof Error ? error.message : 'An unexpected error occurred'}
              </p>
            </div>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 overflow-y-auto">
          {chats.map((chat) => (
            <ChatCard
              key={chat.id}
              chat={chat}
              isActive={location.pathname === `/chat/${chat.id}`}
            />
          ))}
        </div>
      </div>
    </div>
  )
}

export const Route = createFileRoute('/')({
  loader: async () => {
    try {
      const { data, loading, error } = await client.query({
        query: GetChatsDocument,
        variables: { first: 20, offset: 0 }
      })
      return { data, loading, error, success: true }
    } catch (error) {
      console.error('Error loading chats:', error)
      return {
        data: null,
        loading: false,
        error: error instanceof Error ? error : new Error('An unexpected error occurred'),
        success: false
      }
    }
  },
  component: IndexComponent,
  beforeLoad: () => {
    const onboardingStore = useOnboardingStore.getState()
    if (!onboardingStore.isCompleted) {
      throw redirect({ to: '/onboarding' })
    }
  }
})
