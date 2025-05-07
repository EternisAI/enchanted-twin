// ChatHome.tsx
import { useNavigate, useRouter } from '@tanstack/react-router'
import MessageInput from './MessageInput'
import { useMutation, useQuery, gql } from '@apollo/client'
import {
  CreateChatDocument,
  GetProfileDocument,
  SendMessageDocument
} from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { ContextCard } from './ContextCard'
import { useState } from 'react'
import { Input } from '@renderer/components/ui/input'
import { toast } from 'sonner'

const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

export default function ChatHome() {
  const navigate = useNavigate()
  const router = useRouter()
  const [createChat] = useMutation(CreateChatDocument)
  const [sendMessage] = useMutation(SendMessageDocument)
  const [updateProfile] = useMutation(UPDATE_PROFILE)
  const { data: profile, refetch: refetchProfile } = useQuery(GetProfileDocument)
  const [isEditingName, setIsEditingName] = useState(false)
  const [editedName, setEditedName] = useState('')

  const handleStartChat = async (text: string) => {
    try {
      const { data: createData } = await createChat({
        variables: { name: text }
      })
      const newChatId = createData?.createChat?.id

      if (newChatId) {
        navigate({
          to: `/chat/${newChatId}`,
          search: { initialMessage: text }
        })

        // Refetch all chats
        await client.cache.evict({ fieldName: 'getChats' })
        await router.invalidate({
          filter: (match) => match.routeId === '/chat/$chatId'
        })

        await sendMessage({ variables: { chatId: newChatId, text } })
      }
    } catch (error) {
      console.error('Failed to start chat:', error)
    }
  }

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
    <div className="flex flex-col items-center h-full w-full">
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 w-full max-w-4xl justify-between">
        <div className="flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent gap-12">
          <div className="py-8 w-full flex flex-col items-center gap-4">
            {isEditingName ? (
              <div className="flex items-center gap-2">
                <Input
                  value={editedName}
                  onChange={(e) => setEditedName(e.target.value)}
                  onKeyDown={handleKeyDown}
                  onBlur={handleNameUpdate}
                  autoFocus
                  className="text-3xl font-bold text-center"
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
            <div className="w-full max-w-lg mx-auto">
              <ContextCard />
            </div>
          </div>
        </div>
        <div className="px-6 h-[130px] w-full flex flex-col justify-end">
          <MessageInput isWaitingTwinResponse={false} onSend={handleStartChat} />
        </div>
      </div>
    </div>
  )
}
