import { renderHook } from '@testing-library/react-hooks'
import { MockedProvider } from '@apollo/client/testing'
import { useTelegramMessageSubscription } from '@renderer/hooks/useTelegramMessageSubscription'
import { TelegramMessageAddedDocument } from '@renderer/graphql/generated/graphql'

describe('useTelegramMessageSubscription', () => {
  it('should subscribe to telegram messages', () => {
    const onNewMessage = jest.fn()
    const mocks = [
      {
        request: {
          query: TelegramMessageAddedDocument,
          variables: { chatUUID: 'test-chat-uuid' }
        },
        result: {
          data: {
            telegramMessageAdded: {
              id: '1',
              text: 'Test message',
              role: 'USER',
              createdAt: new Date().toISOString(),
              imageUrls: [],
              toolCalls: [],
              toolResults: []
            }
          }
        }
      }
    ]

    renderHook(
      () => useTelegramMessageSubscription('test-chat-uuid', onNewMessage),
      {
        wrapper: ({ children }) => (
          <MockedProvider mocks={mocks} addTypename={false}>
            {children}
          </MockedProvider>
        )
      }
    )

  })
})
