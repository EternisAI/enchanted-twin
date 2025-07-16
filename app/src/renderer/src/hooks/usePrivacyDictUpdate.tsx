import { useSubscription } from '@apollo/client'
import { PrivacyDictUpdatedDocument } from '@renderer/graphql/generated/graphql'

export function usePrivacyDictUpdate(chatId: string, onUpdate: (privacyDict: string) => void) {
  useSubscription(PrivacyDictUpdatedDocument, {
    variables: { chatId },
    onData: ({ data }) => {
      onUpdate(data.data?.privacyDictUpdated.privacyDictJson)
    }
  })
}
