import { gql, useQuery, useMutation } from '@apollo/client'
import { CheckIcon, PencilIcon, UserIcon, XIcon } from 'lucide-react'
import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Button } from '../ui/button'
import { Textarea } from '../ui/textarea'
import { toast } from 'sonner'
import { cn } from '@renderer/lib/utils'

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
      e.preventDefault()
      handleCancel()
    }
  }

  const handleStartEditing = () => {
    setContext('')
    setIsEditing(true)
  }

  return (
    <motion.div
      className={cn(
        'relative bg-transparent w-full rounded-lg p-2 hover:bg-muted focus-within:bg-muted ',
        isEditing && 'w-full !bg-card'
      )}
      transition={{ duration: 0.3, ease: [0.4, 0, 0.2, 1] }}
      layout
    >
      <AnimatePresence mode="wait">
        {userData?.profile?.bio || isEditing ? (
          <motion.div
            key="context"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
            className="space-y-2 w-full"
          >
            <AnimatePresence mode="wait">
              {isEditing ? (
                <motion.div
                  key="editing"
                  layoutId="context-container"
                  className="relative w-full"
                  initial={{ opacity: 0, scale: 0.98 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.98 }}
                  transition={{
                    duration: 0.2,
                    ease: [0.4, 0, 0.2, 1],
                    layout: { duration: 0.3, ease: [0.4, 0, 0.2, 1] }
                  }}
                >
                  <motion.div
                    initial={{ opacity: 0, y: 5 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: 5 }}
                    transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
                    layout="position"
                    className="relative w-full pb-8"
                  >
                    <motion.textarea
                      layout="position"
                      value={context}
                      onChange={(e) => setContext(e.target.value)}
                      onKeyDown={handleKeyDown}
                      onBlur={() => !context.trim() && handleSubmit({} as React.FormEvent)}
                      readOnly={!isEditing}
                      placeholder="Share something about yourself..."
                      onClick={() => !isEditing && setIsEditing(true)}
                      className={cn(
                        'w-full text-sm !bg-transparent hover:bg-transparent outline-none border-none resize-none transition-all duration-200 p-2 focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0',
                        !isEditing
                          ? 'min-h-0 min-w-[200px] max-h-[150px] border-transparent'
                          : 'max-h-[150px]'
                      )}
                      style={{}}
                      autoFocus={isEditing}
                    />
                  </motion.div>
                  <motion.div
                    className="flex justify-end gap-2 mt-2 absolute bottom-1 right-1"
                    initial={{ opacity: 0, y: -5 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -5 }}
                    transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
                  >
                    <Button variant="ghost" size="icon" onClick={handleCancel}>
                      <XIcon className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={handleSubmit}
                      disabled={updateLoading}
                    >
                      <CheckIcon className="size-4" />
                    </Button>
                  </motion.div>
                </motion.div>
              ) : (
                <motion.div
                  key="display"
                  layoutId="context-container"
                  className="relative"
                  initial={{ opacity: 0, scale: 0.98 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.98 }}
                  transition={{
                    duration: 0.2,
                    ease: [0.4, 0, 0.2, 1],
                    layout: { duration: 0.3, ease: [0.4, 0, 0.2, 1] }
                  }}
                >
                  <motion.p
                    className={`text-sm text-muted-foreground cursor-pointer p-2 rounded-lg transition-colors ${
                      context.split('\n').length === 1 ? 'text-center' : 'text-left'
                    }`}
                    onClick={() => setIsEditing(true)}
                    initial={{ opacity: 0, y: 5 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: 5 }}
                    transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
                    style={{
                      display: '-webkit-box',
                      WebkitLineClamp: 5,
                      WebkitBoxOrient: 'vertical',
                      overflow: 'hidden',
                      whiteSpace: 'pre-wrap'
                    }}
                    layout="position"
                  >
                    {context}
                  </motion.p>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        ) : (
          <motion.div
            key="empty"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
            className="flex items-center justify-center w-full"
            layout
          >
            <Button
              variant="ghost"
              size="sm"
              onClick={handleStartEditing}
              className="flex items-center gap-2 text-muted-foreground hover:text-foreground"
            >
              <UserIcon className="size-4" />
              Personalize
            </Button>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}
