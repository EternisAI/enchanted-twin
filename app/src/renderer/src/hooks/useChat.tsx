import { useMutation } from '@apollo/client'
import { SendMessageDocument, Message, Role } from '@renderer/graphql/generated/graphql'

interface AnonymizationResponse {
  choices: Array<{
    message: {
      content: string
    }
  }>
}

export function useSendMessage(
  chatId: string,
  onMessageSent: (msg: Message) => void,
  onError: (error: Message) => void
) {
  const [sendMessageMutation] = useMutation(SendMessageDocument, {
    update(cache, { data }) {
      if (!data?.sendMessage) return
      cache.modify({
        fields: {
          getChat(existing = {}) {
            return {
              ...existing,
              messages: [...(existing.messages || []), data.sendMessage]
            }
          }
        }
      })
    },
    onError(error) {
      console.error('Error sending message', error)
      const errorMessage: Message = {
        id: `error-${Date.now()}`,
        text: error instanceof Error ? error.message : 'Error sending message',
        role: Role.Assistant,
        imageUrls: [],
        toolCalls: [],
        toolResults: [],
        createdAt: new Date().toISOString()
      }

      onError(errorMessage)
    }
  })

  const anonymizeText = async (text: string): Promise<string> => {
    try {
      const prompt = `You are an anonymizer.
Return ONLY <json>{"orig": "replacement", …}</json>.
Example
user: "John Doe is a software engineer at Google"
assistant: "<json>{"John Doe":"Dave Smith","Google":"TechCorp"}</json>"
anonymize this:
${text}
/no_think`

      const response = await fetch('http://localhost:8000/v1/chat/completions', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          messages: [
            {
              role: 'user',
              content: prompt
            }
          ],
          model: 'qwen3-0.6b-q4_k_m',
          temperature: 0.5,
          max_tokens: 1000
        })
      })

      if (!response.ok) {
        throw new Error(`Anonymization failed: ${response.status}`)
      }

      const data: AnonymizationResponse = await response.json()
      const anonymizedContent = data.choices[0]?.message?.content

      console.log('anonymizedContent', anonymizedContent)

      if (!anonymizedContent) {
        throw new Error('No anonymized content received')
      }

      // Extract JSON from the response - capture everything after <json> tag until closing > or </json>
      const jsonMatch = anonymizedContent.match(/<json>(.*?)(?:>|<\/json>)/s)
      if (!jsonMatch) {
        console.warn('No JSON found in anonymization response, using original text')
        return text
      }

      try {
        console.log('jsonMatch', jsonMatch)
        // Replace single quotes with double quotes for valid JSON
        const jsonString = jsonMatch[1].replace(/'/g, '"')
        const anonymizationMap = JSON.parse(jsonString)

        // Apply anonymization replacements
        let anonymizedText = text
        Object.entries(anonymizationMap).forEach(([original, replacement]) => {
          anonymizedText = anonymizedText.replace(new RegExp(original, 'gi'), replacement as string)
        })

        return anonymizedText
      } catch (parseError) {
        console.warn('Failed to parse anonymization JSON, using original text', parseError)
        return text
      }
    } catch (error) {
      console.error('Anonymization failed:', error)
      // Return original text if anonymization fails
      return text
    }
  }

  const sendMessage = async (text: string, reasoning: boolean, voice: boolean) => {
    const anonymizedText = await anonymizeText(text)

    console.log('anonymizedText', anonymizedText)
    const optimisticMessage: Message = {
      id: crypto.randomUUID(),
      text: anonymizedText,
      role: Role.User,
      imageUrls: [],
      toolCalls: [],
      toolResults: [],
      createdAt: new Date().toISOString()
    }

    onMessageSent(optimisticMessage)

    try {
      await sendMessageMutation({
        variables: {
          chatId,
          text: anonymizedText,
          reasoning,
          voice
        }
      })
    } catch (error) {
      console.error('Error sending message', error)
      const errorMessage: Message = {
        id: `error-${Date.now()}`,
        text: error instanceof Error ? error.message : 'Error sending message',
        role: Role.Assistant,
        imageUrls: [],
        toolCalls: [],
        toolResults: [],
        createdAt: new Date().toISOString()
      }

      onError(errorMessage)
    }
  }

  return { sendMessage }
}
