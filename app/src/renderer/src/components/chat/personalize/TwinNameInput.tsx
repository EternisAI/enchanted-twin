import { CheckIcon, XIcon } from 'lucide-react'
import { useQuery, useMutation, gql } from '@apollo/client'
import { cn } from '@renderer/lib/utils'
import { GetProfileDocument } from '@renderer/graphql/generated/graphql'
import { Button } from '@renderer/components/ui/button'
import { motion, AnimatePresence } from 'framer-motion'
import { useRef, useState, useLayoutEffect } from 'react'
import { toast } from 'sonner'
const UPDATE_PROFILE = gql`
  mutation UpdateProfile($input: UpdateProfileInput!) {
    updateProfile(input: $input)
  }
`

export function TwinNameInput() {
  const { data: profile, refetch: refetchProfile } = useQuery(GetProfileDocument)
  const [updateProfile] = useMutation(UPDATE_PROFILE)
  const nameEditRef = useRef<HTMLParagraphElement>(null)

  const twinName = profile?.profile?.name || 'Your Twin'

  const [isEditingName, setIsEditingName] = useState(false)

  // Focus the element when editing mode is activated
  useLayoutEffect(() => {
    if (isEditingName && nameEditRef.current) {
      nameEditRef.current.focus()
    }
  }, [isEditingName])

  const handleNameEditKeyDown = (e: React.KeyboardEvent<HTMLParagraphElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleNameUpdate()
    } else if (e.key === 'Escape') {
      if (nameEditRef.current) {
        nameEditRef.current.textContent = twinName
      }
      setIsEditingName(false)
    }
  }

  const handleNameUpdate = async () => {
    const currentName = nameEditRef.current?.textContent?.trim() || ''
    if (currentName === twinName) {
      // If the name is the same as the twin's name, don't update it
      setIsEditingName(false)
      return
    }
    if (!currentName) {
      toast.error('Name cannot be empty')
      return
    }

    try {
      await updateProfile({
        variables: {
          input: {
            name: currentName
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

  return (
    <motion.div className="relative w-fit" layout>
      <motion.div
        className={cn(
          'absolute inset-0 rounded-lg transition-all duration-300 hover:bg-muted',
          isEditingName ? 'bg-muted' : 'bg-transparent'
        )}
        layout
      />

      <motion.p
        ref={nameEditRef}
        contentEditable={isEditingName}
        suppressContentEditableWarning
        onKeyDown={handleNameEditKeyDown}
        onBlur={handleNameUpdate}
        onFocus={() => {
          if (!isEditingName) {
            setIsEditingName(true)
          }
        }}
        onSelect={() => {
          if (!isEditingName) {
            setIsEditingName(true)
          }
        }}
        tabIndex={0}
        className={cn(
          'relative cursor-text w-fit z-10 text-2xl font-bold transition-all duration-200 p-2 focus-visible:bg-muted focus-visible:outline-none focus-visible:ring-0 focus-visible:ring-offset-0 text-center',
          isEditingName
            ? '!bg-transparent hover:bg-transparent outline-none border-none !px-24'
            : 'rounded-lg hover:bg-transparent'
        )}
        style={{ minWidth: '200px' }}
      >
        {twinName}
      </motion.p>

      <AnimatePresence>
        {isEditingName && (
          <motion.div
            className="absolute right-2 top-1/2 -translate-y-1/2 flex gap-1 z-20"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.1, ease: [0.4, 0, 0.2, 1] }}
          >
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              aria-label="Cancel"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => {
                if (nameEditRef.current) {
                  nameEditRef.current.textContent = twinName
                }
                setIsEditingName(false)
              }}
            >
              <XIcon className="size-4" />
            </Button>
            <Button
              variant="default"
              size="icon"
              className="h-8 w-8"
              aria-label="Save changes"
              onMouseDown={(e) => e.preventDefault()}
              onClick={handleNameUpdate}
            >
              <CheckIcon className="size-4" />
            </Button>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}
