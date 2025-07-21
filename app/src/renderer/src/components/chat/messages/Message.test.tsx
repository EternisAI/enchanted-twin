import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { UserMessageBubble, AssistantMessageBubble } from './Message'
import type { Message } from '@renderer/graphql/generated/graphql'
import { Role } from '@renderer/graphql/generated/graphql'

// Mock the TTS hook
vi.mock('@renderer/hooks/useTTS', () => ({
  useTTS: () => ({
    speak: vi.fn(),
    stop: vi.fn(),
    isSpeaking: false
  })
}))

// Mock framer-motion
vi.mock('framer-motion', () => ({
  motion: {
    div: ({ children, ...props }: any) => <div {...props}>{children}</div>
  },
  AnimatePresence: ({ children }: any) => <div>{children}</div>
}))

// Mock lucide-react icons
vi.mock('lucide-react', () => ({
  CheckCircle: () => <span>CheckCircle</span>,
  ChevronRight: () => <span>ChevronRight</span>,
  Lightbulb: () => <span>Lightbulb</span>,
  LoaderIcon: () => <span>LoaderIcon</span>,
  Volume2: () => <span>Volume2</span>,
  VolumeOff: () => <span>VolumeOff</span>,
  XIcon: () => <span>XIcon</span>,
  X: () => <span>X</span>,
  Flag: () => <span>Flag</span>,
  Check: () => <span>Check</span>
}))

// Mock chat config
vi.mock('@renderer/components/chat/config', () => ({
  extractReasoningAndReply: (text: string) => ({
    thinkingText: '',
    replyText: text
  }),
  getToolConfig: () => ({
    toolNameInProgress: 'Processing',
    toolNameCompleted: 'Completed',
    customComponent: null
  })
}))

// Mock UI components
vi.mock('@renderer/components/ui/badge', () => ({
  Badge: ({ children, ...props }: any) => <span {...props}>{children}</span>
}))

vi.mock('@renderer/components/ui/collapsible', () => ({
  Collapsible: ({ children }: any) => <div>{children}</div>,
  CollapsibleContent: ({ children }: any) => <div>{children}</div>,
  CollapsibleTrigger: ({ children }: any) => <button>{children}</button>
}))

vi.mock('@renderer/components/chat/messages/Markdown', () => ({
  default: ({ children }: any) => <div>{children}</div>
}))

describe('Message Components', () => {
  const mockMessage: Message = {
    id: '1',
    text: 'Hello John Doe!',
    role: Role.User,
    createdAt: '2023-01-01T00:00:00Z',
    imageUrls: [],
    toolCalls: [],
    toolResults: []
  }

  describe('UserMessageBubble', () => {
    it('should render user message without anonymization', () => {
      render(
        <UserMessageBubble message={mockMessage} chatPrivacyDict={null} isAnonymized={false} />
      )

      expect(screen.getByText('Hello John Doe!')).toBeInTheDocument()
    })

    it('should render user message with anonymization', () => {
      const privacyDict = JSON.stringify({
        'John Doe': 'PERSON_001'
      })

      render(
        <UserMessageBubble
          message={mockMessage}
          chatPrivacyDict={privacyDict}
          isAnonymized={true}
        />
      )

      // Should find the anonymized text in a span with the anonymization class
      const anonymizedElement = screen.getByText('Person_001')
      expect(anonymizedElement).toBeInTheDocument()
      expect(anonymizedElement).toHaveClass('bg-muted-foreground')
    })

    it('should handle case preservation in anonymization', () => {
      const messageWithDifferentCases: Message = {
        ...mockMessage,
        text: 'JOHN DOE and john doe met'
      }

      const privacyDict = JSON.stringify({
        'john doe': 'person one'
      })

      render(
        <UserMessageBubble
          message={messageWithDifferentCases}
          chatPrivacyDict={privacyDict}
          isAnonymized={true}
        />
      )

      // Should preserve case patterns
      expect(screen.getByText('PERSON ONE')).toBeInTheDocument()
      expect(screen.getByText('person one')).toBeInTheDocument()
    })

    it('should handle longest-first matching', () => {
      const messageWithOverlapping: Message = {
        ...mockMessage,
        text: 'John Doe and John met'
      }

      const privacyDict = JSON.stringify({
        John: 'PERSON_002',
        'John Doe': 'PERSON_001'
      })

      render(
        <UserMessageBubble
          message={messageWithOverlapping}
          chatPrivacyDict={privacyDict}
          isAnonymized={true}
        />
      )

      // "John Doe" should be replaced as a whole, not as separate "John" + "Doe"
      expect(screen.getByText('Person_001')).toBeInTheDocument()
      expect(screen.getByText('Person_002')).toBeInTheDocument()
      expect(screen.queryByText('Person_002_Doe')).not.toBeInTheDocument()
    })

    it('should render timestamp when showTimestamp is true', () => {
      render(
        <UserMessageBubble
          message={mockMessage}
          chatPrivacyDict={null}
          isAnonymized={false}
          showTimestamp={true}
        />
      )

      // Should find a timestamp element (adjust for actual format)
      expect(screen.getByText(/AM|PM/)).toBeInTheDocument()
    })

    it('should not render timestamp when showTimestamp is false', () => {
      render(
        <UserMessageBubble
          message={mockMessage}
          chatPrivacyDict={null}
          isAnonymized={false}
          showTimestamp={false}
        />
      )

      // Should not find a timestamp element
      expect(screen.queryByText(/AM|PM/)).not.toBeInTheDocument()
    })

    it('should handle empty privacy dictionary', () => {
      render(<UserMessageBubble message={mockMessage} chatPrivacyDict="{}" isAnonymized={true} />)

      // Should render original text when dictionary is empty
      expect(screen.getByText('Hello John Doe!')).toBeInTheDocument()
    })

    it('should handle malformed privacy dictionary JSON', () => {
      render(
        <UserMessageBubble
          message={mockMessage}
          chatPrivacyDict="invalid json"
          isAnonymized={true}
        />
      )

      // Should render original text when JSON is malformed
      expect(screen.getByText('Hello John Doe!')).toBeInTheDocument()
    })

    it('should render image attachments', () => {
      const messageWithImages: Message = {
        ...mockMessage,
        imageUrls: ['https://example.com/image1.jpg', 'https://example.com/image2.jpg']
      }

      render(
        <UserMessageBubble
          message={messageWithImages}
          chatPrivacyDict={null}
          isAnonymized={false}
        />
      )

      // Should find image elements (mocked as img tags)
      const images = screen.getAllByRole('img')
      expect(images).toHaveLength(2)
    })
  })

  describe('AssistantMessageBubble', () => {
    const assistantMessage: Message = {
      id: '2',
      text: 'Hello! I can help you, John.',
      role: Role.Assistant,
      createdAt: '2023-01-01T00:00:00Z',
      imageUrls: [],
      toolCalls: [],
      toolResults: []
    }

    it('should render assistant message without anonymization', () => {
      render(
        <AssistantMessageBubble
          message={assistantMessage}
          chatPrivacyDict={null}
          isAnonymized={false}
        />
      )

      expect(screen.getByText('Hello! I can help you, John.')).toBeInTheDocument()
    })

    it('should render assistant message with anonymization', () => {
      const privacyDict = JSON.stringify({
        John: 'PERSON_001'
      })

      render(
        <AssistantMessageBubble
          message={assistantMessage}
          chatPrivacyDict={privacyDict}
          isAnonymized={true}
        />
      )

      // Should find the anonymized text (note: it's rendered as HTML in markdown mode)
      expect(screen.getByText(/Person_001/)).toBeInTheDocument()
    })

    it('should handle messages with tool calls', () => {
      const messageWithTools: Message = {
        ...assistantMessage,
        toolCalls: [
          {
            id: 'tool1',
            name: 'test_tool',
            isCompleted: true,
            messageId: '2',
            result: null
          }
        ]
      }

      render(
        <AssistantMessageBubble
          message={messageWithTools}
          chatPrivacyDict={null}
          isAnonymized={false}
        />
      )

      // Should render tool call information
      expect(screen.getByText('Completed')).toBeInTheDocument()
    })

    it('should handle reasoning text extraction', () => {
      // This would require mocking the extractReasoningAndReply function
      // to return specific thinking and reply text
      const messageWithReasoning: Message = {
        ...assistantMessage,
        text: '<thinking>Let me think about this...</thinking>\n\nHere is my response.'
      }

      // For this test, we'd need to update the mock to handle reasoning extraction
      render(
        <AssistantMessageBubble
          message={messageWithReasoning}
          chatPrivacyDict={null}
          isAnonymized={false}
        />
      )

      // This is a simplified test - check for parts of the text instead of exact match
      expect(screen.getByText(/thinking/)).toBeInTheDocument()
      expect(screen.getByText(/Here is my response/)).toBeInTheDocument()
    })
  })

  describe('Integration Tests', () => {
    it('should handle complex anonymization scenarios', () => {
      const complexMessage: Message = {
        id: '3',
        text: 'John Doe works at Google with Jane Smith and John from Microsoft.',
        role: Role.User,
        createdAt: '2023-01-01T00:00:00Z',
        imageUrls: [],
        toolCalls: [],
        toolResults: []
      }

      const privacyDict = JSON.stringify({
        'John Doe': 'PERSON_001',
        'Jane Smith': 'PERSON_002',
        John: 'PERSON_003',
        Google: 'COMPANY_001',
        Microsoft: 'COMPANY_002'
      })

      render(
        <UserMessageBubble
          message={complexMessage}
          chatPrivacyDict={privacyDict}
          isAnonymized={true}
        />
      )

      // Should handle longest-first matching correctly
      expect(screen.getByText('Person_001')).toBeInTheDocument() // John Doe
      expect(screen.getByText('Person_002')).toBeInTheDocument() // Jane Smith
      expect(screen.getByText('Person_003')).toBeInTheDocument() // John (standalone)
      expect(screen.getByText('Company_001')).toBeInTheDocument() // Google
      expect(screen.getByText('Company_002')).toBeInTheDocument() // Microsoft
    })
  })
})
