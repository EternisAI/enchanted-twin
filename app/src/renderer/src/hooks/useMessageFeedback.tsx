import { useMutation } from '@apollo/client'
import { useState } from 'react'
import { toast } from 'sonner'

import { FeedbackType, Message, UpdateFeedbackDocument } from '@renderer/graphql/generated/graphql'

export default function useMessageFeedback(message: Message) {
  const [feedback, setFeedback] = useState<FeedbackType | null>(message.feedback || null)

  const [updateFeedback, { loading, error }] = useMutation(UpdateFeedbackDocument)

  const handleUpdateFeedback = async (feedback: FeedbackType) => {
    try {
      const result = await updateFeedback({ variables: { messageId: message.id, feedback } })
      if (result.data?.updateFeedback) {
        setFeedback(feedback)
        toast.success('Feedback updated')
      }
    } catch (error: unknown) {
      console.error(error)
      toast.error('Failed to update feedback')
    }
  }

  return { handleUpdateFeedback, loading, error, feedback }
}
