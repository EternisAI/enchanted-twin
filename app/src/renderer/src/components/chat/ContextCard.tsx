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
    <motion.div
      className="relative"
      transition={{ duration: 0.15, ease: 'easeOut' }}
      animate={{ height: isEditing ? 'auto' : 'fit-content' }}
      layout="position"
    >
      <AnimatePresence mode="wait">
        {userData?.profile?.bio || isEditing ? (
          <motion.div
            key="context"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="space-y-2"
            layout="position"
          >
            {isEditing ? (
              <motion.div
                layoutId="context-textarea"
                className="relative"
                initial={{ opacity: 0, scale: 0.98 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.98 }}
                transition={{
                  duration: 0.2,
                  ease: [0.4, 0, 0.2, 1],
                  opacity: { duration: 0.15 },
                  layout: { duration: 0.2, ease: [0.4, 0, 0.2, 1] }
                }}
              >
                <motion.div
                  initial={{ opacity: 0, y: 5 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: 5 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                  layout="position"
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
                <motion.div
                  className="flex justify-end gap-2 mt-2"
                  initial={{ opacity: 0, y: -5 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -5 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
                  layout="position"
                >
                  <Button variant="ghost" size="sm" onClick={handleCancel}>
                    Cancel
                  </Button>
                  <Button size="sm" onClick={handleSubmit} disabled={updateLoading}>
                    Save
                  </Button>
                </motion.div>
              </motion.div>
            ) : (
              <motion.div
                layoutId="context-textarea"
                className="relative"
                initial={{ opacity: 0, scale: 0.98 }}
                animate={{ opacity: 1, scale: 1 }}
                exit={{ opacity: 0, scale: 0.98 }}
                transition={{
                  duration: 0.2,
                  ease: [0.4, 0, 0.2, 1],
                  opacity: { duration: 0.15 },
                  layout: { duration: 0.2, ease: [0.4, 0, 0.2, 1] }
                }}
              >
                <motion.p
                  className={`text-sm text-muted-foreground cursor-pointer hover:bg-muted/50 p-2 rounded-lg transition-colors ${
                    context.split('\n').length === 1 ? 'text-center' : 'text-left'
                  }`}
                  onClick={() => setIsEditing(true)}
                  initial={{ opacity: 0, y: 5 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: 5 }}
                  transition={{ duration: 0.15, ease: 'easeOut' }}
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
          </motion.div>
        ) : (
          <motion.div
            key="empty"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            className="flex items-center justify-center"
            layout="position"
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
