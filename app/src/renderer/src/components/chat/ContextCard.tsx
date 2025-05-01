import { gql, useQuery, useMutation } from '@apollo/client'
import { PencilIcon } from 'lucide-react'
import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Button } from '../ui/button'
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
  const { data: userData, refetch } = useQuery(GET_PROFILE)
  const [updateProfile, { loading: updateLoading }] = useMutation(UPDATE_PROFILE)
  const [context, setContext] = useState('')
  const [isEditing, setIsEditing] = useState(false)

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
      setIsEditing(false)
      toast.success('Context updated successfully')
    } else {
      toast.error('Failed to update context')
    }
  }

  const handleCancel = () => {
    setContext(userData?.profile?.bio || '')
    setIsEditing(false)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault()
      handleSubmit(e as unknown as React.FormEvent)
    }
    if (e.key === 'Escape') {
      handleCancel()
    }
  }

  const handleStartEditing = () => {
    setContext('')
    setIsEditing(true)
  }

  return (
    <motion.div className="relative" transition={{ duration: 0.15, ease: 'easeOut' }}>
      <AnimatePresence mode="wait">
        {userData?.profile?.bio || isEditing ? (
          <motion.div
            key="context"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="space-y-2"
          >
            <motion.div
              className="relative"
              animate={{ height: isEditing ? 'auto' : 'auto' }}
              transition={{ duration: 0.2, ease: 'easeOut' }}
            >
              <Textarea
                value={context}
                onChange={(e) => setContext(e.target.value)}
                onKeyDown={handleKeyDown}
                onBlur={() => !context.trim() && handleSubmit({} as React.FormEvent)}
                readOnly={!isEditing}
                placeholder="Add context..."
                onClick={() => !isEditing && setIsEditing(true)}
                className={`w-full resize-none transition-all duration-200 rounded-lg ${
                  !isEditing
                    ? 'min-h-0 max-h-[150px] cursor-pointer hover:bg-muted/50 border-transparent'
                    : 'max-h-[150px] h-fit'
                }`}
                style={{
                  height: !isEditing ? 'auto' : '150px'
                }}
                autoFocus={isEditing}
              />
            </motion.div>
            {isEditing && (
              <motion.div
                className="flex justify-end gap-2"
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                transition={{ duration: 0.15, ease: 'easeOut' }}
              >
                <Button variant="ghost" size="sm" onClick={handleCancel}>
                  Cancel
                </Button>
                <Button size="sm" onClick={handleSubmit} disabled={updateLoading}>
                  Save
                </Button>
              </motion.div>
            )}
          </motion.div>
        ) : (
          <motion.div
            key="empty"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="flex items-center justify-center"
          >
            <Button
              variant="ghost"
              size="sm"
              onClick={handleStartEditing}
              className="flex items-center gap-2 text-muted-foreground hover:text-foreground"
            >
              <PencilIcon className="size-4" />
              Add Context
            </Button>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}
